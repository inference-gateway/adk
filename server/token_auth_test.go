package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	zaptest "go.uber.org/zap/zaptest"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

const testBearerToken = "s3cret-token"

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = l.Close() }()
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	return port
}

func buildTokenAuthServer(t *testing.T, cfg serverConfig.Config) server.A2AServer {
	t.Helper()
	card := createTestAgentCard()
	card.Capabilities.Streaming = new(false)
	card.SecuritySchemes, card.SecurityRequirements = server.BearerTokenSecuritySchemes()

	srv, err := server.NewA2AServerBuilder(cfg, zaptest.NewLogger(t)).
		WithAgentCard(card).
		WithDefaultBackgroundTaskHandler().
		Build()
	require.NoError(t, err)
	return srv
}

func postA2A(t *testing.T, baseURL, authorization string) *http.Response {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":"1","method":"tasks/get","params":{"id":"missing"}}`
	req, err := http.NewRequest(http.MethodPost, baseURL+"/a2a", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestServer_BindAddressAndStaticBearerToken(t *testing.T) {
	cfg := serverConfig.Config{}
	cfg.ServerConfig.Host = "127.0.0.1"
	cfg.ServerConfig.Port = freePort(t)
	cfg.AuthConfig.Token = testBearerToken
	srv := buildTokenAuthServer(t, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		_ = srv.Stop(stopCtx)
	})

	baseURL := fmt.Sprintf("http://127.0.0.1:%s", cfg.ServerConfig.Port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/health")
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond, "server did not come up on the loopback bind address")

	t.Run("public card declares the bearer scheme", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/.well-known/agent-card.json")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		var card types.AgentCard
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&card))
		scheme, ok := card.SecuritySchemes[server.BearerTokenSchemeName]
		require.True(t, ok)
		require.NotNil(t, scheme.HTTPAuthSecurityScheme)
		assert.Equal(t, "bearer", scheme.HTTPAuthSecurityScheme.Scheme)
	})

	t.Run("missing token is rejected", func(t *testing.T) {
		resp := postA2A(t, baseURL, "")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, "Bearer", resp.Header.Get("WWW-Authenticate"))
	})

	t.Run("wrong token is rejected", func(t *testing.T) {
		resp := postA2A(t, baseURL, "Bearer not-the-token")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("WWW-Authenticate"), "invalid_token")
	})

	t.Run("right token is served", func(t *testing.T) {
		resp := postA2A(t, baseURL, "Bearer "+testBearerToken)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestServer_TokenAndOIDCAreMutuallyExclusive(t *testing.T) {
	cfg := serverConfig.Config{}
	cfg.ServerConfig.Port = freePort(t)
	cfg.AuthConfig.Token = testBearerToken
	cfg.AuthConfig.Enabled = true
	srv := buildTokenAuthServer(t, cfg)

	err := srv.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}
