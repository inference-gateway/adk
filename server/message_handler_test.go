package server_test

import (
	"context"
	"testing"
	"time"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	mocks "github.com/inference-gateway/adk/server/mocks"
	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zap "go.uber.org/zap"
)

func TestDefaultMessageHandler_HandleMessageSend(t *testing.T) {
	tests := []struct {
		name           string
		params         types.MessageSendParams
		setupMocks     func(*mocks.FakeTaskManager)
		expectError    bool
		expectedTaskID string
	}{
		{
			name: "successful message send",
			params: types.MessageSendParams{
				Message: types.Message{
					Kind:      "message",
					MessageID: "test-msg-1",
					Role:      "user",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Hello world",
						},
					},
				},
			},
			setupMocks: func(taskManager *mocks.FakeTaskManager) {
				task := &types.Task{
					ID:        "test-task-1",
					ContextID: "test-context",
					Status: types.TaskStatus{
						State: types.TaskStateSubmitted,
						Message: &types.Message{
							Kind:      "message",
							MessageID: "test-msg-1",
							Role:      "user",
						},
					},
				}
				taskManager.CreateTaskReturns(task)
			},
			expectError:    false,
			expectedTaskID: "test-task-1",
		},
		{
			name: "message with empty parts",
			params: types.MessageSendParams{
				Message: types.Message{
					Kind:      "message",
					MessageID: "test-msg-2",
					Role:      "user",
					Parts:     []types.Part{},
				},
			},
			setupMocks: func(taskManager *mocks.FakeTaskManager) {
				// No setup needed for error case
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockTaskManager := &mocks.FakeTaskManager{}
			tt.setupMocks(mockTaskManager)

			cfg := &config.Config{
				AgentConfig: config.AgentConfig{
					MaxChatCompletionIterations: 10,
				},
			}

			messageHandler := server.NewDefaultMessageHandler(logger, mockTaskManager, cfg)
			ctx := context.Background()

			task, err := messageHandler.HandleMessageSend(ctx, tt.params)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, task)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, task)
				assert.Equal(t, tt.expectedTaskID, task.ID)
			}
		})
	}
}

func TestDefaultMessageHandler_HandleMessageStream(t *testing.T) {
	logger := zap.NewNop()
	mockTaskManager := &mocks.FakeTaskManager{}

	expectedTask := &types.Task{
		ID:        "test-task-123",
		Kind:      "task",
		ContextID: "test-context",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
	}
	mockTaskManager.CreateTaskReturns(expectedTask)
	mockTaskManager.UpdateStateReturns(nil)

	cfg := &config.Config{
		AgentConfig: config.AgentConfig{
			MaxChatCompletionIterations: 10,
		},
	}

	messageHandler := server.NewDefaultMessageHandler(logger, mockTaskManager, cfg)

	params := types.MessageSendParams{
		Message: types.Message{
			Kind:      "message",
			MessageID: "test-msg-stream",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hello streaming",
				},
			},
		},
	}

	responseChan := make(chan types.SendStreamingMessageResponse, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var responses []types.SendStreamingMessageResponse
	done := make(chan bool)
	go func() {
		defer close(done)
		for {
			select {
			case response, ok := <-responseChan:
				if !ok {
					return
				}
				responses = append(responses, response)
			case <-ctx.Done():
				return
			}
		}
	}()

	err := messageHandler.HandleMessageStream(ctx, params, responseChan)
	assert.NoError(t, err)

	close(responseChan)

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("test timed out waiting for response collection")
	}

	assert.Equal(t, 1, mockTaskManager.CreateTaskCallCount())

	assert.GreaterOrEqual(t, len(responses), 1)

	if len(responses) > 0 {
		if statusEvent, ok := responses[0].(types.TaskStatusUpdateEvent); ok {
			assert.Equal(t, "status-update", statusEvent.Kind)
			assert.Equal(t, expectedTask.ID, statusEvent.TaskID)
			assert.Equal(t, expectedTask.ContextID, statusEvent.ContextID)
			assert.Equal(t, types.TaskStateWorking, statusEvent.Status.State)
		} else {
			t.Errorf("Expected first response to be TaskStatusUpdateEvent, got %T", responses[0])
		}
	}

	assert.Eventually(t, func() bool {
		return mockTaskManager.UpdateStateCallCount() > 0
	}, 200*time.Millisecond, 10*time.Millisecond)
}

func TestDefaultMessageHandler_ValidateMessage(t *testing.T) {
	tests := []struct {
		name        string
		message     types.Message
		expectError bool
		errorType   string
	}{
		{
			name: "valid message with text part",
			message: types.Message{
				Kind:      "message",
				MessageID: "valid-msg",
				Role:      "user",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Valid message",
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty parts",
			message: types.Message{
				Kind:      "message",
				MessageID: "empty-parts",
				Role:      "user",
				Parts:     []types.Part{},
			},
			expectError: true,
			errorType:   "empty message parts",
		},
		{
			name: "nil parts",
			message: types.Message{
				Kind:      "message",
				MessageID: "nil-parts",
				Role:      "user",
				Parts:     nil,
			},
			expectError: true,
			errorType:   "empty message parts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockTaskManager := &mocks.FakeTaskManager{}

			cfg := &config.Config{
				AgentConfig: config.AgentConfig{
					MaxChatCompletionIterations: 10,
					SystemPrompt:                "You are a helpful AI assistant.",
				},
			}

			messageHandler := server.NewDefaultMessageHandler(logger, mockTaskManager, cfg)

			params := types.MessageSendParams{Message: tt.message}
			ctx := context.Background()

			_, err := messageHandler.HandleMessageSend(ctx, params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != "" {
					assert.Contains(t, err.Error(), tt.errorType)
				}
			} else {
				task := &types.Task{
					ID:        "test-task",
					ContextID: "test-context",
					Status: types.TaskStatus{
						State:   types.TaskStateSubmitted,
						Message: &tt.message,
					},
				}
				mockTaskManager.CreateTaskReturns(task)

				task, err = messageHandler.HandleMessageSend(ctx, params)
				assert.NoError(t, err)
				assert.NotNil(t, task)
			}
		})
	}
}

func TestMessageHandler_HandleMessageStream_WithoutAgent(t *testing.T) {
	logger := zap.NewNop()

	taskManager := server.NewDefaultTaskManager(logger, 10)

	cfg := &config.Config{
		AgentConfig: config.AgentConfig{
			MaxChatCompletionIterations: 10,
			SystemPrompt:                "You are a helpful AI assistant.",
		},
	}

	messageHandler := server.NewDefaultMessageHandler(logger, taskManager, cfg)

	contextID := "test-context"
	params := types.MessageSendParams{
		Message: types.Message{
			ContextID: &contextID,
			Kind:      "message",
			MessageID: "test-message",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hello, how are you?",
				},
			},
		},
	}

	responseChan := make(chan types.SendStreamingMessageResponse, 10)
	defer close(responseChan)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := messageHandler.HandleMessageStream(ctx, params, responseChan)
	require.NoError(t, err)

	var responses []types.SendStreamingMessageResponse
	timeout := time.After(500 * time.Millisecond)

responseLoop:
	for {
		select {
		case response := <-responseChan:
			responses = append(responses, response)
			if statusUpdate, ok := response.(types.TaskStatusUpdateEvent); ok && statusUpdate.Final {
				break responseLoop
			}
		case <-timeout:
			t.Fatal("Timeout waiting for streaming responses")
		}
	}

	assert.GreaterOrEqual(t, len(responses), 5, "Should have initial status + 4 mock chunks + final completion")

	if statusUpdate, ok := responses[0].(types.TaskStatusUpdateEvent); ok {
		assert.Equal(t, "status-update", statusUpdate.Kind)
		assert.False(t, statusUpdate.Final)
	} else {
		t.Fatalf("First response should be TaskStatusUpdateEvent, got %T", responses[0])
	}

	var statusUpdates []types.TaskStatusUpdateEvent
	for _, resp := range responses {
		if statusUpdate, ok := resp.(types.TaskStatusUpdateEvent); ok && !statusUpdate.Final {
			statusUpdates = append(statusUpdates, statusUpdate)
		}
	}

	assert.GreaterOrEqual(t, len(statusUpdates), 4, "Should have at least 4 mock status updates")

	foundMockText := false
	for _, update := range statusUpdates {
		if update.Status.Message != nil && len(update.Status.Message.Parts) > 0 {
			if textPart, ok := update.Status.Message.Parts[0].(map[string]interface{}); ok {
				if text, exists := textPart["text"]; exists {
					if textStr, ok := text.(string); ok && textStr == "Starting to process your request..." {
						foundMockText = true
						break
					}
				}
			}
		}
	}
	assert.True(t, foundMockText, "Should find mock text in status updates")

	lastResponse := responses[len(responses)-1]
	if statusUpdate, ok := lastResponse.(types.TaskStatusUpdateEvent); ok {
		assert.Equal(t, "status-update", statusUpdate.Kind)
		assert.True(t, statusUpdate.Final)
		assert.Equal(t, types.TaskStateCompleted, statusUpdate.Status.State)
	} else {
		t.Fatalf("Last response should be TaskStatusUpdateEvent, got %T", lastResponse)
	}
}
