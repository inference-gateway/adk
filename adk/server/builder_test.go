package server_test

import (
	"testing"

	"github.com/inference-gateway/a2a/adk/server"
	"github.com/inference-gateway/a2a/adk/server/config"
	"github.com/inference-gateway/a2a/adk/server/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestA2AServerBuilder_BasicConstruction(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() config.Config
		expectPanic bool
	}{
		{
			name: "build with valid config",
			setupConfig: func() config.Config {
				return config.Config{
					AgentName:        "test-agent",
					AgentDescription: "Test agent description",
					Port:             "8080",
				}
			},
			expectPanic: false,
		},
		{
			name: "build with minimal config",
			setupConfig: func() config.Config {
				return config.Config{}
			},
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			logger := zap.NewNop()

			if tt.expectPanic {
				assert.Panics(t, func() {
					server.NewA2AServerBuilder(cfg, logger).Build()
				})
			} else {
				assert.NotPanics(t, func() {
					a2aServer := server.NewA2AServerBuilder(cfg, logger).Build()
					assert.NotNil(t, a2aServer)
				})
			}
		})
	}
}

func TestA2AServerBuilder_WithTaskHandler(t *testing.T) {
	cfg := config.Config{
		AgentName: "test-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()
	mockTaskHandler := &mocks.FakeTaskHandler{}

	a2aServer := server.NewA2AServerBuilder(cfg, logger).
		WithTaskHandler(mockTaskHandler).
		Build()

	assert.NotNil(t, a2aServer)
	assert.Equal(t, mockTaskHandler, a2aServer.GetTaskHandler())
}

func TestA2AServerBuilder_WithTaskResultProcessor(t *testing.T) {
	cfg := config.Config{
		AgentName: "test-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()
	mockProcessor := &mocks.FakeTaskResultProcessor{}

	a2aServer := server.NewA2AServerBuilder(cfg, logger).
		WithTaskResultProcessor(mockProcessor).
		Build()

	assert.NotNil(t, a2aServer)
}

func TestA2AServerBuilder_WithAgentInfoProvider(t *testing.T) {
	cfg := config.Config{
		AgentName: "test-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()
	mockProvider := &mocks.FakeAgentInfoProvider{}

	a2aServer := server.NewA2AServerBuilder(cfg, logger).
		WithAgentInfoProvider(mockProvider).
		Build()

	assert.NotNil(t, a2aServer)
}

func TestA2AServerBuilder_WithSystemPrompt(t *testing.T) {
	cfg := config.Config{
		AgentName: "test-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()
	systemPrompt := "You are a helpful assistant"

	a2aServer := server.NewA2AServerBuilder(cfg, logger).
		WithSystemPrompt(systemPrompt).
		Build()

	assert.NotNil(t, a2aServer)
}

func TestA2AServerBuilder_WithOpenAICompatibleLLMAndTaskHandler(t *testing.T) {
	cfg := config.Config{
		AgentName: "test-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()
	llmConfig := &config.LLMProviderClientConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "test-key",
		BaseURL:  "https://api.openai.com/v1",
	}

	a2aServer := server.NewA2AServerBuilder(cfg, logger).
		WithOpenAICompatibleLLMAndTaskHandler(llmConfig).
		Build()

	assert.NotNil(t, a2aServer)
	assert.NotNil(t, a2aServer.GetTaskHandler())
	assert.NotNil(t, a2aServer.GetLLMClient())
}

func TestA2AServerBuilder_ChainedCalls(t *testing.T) {
	cfg := config.Config{
		AgentName: "test-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()
	mockTaskHandler := &mocks.FakeTaskHandler{}
	mockProcessor := &mocks.FakeTaskResultProcessor{}
	mockProvider := &mocks.FakeAgentInfoProvider{}

	a2aServer := server.NewA2AServerBuilder(cfg, logger).
		WithTaskHandler(mockTaskHandler).
		WithTaskResultProcessor(mockProcessor).
		WithAgentInfoProvider(mockProvider).
		WithSystemPrompt("Test prompt").
		Build()

	assert.NotNil(t, a2aServer)
	assert.Equal(t, mockTaskHandler, a2aServer.GetTaskHandler())
}

func TestSimpleA2AServer(t *testing.T) {
	cfg := config.Config{
		AgentName: "simple-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()

	a2aServer := server.SimpleA2AServer(cfg, logger)

	assert.NotNil(t, a2aServer)
}

func TestCustomA2AServer(t *testing.T) {
	cfg := config.Config{
		AgentName: "custom-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()
	mockTaskHandler := &mocks.FakeTaskHandler{}
	mockProcessor := &mocks.FakeTaskResultProcessor{}
	mockProvider := &mocks.FakeAgentInfoProvider{}

	a2aServer := server.CustomA2AServer(cfg, logger, mockTaskHandler, mockProcessor, mockProvider)

	assert.NotNil(t, a2aServer)
	assert.Equal(t, mockTaskHandler, a2aServer.GetTaskHandler())
}
