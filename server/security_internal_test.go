package server

import (
	"testing"

	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestOIDCSecuritySchemes(t *testing.T) {
	schemes, security := OIDCSecuritySchemes(config.AuthConfig{
		IssuerURL: "https://issuer.example.com/realms/test/",
	})

	scheme, ok := schemes[OIDCSchemeName]
	require.True(t, ok, "openId scheme must be present")
	require.NotNil(t, scheme.OpenIDConnectSecurityScheme)
	assert.Equal(t,
		"https://issuer.example.com/realms/test/.well-known/openid-configuration",
		scheme.OpenIDConnectSecurityScheme.OpenIDConnectURL,
		"trailing slash on issuer must not double up")

	require.Len(t, security, 1)
	_, ok = security[0].Schemes[OIDCSchemeName]
	assert.True(t, ok, "security requirement must reference the declared scheme")
}

func TestValidateAuthConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		authEnabled   bool
		declareScheme bool
		wantWarnMatch string
	}{
		{
			name:          "auth enabled without schemes warns",
			authEnabled:   true,
			declareScheme: false,
			wantWarnMatch: "declares no securitySchemes",
		},
		{
			name:          "schemes without auth warns",
			authEnabled:   false,
			declareScheme: true,
			wantWarnMatch: "authentication is disabled",
		},
		{
			name:          "auth enabled with schemes is quiet",
			authEnabled:   true,
			declareScheme: true,
		},
		{
			name:          "no auth no schemes is quiet",
			authEnabled:   false,
			declareScheme: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			card := &types.AgentCard{Name: "agent"}
			if tt.declareScheme {
				schemes, _ := OIDCSecuritySchemes(config.AuthConfig{IssuerURL: "https://x"})
				card.SecuritySchemes = schemes
			}
			s := &A2AServerImpl{
				logger:          zap.New(core),
				customAgentCard: card,
			}
			s.cfg = &config.Config{}
			s.cfg.AuthConfig.Enable = tt.authEnabled

			s.validateAuthConfiguration()

			if tt.wantWarnMatch == "" {
				assert.Equal(t, 0, logs.Len(), "expected no warning")
				return
			}
			require.Equal(t, 1, logs.Len(), "expected exactly one warning")
			assert.Contains(t, logs.All()[0].Message, tt.wantWarnMatch)
		})
	}
}
