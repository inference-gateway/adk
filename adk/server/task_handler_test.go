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

func TestDefaultTaskHandler_HandleTask(t *testing.T) {
	tests := []struct {
		name        string
		task        *adk.Task
		expectError bool
		expectedMsg string
	}{
		{
			name: "successful task handling",
			task: &adk.Task{
				ID:        "test-task-1",
				ContextID: "test-context",
				Status: adk.TaskStatus{
					State: adk.TaskStateSubmitted,
					Message: &adk.Message{
						Kind:      "message",
						MessageID: "test-msg",
						Role:      "user",
						Parts: []adk.Part{
							map[string]interface{}{
								"kind": "text",
								"text": "Hello from task",
							},
						},
					},
				},
			},
			expectError: false,
			expectedMsg: "Task processed successfully",
		},
		{
			name: "task with nil message",
			task: &adk.Task{
				ID:        "test-task-2",
				ContextID: "test-context",
				Status: adk.TaskStatus{
					State:   adk.TaskStateSubmitted,
					Message: nil,
				},
			},
			expectError: false,
			expectedMsg: "Task processed successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			taskHandler := server.NewDefaultTaskHandler(logger)

			ctx := context.Background()
			message := tt.task.Status.Message
			result, err := taskHandler.HandleTask(ctx, tt.task, message)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)

				assert.Equal(t, adk.TaskStateCompleted, result.Status.State)
				assert.NotEmpty(t, result.History)

				if len(result.History) > 0 {
					lastMessage := result.History[len(result.History)-1]
					assert.Equal(t, "assistant", lastMessage.Role)
					assert.NotEmpty(t, lastMessage.Parts)

					if len(lastMessage.Parts) > 0 {
						if textPart, ok := lastMessage.Parts[0].(map[string]interface{}); ok {
							if text, exists := textPart["text"]; exists {
								assert.Contains(t, text.(string), tt.expectedMsg)
							}
						}
					}
				}
			}
		})
	}
}

func TestDefaultTaskHandlerWithLLM_Creation(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	taskHandler := server.NewDefaultTaskHandlerWithLLM(logger, mockLLMClient)

	assert.NotNil(t, taskHandler)
}

func TestDefaultTaskHandlerWithLLM_HandleTask(t *testing.T) {
	tests := []struct {
		name           string
		task           *adk.Task
		setupMocks     func(*mocks.FakeLLMClient)
		expectError    bool
		expectedResult bool
	}{
		{
			name: "successful LLM task handling",
			task: &adk.Task{
				ID:        "llm-task-1",
				ContextID: "test-context",
				Status: adk.TaskStatus{
					State: adk.TaskStateSubmitted,
					Message: &adk.Message{
						Kind:      "message",
						MessageID: "test-msg",
						Role:      "user",
						Parts: []adk.Part{
							map[string]interface{}{
								"kind": "text",
								"text": "What is 2+2?",
							},
						},
					},
				},
			},
			setupMocks: func(mockLLM *mocks.FakeLLMClient) {
				mockLLM.CreateChatCompletionReturns(&adk.Message{
					Kind:      "message",
					MessageID: "llm-response",
					Role:      "assistant",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "2+2 equals 4",
						},
					},
				}, nil)
			},
			expectError:    false,
			expectedResult: true,
		},
		{
			name: "LLM client returns error",
			task: &adk.Task{
				ID:        "llm-task-2",
				ContextID: "test-context",
				Status: adk.TaskStatus{
					State: adk.TaskStateSubmitted,
					Message: &adk.Message{
						Kind:      "message",
						MessageID: "test-msg",
						Role:      "user",
						Parts: []adk.Part{
							map[string]interface{}{
								"kind": "text",
								"text": "Cause an error",
							},
						},
					},
				},
			},
			setupMocks: func(mockLLM *mocks.FakeLLMClient) {
				mockLLM.CreateChatCompletionReturns(nil, assert.AnError)
			},
			expectError:    true,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockLLMClient := &mocks.FakeLLMClient{}
			tt.setupMocks(mockLLMClient)

			taskHandler := server.NewDefaultTaskHandlerWithLLM(logger, mockLLMClient)

			ctx := context.Background()
			message := tt.task.Status.Message
			result, err := taskHandler.HandleTask(ctx, tt.task, message)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectedResult {
					assert.NotNil(t, result)
					assert.Equal(t, adk.TaskStateCompleted, result.Status.State)
				}
			}

			assert.Equal(t, 1, mockLLMClient.CreateChatCompletionCallCount())
		})
	}
}

func TestDefaultTaskHandlerWithLLM_SetLLMClient(t *testing.T) {
	logger := zap.NewNop()
	originalMockLLM := &mocks.FakeLLMClient{}
	newMockLLM := &mocks.FakeLLMClient{}

	taskHandler := server.NewDefaultTaskHandlerWithLLM(logger, originalMockLLM)

	taskHandler.SetLLMClient(newMockLLM)

	task := &adk.Task{
		ID:        "test-task",
		ContextID: "test-context",
		Status: adk.TaskStatus{
			State: adk.TaskStateSubmitted,
			Message: &adk.Message{
				Kind:      "message",
				MessageID: "test-msg",
				Role:      "user",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Test message",
					},
				},
			},
		},
	}

	newMockLLM.CreateChatCompletionReturns(&adk.Message{
		Kind:      "message",
		MessageID: "response",
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Response from new client",
			},
		},
	}, nil)

	ctx := context.Background()
	message := task.Status.Message
	result, err := taskHandler.HandleTask(ctx, task, message)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, 1, newMockLLM.CreateChatCompletionCallCount())
	assert.Equal(t, 0, originalMockLLM.CreateChatCompletionCallCount())
}
