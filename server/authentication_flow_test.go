package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	client "github.com/inference-gateway/adk/client"
	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zaptest "go.uber.org/zap/zaptest"
)

// startAuthFlowServer builds and starts a real A2A server on the given port and
// returns a stop function. Authentication is left disabled so the test does not
// require a live OIDC provider; the discovery half of the flow (securitySchemes
// on the card) and the extended-card contract are exercised end to end.
func startAuthFlowServer(t *testing.T, port string, extended *types.AgentCard) func() {
	t.Helper()

	card := createTestAgentCard()
	card.Capabilities.Streaming = new(false)
	schemes, security := server.OIDCSecuritySchemes(config.AuthConfig{
		IssuerURL: "https://issuer.example.com/realms/test",
	})
	card.SecuritySchemes = schemes
	card.Security = security

	cfg := config.Config{}
	cfg.ServerConfig.Port = port

	builder := server.NewA2AServerBuilder(cfg, zaptest.NewLogger(t)).
		WithAgentCard(card).
		WithDefaultBackgroundTaskHandler()
	if extended != nil {
		builder = builder.WithExtendedAgentCard(*extended)
	}

	srv, err := builder.Build()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Start(ctx) }()

	require.Eventually(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/health", port))
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond, "server did not become healthy")

	return func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		_ = srv.Stop(stopCtx)
	}
}

// TestAuthenticationFlow_DiscoveryToExtendedCard walks the full documented flow with
// the real client: fetch the public card, read securitySchemes, set the auth header,
// then call GetAuthenticatedExtendedCard and receive the extended card.
func TestAuthenticationFlow_DiscoveryToExtendedCard(t *testing.T) {
	extended := createTestAgentCard()
	extended.Name = "extended-agent"
	extended.Description = "richer card for authenticated callers"

	stop := startAuthFlowServer(t, "18085", &extended)
	defer stop()

	baseURL := "http://localhost:18085"
	ctx := context.Background()

	// Discovery: public card served unauthenticated, declaring how to authenticate.
	discoveryClient := client.NewClient(baseURL)
	publicCard, err := discoveryClient.GetAgentCard(ctx)
	require.NoError(t, err)
	require.NotNil(t, publicCard)
	assert.Equal(t, "test-agent", publicCard.Name)
	require.Contains(t, publicCard.SecuritySchemes, server.OIDCSchemeName,
		"public card must declare how to authenticate")
	require.NotNil(t, publicCard.SupportsExtendedAgentCard)
	assert.True(t, *publicCard.SupportsExtendedAgentCard)

	// Out-of-band credential transmitted on every request via a header.
	authClient := client.NewClientWithConfig(&client.Config{
		BaseURL: baseURL,
		Headers: map[string]string{"Authorization": "Bearer out-of-band-token"},
	})

	resp, err := authClient.GetAuthenticatedExtendedCard(ctx, types.GetAuthenticatedExtendedCardParams{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	resultBytes, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	var gotCard types.AgentCard
	require.NoError(t, json.Unmarshal(resultBytes, &gotCard))
	assert.Equal(t, "extended-agent", gotCard.Name)
}

// TestAuthenticationFlow_ExtendedCardUnsupported verifies the error contract: when no
// extended card is configured the card does not advertise support, so the RPC returns
// UnsupportedOperationError (-32004).
func TestAuthenticationFlow_ExtendedCardUnsupported(t *testing.T) {
	stop := startAuthFlowServer(t, "18086", nil)
	defer stop()

	authClient := client.NewClient("http://localhost:18086")
	_, err := authClient.GetAuthenticatedExtendedCard(context.Background(), types.GetAuthenticatedExtendedCardParams{})
	require.Error(t, err, "unsupported extended card must surface as a JSON-RPC error")
	assert.Contains(t, err.Error(), "-32004")
}
