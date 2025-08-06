package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetAuthenticatedExtendedCard(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		serverResponse interface{}
		statusCode     int
		expectedError  bool
	}{
		{
			name: "success",
			serverResponse: types.JSONRPCSuccessResponse{
				JSONRPC: "2.0",
				ID:      "test-id",
				Result: json.RawMessage(`{
					"name": "test-agent",
					"version": "1.0.0",
					"description": "Test agent",
					"url": "http://localhost:8080",
					"protocolVersion": "1.0.0",
					"supportsAuthenticatedExtendedCard": true,
					"security": [{"oidc": []}],
					"securitySchemes": {
						"oidc": {
							"type": "openIdConnect",
							"openIdConnectUrl": "https://auth.example.com"
						}
					},
					"capabilities": {},
					"defaultInputModes": [],
					"defaultOutputModes": [],
					"skills": []
				}`),
			},
			statusCode:    http.StatusOK,
			expectedError: false,
		},
		{
			name: "authentication required error",
			serverResponse: types.JSONRPCErrorResponse{
				JSONRPC: "2.0",
				ID:      "test-id",
				Error: types.JSONRPCError{
					Code:    -32007,
					Message: "Authenticated extended card is not configured",
				},
			},
			statusCode:    http.StatusOK,
			expectedError: true,
		},
		{
			name:           "server error",
			serverResponse: `{"error": "server error"}`,
			statusCode:     http.StatusInternalServerError,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/a2a", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.serverResponse)
			}))
			defer server.Close()

			config := DefaultConfig(server.URL)
			config.Logger = logger
			client := NewClientWithConfig(config)

			card, err := client.GetAuthenticatedExtendedCard(context.Background())

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, card)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, card)
				assert.Equal(t, "test-agent", card.Name)
				assert.Equal(t, "1.0.0", card.Version)
				assert.NotNil(t, card.SupportsAuthenticatedExtendedCard)
				assert.True(t, *card.SupportsAuthenticatedExtendedCard)
			}
		})
	}
}

func TestClientAuthentication(t *testing.T) {
	logger := zap.NewNop()

	t.Run("Bearer token authentication", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer test-token", authHeader)

			response := types.JSONRPCSuccessResponse{
				JSONRPC: "2.0",
				ID:      "test-id",
				Result:  json.RawMessage(`{"name": "test-agent"}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := DefaultConfig(server.URL)
		config.Logger = logger
		client := NewClientWithConfig(config)

		client.SetAuthToken("test-token")

		_, err := client.GetAuthenticatedExtendedCard(context.Background())
		assert.NoError(t, err)
	})

	t.Run("API key authentication", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			assert.Equal(t, "test-api-key", apiKey)

			response := types.JSONRPCSuccessResponse{
				JSONRPC: "2.0",
				ID:      "test-id",
				Result:  json.RawMessage(`{"name": "test-agent"}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := DefaultConfig(server.URL)
		config.Logger = logger
		client := NewClientWithConfig(config)

		client.SetAuthToken("test-api-key", "X-API-Key")

		_, err := client.GetAuthenticatedExtendedCard(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Custom API key header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-Custom-Key")
			assert.Equal(t, "test-api-key", apiKey)

			response := types.JSONRPCSuccessResponse{
				JSONRPC: "2.0",
				ID:      "test-id",
				Result:  json.RawMessage(`{"name": "test-agent"}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := DefaultConfig(server.URL)
		config.Logger = logger
		client := NewClientWithConfig(config)

		client.SetAuthToken("test-api-key", "X-Custom-Key")

		_, err := client.GetAuthenticatedExtendedCard(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Multiple authentication methods", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			apiKey := r.Header.Get("X-API-Key")

			assert.Empty(t, authHeader)
			assert.Equal(t, "test-api-key", apiKey)

			response := types.JSONRPCSuccessResponse{
				JSONRPC: "2.0",
				ID:      "test-id",
				Result:  json.RawMessage(`{"name": "test-agent"}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := DefaultConfig(server.URL)
		config.Logger = logger
		client := NewClientWithConfig(config)

		client.SetAuthToken("test-token")
		client.SetAuthToken("test-api-key", "X-API-Key")

		_, err := client.GetAuthenticatedExtendedCard(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Clear authentication", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			apiKey := r.Header.Get("X-API-Key")

			assert.Empty(t, authHeader)
			assert.Empty(t, apiKey)

			response := types.JSONRPCSuccessResponse{
				JSONRPC: "2.0",
				ID:      "test-id",
				Result:  json.RawMessage(`{"name": "test-agent"}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := DefaultConfig(server.URL)
		config.Logger = logger
		client := NewClientWithConfig(config)

		// Set authentication then clear it
		client.SetAuthToken("test-token")
		client.ClearAuth()

		_, err := client.GetAuthenticatedExtendedCard(context.Background())
		assert.NoError(t, err)
	})
}

func TestAuthConfig(t *testing.T) {
	t.Run("AuthConfig initialization", func(t *testing.T) {
		authConfig := &AuthConfig{
			Token:  "test-token",
			Header: "X-Custom-API-Key",
		}

		assert.Equal(t, "test-token", authConfig.Token)
		assert.Equal(t, "X-Custom-API-Key", authConfig.Header)
	})

	t.Run("Config with authentication", func(t *testing.T) {
		config := &Config{
			BaseURL: "http://localhost:8080",
			Auth: &AuthConfig{
				Token:  "test-token",
				Header: "X-Custom-Key",
			},
		}

		assert.NotNil(t, config.Auth)
		assert.Equal(t, "test-token", config.Auth.Token)
		assert.Equal(t, "X-Custom-Key", config.Auth.Header)
	})
}
