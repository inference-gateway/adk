package middlewares

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestSecurityValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()

	t.Run("NewSecurityValidator with auth disabled", func(t *testing.T) {
		cfg := config.Config{
			AuthConfig: config.AuthConfig{
				Enable: false,
			},
		}

		validator := NewSecurityValidator(logger, cfg)
		_, ok := validator.(*SecurityValidatorNoop)
		assert.True(t, ok)
	})

	t.Run("NewSecurityValidator with auth enabled", func(t *testing.T) {
		cfg := config.Config{
			AuthConfig: config.AuthConfig{
				Enable: true,
			},
		}

		validator := NewSecurityValidator(logger, cfg)
		_, ok := validator.(*SecurityValidatorImpl)
		assert.True(t, ok)
	})
}

func TestSecurityValidatorImpl_ValidateSecurityRequirements(t *testing.T) {
	logger := zap.NewNop()
	cfg := config.Config{
		AuthConfig: config.AuthConfig{
			Enable: true,
		},
	}
	validator := &SecurityValidatorImpl{
		logger: logger,
		config: cfg,
	}

	tests := []struct {
		name           string
		agentCard      *types.AgentCard
		setupRequest   func(*gin.Context)
		expectedStatus int
	}{
		{
			name:           "no agent card - allow through",
			agentCard:      nil,
			setupRequest:   func(c *gin.Context) {},
			expectedStatus: http.StatusOK,
		},
		{
			name: "no security requirements - allow through",
			agentCard: &types.AgentCard{
				Name:     "test-agent",
				Security: nil,
			},
			setupRequest:   func(c *gin.Context) {},
			expectedStatus: http.StatusOK,
		},
		{
			name: "OIDC security satisfied",
			agentCard: &types.AgentCard{
				Name: "test-agent",
				Security: []map[string][]string{
					{"oidc": {}},
				},
				SecuritySchemes: map[string]types.SecurityScheme{
					"oidc": types.OpenIdConnectSecurityScheme{
						Type:             "openIdConnect",
						OpenIDConnectURL: "https://auth.example.com",
					},
				},
			},
			setupRequest: func(c *gin.Context) {
				// Simulate OIDC middleware setting the token
				c.Set(string(IDTokenContextKey), "valid-token")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "OIDC security not satisfied",
			agentCard: &types.AgentCard{
				Name: "test-agent",
				Security: []map[string][]string{
					{"oidc": {}},
				},
				SecuritySchemes: map[string]types.SecurityScheme{
					"oidc": types.OpenIdConnectSecurityScheme{
						Type:             "openIdConnect",
						OpenIDConnectURL: "https://auth.example.com",
					},
				},
			},
			setupRequest:   func(c *gin.Context) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Bearer token security satisfied",
			agentCard: &types.AgentCard{
				Name: "test-agent",
				Security: []map[string][]string{
					{"bearer": {}},
				},
				SecuritySchemes: map[string]types.SecurityScheme{
					"bearer": types.HTTPAuthSecurityScheme{
						Type:   "http",
						Scheme: "bearer",
					},
				},
			},
			setupRequest: func(c *gin.Context) {
				c.Request.Header.Set("Authorization", "Bearer test-token")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Bearer token security not satisfied",
			agentCard: &types.AgentCard{
				Name: "test-agent",
				Security: []map[string][]string{
					{"bearer": {}},
				},
				SecuritySchemes: map[string]types.SecurityScheme{
					"bearer": types.HTTPAuthSecurityScheme{
						Type:   "http",
						Scheme: "bearer",
					},
				},
			},
			setupRequest:   func(c *gin.Context) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "API key security satisfied",
			agentCard: &types.AgentCard{
				Name: "test-agent",
				Security: []map[string][]string{
					{"api_key": {}},
				},
				SecuritySchemes: map[string]types.SecurityScheme{
					"api_key": types.APIKeySecurityScheme{
						Type: "apiKey",
						Name: "X-API-Key",
						In:   "header",
					},
				},
			},
			setupRequest: func(c *gin.Context) {
				c.Request.Header.Set("X-API-Key", "test-key")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "API key security not satisfied",
			agentCard: &types.AgentCard{
				Name: "test-agent",
				Security: []map[string][]string{
					{"api_key": {}},
				},
				SecuritySchemes: map[string]types.SecurityScheme{
					"api_key": types.APIKeySecurityScheme{
						Type: "apiKey",
						Name: "X-API-Key",
						In:   "header",
					},
				},
			},
			setupRequest:   func(c *gin.Context) {},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			nextCalled := false
			middleware := validator.ValidateSecurityRequirements(tt.agentCard)

			// Add setup middleware before the security middleware
			setupMiddleware := func(c *gin.Context) {
				tt.setupRequest(c)
				c.Next()
			}

			router.POST("/a2a", setupMiddleware, middleware, func(c *gin.Context) {
				nextCalled = true
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/a2a", nil)

			router.ServeHTTP(w, req)

			if tt.expectedStatus == http.StatusOK {
				assert.True(t, nextCalled)
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.False(t, nextCalled)
				assert.Equal(t, tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestValidateSecurityScheme(t *testing.T) {
	logger := zap.NewNop()
	cfg := config.Config{}
	validator := &SecurityValidatorImpl{
		logger: logger,
		config: cfg,
	}

	tests := []struct {
		name         string
		scheme       types.SecurityScheme
		setupRequest func(*gin.Context)
		expected     bool
	}{
		{
			name: "OIDC with valid token",
			scheme: types.OpenIdConnectSecurityScheme{
				Type:             "openIdConnect",
				OpenIDConnectURL: "https://auth.example.com",
			},
			setupRequest: func(c *gin.Context) {
				c.Set(string(IDTokenContextKey), "valid-token")
			},
			expected: true,
		},
		{
			name: "OIDC without token",
			scheme: types.OpenIdConnectSecurityScheme{
				Type:             "openIdConnect",
				OpenIDConnectURL: "https://auth.example.com",
			},
			setupRequest: func(c *gin.Context) {},
			expected:     false,
		},
		{
			name: "Bearer token valid",
			scheme: types.HTTPAuthSecurityScheme{
				Type:   "http",
				Scheme: "bearer",
			},
			setupRequest: func(c *gin.Context) {
				c.Request.Header.Set("Authorization", "Bearer test-token")
			},
			expected: true,
		},
		{
			name: "Basic auth valid",
			scheme: types.HTTPAuthSecurityScheme{
				Type:   "http",
				Scheme: "basic",
			},
			setupRequest: func(c *gin.Context) {
				c.Request.Header.Set("Authorization", "Basic dGVzdDp0ZXN0")
			},
			expected: true,
		},
		{
			name: "API key in header",
			scheme: types.APIKeySecurityScheme{
				Type: "apiKey",
				Name: "X-API-Key",
				In:   "header",
			},
			setupRequest: func(c *gin.Context) {
				c.Request.Header.Set("X-API-Key", "test-key")
			},
			expected: true,
		},
		{
			name: "API key in query",
			scheme: types.APIKeySecurityScheme{
				Type: "apiKey",
				Name: "api_key",
				In:   "query",
			},
			setupRequest: func(c *gin.Context) {
				c.Request.URL.RawQuery = "api_key=test-key"
			},
			expected: true,
		},
		{
			name: "Mutual TLS with certificate",
			scheme: types.MutualTLSSecurityScheme{
				Type: "mutualTLS",
			},
			setupRequest: func(c *gin.Context) {
				c.Request.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{{}}, // Mock certificate
				}
			},
			expected: true,
		},
		{
			name: "Mutual TLS without certificate",
			scheme: types.MutualTLSSecurityScheme{
				Type: "mutualTLS",
			},
			setupRequest: func(c *gin.Context) {},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/a2a", nil)

			tt.setupRequest(c)

			result := validator.validateSecurityScheme(c, "test-scheme", tt.scheme)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecurityValidatorNoop(t *testing.T) {
	validator := &SecurityValidatorNoop{}
	agentCard := &types.AgentCard{
		Name: "test-agent",
		Security: []map[string][]string{
			{"oidc": {}},
		},
	}

	router := gin.New()

	nextCalled := false
	middleware := validator.ValidateSecurityRequirements(agentCard)

	router.POST("/a2a", middleware, func(c *gin.Context) {
		nextCalled = true
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/a2a", nil)

	router.ServeHTTP(w, req)

	assert.True(t, nextCalled, "Noop validator should always call next")
	assert.Equal(t, http.StatusOK, w.Code)
}
