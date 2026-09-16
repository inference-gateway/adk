package middlewares_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	gin "github.com/gin-gonic/gin"
	jose "github.com/go-jose/go-jose/v4"
	jwt "github.com/go-jose/go-jose/v4/jwt"
	zap "go.uber.org/zap"

	serverConfig "github.com/inference-gateway/adk/server/config"
	middlewares "github.com/inference-gateway/adk/server/middlewares"
)

const (
	testClientID    = "inference-gateway-client"
	testAPIAudience = "https://api.example.com"
	testSubject     = "user-1"
	testKeyID       = "test-key"
	testTokenTTL    = time.Hour
	testRSABits     = 2048

	challengeMissing = "Bearer"
	challengeFormat  = `Bearer error="invalid_request"`
	challengeInvalid = `Bearer error="invalid_token"`
)

// fakeIdP serves the two OIDC endpoints go-oidc needs (discovery and JWKS) and mints RS256 tokens signed by the key it publishes
type fakeIdP struct {
	issuer string
	signer jose.Signer
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, testRSABits)
	require.NoError(t, err)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                srv.URL,
			"jwks_uri":                              srv.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{string(jose.RS256)},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, KeyID: testKeyID, Algorithm: string(jose.RS256), Use: "sig"},
		}})
	})

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: &jose.JSONWebKey{Key: key, KeyID: testKeyID}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)
	return &fakeIdP{issuer: srv.URL, signer: signer}
}

// mint signs a valid token for this issuer; override tampers with the registered claims and private adds non-standard ones
func (p *fakeIdP) mint(t *testing.T, override func(*jwt.Claims), private ...map[string]any) string {
	t.Helper()
	now := time.Now()
	claims := jwt.Claims{
		Issuer:   p.issuer,
		Subject:  testSubject,
		Audience: jwt.Audience{testClientID},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(testTokenTTL)),
	}
	if override != nil {
		override(&claims)
	}
	builder := jwt.Signed(p.signer).Claims(claims)
	for _, extra := range private {
		builder = builder.Claims(extra)
	}
	raw, err := builder.Serialize()
	require.NoError(t, err)
	return raw
}

// newAuthEngine wires the real middleware in front of a handler that echoes the claims the middleware stored on the context
func newAuthEngine(t *testing.T, auth serverConfig.AuthConfig) *gin.Engine {
	t.Helper()
	mw, err := middlewares.NewOIDCAuthenticatorMiddleware(zap.NewNop(), serverConfig.Config{AuthConfig: auth})
	require.NoError(t, err)

	r := gin.New()
	r.Use(mw.Middleware())
	r.POST("/a2a", func(c *gin.Context) {
		claims, _ := c.Request.Context().Value(middlewares.ClaimsContextKey).(map[string]any)
		c.JSON(http.StatusOK, gin.H{"sub": claims["sub"]})
	})
	return r
}

func TestNewOIDCAuthenticatorMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	idp := newFakeIdP(t)

	tests := []struct {
		name    string
		auth    serverConfig.AuthConfig
		wantErr bool
	}{
		{name: "disabled returns noop", auth: serverConfig.AuthConfig{Enabled: false}},
		{name: "enabled without issuer fails", auth: serverConfig.AuthConfig{Enabled: true, ClientID: testClientID}, wantErr: true},
		{name: "enabled without audience or client id fails", auth: serverConfig.AuthConfig{Enabled: true, IssuerURL: idp.issuer}, wantErr: true},
		{name: "enabled with unreachable issuer fails", auth: serverConfig.AuthConfig{Enabled: true, IssuerURL: "http://127.0.0.1:1", ClientID: testClientID}, wantErr: true},
		{name: "enabled with issuer and client id", auth: serverConfig.AuthConfig{Enabled: true, IssuerURL: idp.issuer, ClientID: testClientID}},
		{name: "enabled with issuer and audience only", auth: serverConfig.AuthConfig{Enabled: true, IssuerURL: idp.issuer, Audience: testAPIAudience}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw, err := middlewares.NewOIDCAuthenticatorMiddleware(zap.NewNop(), serverConfig.Config{AuthConfig: tt.auth})
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, mw)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, mw)
		})
	}
}

func TestOIDCAuthenticatorMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	idp := newFakeIdP(t)
	other := newFakeIdP(t)

	withClientID := newAuthEngine(t, serverConfig.AuthConfig{Enabled: true, IssuerURL: idp.issuer, ClientID: testClientID})
	withAudienceList := newAuthEngine(t, serverConfig.AuthConfig{Enabled: true, IssuerURL: idp.issuer, ClientID: testClientID, Audience: testAPIAudience + ", second"})

	valid := idp.mint(t, nil)
	forAPI := idp.mint(t, func(c *jwt.Claims) { c.Audience = jwt.Audience{testAPIAudience} })

	tests := []struct {
		name          string
		engine        *gin.Engine
		header        string
		wantStatus    int
		wantChallenge string
	}{
		{name: "missing header", engine: withClientID, wantStatus: http.StatusUnauthorized, wantChallenge: challengeMissing},
		{name: "wrong scheme", engine: withClientID, header: "Basic abc", wantStatus: http.StatusUnauthorized, wantChallenge: challengeFormat},
		{name: "empty bearer", engine: withClientID, header: "Bearer ", wantStatus: http.StatusUnauthorized, wantChallenge: challengeFormat},
		{name: "valid token", engine: withClientID, header: "Bearer " + valid, wantStatus: http.StatusOK},
		{name: "scheme is case insensitive", engine: withClientID, header: "bearer " + valid, wantStatus: http.StatusOK},
		{name: "garbage token", engine: withClientID, header: "Bearer not-a-jwt", wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid},
		{name: "expired token", engine: withClientID, header: "Bearer " + idp.mint(t, func(c *jwt.Claims) { c.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }), wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid},
		{name: "wrong issuer", engine: withClientID, header: "Bearer " + other.mint(t, nil), wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid},
		{name: "wrong audience", engine: withClientID, header: "Bearer " + forAPI, wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid},
		{name: "client id rejected when audience list is set", engine: withAudienceList, header: "Bearer " + valid, wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid},
		{name: "api audience accepted from list", engine: withAudienceList, header: "Bearer " + forAPI, wantStatus: http.StatusOK},
		{name: "second audience accepted from list", engine: withAudienceList, header: "Bearer " + idp.mint(t, func(c *jwt.Claims) { c.Audience = jwt.Audience{"second"} }), wantStatus: http.StatusOK},
		{name: "no aud falls back to client_id claim", engine: withClientID, header: "Bearer " + idp.mint(t, func(c *jwt.Claims) { c.Audience = nil }, map[string]any{"client_id": testClientID}), wantStatus: http.StatusOK},
		{name: "no aud and unknown client_id rejected", engine: withClientID, header: "Bearer " + idp.mint(t, func(c *jwt.Claims) { c.Audience = nil }, map[string]any{"client_id": "someone-else"}), wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid},
		{name: "no aud and no client_id rejected", engine: withClientID, header: "Bearer " + idp.mint(t, func(c *jwt.Claims) { c.Audience = nil }), wantStatus: http.StatusUnauthorized, wantChallenge: challengeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/a2a", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			tt.engine.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantChallenge, w.Header().Get("WWW-Authenticate"))
			if tt.wantStatus == http.StatusOK {
				assert.Contains(t, w.Body.String(), testSubject)
			}
		})
	}
}
