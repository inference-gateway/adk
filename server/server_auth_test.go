package server_test

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

func TestA2AServer_Start_FailsWhenOIDCAuthenticatorCannotBeCreated(t *testing.T) {
	cfg := serverConfig.Config{
		AgentName:    "test-agent",
		ServerConfig: serverConfig.ServerConfig{Port: "0"},
		AuthConfig: serverConfig.AuthConfig{
			Enabled:   true,
			IssuerURL: "http://127.0.0.1:1",
			ClientID:  "test-client",
		},
	}

	srv, err := server.NewA2AServerBuilder(cfg, zap.NewNop()).
		WithAgentCard(types.AgentCard{Name: "test-agent"}).
		WithDefaultTaskHandlers().
		Build()
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create OIDC authenticator")
}
