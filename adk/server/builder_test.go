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

func TestA2AServerBuilder_WithAgent(t *testing.T) {
	cfg := config.Config{
		AgentName: "test-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()
	systemPrompt := "You are a helpful assistant"

	agent := server.NewDefaultOpenAICompatibleAgent(logger)
	agent.SetSystemPrompt(systemPrompt)

	a2aServer := server.NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		Build()

	assert.NotNil(t, a2aServer)
	assert.NotNil(t, a2aServer.GetAgent())
}

func TestA2AServerBuilder_WithAgentAndLLMConfig(t *testing.T) {
	cfg := config.Config{
		AgentName: "test-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()
	llmConfig := &config.AgentConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "test-key",
		BaseURL:  "https://api.openai.com/v1",
	}

	agent, err := server.NewOpenAICompatibleAgentWithConfig(logger, llmConfig)
	assert.NoError(t, err)

	a2aServer := server.NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		Build()

	assert.NotNil(t, a2aServer)
	assert.NotNil(t, a2aServer.GetAgent())
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

	agent := server.NewDefaultOpenAICompatibleAgent(logger)
	agent.SetSystemPrompt("Test prompt")

	a2aServer := server.NewA2AServerBuilder(cfg, logger).
		WithTaskHandler(mockTaskHandler).
		WithTaskResultProcessor(mockProcessor).
		WithAgentInfoProvider(mockProvider).
		WithAgent(agent).
		Build()

	assert.NotNil(t, a2aServer)
	assert.Equal(t, mockTaskHandler, a2aServer.GetTaskHandler())
}

func TestNewDefaultA2AServer(t *testing.T) {
	a2aServer := server.NewDefaultA2AServer()

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

func TestA2AServerBuilderInterface_WithMocks(t *testing.T) {
	fakeBuilder := &mocks.FakeA2AServerBuilder{}
	mockServer := &mocks.FakeA2AServer{}

	fakeBuilder.WithLoggerReturns(fakeBuilder)
	fakeBuilder.WithAgentReturns(fakeBuilder)
	fakeBuilder.WithTaskHandlerReturns(fakeBuilder)
	fakeBuilder.WithTaskResultProcessorReturns(fakeBuilder)
	fakeBuilder.WithAgentInfoProviderReturns(fakeBuilder)
	fakeBuilder.BuildReturns(mockServer)

	logger := zap.NewNop()
	agent := server.NewDefaultOpenAICompatibleAgent(logger)

	result := fakeBuilder.
		WithLogger(logger).
		WithAgent(agent).
		Build()

	assert.Equal(t, 1, fakeBuilder.WithLoggerCallCount())
	assert.Equal(t, 1, fakeBuilder.WithAgentCallCount())
	assert.Equal(t, 1, fakeBuilder.BuildCallCount())

	loggerArg := fakeBuilder.WithLoggerArgsForCall(0)
	assert.Equal(t, logger, loggerArg)

	agentArg := fakeBuilder.WithAgentArgsForCall(0)
	assert.Equal(t, agent, agentArg)

	assert.Equal(t, mockServer, result)
}

func TestA2AServerBuilderInterface_Polymorphism(t *testing.T) {
	cfg := config.Config{
		AgentName: "test-agent",
		Port:      "8080",
	}
	logger := zap.NewNop()

	builder := server.NewA2AServerBuilder(cfg, logger)

	result := builder.
		WithLogger(logger).
		Build()

	assert.NotNil(t, result)
	assert.NotNil(t, builder)
}

func TestA2AServerBuilderInterface_AllMethods(t *testing.T) {
	fakeBuilder := &mocks.FakeA2AServerBuilder{}

	fakeBuilder.WithLoggerReturns(fakeBuilder)
	fakeBuilder.WithAgentReturns(fakeBuilder)
	fakeBuilder.WithTaskHandlerReturns(fakeBuilder)
	fakeBuilder.WithTaskResultProcessorReturns(fakeBuilder)
	fakeBuilder.WithAgentInfoProviderReturns(fakeBuilder)

	mockServer := &mocks.FakeA2AServer{}
	fakeBuilder.BuildReturns(mockServer)

	logger := zap.NewNop()
	agent := server.NewDefaultOpenAICompatibleAgent(logger)
	taskHandler := &mocks.FakeTaskHandler{}
	taskResultProcessor := &mocks.FakeTaskResultProcessor{}
	agentInfoProvider := &mocks.FakeAgentInfoProvider{}

	result := fakeBuilder.
		WithLogger(logger).
		WithAgent(agent).
		WithTaskHandler(taskHandler).
		WithTaskResultProcessor(taskResultProcessor).
		WithAgentInfoProvider(agentInfoProvider).
		Build()

	assert.Equal(t, 1, fakeBuilder.WithLoggerCallCount())
	assert.Equal(t, 1, fakeBuilder.WithAgentCallCount())
	assert.Equal(t, 1, fakeBuilder.WithTaskHandlerCallCount())
	assert.Equal(t, 1, fakeBuilder.WithTaskResultProcessorCallCount())
	assert.Equal(t, 1, fakeBuilder.WithAgentInfoProviderCallCount())
	assert.Equal(t, 1, fakeBuilder.BuildCallCount())

	assert.Equal(t, mockServer, result)
}
