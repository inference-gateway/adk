package server_test

import (
	"context"
	"testing"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	mocks "github.com/inference-gateway/adk/server/mocks"
	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
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

