package config_test

import (
	"context"
	"testing"
	"time"

	config "github.com/inference-gateway/adk/server/config"
	envconfig "github.com/sethvargo/go-envconfig"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestConfig_LoadWithLookuper(t *testing.T) {
	tests := []struct {
		name         string
		envVars      map[string]string
		validateFunc func(t *testing.T, cfg *config.Config)
	}{
		{
			name:    "loads defaults when no env vars set",
			envVars: map[string]string{},
			validateFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "", cfg.AgentName)
				assert.Equal(t, "", cfg.AgentDescription)
				assert.Equal(t, "", cfg.AgentURL)
				assert.Equal(t, "", cfg.AgentVersion)
				assert.False(t, cfg.Debug)
				assert.Equal(t, "8080", cfg.ServerConfig.Port)
				assert.Equal(t, 1*time.Second, cfg.StreamingStatusUpdateInterval)

				require.NotNil(t, cfg.AgentConfig)
				assert.Equal(t, "", cfg.AgentConfig.Provider)
				assert.Equal(t, "", cfg.AgentConfig.Model)
				assert.Equal(t, 30*time.Second, cfg.AgentConfig.Timeout)
				assert.Equal(t, 3, cfg.AgentConfig.MaxRetries)
				assert.Equal(t, 10, cfg.AgentConfig.MaxChatCompletionIterations)
				assert.Equal(t, "a2a-agent/1.0", cfg.AgentConfig.UserAgent)
				assert.Equal(t, 4096, cfg.AgentConfig.MaxTokens)
				assert.Equal(t, 0.7, cfg.AgentConfig.Temperature)
				assert.Equal(t, 1.0, cfg.AgentConfig.TopP)

				require.NotNil(t, cfg.CapabilitiesConfig)
				assert.True(t, cfg.CapabilitiesConfig.Streaming)
				assert.True(t, cfg.CapabilitiesConfig.PushNotifications)
				assert.False(t, cfg.CapabilitiesConfig.StateTransitionHistory)

				require.NotNil(t, cfg.AuthConfig)
				assert.False(t, cfg.AuthConfig.Enable)
				assert.Equal(t, "http://keycloak:8080/realms/inference-gateway-realm", cfg.AuthConfig.IssuerURL)
				assert.Equal(t, "inference-gateway-client", cfg.AuthConfig.ClientID)

				require.NotNil(t, cfg.QueueConfig)
				assert.Equal(t, 100, cfg.QueueConfig.MaxSize)
				assert.Equal(t, 120*time.Second, cfg.QueueConfig.CleanupInterval)

				require.NotNil(t, cfg.ServerConfig)
				assert.Equal(t, "8080", cfg.ServerConfig.Port)
				assert.Equal(t, 120*time.Second, cfg.ServerConfig.ReadTimeout)
				assert.Equal(t, 120*time.Second, cfg.ServerConfig.WriteTimeout)
				assert.Equal(t, 120*time.Second, cfg.ServerConfig.IdleTimeout)
			},
		},
		{
			name: "overrides defaults with custom env vars",
			envVars: map[string]string{
				"AGENT_URL":                                   "http://localhost:9090",
				"DEBUG":                                       "true",
				"SERVER_PORT":                                 "9090",
				"STREAMING_STATUS_UPDATE_INTERVAL":            "5s",
				"AGENT_CLIENT_PROVIDER":                       "openai",
				"AGENT_CLIENT_MODEL":                          "gpt-4",
				"AGENT_CLIENT_BASE_URL":                       "https://api.openai.com/v1",
				"AGENT_CLIENT_API_KEY":                        "test-key",
				"AGENT_CLIENT_TIMEOUT":                        "45s",
				"AGENT_CLIENT_MAX_RETRIES":                    "5",
				"AGENT_CLIENT_MAX_CHAT_COMPLETION_ITERATIONS": "15",
				"AGENT_CLIENT_USER_AGENT":                     "custom-agent/2.0",
				"AGENT_CLIENT_MAX_TOKENS":                     "8192",
				"AGENT_CLIENT_TEMPERATURE":                    "0.8",
				"AGENT_CLIENT_TOP_P":                          "0.9",
				"CAPABILITIES_STREAMING":                      "false",
				"CAPABILITIES_PUSH_NOTIFICATIONS":             "false",
				"CAPABILITIES_STATE_TRANSITION_HISTORY":       "true",
				"SERVER_TLS_ENABLE":                           "true",
				"SERVER_TLS_CERT_PATH":                        "/custom/cert.pem",
				"SERVER_TLS_KEY_PATH":                         "/custom/key.pem",
				"AUTH_ENABLE":                                 "true",
				"AUTH_ISSUER_URL":                             "http://custom-keycloak:8080/realms/custom",
				"AUTH_CLIENT_ID":                              "custom-client",
				"AUTH_CLIENT_SECRET":                          "custom-secret",
				"QUEUE_MAX_SIZE":                              "500",
				"QUEUE_CLEANUP_INTERVAL":                      "60s",
				"SERVER_READ_TIMEOUT":                         "180s",
				"SERVER_WRITE_TIMEOUT":                        "180s",
				"SERVER_IDLE_TIMEOUT":                         "300s",
			},
			validateFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "", cfg.AgentName)
				assert.Equal(t, "", cfg.AgentDescription)
				assert.Equal(t, "http://localhost:9090", cfg.AgentURL)
				assert.Equal(t, "", cfg.AgentVersion)
				assert.True(t, cfg.Debug)
				assert.Equal(t, "9090", cfg.ServerConfig.Port)
				assert.Equal(t, 5*time.Second, cfg.StreamingStatusUpdateInterval)

				// Test LLM config overrides
				require.NotNil(t, cfg.AgentConfig)
				assert.Equal(t, "openai", cfg.AgentConfig.Provider)
				assert.Equal(t, "gpt-4", cfg.AgentConfig.Model)
				assert.Equal(t, "https://api.openai.com/v1", cfg.AgentConfig.BaseURL)
				assert.Equal(t, "test-key", cfg.AgentConfig.APIKey)
				assert.Equal(t, 45*time.Second, cfg.AgentConfig.Timeout)
				assert.Equal(t, 5, cfg.AgentConfig.MaxRetries)
				assert.Equal(t, 15, cfg.AgentConfig.MaxChatCompletionIterations)
				assert.Equal(t, "custom-agent/2.0", cfg.AgentConfig.UserAgent)
				assert.Equal(t, 8192, cfg.AgentConfig.MaxTokens)
				assert.Equal(t, 0.8, cfg.AgentConfig.Temperature)
				assert.Equal(t, 0.9, cfg.AgentConfig.TopP)

				// Test Capabilities config overrides
				require.NotNil(t, cfg.CapabilitiesConfig)
				assert.False(t, cfg.CapabilitiesConfig.Streaming)
				assert.False(t, cfg.CapabilitiesConfig.PushNotifications)
				assert.True(t, cfg.CapabilitiesConfig.StateTransitionHistory)

				// Test TLS config overrides
				require.NotNil(t, cfg.ServerConfig.TLSConfig)
				assert.True(t, cfg.ServerConfig.TLSConfig.Enable)
				assert.Equal(t, "/custom/cert.pem", cfg.ServerConfig.TLSConfig.CertPath)
				assert.Equal(t, "/custom/key.pem", cfg.ServerConfig.TLSConfig.KeyPath)

				// Test Auth config overrides
				require.NotNil(t, cfg.AuthConfig)
				assert.True(t, cfg.AuthConfig.Enable)
				assert.Equal(t, "http://custom-keycloak:8080/realms/custom", cfg.AuthConfig.IssuerURL)
				assert.Equal(t, "custom-client", cfg.AuthConfig.ClientID)
				assert.Equal(t, "custom-secret", cfg.AuthConfig.ClientSecret)

				// Test Queue config overrides
				require.NotNil(t, cfg.QueueConfig)
				assert.Equal(t, 500, cfg.QueueConfig.MaxSize)
				assert.Equal(t, 60*time.Second, cfg.QueueConfig.CleanupInterval)

				// Test Server config overrides
				require.NotNil(t, cfg.ServerConfig)
				assert.Equal(t, "9090", cfg.ServerConfig.Port)
				assert.Equal(t, 180*time.Second, cfg.ServerConfig.ReadTimeout)
				assert.Equal(t, 180*time.Second, cfg.ServerConfig.WriteTimeout)
				assert.Equal(t, 300*time.Second, cfg.ServerConfig.IdleTimeout)
			},
		},
		{
			name: "partial override with remaining defaults",
			envVars: map[string]string{
				"DEBUG":                 "true",
				"AGENT_CLIENT_PROVIDER": "anthropic",
				"AGENT_CLIENT_MODEL":    "claude-3",
				"QUEUE_MAX_SIZE":        "200",
			},
			validateFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "", cfg.AgentName)
				assert.True(t, cfg.Debug)

				assert.Equal(t, "", cfg.AgentDescription)
				assert.Equal(t, "", cfg.AgentVersion)
				assert.Equal(t, "8080", cfg.ServerConfig.Port)

				require.NotNil(t, cfg.AgentConfig)
				assert.Equal(t, "anthropic", cfg.AgentConfig.Provider)
				assert.Equal(t, "claude-3", cfg.AgentConfig.Model)
				assert.Equal(t, 30*time.Second, cfg.AgentConfig.Timeout)
				assert.Equal(t, 3, cfg.AgentConfig.MaxRetries)

				require.NotNil(t, cfg.QueueConfig)
				assert.Equal(t, 200, cfg.QueueConfig.MaxSize)
				assert.Equal(t, 120*time.Second, cfg.QueueConfig.CleanupInterval)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			lookuper := envconfig.MapLookuper(tt.envVars)
			cfg, err := config.LoadWithLookuper(ctx, nil, lookuper)
			require.NoError(t, err, "should process config without error")
			tt.validateFunc(t, cfg)
		})
	}
}

func TestConfig_LoadWithLookuper_InvalidValues(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		errorText   string
	}{
		{
			name: "invalid duration format",
			envVars: map[string]string{
				"STREAMING_STATUS_UPDATE_INTERVAL": "invalid-duration",
			},
			expectError: true,
			errorText:   "time",
		},
		{
			name: "invalid integer format",
			envVars: map[string]string{
				"AGENT_CLIENT_MAX_RETRIES": "not-a-number",
			},
			expectError: true,
			errorText:   "strconv",
		},
		{
			name: "invalid boolean format",
			envVars: map[string]string{
				"DEBUG": "maybe",
			},
			expectError: true,
			errorText:   "strconv",
		},
		{
			name: "invalid float format",
			envVars: map[string]string{
				"AGENT_CLIENT_TEMPERATURE": "not-a-float",
			},
			expectError: true,
			errorText:   "strconv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			lookuper := envconfig.MapLookuper(tt.envVars)
			_, err := config.LoadWithLookuper(ctx, nil, lookuper)

			if tt.expectError {
				require.Error(t, err, "should return error for invalid input")
				assert.Contains(t, err.Error(), tt.errorText, "error should contain expected text")
			} else {
				require.NoError(t, err, "should not return error for valid input")
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name               string
		envVars            map[string]string
		expectedIterations int
	}{
		{
			name:               "corrects zero max chat completion iterations to 1",
			envVars:            map[string]string{"AGENT_CLIENT_MAX_CHAT_COMPLETION_ITERATIONS": "0"},
			expectedIterations: 1,
		},
		{
			name:               "corrects negative max chat completion iterations to 1",
			envVars:            map[string]string{"AGENT_CLIENT_MAX_CHAT_COMPLETION_ITERATIONS": "-5"},
			expectedIterations: 1,
		},
		{
			name:               "preserves valid max chat completion iterations",
			envVars:            map[string]string{"AGENT_CLIENT_MAX_CHAT_COMPLETION_ITERATIONS": "15"},
			expectedIterations: 15,
		},
		{
			name:               "uses default when not specified",
			envVars:            map[string]string{},
			expectedIterations: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			lookuper := envconfig.MapLookuper(tt.envVars)

			cfg, err := config.LoadWithLookuper(ctx, nil, lookuper)

			require.NoError(t, err)
			require.NotNil(t, cfg.AgentConfig)
			assert.Equal(t, tt.expectedIterations, cfg.AgentConfig.MaxChatCompletionIterations)
		})
	}
}
