package server_test

import (
	"context"
	"testing"

	"github.com/inference-gateway/a2a/adk"
	"github.com/inference-gateway/a2a/adk/server"
	"github.com/inference-gateway/a2a/adk/server/config"
	"github.com/inference-gateway/a2a/adk/server/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewAgentBuilder(t *testing.T) {
	logger := zap.NewNop()
	builder := server.NewAgentBuilder(logger)

	assert.NotNil(t, builder)
	assert.Implements(t, (*server.AgentBuilder)(nil), builder)
}

func TestAgentBuilder_Build_WithDefaults(t *testing.T) {
	logger := zap.NewNop()

	agent, err := server.NewAgentBuilder(logger).Build()

	require.NoError(t, err)
	assert.NotNil(t, agent)

	task := &adk.Task{
		ID:      "test-task",
		Status:  adk.TaskStatus{State: adk.TaskStateSubmitted},
		History: []adk.Message{},
	}
	message := &adk.Message{
		Role: "user",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Hello",
			},
		},
	}
	result, err := agent.ProcessTask(context.TODO(), task, message)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAgentBuilder_WithConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.AgentConfig
		expected func(*testing.T, *server.DefaultOpenAICompatibleAgent)
	}{
		{
			name: "custom_system_prompt",
			config: &config.AgentConfig{
				SystemPrompt:                "Custom test prompt",
				MaxChatCompletionIterations: 5,
			},
			expected: func(t *testing.T, agent *server.DefaultOpenAICompatibleAgent) {
				assert.NotNil(t, agent)
			},
		},
		{
			name: "custom_max_iterations",
			config: &config.AgentConfig{
				SystemPrompt:                "You are a helpful AI assistant.",
				MaxChatCompletionIterations: 20,
			},
			expected: func(t *testing.T, agent *server.DefaultOpenAICompatibleAgent) {
				assert.NotNil(t, agent)
			},
		},
		{
			name: "full_config",
			config: &config.AgentConfig{
				Provider:                    "openai",
				Model:                       "gpt-4",
				SystemPrompt:                "Test system prompt",
				MaxChatCompletionIterations: 15,
				Temperature:                 0.8,
				MaxTokens:                   2048,
			},
			expected: func(t *testing.T, agent *server.DefaultOpenAICompatibleAgent) {
				assert.NotNil(t, agent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockLLMClient := &mocks.FakeLLMClient{}

			agent, err := server.NewAgentBuilder(logger).
				WithConfig(tt.config).
				WithLLMClient(mockLLMClient).
				Build()

			require.NoError(t, err)
			tt.expected(t, agent)
		})
	}
}

func TestAgentBuilder_WithLLMClient(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestAgentBuilder_WithToolBox(t *testing.T) {
	logger := zap.NewNop()
	mockToolBox := server.NewDefaultToolBox()

	agent, err := server.NewAgentBuilder(logger).
		WithToolBox(mockToolBox).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestAgentBuilder_WithSystemPrompt(t *testing.T) {
	tests := []struct {
		name         string
		systemPrompt string
	}{
		{
			name:         "custom_prompt",
			systemPrompt: "You are a specialized AI assistant for testing.",
		},
		{
			name:         "empty_prompt",
			systemPrompt: "",
		},
		{
			name:         "long_prompt",
			systemPrompt: "You are a comprehensive AI assistant designed to help with complex tasks involving multiple steps, detailed analysis, and creative problem-solving approaches.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()

			agent, err := server.NewAgentBuilder(logger).
				WithSystemPrompt(tt.systemPrompt).
				Build()

			require.NoError(t, err)
			assert.NotNil(t, agent)
		})
	}
}

func TestAgentBuilder_WithMaxChatCompletion(t *testing.T) {
	tests := []struct {
		name          string
		maxIterations int
		shouldSucceed bool
	}{
		{
			name:          "valid_iterations",
			maxIterations: 5,
			shouldSucceed: true,
		},
		{
			name:          "high_iterations",
			maxIterations: 100,
			shouldSucceed: true,
		},
		{
			name:          "zero_iterations",
			maxIterations: 0,
			shouldSucceed: true,
		},
		{
			name:          "negative_iterations",
			maxIterations: -5,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockLLMClient := &mocks.FakeLLMClient{}

			agent, err := server.NewAgentBuilder(logger).
				WithMaxChatCompletion(tt.maxIterations).
				WithLLMClient(mockLLMClient).
				Build()

			if tt.shouldSucceed {
				require.NoError(t, err)
				assert.NotNil(t, agent)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAgentBuilder_ChainedCalls(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}
	mockToolBox := server.NewDefaultToolBox()
	customConfig := &config.AgentConfig{
		Provider:                    "openai",
		Model:                       "gpt-4",
		SystemPrompt:                "Original prompt",
		MaxChatCompletionIterations: 10,
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(customConfig).
		WithLLMClient(mockLLMClient).
		WithToolBox(mockToolBox).
		WithSystemPrompt("Overridden prompt").
		WithMaxChatCompletion(15).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestAgentBuilder_ConfigFromLLMClient(t *testing.T) {
	logger := zap.NewNop()

	agentConfig := &config.AgentConfig{
		Provider:                    "openai",
		Model:                       "gpt-3.5-turbo",
		BaseURL:                     "https://api.openai.com/v1",
		SystemPrompt:                "Test prompt",
		MaxChatCompletionIterations: 5,
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(agentConfig).
		Build()

	if err != nil {
		assert.Contains(t, err.Error(), "failed to create llm client from config")
	} else {
		assert.NotNil(t, agent)
	}
}

func TestAgentBuilder_NilConfig(t *testing.T) {
	logger := zap.NewNop()

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(nil).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestAgentBuilder_OverrideSystemPrompt(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	configWithPrompt := &config.AgentConfig{
		SystemPrompt:                "Config prompt",
		MaxChatCompletionIterations: 10,
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(configWithPrompt).
		WithSystemPrompt("Builder prompt").
		WithLLMClient(mockLLMClient).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestAgentBuilder_WithCompleteConfiguration(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}
	mockToolBox := server.NewDefaultToolBox()

	testTool := server.NewBasicTool(
		"test_tool",
		"A test tool for demonstration",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Test input parameter",
				},
			},
			"required": []string{"input"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			return "test result", nil
		},
	)
	mockToolBox.AddTool(testTool)

	fullConfig := &config.AgentConfig{
		Provider:                    "openai",
		Model:                       "gpt-4",
		SystemPrompt:                "You are a test assistant",
		MaxChatCompletionIterations: 8,
		Temperature:                 0.7,
		MaxTokens:                   1024,
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(fullConfig).
		WithLLMClient(mockLLMClient).
		WithToolBox(mockToolBox).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, agent)
}

// Test convenience functions
func TestSimpleAgent(t *testing.T) {
	logger := zap.NewNop()

	agent, err := server.SimpleAgent(logger)

	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestAgentWithConfig(t *testing.T) {
	logger := zap.NewNop()
	testConfig := &config.AgentConfig{
		Provider:                    "openai",
		Model:                       "gpt-3.5-turbo",
		SystemPrompt:                "Test config prompt",
		MaxChatCompletionIterations: 7,
	}

	agent, err := server.AgentWithConfig(logger, testConfig)

	if err != nil {
		assert.Contains(t, err.Error(), "failed to create llm client")
	} else {
		assert.NotNil(t, agent)
	}
}

func TestAgentWithLLM(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	agent, err := server.AgentWithLLM(logger, mockLLMClient)

	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestFullyConfiguredAgent(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}
	mockToolBox := server.NewDefaultToolBox()
	testConfig := &config.AgentConfig{
		SystemPrompt:                "Fully configured prompt",
		MaxChatCompletionIterations: 12,
	}

	agent, err := server.FullyConfiguredAgent(logger, testConfig, mockLLMClient, mockToolBox)

	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestAgentBuilder_BuilderInterface(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	builder := server.NewAgentBuilder(logger)

	builder = builder.WithSystemPrompt("test")
	builder = builder.WithMaxChatCompletion(5)
	builder = builder.WithLLMClient(mockLLMClient)

	agent, err := builder.Build()
	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestAgentBuilder_MultipleBuilds(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}
	builder := server.NewAgentBuilder(logger).
		WithSystemPrompt("Shared prompt").
		WithMaxChatCompletion(10).
		WithLLMClient(mockLLMClient)

	agent1, err1 := builder.Build()
	agent2, err2 := builder.Build()

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotNil(t, agent1)
	assert.NotNil(t, agent2)

	assert.NotSame(t, agent1, agent2)
}

func TestAgentBuilder_ErrorHandling(t *testing.T) {
	logger := zap.NewNop()

	invalidConfig := &config.AgentConfig{
		Provider: "invalid-provider",
		Model:    "",
		BaseURL:  "invalid-url",
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(invalidConfig).
		Build()

	assert.Error(t, err)
	assert.Nil(t, agent)
}

func TestAgentBuilder_FluentInterface(t *testing.T) {
	logger := zap.NewNop()

	builder := server.NewAgentBuilder(logger)

	result1 := builder.WithConfig(&config.AgentConfig{})
	result2 := result1.WithSystemPrompt("test")
	result3 := result2.WithMaxChatCompletion(5)
	result4 := result3.WithLLMClient(&mocks.FakeLLMClient{})
	result5 := result4.WithToolBox(server.NewDefaultToolBox())

	assert.Implements(t, (*server.AgentBuilder)(nil), result1)
	assert.Implements(t, (*server.AgentBuilder)(nil), result2)
	assert.Implements(t, (*server.AgentBuilder)(nil), result3)
	assert.Implements(t, (*server.AgentBuilder)(nil), result4)
	assert.Implements(t, (*server.AgentBuilder)(nil), result5)

	agent, err := result5.Build()
	require.NoError(t, err)
	assert.NotNil(t, agent)
}
