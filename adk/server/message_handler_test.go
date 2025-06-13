package server_test

import (
	"context"
	"testing"

	"github.com/inference-gateway/a2a/adk"
	"github.com/inference-gateway/a2a/adk/server"
	"github.com/inference-gateway/a2a/adk/server/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultMessageHandler_HandleMessageSend(t *testing.T) {
	tests := []struct {
		name           string
		params         adk.MessageSendParams
		setupMocks     func(*mocks.FakeTaskManager)
		expectError    bool
		expectedTaskID string
	}{
		{
			name: "successful message send",
			params: adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "test-msg-1",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Hello world",
						},
					},
				},
			},
			setupMocks: func(taskManager *mocks.FakeTaskManager) {
				task := &adk.Task{
					ID:        "test-task-1",
					ContextID: "test-context",
					Status: adk.TaskStatus{
						State: adk.TaskStateSubmitted,
						Message: &adk.Message{
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
			params: adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "test-msg-2",
					Role:      "user",
					Parts:     []adk.Part{},
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

			messageHandler := server.NewDefaultMessageHandler(logger, mockTaskManager)
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
	messageHandler := server.NewDefaultMessageHandler(logger, mockTaskManager)

	params := adk.MessageSendParams{
		Message: adk.Message{
			Kind:      "message",
			MessageID: "test-msg-stream",
			Role:      "user",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hello streaming",
				},
			},
		},
	}

	ctx := context.Background()
	err := messageHandler.HandleMessageStream(ctx, params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "streaming not implemented")
}

func TestDefaultMessageHandler_ValidateMessage(t *testing.T) {
	tests := []struct {
		name        string
		message     adk.Message
		expectError bool
		errorType   string
	}{
		{
			name: "valid message with text part",
			message: adk.Message{
				Kind:      "message",
				MessageID: "valid-msg",
				Role:      "user",
				Parts: []adk.Part{
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
			message: adk.Message{
				Kind:      "message",
				MessageID: "empty-parts",
				Role:      "user",
				Parts:     []adk.Part{},
			},
			expectError: true,
			errorType:   "empty message parts",
		},
		{
			name: "nil parts",
			message: adk.Message{
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
			messageHandler := server.NewDefaultMessageHandler(logger, mockTaskManager)

			params := adk.MessageSendParams{Message: tt.message}
			ctx := context.Background()

			_, err := messageHandler.HandleMessageSend(ctx, params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != "" {
					assert.Contains(t, err.Error(), tt.errorType)
				}
			} else {
				task := &adk.Task{
					ID:        "test-task",
					ContextID: "test-context",
					Status: adk.TaskStatus{
						State:   adk.TaskStateSubmitted,
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
