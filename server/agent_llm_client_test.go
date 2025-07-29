package server_test

import (
	"testing"

	"github.com/inference-gateway/adk/server"
	"github.com/inference-gateway/adk/server/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewOpenAICompatibleLLMClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.AgentConfig
		expectError bool
	}{
		{
			name: "valid OpenAI config",
			config: &config.AgentConfig{
				Provider: "openai",
				Model:    "gpt-4",
				APIKey:   "test-key",
				BaseURL:  "https://api.openai.com/v1",
			},
			expectError: false,
		},
		{
			name: "valid Anthropic config",
			config: &config.AgentConfig{
				Provider: "anthropic",
				Model:    "claude-3",
				APIKey:   "test-key",
				BaseURL:  "https://api.anthropic.com",
			},
			expectError: false,
		},
		{
			name: "config with custom parameters",
			config: &config.AgentConfig{
				Provider:    "openai",
				Model:       "gpt-3.5-turbo",
				APIKey:      "test-key",
				BaseURL:     "https://api.openai.com/v1",
				MaxTokens:   2048,
				Temperature: 0.8,
				TopP:        0.9,
			},
			expectError: false,
		},
		{
			name: "config without API key (optional)",
			config: &config.AgentConfig{
				Provider: "openai",
				Model:    "gpt-4",
				BaseURL:  "https://api.openai.com/v1",
			},
			expectError: false,
		},
		{
			name: "missing provider",
			config: &config.AgentConfig{
				Model:   "gpt-4",
				APIKey:  "test-key",
				BaseURL: "https://api.openai.com/v1",
			},
			expectError: true,
		},
		{
			name: "missing model",
			config: &config.AgentConfig{
				Provider: "openai",
				APIKey:   "test-key",
				BaseURL:  "https://api.openai.com/v1",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()

			client, err := server.NewOpenAICompatibleLLMClient(tt.config, logger)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestLLMClient_Interface(t *testing.T) {
	logger := zap.NewNop()
	config := &config.AgentConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "test-key",
		BaseURL:  "https://api.openai.com/v1",
	}

	client, err := server.NewOpenAICompatibleLLMClient(config, logger)
	assert.NoError(t, err)

	var _ server.LLMClient = client
}

func TestLLMClient_ConfigValidation(t *testing.T) {
	logger := zap.NewNop()

	client, err := server.NewOpenAICompatibleLLMClient(nil, logger)
	assert.Error(t, err)
	assert.Nil(t, client)

	emptyConfig := &config.AgentConfig{}
	client, err = server.NewOpenAICompatibleLLMClient(emptyConfig, logger)
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestLLMClient_WithMockSDK(t *testing.T) {
	logger := zap.NewNop()

	config := &config.AgentConfig{
		Provider:         "openai",
		Model:            "gpt-4",
		APIKey:           "test-key",
		BaseURL:          "https://api.openai.com/v1",
		MaxTokens:        2048,
		Temperature:      0.7,
		TopP:             0.9,
		FrequencyPenalty: 0.1,
		PresencePenalty:  0.2,
	}

	client, err := server.NewOpenAICompatibleLLMClient(config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}
