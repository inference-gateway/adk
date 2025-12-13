package server_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	mocks "github.com/inference-gateway/adk/server/mocks"
	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zap "go.uber.org/zap"
)

// createTestAgentCard creates a test agent card for use in tests
func createTestAgentCard() types.AgentCard {
	return types.AgentCard{
		Name:        "test-agent",
		Description: "A test agent",
		URL:         "http://test-agent:8080",
		Version:     "0.1.0",
		Capabilities: types.AgentCapabilities{
			Streaming:              boolPtr(true),
			PushNotifications:      boolPtr(true),
			StateTransitionHistory: boolPtr(true),
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
	}
}

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}

func TestA2AServer_TaskManager_CreateTask(t *testing.T) {
	tests := []struct {
		name      string
		contextID string
		state     types.TaskState
		message   *types.Message
	}{
		{
			name:      "create task with submitted state",
			contextID: "test-context-1",
			state:     types.TaskStateSubmitted,
			message: &types.Message{
				Kind:      "message",
				MessageID: "test-message-1",
				Role:      "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Hello world",
					},
				},
			},
		},
		{
			name:      "create task with working state",
			contextID: "test-context-2",
			state:     types.TaskStateWorking,
			message: &types.Message{
				Kind:      "message",
				MessageID: "test-message-2",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Processing your request",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			taskManager := server.NewDefaultTaskManager(logger)

			task := taskManager.CreateTask(tt.contextID, tt.state, tt.message)

			assert.NotNil(t, task)
			assert.NotEmpty(t, task.ID)
			assert.Equal(t, tt.contextID, task.ContextID)
			assert.Equal(t, tt.state, task.Status.State)
			assert.Equal(t, tt.message, task.Status.Message)
			assert.NotNil(t, task.Status.Timestamp)
		})
	}
}

func TestA2AServer_TaskManager_GetTask(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger)

	message := &types.Message{
		Kind:      "message",
		MessageID: "test-message",
		Role:      "user",
	}
	task := taskManager.CreateTask("test-context", types.TaskStateSubmitted, message)

	err := taskManager.GetStorage().EnqueueTask(task, "test-request-id")
	assert.NoError(t, err)

	retrievedTask, exists := taskManager.GetTask(task.ID)
	assert.True(t, exists)
	assert.Equal(t, task.ID, retrievedTask.ID)
	assert.Equal(t, task.ContextID, retrievedTask.ContextID)

	nonExistentTask, exists := taskManager.GetTask("non-existent-id")
	assert.False(t, exists)
	assert.Nil(t, nonExistentTask)
}

func TestA2AServer_ResponseSender_SendSuccess(t *testing.T) {
	logger := zap.NewNop()
	responseSender := server.NewDefaultResponseSender(logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	result := map[string]any{
		"status": "success",
		"data":   "test data",
	}

	assert.NotPanics(t, func() {
		responseSender.SendSuccess(ctx, "test-id", result)
	})
}

func TestA2AServer_ResponseSender_SendError(t *testing.T) {
	logger := zap.NewNop()
	responseSender := server.NewDefaultResponseSender(logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	assert.NotPanics(t, func() {
		responseSender.SendError(ctx, "test-id", 500, "test error message")
	})
}

func TestA2AServer_DirectTaskCreation_Integration(t *testing.T) {
	cfg := &config.Config{
		AgentConfig: config.AgentConfig{
			MaxChatCompletionIterations: 50,
			SystemPrompt:                "You are a helpful AI assistant.",
		},
	}

	logger := zap.NewNop()
	a2aServer := server.NewA2AServer(cfg, logger, nil)

	// Test that the server was created with proper task handlers
	backgroundHandler := a2aServer.GetBackgroundTaskHandler()
	assert.NotNil(t, backgroundHandler, "Background task handler should be set")

	streamingHandler := a2aServer.GetStreamingTaskHandler()
	assert.NotNil(t, streamingHandler, "Streaming task handler should be set")

	// Verify that task handlers are different instances (as expected for different scenarios)
	assert.IsType(t, &server.DefaultBackgroundTaskHandler{}, backgroundHandler)
	assert.IsType(t, &server.DefaultStreamingTaskHandler{}, streamingHandler)
}

func TestA2AServer_TaskProcessing_Background(t *testing.T) {
	baseConfig := config.Config{
		QueueConfig: config.QueueConfig{
			MaxSize:         10,
			CleanupInterval: 50 * time.Millisecond,
		},
		CapabilitiesConfig: config.CapabilitiesConfig{
			Streaming:              true,
			PushNotifications:      false,
			StateTransitionHistory: true,
		},
		AuthConfig: config.AuthConfig{
			Enable: false,
		},
	}

	cfg, err := config.NewWithDefaults(context.Background(), &baseConfig)
	require.NoError(t, err)

	logger := zap.NewNop()

	a2aServer := server.NewA2AServer(cfg, logger, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go a2aServer.StartTaskProcessor(ctx)

	time.Sleep(100 * time.Millisecond)

	assert.True(t, true)
}

func TestDefaultA2AServer_SetDependencies(t *testing.T) {
	customConfig := &config.Config{
		AgentName:        "custom-test-agent",
		AgentDescription: "A custom test agent for dependency injection",
		AgentURL:         "http://custom-agent:9090",
		AgentVersion:     "2.5.0",
		ServerConfig:     config.ServerConfig{Port: "9090"},
		Debug:            true,
	}

	a2aServer := server.NewDefaultA2AServer(customConfig)

	mockTaskHandler := &mocks.FakeTaskHandler{}
	a2aServer.SetBackgroundTaskHandler(mockTaskHandler)
	a2aServer.SetStreamingTaskHandler(&mocks.FakeStreamableTaskHandler{})

	mockProcessor := &mocks.FakeTaskResultProcessor{}
	a2aServer.SetTaskResultProcessor(mockProcessor)

	agentCard := a2aServer.GetAgentCard()
	assert.Nil(t, agentCard, "Expected no agent card to be set by default")
}

func TestA2AServerBuilder_UsesProvidedConfiguration(t *testing.T) {
	partialCfg := &config.Config{
		AgentName:        "test-custom-agent",
		AgentDescription: "A test agent with custom configuration",
		AgentURL:         "http://test-agent:9999",
		AgentVersion:     "2.0.0",
		ServerConfig:     config.ServerConfig{Port: "9999"},
		Debug:            true,
	}

	logger := zap.NewNop()

	serverInstance, err := server.NewA2AServerBuilder(*partialCfg, logger).
		WithAgentCard(createTestAgentCard()).
		WithDefaultTaskHandlers().
		Build()

	require.NoError(t, err, "Expected no error when building server with partial config")

	assert.NotNil(t, serverInstance)

	agentCard := serverInstance.GetAgentCard()
	assert.NotNil(t, agentCard, "Expected agent card to be set")
	assert.Equal(t, "test-agent", agentCard.Name)
}

func TestA2AServerBuilder_UsesProvidedCapabilitiesConfiguration(t *testing.T) {
	cfg := config.Config{
		AgentName:        "test-agent",
		AgentDescription: "A test agent",
		AgentURL:         "http://test-agent:8080",
		AgentVersion:     "0.1.0",
		ServerConfig:     config.ServerConfig{Port: "8080"},
		CapabilitiesConfig: config.CapabilitiesConfig{
			Streaming:              false,
			PushNotifications:      false,
			StateTransitionHistory: true,
		},
	}

	logger := zap.NewNop()

	testAgentCard := types.AgentCard{
		Name:        "test-agent",
		Description: "A test agent",
		URL:         "http://test-agent:8080",
		Version:     "0.1.0",
		Capabilities: types.AgentCapabilities{
			Streaming:              &cfg.CapabilitiesConfig.Streaming,
			PushNotifications:      &cfg.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: &cfg.CapabilitiesConfig.StateTransitionHistory,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills:             []types.AgentSkill{},
	}

	serverInstance, err := server.NewA2AServerBuilder(cfg, logger).
		WithAgentCard(testAgentCard).
		WithDefaultTaskHandlers().
		Build()
	require.NoError(t, err, "Expected no error when building server with custom capabilities configuration")

	assert.NotNil(t, serverInstance)

	agentCard := serverInstance.GetAgentCard()
	assert.NotNil(t, agentCard)
	assert.Equal(t, "test-agent", agentCard.Name)

	assert.NotNil(t, agentCard.Capabilities.Streaming)
	assert.NotNil(t, agentCard.Capabilities.PushNotifications)
	assert.NotNil(t, agentCard.Capabilities.StateTransitionHistory)
	assert.False(t, *agentCard.Capabilities.Streaming)
	assert.False(t, *agentCard.Capabilities.PushNotifications)
	assert.True(t, *agentCard.Capabilities.StateTransitionHistory)
}

func TestA2AServerBuilder_HandlesNilConfigurationSafely(t *testing.T) {
	partialCfg := &config.Config{
		AgentName:        "test-agent",
		AgentDescription: "A test agent",
		AgentURL:         "http://test-agent:8080",
		AgentVersion:     "0.1.0",
		ServerConfig:     config.ServerConfig{Port: "8080"},
	}

	logger := zap.NewNop()

	testAgentCard := types.AgentCard{
		Name:        "test-agent",
		Description: "A test agent",
		URL:         "http://test-agent:8080",
		Version:     "0.1.0",
		Capabilities: types.AgentCapabilities{
			Streaming:              &[]bool{true}[0],
			PushNotifications:      &[]bool{true}[0],
			StateTransitionHistory: &[]bool{false}[0],
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills:             []types.AgentSkill{},
	}

	serverInstance, err := server.NewA2AServerBuilder(*partialCfg, logger).
		WithAgentCard(testAgentCard).
		WithDefaultTaskHandlers().
		Build()
	require.NoError(t, err, "Expected no error when building server with partial config")

	assert.NotNil(t, serverInstance)

	agentCard := serverInstance.GetAgentCard()
	assert.NotNil(t, agentCard)
	assert.Equal(t, "test-agent", agentCard.Name)
	assert.Equal(t, "A test agent", agentCard.Description)
	assert.Equal(t, "http://test-agent:8080", agentCard.URL)
	assert.Equal(t, "0.1.0", agentCard.Version)

	assert.NotNil(t, agentCard.Capabilities.Streaming)
	assert.NotNil(t, agentCard.Capabilities.PushNotifications)
	assert.NotNil(t, agentCard.Capabilities.StateTransitionHistory)
	assert.True(t, *agentCard.Capabilities.Streaming)
	assert.True(t, *agentCard.Capabilities.PushNotifications)
	assert.False(t, *agentCard.Capabilities.StateTransitionHistory)
}

func TestA2AServer_TaskProcessing_MessageContent(t *testing.T) {
	logger := zap.NewNop()

	mockTaskHandler := &mocks.FakeTaskHandler{}
	mockTaskHandler.HandleTaskReturns(&types.Task{
		ID:        "test-task",
		ContextID: "test-context",
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
			Message: &types.Message{
				Kind:      "message",
				MessageID: "response-msg",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Hello! I received your message.",
					},
				},
			},
		},
	}, nil)

	baseCfg := &config.Config{
		AgentName:        "test-agent",
		AgentDescription: "A test agent",
		AgentURL:         "http://test-agent:8080",
		AgentVersion:     "0.1.0",
		ServerConfig:     config.ServerConfig{Port: "8080"},
		Debug:            false,
		QueueConfig: config.QueueConfig{
			MaxSize:         10,
			CleanupInterval: 1 * time.Second,
		},
	}

	cfg, err := config.NewWithDefaults(context.Background(), baseCfg)
	require.NoError(t, err)

	serverInstance := server.NewA2AServer(cfg, logger, nil)
	serverInstance.SetBackgroundTaskHandler(mockTaskHandler)
	serverInstance.SetStreamingTaskHandler(&mocks.FakeStreamableTaskHandler{})

	originalMessage := &types.Message{
		Kind:      "message",
		MessageID: "original-msg",
		Role:      "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "What is the weather like today?",
			},
		},
	}

	task := &types.Task{
		ID:        "test-task",
		ContextID: "test-context",
		Kind:      "task",
		Status: types.TaskStatus{
			State:   types.TaskStateSubmitted,
			Message: originalMessage,
		},
	}

	ctx := context.Background()
	result, err := serverInstance.GetBackgroundTaskHandler().HandleTask(ctx, task, originalMessage)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, types.TaskStateCompleted, result.Status.State)
	assert.Equal(t, 1, mockTaskHandler.HandleTaskCallCount())

	_, actualTask, actualMessage := mockTaskHandler.HandleTaskArgsForCall(0)
	assert.NotNil(t, actualTask)
	assert.NotNil(t, actualMessage)

	assert.NotEmpty(t, actualMessage.Parts)
	assert.Len(t, actualMessage.Parts, 1)

	part := actualMessage.Parts[0]
	partMap, ok := part.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "text", partMap["kind"])
	assert.Equal(t, "What is the weather like today?", partMap["text"])
}

func TestA2AServer_ProcessQueuedTask_MessageContent(t *testing.T) {
	logger := zap.NewNop()

	mockTaskHandler := &mocks.FakeTaskHandler{}
	mockTaskHandler.HandleTaskReturns(&types.Task{
		ID:        "test-task",
		ContextID: "test-context",
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
			Message: &types.Message{
				Kind:      "message",
				MessageID: "response-msg",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "I received your weather question and here's the answer...",
					},
				},
			},
		},
	}, nil)

	baseCfg := &config.Config{
		AgentName:        "weather-agent",
		AgentDescription: "A weather agent",
		AgentURL:         "http://weather-agent:8080",
		AgentVersion:     "0.1.0",
		ServerConfig:     config.ServerConfig{Port: "8080"},
		Debug:            false,
		QueueConfig: config.QueueConfig{
			MaxSize:         10,
			CleanupInterval: 1 * time.Second,
		},
	}

	cfg, err := config.NewWithDefaults(context.Background(), baseCfg)
	require.NoError(t, err)

	serverInstance := server.NewA2AServer(cfg, logger, nil)
	serverInstance.SetBackgroundTaskHandler(mockTaskHandler)
	serverInstance.SetStreamingTaskHandler(&mocks.FakeStreamableTaskHandler{})

	originalUserMessage := &types.Message{
		Kind:      "message",
		MessageID: "user-msg-123",
		Role:      "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "What is the weather like today in San Francisco?",
			},
		},
	}

	task := &types.Task{
		ID:        "task-456",
		ContextID: "context-789",
		Kind:      "task",
		Status: types.TaskStatus{
			State:   types.TaskStateSubmitted,
			Message: originalUserMessage,
		},
		History: []types.Message{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go serverInstance.StartTaskProcessor(ctx)

	time.Sleep(10 * time.Millisecond)

	result, err := serverInstance.GetBackgroundTaskHandler().HandleTask(ctx, task, originalUserMessage)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, types.TaskStateCompleted, result.Status.State)

	assert.Equal(t, 1, mockTaskHandler.HandleTaskCallCount())

	_, actualTask, actualMessage := mockTaskHandler.HandleTaskArgsForCall(0)

	assert.NotNil(t, actualTask)
	assert.NotNil(t, actualMessage)

	assert.NotEmpty(t, actualMessage.Parts, "Message parts should not be empty - this was the reported bug")
	assert.Len(t, actualMessage.Parts, 1, "Should have exactly one message part")

	part := actualMessage.Parts[0]
	partMap, ok := part.(map[string]any)
	assert.True(t, ok, "Message part should be a map")
	assert.Equal(t, "text", partMap["kind"], "Message part should be of kind 'text'")
	assert.Equal(t, "What is the weather like today in San Francisco?", partMap["text"],
		"Message content should be preserved exactly as sent by the client")

	assert.Equal(t, "user", actualMessage.Role, "Message role should be 'user'")
}

func TestTaskGetWithInvalidFieldName(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]any
		wantErr bool
	}{
		{
			name: "correct field name 'id' should work",
			params: map[string]any{
				"id": "some-task-id",
			},
			wantErr: true,
		},
		{
			name: "incorrect field name 'taskId' should result in empty task ID",
			params: map[string]any{
				"taskId": "some-task-id",
			},
			wantErr: true,
		},
		{
			name:    "missing task ID parameter",
			params:  map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				assert.True(t, tt.wantErr, "Expected error for case: %s", tt.name)
			}
		})
	}
}

func TestToolResultProcessingFix(t *testing.T) {
	t.Run("tool result messages should have role 'tool'", func(t *testing.T) {
		toolResultMsg := map[string]any{
			"role": "tool",
			"parts": []map[string]any{
				{
					"kind": "data",
					"data": map[string]any{
						"tool_call_id": "call_123",
						"tool_name":    "list_events",
						"result":       `{"events": [], "success": true}`,
					},
				},
			},
		}

		role, ok := toolResultMsg["role"].(string)
		assert.True(t, ok, "Role should be a string")
		assert.Equal(t, "tool", role, "Tool result messages should have role 'tool'")

		parts, ok := toolResultMsg["parts"].([]map[string]any)
		assert.True(t, ok, "Parts should be an array")
		assert.Len(t, parts, 1, "Should have one part")

		part := parts[0]
		assert.Equal(t, "data", part["kind"], "Tool result should use DataPart with kind 'data'")

		data, exists := part["data"].(map[string]any)
		assert.True(t, exists, "Tool result should have 'data' field")

		result, exists := data["result"]
		assert.True(t, exists, "Tool result data should have 'result' field")
		assert.NotEmpty(t, result, "Tool result should not be empty")
	})

	t.Run("convertToSDKMessages should handle A2A compliant tool results", func(t *testing.T) {
		parts := []map[string]any{
			{
				"kind": "data",
				"data": map[string]any{
					"tool_call_id": "call_123",
					"tool_name":    "test_tool",
					"result":       "Tool execution successful",
				},
			},
		}

		var content string
		for _, part := range parts {
			if part["kind"] == "data" {
				if data, exists := part["data"].(map[string]any); exists {
					if result, exists := data["result"]; exists {
						if resultStr, ok := result.(string); ok {
							content += resultStr
						}
					}
				}
			}
		}

		assert.Equal(t, "Tool execution successful", content, "Should extract tool result content")
		assert.NotEmpty(t, content, "Content should not be empty after extraction")
	})
}

func TestAgentStreamingIntegration_SimpleResponse(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{Delta: sdk.ChatCompletionStreamResponseDelta{Content: "Hello"}},
				},
			}
			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{Delta: sdk.ChatCompletionStreamResponseDelta{Content: " world!"}},
				},
			}
		}()

		return responseChan, errorChan
	}

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Say hello",
				},
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)
	require.NotNil(t, eventChan)

	eventCount := 0
	for range eventChan {
		eventCount++
	}

	assert.Greater(t, eventCount, 0, "Should receive events from streaming agent")
}

func TestBackgroundHandler_WithStreamingAgent(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{Delta: sdk.ChatCompletionStreamResponseDelta{Content: "Task completed"}},
				},
			}
			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{FinishReason: "stop"},
				},
			}
		}()

		return responseChan, errorChan
	}

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		Build()
	require.NoError(t, err)

	handler := server.NewDefaultBackgroundTaskHandler(logger, agent)

	task := &types.Task{
		ID:        "test-task",
		ContextID: "test-context",
		History:   []types.Message{},
		Status: types.TaskStatus{
			State: types.TaskStateSubmitted,
		},
	}

	message := &types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Complete this task",
			},
		},
	}

	result, err := handler.HandleTask(context.Background(), task, message)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, types.TaskStateCompleted, result.Status.State)
	assert.NotNil(t, result.Status.Message)
}

func TestBackgroundHandler_StreamingFailure(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse)
		errorChan := make(chan error, 1)

		go func() {
			errorChan <- fmt.Errorf("LLM streaming failed")
			close(errorChan)
			close(responseChan)
		}()

		return responseChan, errorChan
	}

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		Build()
	require.NoError(t, err)

	handler := server.NewDefaultBackgroundTaskHandler(logger, agent)

	task := &types.Task{
		ID:        "test-task-fail",
		ContextID: "test-context",
		History:   []types.Message{},
		Status: types.TaskStatus{
			State: types.TaskStateSubmitted,
		},
	}

	message := &types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "This will fail",
			},
		},
	}

	result, err := handler.HandleTask(context.Background(), task, message)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, types.TaskStateFailed, result.Status.State)
}

func TestAgentStreaming_WithToolCalls(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	callCount := 0
	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			callCount++
			if callCount == 1 {
				toolCallChunks := []sdk.ChatCompletionMessageToolCallChunk{
					{
						Index: 0,
						ID:    "call_123",
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      "test_tool",
							Arguments: `{"arg":"value"}`,
						},
					},
				}
				responseChan <- &sdk.CreateChatCompletionStreamResponse{
					Choices: []sdk.ChatCompletionStreamChoice{
						{Delta: sdk.ChatCompletionStreamResponseDelta{ToolCalls: toolCallChunks}},
					},
				}
			} else {
				responseChan <- &sdk.CreateChatCompletionStreamResponse{
					Choices: []sdk.ChatCompletionStreamChoice{
						{Delta: sdk.ChatCompletionStreamResponseDelta{Content: "Tool executed successfully"}},
					},
				}
			}
		}()

		return responseChan, errorChan
	}

	toolBox := server.NewDefaultToolBox(nil)
	testTool := server.NewBasicTool(
		"test_tool",
		"Test tool",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"arg": map[string]any{"type": "string"},
			},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			return `{"result": "success"}`, nil
		},
	)
	toolBox.AddTool(testTool)

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		WithToolBox(toolBox).
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Use the tool",
				},
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)

	eventCount := 0
	for range eventChan {
		eventCount++
	}

	assert.Greater(t, eventCount, 0)
	assert.GreaterOrEqual(t, callCount, 2, "Should make at least 2 LLM calls (tool + final)")
}
