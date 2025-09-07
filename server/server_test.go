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
		Version:     "1.0.0",
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
			MaxChatCompletionIterations: 10,
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
		AgentVersion:     "1.0.0",
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
		Version:     "1.0.0",
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
		AgentVersion:     "1.0.0",
		ServerConfig:     config.ServerConfig{Port: "8080"},
	}

	logger := zap.NewNop()

	testAgentCard := types.AgentCard{
		Name:        "test-agent",
		Description: "A test agent",
		URL:         "http://test-agent:8080",
		Version:     "1.0.0",
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
	assert.Equal(t, "1.0.0", agentCard.Version)

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
		AgentVersion:     "1.0.0",
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
		AgentVersion:     "1.0.0",
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

func TestLLMIntegration_CompleteWorkflow(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		userMessage    string
		llmResponses   []any
		expectedStates []types.TaskState
		toolCallsCount int
		description    string
	}{
		{
			name:        "simple text response without tools",
			userMessage: "Hello, how are you?",
			llmResponses: []any{
				&sdk.CreateChatCompletionResponse{
					Id:     "chatcmpl-simple-response",
					Object: "chat.completion",
					Choices: []sdk.ChatCompletionChoice{
						{
							Index: 0,
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "Hello! I'm doing well, thank you for asking. How can I help you today?",
							},
							FinishReason: "stop",
						},
					},
				},
			},
			expectedStates: []types.TaskState{types.TaskStateCompleted},
			toolCallsCount: 0,
			description:    "Simple conversation without tool usage",
		},
		{
			name:        "workflow with tool calls",
			userMessage: "What's the weather like today?",
			llmResponses: []any{
				&sdk.CreateChatCompletionResponse{
					Choices: []sdk.ChatCompletionChoice{
						{
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "",
								ToolCalls: &[]sdk.ChatCompletionMessageToolCall{
									{
										Id:   "call_weather_123",
										Type: "function",
										Function: sdk.ChatCompletionMessageToolCallFunction{
											Name:      "get_weather",
											Arguments: `{"location": "current"}`,
										},
									},
								},
							},
						},
					},
				},
				&sdk.CreateChatCompletionResponse{
					Id:     "chatcmpl-weather-final",
					Object: "chat.completion",
					Choices: []sdk.ChatCompletionChoice{
						{
							Index: 0,
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "Based on the weather data, it's currently sunny with a temperature of 72°F. Perfect weather for outdoor activities!",
							},
							FinishReason: "stop",
						},
					},
				},
			},
			expectedStates: []types.TaskState{types.TaskStateCompleted},
			toolCallsCount: 1,
			description:    "Tool calling workflow with weather check",
		},
		{
			name:        "multiple tool calls in sequence",
			userMessage: "Schedule a meeting and check my calendar",
			llmResponses: []any{
				&sdk.CreateChatCompletionResponse{
					Choices: []sdk.ChatCompletionChoice{
						{
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "",
								ToolCalls: &[]sdk.ChatCompletionMessageToolCall{
									{
										Id:   "call_calendar_check",
										Type: "function",
										Function: sdk.ChatCompletionMessageToolCallFunction{
											Name:      "check_calendar",
											Arguments: `{"date": "today"}`,
										},
									},
									{
										Id:   "call_schedule_meeting",
										Type: "function",
										Function: sdk.ChatCompletionMessageToolCallFunction{
											Name:      "schedule_meeting",
											Arguments: `{"title": "Team Sync", "duration": 60}`,
										},
									},
								},
							},
						},
					},
				},
				&sdk.CreateChatCompletionResponse{
					Id:     "chatcmpl-calendar-final",
					Object: "chat.completion",
					Choices: []sdk.ChatCompletionChoice{
						{
							Index: 0,
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "I've checked your calendar and scheduled the meeting. You have 2 free slots today, and I've booked the Team Sync meeting for 1 hour.",
							},
							FinishReason: "stop",
						},
					},
				},
			},
			expectedStates: []types.TaskState{types.TaskStateCompleted},
			toolCallsCount: 2,
			description:    "Multiple tool calls in single workflow",
		},
		{
			name:        "LLM error handling",
			userMessage: "Process this complex request",
			llmResponses: []any{
				fmt.Errorf("LLM service temporarily unavailable"),
			},
			expectedStates: []types.TaskState{types.TaskStateFailed},
			toolCallsCount: 0,
			description:    "LLM error should result in failed task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLMClient := &mocks.FakeLLMClient{}
			responseIndex := 0

			mockLLMClient.CreateChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (*sdk.CreateChatCompletionResponse, error) {
				if responseIndex >= len(tt.llmResponses) {
					return nil, fmt.Errorf("unexpected additional LLM call")
				}

				response := tt.llmResponses[responseIndex]
				responseIndex++

				if err, ok := response.(error); ok {
					return nil, err
				}

				if sdkResp, ok := response.(*sdk.CreateChatCompletionResponse); ok {
					return sdkResp, nil
				}

				return nil, fmt.Errorf("unexpected response type in test")
			}

			mockToolBox := server.NewDefaultToolBox()

			weatherTool := server.NewBasicTool(
				"get_weather",
				"Get current weather information",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{
							"type":        "string",
							"description": "Location for weather",
						},
					},
					"required": []string{"location"},
				},
				func(ctx context.Context, args map[string]any) (string, error) {
					return `{"temperature": 72, "condition": "sunny", "humidity": 45}`, nil
				},
			)

			calendarTool := server.NewBasicTool(
				"check_calendar",
				"Check calendar availability",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"date": map[string]any{
							"type":        "string",
							"description": "Date to check",
						},
					},
					"required": []string{"date"},
				},
				func(ctx context.Context, args map[string]any) (string, error) {
					return `{"free_slots": 2, "busy_slots": 3, "next_meeting": "3:00 PM"}`, nil
				},
			)

			meetingTool := server.NewBasicTool(
				"schedule_meeting",
				"Schedule a new meeting",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "Meeting title",
						},
						"time": map[string]any{
							"type":        "string",
							"description": "Meeting time",
						},
					},
					"required": []string{"title", "time"},
				},
				func(ctx context.Context, args map[string]any) (string, error) {
					return `{"meeting_id": "mtg_456", "status": "scheduled", "time": "2:00 PM"}`, nil
				},
			)

			mockToolBox.AddTool(weatherTool)
			mockToolBox.AddTool(calendarTool)
			mockToolBox.AddTool(meetingTool)

			agentBuilder := server.NewAgentBuilder(logger).
				WithLLMClient(mockLLMClient)
			if tt.toolCallsCount > 0 {
				agentBuilder = agentBuilder.WithToolBox(mockToolBox)
			}
			agent, err := agentBuilder.Build()
			require.NoError(t, err)

			task := &types.Task{
				ID:      fmt.Sprintf("test-task-%s", tt.name),
				Kind:    "task",
				History: []types.Message{},
				Status: types.TaskStatus{
					State: types.TaskStateSubmitted,
				},
			}

			message := &types.Message{
				Role: "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": tt.userMessage,
					},
				},
			}

			taskHandler := server.NewDefaultTaskHandler(logger)
			if agent != nil {
				taskHandler.SetAgent(agent)
			}
			result, err := taskHandler.HandleTask(context.Background(), task, message)

			if tt.expectedStates[0] == types.TaskStateFailed {
				assert.NotNil(t, result, "Result should not be nil for %s", tt.description)
				assert.Equal(t, tt.expectedStates[0], result.Status.State, "Task state should be failed for %s", tt.description)
			} else {
				assert.NoError(t, err, "Should not have error for %s", tt.description)
				assert.NotNil(t, result, "Result should not be nil for %s", tt.description)
				assert.Equal(t, tt.expectedStates[0], result.Status.State, "Task state should match expected for %s", tt.description)

				if tt.toolCallsCount > 0 {
					expectedMinHistoryLength := 1 + tt.toolCallsCount
					assert.GreaterOrEqual(t, len(result.History), expectedMinHistoryLength,
						"History should contain at least user message and tool results for %s", tt.description)

					toolResultCount := 0
					for _, historyMsg := range result.History {
						if historyMsg.Role == "tool" {
							toolResultCount++
						}
					}
					assert.Equal(t, tt.toolCallsCount, toolResultCount,
						"Should have %d tool result messages for %s", tt.toolCallsCount, tt.description)
				}

				if result.Status.Message != nil {
					finalContent := ""
					for _, part := range result.Status.Message.Parts {
						if partMap, ok := part.(map[string]any); ok {
							if text, exists := partMap["text"]; exists {
								if textStr, ok := text.(string); ok {
									finalContent += textStr
								}
							}
						}
					}

					if tt.expectedStates[0] == types.TaskStateCompleted {
						assert.NotEmpty(t, finalContent, "Final message should not be empty for completed task: %s", tt.description)
					}
				}
			}

			assert.Equal(t, len(tt.llmResponses), responseIndex,
				"Should have made %d LLM calls for %s", len(tt.llmResponses), tt.description)
		})
	}
}

func TestOpenAICompatibleIntegration_CompleteWorkflows(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name                 string
		userMessage          string
		openAIResponses      []any
		expectedFinalState   types.TaskState
		expectedToolCalls    int
		validateFinalMessage func(t *testing.T, msg *types.Message)
		description          string
	}{
		{
			name:        "openai_simple_text_completion",
			userMessage: "Explain quantum computing in simple terms",
			openAIResponses: []any{
				&sdk.CreateChatCompletionResponse{
					Id:     "chatcmpl-simple-text",
					Object: "chat.completion",
					Choices: []sdk.ChatCompletionChoice{
						{
							Index: 0,
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "Quantum computing is a revolutionary approach to processing information...",
							},
							FinishReason: "stop",
						},
					},
				},
			},
			expectedFinalState: types.TaskStateCompleted,
			expectedToolCalls:  0,
			validateFinalMessage: func(t *testing.T, msg *types.Message) {
				assert.Equal(t, "assistant", msg.Role)
				assert.Len(t, msg.Parts, 1)
				part := msg.Parts[0].(map[string]any)
				assert.Equal(t, "text", part["kind"])
				assert.Contains(t, part["text"], "Quantum computing")
			},
			description: "Simple OpenAI-style completion without tools",
		},
		{
			name:        "openai_function_calling_workflow",
			userMessage: "What's the weather like in San Francisco today?",
			openAIResponses: []any{
				&sdk.CreateChatCompletionResponse{
					Choices: []sdk.ChatCompletionChoice{
						{
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "",
								ToolCalls: &[]sdk.ChatCompletionMessageToolCall{
									{
										Id:   "call_abc123",
										Type: "function",
										Function: sdk.ChatCompletionMessageToolCallFunction{
											Name:      "get_weather",
											Arguments: `{"location": "San Francisco, CA"}`,
										},
									},
								},
							},
						},
					},
				},
				&sdk.CreateChatCompletionResponse{
					Id:     "chatcmpl-final-response",
					Object: "chat.completion",
					Choices: []sdk.ChatCompletionChoice{
						{
							Index: 0,
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "Based on the weather data, it's currently 68°F and sunny in San Francisco today. Perfect weather for outdoor activities!",
							},
							FinishReason: "stop",
						},
					},
				},
			},
			expectedFinalState: types.TaskStateCompleted,
			expectedToolCalls:  1,
			validateFinalMessage: func(t *testing.T, msg *types.Message) {
				assert.Equal(t, "assistant", msg.Role)
				assert.Len(t, msg.Parts, 1)
				part := msg.Parts[0].(map[string]any)
				assert.Equal(t, "text", part["kind"])
				assert.Contains(t, part["text"], "68°F")
				assert.Contains(t, part["text"], "San Francisco")
			},
			description: "OpenAI function calling with tool execution",
		},
		{
			name:        "openai_multiple_function_calls",
			userMessage: "Schedule a meeting for tomorrow and check if I have any conflicts",
			openAIResponses: []any{
				&sdk.CreateChatCompletionResponse{
					Choices: []sdk.ChatCompletionChoice{
						{
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "",
								ToolCalls: &[]sdk.ChatCompletionMessageToolCall{
									{
										Id:   "call_check_calendar",
										Type: "function",
										Function: sdk.ChatCompletionMessageToolCallFunction{
											Name:      "check_calendar",
											Arguments: `{"date": "tomorrow"}`,
										},
									},
									{
										Id:   "call_schedule_meeting",
										Type: "function",
										Function: sdk.ChatCompletionMessageToolCallFunction{
											Name:      "schedule_meeting",
											Arguments: `{"title": "Team Meeting", "time": "2:00 PM", "duration": "1 hour"}`,
										},
									},
								},
							},
						},
					},
				},
				&sdk.CreateChatCompletionResponse{
					Id:     "chatcmpl-multi-final",
					Object: "chat.completion",
					Choices: []sdk.ChatCompletionChoice{
						{
							Index: 0,
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "I've checked your calendar and successfully scheduled the team meeting for tomorrow at 2:00 PM. You have no conflicts at that time. The meeting has been added to your calendar.",
							},
							FinishReason: "stop",
						},
					},
				},
			},
			expectedFinalState: types.TaskStateCompleted,
			expectedToolCalls:  2,
			validateFinalMessage: func(t *testing.T, msg *types.Message) {
				assert.Equal(t, "assistant", msg.Role)
				assert.Len(t, msg.Parts, 1)
				part := msg.Parts[0].(map[string]any)
				assert.Equal(t, "text", part["kind"])
				content := part["text"].(string)
				assert.Contains(t, content, "scheduled")
				assert.Contains(t, content, "2:00 PM")
				assert.Contains(t, content, "no conflicts")
			},
			description: "Multiple OpenAI function calls in sequence",
		},
		{
			name:        "openai_streaming_simulation",
			userMessage: "Write a short poem about AI",
			openAIResponses: []any{
				&sdk.CreateChatCompletionResponse{
					Id:     "chatcmpl-poem-response",
					Object: "chat.completion",
					Choices: []sdk.ChatCompletionChoice{
						{
							Index: 0,
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "In circuits deep and algorithms bright,\nAI awakens with digital sight.\nThrough data vast and logic clear,\nA future of wonder draws near.",
							},
							FinishReason: "stop",
						},
					},
				},
			},
			expectedFinalState: types.TaskStateCompleted,
			expectedToolCalls:  0,
			validateFinalMessage: func(t *testing.T, msg *types.Message) {
				assert.Equal(t, "assistant", msg.Role)
				assert.Len(t, msg.Parts, 1)
				part := msg.Parts[0].(map[string]any)
				assert.Equal(t, "text", part["kind"])
				content := part["text"].(string)
				assert.Contains(t, content, "AI")
				assert.Contains(t, content, "\n")
			},
			description: "OpenAI response with creative content",
		},
		{
			name:        "openai_error_response_simulation",
			userMessage: "This request will trigger an error",
			openAIResponses: []any{
				fmt.Errorf("openai api error: rate limit exceeded (status: 429)"),
			},
			expectedFinalState: types.TaskStateFailed,
			expectedToolCalls:  0,
			validateFinalMessage: func(t *testing.T, msg *types.Message) {
				assert.Equal(t, "assistant", msg.Role)
				assert.Len(t, msg.Parts, 1)
				part := msg.Parts[0].(map[string]any)
				assert.Equal(t, "text", part["kind"])
				content := part["text"].(string)
				assert.Contains(t, content, "LLM request failed")
			},
			description: "OpenAI API error handling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLMClient := &mocks.FakeLLMClient{}
			responseIndex := 0

			mockLLMClient.CreateChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (*sdk.CreateChatCompletionResponse, error) {
				if responseIndex >= len(tt.openAIResponses) {
					return nil, fmt.Errorf("unexpected additional LLM call")
				}

				response := tt.openAIResponses[responseIndex]
				responseIndex++

				if err, ok := response.(error); ok {
					return nil, err
				}

				return response.(*sdk.CreateChatCompletionResponse), nil
			}

			mockToolBox := server.NewDefaultToolBox()

			weatherTool := server.NewBasicTool(
				"get_weather",
				"Get current weather information for a location",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{
							"type":        "string",
							"description": "The location to get weather for",
						},
					},
					"required": []string{"location"},
				},
				func(ctx context.Context, args map[string]any) (string, error) {
					location := args["location"].(string)
					return fmt.Sprintf(`{"location": "%s", "temperature": 68, "condition": "sunny", "humidity": 65}`, location), nil
				},
			)

			calendarTool := server.NewBasicTool(
				"check_calendar",
				"Check calendar for availability",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"date": map[string]any{
							"type":        "string",
							"description": "Date to check",
						},
					},
					"required": []string{"date"},
				},
				func(ctx context.Context, args map[string]any) (string, error) {
					return `{"available_slots": ["2:00 PM", "3:00 PM"], "conflicts": []}`, nil
				},
			)

			scheduleTool := server.NewBasicTool(
				"schedule_meeting",
				"Schedule a new meeting",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title":    map[string]any{"type": "string"},
						"time":     map[string]any{"type": "string"},
						"duration": map[string]any{"type": "string"},
					},
					"required": []string{"title", "time"},
				},
				func(ctx context.Context, args map[string]any) (string, error) {
					title := args["title"].(string)
					timeArg := args["time"].(string)
					return fmt.Sprintf(`{"meeting_id": "mtg_%d", "title": "%s", "time": "%s", "status": "scheduled"}`,
						time.Now().Unix(), title, timeArg), nil
				},
			)

			mockToolBox.AddTool(weatherTool)
			mockToolBox.AddTool(calendarTool)
			mockToolBox.AddTool(scheduleTool)

			agentBuilder := server.NewAgentBuilder(logger).
				WithLLMClient(mockLLMClient)
			if tt.expectedToolCalls > 0 {
				agentBuilder = agentBuilder.WithToolBox(mockToolBox)
			}
			agent, err := agentBuilder.Build()
			require.NoError(t, err)

			task := &types.Task{
				ID:      fmt.Sprintf("openai-test-%s", tt.name),
				Kind:    "task",
				History: []types.Message{},
				Status: types.TaskStatus{
					State: types.TaskStateSubmitted,
				},
			}

			message := &types.Message{
				Role: "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": tt.userMessage,
					},
				},
			}

			taskHandler := server.NewDefaultTaskHandler(logger)
			if agent != nil {
				taskHandler.SetAgent(agent)
			}
			result, err := taskHandler.HandleTask(context.Background(), task, message)

			if tt.expectedFinalState == types.TaskStateFailed {
				assert.NotNil(t, result, "Result should not be nil for %s", tt.description)
				assert.Equal(t, tt.expectedFinalState, result.Status.State, "Task state should be failed for %s", tt.description)
			} else {
				assert.NoError(t, err, "Should not have error for %s", tt.description)
				assert.NotNil(t, result, "Result should not be nil for %s", tt.description)
				assert.Equal(t, tt.expectedFinalState, result.Status.State, "Task state should match expected for %s", tt.description)

				if tt.expectedToolCalls > 0 {
					toolResultCount := 0
					for _, historyMsg := range result.History {
						if historyMsg.Role == "tool" {
							toolResultCount++
						}
					}
					assert.Equal(t, tt.expectedToolCalls, toolResultCount,
						"Should have %d tool result messages for %s", tt.expectedToolCalls, tt.description)
				}
			}

			if tt.validateFinalMessage != nil && result.Status.Message != nil {
				tt.validateFinalMessage(t, result.Status.Message)
			}

			assert.Equal(t, len(tt.openAIResponses), responseIndex,
				"Should have made %d LLM calls for %s", len(tt.openAIResponses), tt.description)
		})
	}
}

func TestOpenAICompatibleIntegration_ResponseFormatValidation(t *testing.T) {
	logger := zap.NewNop()

	t.Run("openai_response_to_a2a_message_conversion", func(t *testing.T) {
		mockLLMClient := &mocks.FakeLLMClient{}

		mockResponse := &sdk.CreateChatCompletionResponse{
			Id:     "chatcmpl-test",
			Object: "chat.completion",
			Choices: []sdk.ChatCompletionChoice{
				{
					Index: 0,
					Message: sdk.Message{
						Role:    sdk.Assistant,
						Content: "This is a test response from OpenAI format",
					},
					FinishReason: "stop",
				},
			},
		}

		mockLLMClient.CreateChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (*sdk.CreateChatCompletionResponse, error) {
			return mockResponse, nil
		}

		agent, err := server.NewAgentBuilder(logger).
			WithLLMClient(mockLLMClient).
			Build()
		require.NoError(t, err)

		task := &types.Task{
			ID:      "format-validation-test",
			Kind:    "task",
			History: []types.Message{},
			Status:  types.TaskStatus{State: types.TaskStateSubmitted},
		}

		message := &types.Message{
			Role: "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Test message",
				},
			},
		}

		taskHandler := server.NewDefaultTaskHandler(logger)
		if agent != nil {
			taskHandler.SetAgent(agent)
		}
		result, err := taskHandler.HandleTask(context.Background(), task, message)

		assert.NoError(t, err)
		assert.Equal(t, types.TaskStateCompleted, result.Status.State)
		assert.NotNil(t, result.Status.Message)

		finalMessage := result.Status.Message
		assert.Equal(t, "assistant", finalMessage.Role)
		assert.Equal(t, "message", finalMessage.Kind)
		assert.NotEmpty(t, finalMessage.MessageID)
		assert.Len(t, finalMessage.Parts, 1)

		part := finalMessage.Parts[0].(map[string]any)
		assert.Equal(t, "text", part["kind"])
		assert.Equal(t, "This is a test response from OpenAI format", part["text"])
	})

	t.Run("openai_tool_call_format_validation", func(t *testing.T) {
		mockLLMClient := &mocks.FakeLLMClient{}
		callCount := 0

		mockLLMClient.CreateChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (*sdk.CreateChatCompletionResponse, error) {
			callCount++

			if callCount == 1 {
				return &sdk.CreateChatCompletionResponse{
					Id:      "chatcmpl-tool123",
					Object:  "chat.completion",
					Created: 1677649450,
					Model:   "gpt-3.5-turbo",
					Choices: []sdk.ChatCompletionChoice{
						{
							Index: 0,
							Message: sdk.Message{
								Role:    sdk.Assistant,
								Content: "",
								ToolCalls: &[]sdk.ChatCompletionMessageToolCall{
									{
										Id:   "call_test_function",
										Type: "function",
										Function: sdk.ChatCompletionMessageToolCallFunction{
											Name:      "test_function",
											Arguments: `{"param": "test_value"}`,
										},
									},
								},
							},
							FinishReason: "tool_calls",
						},
					},
					Usage: &sdk.CompletionUsage{
						PromptTokens:     15,
						CompletionTokens: 10,
						TotalTokens:      25,
					},
				}, nil
			}

			return &sdk.CreateChatCompletionResponse{
				Id:     "chatcmpl-final",
				Object: "chat.completion",
				Choices: []sdk.ChatCompletionChoice{
					{
						Index: 0,
						Message: sdk.Message{
							Role:    sdk.Assistant,
							Content: "Function executed successfully with result: test_result",
						},
						FinishReason: "stop",
					},
				},
			}, nil
		}

		mockToolBox := server.NewDefaultToolBox()
		testTool := server.NewBasicTool(
			"test_function",
			"A test function for validation",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"param": map[string]any{
						"type":        "string",
						"description": "Test parameter",
					},
				},
				"required": []string{"param"},
			},
			func(ctx context.Context, args map[string]any) (string, error) {
				return "test_result", nil
			},
		)
		mockToolBox.AddTool(testTool)

		agent, err := server.NewAgentBuilder(logger).
			WithLLMClient(mockLLMClient).
			WithToolBox(mockToolBox).
			Build()
		require.NoError(t, err)

		task := &types.Task{
			ID:      "tool-format-test",
			Kind:    "task",
			History: []types.Message{},
			Status:  types.TaskStatus{State: types.TaskStateSubmitted},
		}

		message := &types.Message{
			Role: "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Execute test function",
				},
			},
		}

		taskHandler := server.NewDefaultTaskHandler(logger)
		if agent != nil {
			taskHandler.SetAgent(agent)
		}
		result, err := taskHandler.HandleTask(context.Background(), task, message)

		assert.NoError(t, err)
		assert.Equal(t, types.TaskStateCompleted, result.Status.State)

		toolResultFound := false
		for _, historyMsg := range result.History {
			if historyMsg.Role == "tool" {
				toolResultFound = true
				assert.Len(t, historyMsg.Parts, 1)

				part := historyMsg.Parts[0].(map[string]any)
				assert.Equal(t, "data", part["kind"], "Tool result should use DataPart")

				data := part["data"].(map[string]any)
				assert.Equal(t, "call_test_function", data["tool_call_id"])
				assert.Equal(t, "test_function", data["tool_name"])
				assert.Equal(t, "test_result", data["result"])
			}
		}
		assert.True(t, toolResultFound, "Tool result should be present in history")

		finalContent := ""
		for _, part := range result.Status.Message.Parts {
			if partMap, ok := part.(map[string]any); ok {
				if text, exists := partMap["text"]; exists {
					if textStr, ok := text.(string); ok {
						finalContent += textStr
					}
				}
			}
		}
		assert.Contains(t, finalContent, "test_result")
		assert.Contains(t, finalContent, "successfully")

		assert.Equal(t, 2, callCount, "Should have made 2 LLM calls (tool call + final response)")
	})
}
