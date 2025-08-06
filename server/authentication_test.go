package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestHandleGetAuthenticatedExtendedCard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()

	tests := []struct {
		name           string
		authEnabled    bool
		agentCard      *types.AgentCard
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "authentication disabled",
			authEnabled:    false,
			agentCard:      nil,
			expectedStatus: 400,
			expectedError:  "authentication is disabled",
		},
		{
			name:           "no agent card configured",
			authEnabled:    true,
			agentCard:      nil,
			expectedStatus: 400,
			expectedError:  "no agent card available",
		},
		{
			name:        "authenticated extended card not supported",
			authEnabled: true,
			agentCard: &types.AgentCard{
				Name:                              "test-agent",
				Version:                           "1.0.0",
				SupportsAuthenticatedExtendedCard: BoolPtr(false),
			},
			expectedStatus: 400,
			expectedError:  "not supported by this agent",
		},
		{
			name:        "success",
			authEnabled: true,
			agentCard: &types.AgentCard{
				Name:                              "test-agent",
				Version:                           "1.0.0",
				SupportsAuthenticatedExtendedCard: BoolPtr(true),
			},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				AuthConfig: config.AuthConfig{
					Enable: tt.authEnabled,
				},
			}

			server := &A2AServerImpl{
				cfg:             cfg,
				logger:          logger,
				customAgentCard: tt.agentCard,
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Create JSON-RPC request
			id := interface{}("test-id")
			req := types.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "agent/getAuthenticatedExtendedCard",
				ID:      &id,
			}

			server.responseSender = NewDefaultResponseSender(logger)
			server.handleGetAuthenticatedExtendedCard(c, req)

			if tt.expectedStatus == 200 {
				assert.Equal(t, http.StatusOK, w.Code)

				var response types.JSONRPCSuccessResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "2.0", response.JSONRPC)
				assert.Equal(t, "test-id", response.ID)
			} else {
				// For error cases, the response sender will set appropriate HTTP status
				// and the error message should be in the response body
				responseBody := w.Body.String()
				assert.Contains(t, responseBody, tt.expectedError)
			}
		})
	}
}

func TestConfigureAgentCardSecurity(t *testing.T) {
	tests := []struct {
		name             string
		securityConfig   AgentCardSecurityConfig
		expectedSchemes  int
		expectedSecurity bool
	}{
		{
			name: "OIDC security only",
			securityConfig: AgentCardSecurityConfig{
				EnableOIDC:                        true,
				OIDCIssuerURL:                     "https://auth.example.com",
				SupportsAuthenticatedExtendedCard: true,
			},
			expectedSchemes:  1,
			expectedSecurity: true,
		},
		{
			name: "API key security only",
			securityConfig: AgentCardSecurityConfig{
				EnableAPIKey:   true,
				APIKeyName:     "X-API-Key",
				APIKeyLocation: "header",
			},
			expectedSchemes:  1,
			expectedSecurity: true,
		},
		{
			name: "multiple security schemes",
			securityConfig: AgentCardSecurityConfig{
				EnableOIDC:      true,
				OIDCIssuerURL:   "https://auth.example.com",
				EnableAPIKey:    true,
				APIKeyName:      "X-API-Key",
				EnableMutualTLS: true,
			},
			expectedSchemes:  3,
			expectedSecurity: true,
		},
		{
			name:             "no security",
			securityConfig:   AgentCardSecurityConfig{},
			expectedSchemes:  0,
			expectedSecurity: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &types.AgentCard{
				Name:    "test-agent",
				Version: "1.0.0",
			}

			ConfigureAgentCardSecurity(card, tt.securityConfig)

			assert.Equal(t, tt.expectedSchemes, len(card.SecuritySchemes))

			if tt.expectedSecurity {
				assert.NotNil(t, card.Security)
				assert.Greater(t, len(card.Security), 0)
			} else {
				assert.Nil(t, card.Security)
			}

			// Check authenticated extended card support
			if tt.securityConfig.SupportsAuthenticatedExtendedCard {
				assert.NotNil(t, card.SupportsAuthenticatedExtendedCard)
				assert.True(t, *card.SupportsAuthenticatedExtendedCard)
			}
		})
	}
}

func TestCreateSecurityConfigFromAuthConfig(t *testing.T) {
	tests := []struct {
		name       string
		authConfig config.AuthConfig
		expected   AgentCardSecurityConfig
	}{
		{
			name: "OIDC enabled",
			authConfig: config.AuthConfig{
				Enable:                            true,
				IssuerURL:                         "https://auth.example.com",
				SupportsAuthenticatedExtendedCard: true,
			},
			expected: AgentCardSecurityConfig{
				EnableOIDC:                        true,
				OIDCIssuerURL:                     "https://auth.example.com",
				SupportsAuthenticatedExtendedCard: true,
				EnableAPIKey:                      false,
				APIKeyName:                        "",
				APIKeyLocation:                    "header",
				EnableMutualTLS:                   false,
			},
		},
		{
			name: "API key enabled",
			authConfig: config.AuthConfig{
				Enable:       true,
				EnableAPIKey: true,
				APIKeyHeader: "X-Custom-API-Key",
			},
			expected: AgentCardSecurityConfig{
				EnableOIDC:                        false, // No issuer URL
				OIDCIssuerURL:                     "",
				SupportsAuthenticatedExtendedCard: false,
				EnableAPIKey:                      true,
				APIKeyName:                        "X-Custom-API-Key",
				APIKeyLocation:                    "header",
				EnableMutualTLS:                   false,
			},
		},
		{
			name: "all features enabled",
			authConfig: config.AuthConfig{
				Enable:                            true,
				IssuerURL:                         "https://auth.example.com",
				SupportsAuthenticatedExtendedCard: true,
				EnableAPIKey:                      true,
				APIKeyHeader:                      "X-API-Key",
				EnableMutualTLS:                   true,
			},
			expected: AgentCardSecurityConfig{
				EnableOIDC:                        true,
				OIDCIssuerURL:                     "https://auth.example.com",
				SupportsAuthenticatedExtendedCard: true,
				EnableAPIKey:                      true,
				APIKeyName:                        "X-API-Key",
				APIKeyLocation:                    "header",
				EnableMutualTLS:                   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateSecurityConfigFromAuthConfig(tt.authConfig)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecurityHelperFunctions(t *testing.T) {
	t.Run("CreateOIDCSecurityScheme", func(t *testing.T) {
		scheme := CreateOIDCSecurityScheme("https://auth.example.com", "Test OIDC")
		oidcScheme, ok := scheme.(types.OpenIdConnectSecurityScheme)
		assert.True(t, ok)
		assert.Equal(t, "openIdConnect", oidcScheme.Type)
		assert.Equal(t, "https://auth.example.com", oidcScheme.OpenIDConnectURL)
		assert.Equal(t, "Test OIDC", *oidcScheme.Description)
	})

	t.Run("CreateAPIKeySecurityScheme", func(t *testing.T) {
		scheme := CreateAPIKeySecurityScheme("X-API-Key", "header", "Test API Key")
		apiKeyScheme, ok := scheme.(types.APIKeySecurityScheme)
		assert.True(t, ok)
		assert.Equal(t, "apiKey", apiKeyScheme.Type)
		assert.Equal(t, "X-API-Key", apiKeyScheme.Name)
		assert.Equal(t, "header", apiKeyScheme.In)
		assert.Equal(t, "Test API Key", *apiKeyScheme.Description)
	})

	t.Run("CreateHTTPAuthSecurityScheme", func(t *testing.T) {
		scheme := CreateHTTPAuthSecurityScheme("bearer", StringPtr("JWT"), "Test Bearer")
		httpScheme, ok := scheme.(types.HTTPAuthSecurityScheme)
		assert.True(t, ok)
		assert.Equal(t, "http", httpScheme.Type)
		assert.Equal(t, "bearer", httpScheme.Scheme)
		assert.Equal(t, "JWT", *httpScheme.BearerFormat)
		assert.Equal(t, "Test Bearer", *httpScheme.Description)
	})

	t.Run("CreateMutualTLSSecurityScheme", func(t *testing.T) {
		scheme := CreateMutualTLSSecurityScheme("Test mTLS")
		mtlsScheme, ok := scheme.(types.MutualTLSSecurityScheme)
		assert.True(t, ok)
		assert.Equal(t, "mutualTLS", mtlsScheme.Type)
		assert.Equal(t, "Test mTLS", *mtlsScheme.Description)
	})
}
