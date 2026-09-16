package middlewares

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	gin "github.com/gin-gonic/gin"
	zap "go.uber.org/zap"

	serverConfig "github.com/inference-gateway/adk/server/config"
)

type contextKey string

const (
	ClaimsContextKey contextKey = "claims"

	oidcHTTPTimeout = 10 * time.Second
	bearerScheme    = "Bearer"
	clientIDClaim   = "client_id"

	wwwAuthenticateHeader  = "WWW-Authenticate"
	wwwAuthenticateMissing = "Bearer"
	wwwAuthenticateFormat  = `Bearer error="invalid_request"`
	wwwAuthenticateInvalid = `Bearer error="invalid_token"`
)

// OIDCAuthenticator interface for authentication middleware
type OIDCAuthenticator interface {
	Middleware() gin.HandlerFunc
}

// OIDCAuthenticatorImpl verifies bearer tokens against the issuer's published keys and accepted audiences
type OIDCAuthenticatorImpl struct {
	logger    *zap.Logger
	verifier  *oidc.IDTokenVerifier
	audiences []string
}

// OIDCAuthenticatorNoop is a no-op authenticator for when auth is disabled
type OIDCAuthenticatorNoop struct{}

// NewOIDCAuthenticatorMiddleware creates a new OIDC authenticator middleware.
// The server only verifies tokens, so it needs the issuer and the accepted audiences (AUTH_AUDIENCE, defaulting to AUTH_CLIENT_ID).
func NewOIDCAuthenticatorMiddleware(logger *zap.Logger, cfg serverConfig.Config) (OIDCAuthenticator, error) {
	if !cfg.AuthConfig.Enabled {
		return &OIDCAuthenticatorNoop{}, nil
	}

	if cfg.AuthConfig.IssuerURL == "" {
		return nil, errors.New("AUTH_ISSUER_URL is required when AUTH_ENABLED=true")
	}

	audiences := strings.FieldsFunc(cmp.Or(cfg.AuthConfig.Audience, cfg.AuthConfig.ClientID), isListSeparator)
	if len(audiences) == 0 {
		return nil, errors.New("AUTH_AUDIENCE or AUTH_CLIENT_ID is required when AUTH_ENABLED=true")
	}

	ctx := oidc.ClientContext(context.Background(), &http.Client{Timeout: oidcHTTPTimeout})
	provider, err := oidc.NewProvider(ctx, cfg.AuthConfig.IssuerURL)
	if err != nil {
		return nil, err
	}

	return &OIDCAuthenticatorImpl{
		logger:    logger,
		verifier:  provider.Verifier(&oidc.Config{SkipClientIDCheck: true}),
		audiences: audiences,
	}, nil
}

func isListSeparator(r rune) bool { return r == ',' || unicode.IsSpace(r) }

// Middleware returns the OIDC authentication middleware for OIDCAuthenticatorImpl
func (auth *OIDCAuthenticatorImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			auth.logger.Error("missing authorization header")
			unauthorized(c, wwwAuthenticateMissing, "missing authorization header")
			return
		}

		scheme, token, _ := strings.Cut(authHeader, " ")
		token = strings.TrimSpace(token)
		if !strings.EqualFold(scheme, bearerScheme) || token == "" {
			auth.logger.Error("invalid authorization header format")
			unauthorized(c, wwwAuthenticateFormat, "invalid authorization header format")
			return
		}

		idToken, err := auth.verifier.Verify(c.Request.Context(), token)
		if err != nil {
			auth.logger.Error("failed to verify bearer token", zap.Error(err))
			unauthorized(c, wwwAuthenticateInvalid, "invalid token")
			return
		}

		claims := make(map[string]any)
		if err := idToken.Claims(&claims); err != nil {
			auth.logger.Error("failed to extract bearer token claims", zap.Error(err))
			unauthorized(c, wwwAuthenticateInvalid, "invalid token")
			return
		}

		audiences := idToken.Audience
		if len(audiences) == 0 {
			if clientID, _ := claims[clientIDClaim].(string); clientID != "" {
				audiences = []string{clientID}
			}
		}
		if !slices.ContainsFunc(audiences, func(aud string) bool { return slices.Contains(auth.audiences, aud) }) {
			auth.logger.Error("failed to verify bearer token",
				zap.Error(fmt.Errorf("oidc: expected one of audiences %q got %q", auth.audiences, audiences)))
			unauthorized(c, wwwAuthenticateInvalid, "invalid token")
			return
		}

		reqCtx := context.WithValue(c.Request.Context(), ClaimsContextKey, claims)
		c.Request = c.Request.WithContext(reqCtx)

		c.Next()
	}
}

// Middleware returns a no-op middleware for OIDCAuthenticatorNoop
func (auth *OIDCAuthenticatorNoop) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func unauthorized(c *gin.Context, challenge string, message string) {
	c.Header(wwwAuthenticateHeader, challenge)
	c.JSON(http.StatusUnauthorized, gin.H{"error": message})
	c.Abort()
}
