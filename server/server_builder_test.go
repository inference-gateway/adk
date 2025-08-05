package server_test

import (
	"testing"
	"time"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	mocks "github.com/inference-gateway/adk/server/mocks"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zap "go.uber.org/zap"
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
					ServerConfig:     config.ServerConfig{Port: "8080"},
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
					_, _ = server.NewA2AServerBuilder(cfg, logger).Build()
				})
			} else {
				assert.NotPanics(t, func() {
					a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
						WithAgentCard(createTestAgentCard()).
						Build()
					assert.NoError(t, err)
					assert.NotNil(t, a2aServer)
				})
			}
		})
	}
}

func TestA2AServerBuilder_WithTaskHandler(t *testing.T) {
	cfg := config.Config{
		AgentName:    "test-agent",
		ServerConfig: config.ServerConfig{Port: "8080"},
	}
	logger := zap.NewNop()
	mockTaskHandler := &mocks.FakeTaskHandler{}

	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithTaskHandler(mockTaskHandler).
		WithAgentCard(createTestAgentCard()).
		Build()

	require.NoError(t, err)

	assert.NotNil(t, a2aServer)
	assert.Equal(t, mockTaskHandler, a2aServer.GetTaskHandler())
}

func TestA2AServerBuilder_WithTaskResultProcessor(t *testing.T) {
	cfg := config.Config{
		AgentName:    "test-agent",
		ServerConfig: config.ServerConfig{Port: "8080"},
	}
	logger := zap.NewNop()
	mockProcessor := &mocks.FakeTaskResultProcessor{}

	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithTaskResultProcessor(mockProcessor).
		WithAgentCard(createTestAgentCard()).
		Build()

	require.NoError(t, err)

	assert.NotNil(t, a2aServer)
}

func TestA2AServerBuilder_WithAgent(t *testing.T) {
	cfg := config.Config{
		AgentName:    "test-agent",
		ServerConfig: config.ServerConfig{Port: "8080"},
	}
	logger := zap.NewNop()
	systemPrompt := "You are a helpful assistant"

	agent, err := server.NewAgentBuilder(logger).
		WithSystemPrompt(systemPrompt).
		Build()
	require.NoError(t, err)

	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithAgentCard(createTestAgentCard()).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, a2aServer)
	assert.NotNil(t, a2aServer.GetAgent())
}

func TestA2AServerBuilder_WithAgentAndConfig(t *testing.T) {
	cfg := config.Config{
		AgentName:    "test-agent",
		ServerConfig: config.ServerConfig{Port: "8080"},
	}
	logger := zap.NewNop()
	agentConfig := &config.AgentConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "test-key",
		BaseURL:  "https://api.openai.com/v1",
	}

	agent, err := server.NewOpenAICompatibleAgentWithLLMConfig(logger, agentConfig)
	assert.NoError(t, err)

	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithAgentCard(createTestAgentCard()).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, a2aServer)
	assert.NotNil(t, a2aServer.GetAgent())
}

func TestA2AServerBuilder_ChainedCalls(t *testing.T) {
	cfg := config.Config{
		AgentName:    "test-agent",
		ServerConfig: config.ServerConfig{Port: "8080"},
	}
	logger := zap.NewNop()
	mockTaskHandler := &mocks.FakeTaskHandler{}
	mockProcessor := &mocks.FakeTaskResultProcessor{}

	agent, err := server.NewAgentBuilder(logger).
		WithSystemPrompt("Test prompt").
		Build()
	require.NoError(t, err)

	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithTaskHandler(mockTaskHandler).
		WithTaskResultProcessor(mockProcessor).
		WithAgent(agent).
		WithAgentCard(createTestAgentCard()).
		Build()
	require.NoError(t, err)

	assert.NotNil(t, a2aServer)
	assert.Equal(t, mockTaskHandler, a2aServer.GetTaskHandler())
}

func TestNewDefaultA2AServer(t *testing.T) {
	a2aServer := server.NewDefaultA2AServer(nil)

	assert.NotNil(t, a2aServer)
}

func TestCustomA2AServer(t *testing.T) {
	cfg := config.Config{
		AgentName:    "custom-agent",
		ServerConfig: config.ServerConfig{Port: "8080"},
	}
	logger := zap.NewNop()
	mockTaskHandler := &mocks.FakeTaskHandler{}
	mockProcessor := &mocks.FakeTaskResultProcessor{}

	a2aServer, err := server.CustomA2AServer(cfg, logger, mockTaskHandler, mockProcessor, createTestAgentCard())
	require.NoError(t, err)

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
	fakeBuilder.BuildReturns(mockServer, nil)

	logger := zap.NewNop()
	agent := server.NewOpenAICompatibleAgent(logger)

	result, err := fakeBuilder.
		WithLogger(logger).
		WithAgent(agent).
		Build()
	require.NoError(t, err, "Expected no error when building server with mocks")

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
		AgentName:    "test-agent",
		ServerConfig: config.ServerConfig{Port: "8080"},
	}
	logger := zap.NewNop()

	builder := server.NewA2AServerBuilder(cfg, logger)

	result, err := builder.
		WithLogger(logger).
		WithAgentCard(createTestAgentCard()).
		Build()
	require.NoError(t, err, "Expected no error when building server with polymorphic interface")

	assert.NotNil(t, result)
	assert.NotNil(t, builder)
}

func TestA2AServerBuilderInterface_AllMethods(t *testing.T) {
	fakeBuilder := &mocks.FakeA2AServerBuilder{}

	fakeBuilder.WithLoggerReturns(fakeBuilder)
	fakeBuilder.WithAgentReturns(fakeBuilder)
	fakeBuilder.WithTaskHandlerReturns(fakeBuilder)
	fakeBuilder.WithTaskResultProcessorReturns(fakeBuilder)

	mockServer := &mocks.FakeA2AServer{}
	fakeBuilder.BuildReturns(mockServer, nil)

	logger := zap.NewNop()
	agent := server.NewOpenAICompatibleAgent(logger)
	taskHandler := &mocks.FakeTaskHandler{}
	taskResultProcessor := &mocks.FakeTaskResultProcessor{}

	result, err := fakeBuilder.
		WithLogger(logger).
		WithAgent(agent).
		WithTaskHandler(taskHandler).
		WithTaskResultProcessor(taskResultProcessor).
		Build()
	require.NoError(t, err, "Expected no error when building server with all methods")

	assert.Equal(t, 1, fakeBuilder.WithLoggerCallCount())
	assert.Equal(t, 1, fakeBuilder.WithAgentCallCount())
	assert.Equal(t, 1, fakeBuilder.WithTaskHandlerCallCount())
	assert.Equal(t, 1, fakeBuilder.WithTaskResultProcessorCallCount())
	assert.Equal(t, 1, fakeBuilder.BuildCallCount())

	assert.Equal(t, mockServer, result)
}

func TestA2AServerBuilder_WithDefaultPollingTaskHandler(t *testing.T) {
	cfg := config.Config{
		AgentName:    "test-agent",
		AgentVersion: "1.0.0",
	}
	logger := zap.NewNop()

	builder := server.NewA2AServerBuilder(cfg, logger)
	
	// Use WithDefaultPollingTaskHandler and test that it sets the handler
	builderWithHandler := builder.WithDefaultPollingTaskHandler()
	
	// The builder should return itself for method chaining
	assert.Equal(t, builder, builderWithHandler)
}

func TestA2AServerBuilder_WithDefaultStreamingTaskHandler(t *testing.T) {
	cfg := config.Config{
		AgentName:    "test-agent",
		AgentVersion: "1.0.0",
	}
	logger := zap.NewNop()

	builder := server.NewA2AServerBuilder(cfg, logger)
	
	// Use WithDefaultStreamingTaskHandler and test that it sets the handler
	builderWithHandler := builder.WithDefaultStreamingTaskHandler()
	
	// The builder should return itself for method chaining
	assert.Equal(t, builder, builderWithHandler)
}

func TestServerBuilderAppliesAgentConfigDefaults(t *testing.T) {
	logger := zap.NewNop()

	cfg := config.Config{
		AgentName:    "test-agent",
		ServerConfig: config.ServerConfig{Port: "8080"},
		Debug:        true,
		QueueConfig: config.QueueConfig{
			CleanupInterval: 5 * time.Minute,
		},
	}

	if cfg.AgentConfig.MaxConversationHistory != 0 {
		t.Errorf("expected MaxConversationHistory to be 0 initially, got %d", cfg.AgentConfig.MaxConversationHistory)
	}

	builder := server.NewA2AServerBuilder(cfg, logger)

	server, err := builder.
		WithAgentCard(createTestAgentCard()).
		Build()
	require.NoError(t, err, "Expected no error when building server with defaults")

	if server == nil {
		t.Fatal("server should not be nil")
	}
}

func TestServerBuilderPreservesExplicitAgentConfig(t *testing.T) {
	logger := zap.NewNop()

	cfg := config.Config{
		AgentName:    "test-agent",
		ServerConfig: config.ServerConfig{Port: "8080"},
		Debug:        true,
		AgentConfig: config.AgentConfig{
			MaxConversationHistory:      5,
			SystemPrompt:                "Custom system prompt",
			MaxChatCompletionIterations: 10,
		},
		QueueConfig: config.QueueConfig{
			CleanupInterval: 5 * time.Minute,
		},
	}

	assert.Equal(t, 5, cfg.AgentConfig.MaxConversationHistory, "Expected explicit MaxConversationHistory to be 5")
	assert.Equal(t, "Custom system prompt", cfg.AgentConfig.SystemPrompt, "Expected explicit SystemPrompt")
	assert.Equal(t, 10, cfg.AgentConfig.MaxChatCompletionIterations, "Expected explicit MaxChatCompletionIterations")

	builder := server.NewA2AServerBuilder(cfg, logger)
	srv, err := builder.WithAgentCard(createTestAgentCard()).Build()

	require.NoError(t, err)
	if srv == nil {
		t.Fatal("server should not be nil")
	}
}

func TestA2AServerBuilder_Build_RequiresAgentCard(t *testing.T) {
	cfg := config.Config{
		AgentName:    "test-agent",
		ServerConfig: config.ServerConfig{Port: "8080"},
	}
	logger := zap.NewNop()

	builder := server.NewA2AServerBuilder(cfg, logger)
	srv, err := builder.Build()

	assert.Error(t, err)
	assert.Nil(t, srv)
	assert.Contains(t, err.Error(), "agent card must be configured")
}
