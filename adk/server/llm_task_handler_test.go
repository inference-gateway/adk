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

func TestLLMTaskHandler_HandleTask(t *testing.T) {
	tests := []struct {
		name        string
		task        *adk.Task
		message     *adk.Message
		setupMocks  func(*mocks.FakeLLMClient)
		expectError bool
	}{
		{
			name: "successful task with simple response",
			task: &adk.Task{
				ID:        "simple-task",
				ContextID: "test-context",
				Status: adk.TaskStatus{
					State: adk.TaskStateSubmitted,
				},
			},
			message: &adk.Message{
				Kind:      "message",
				MessageID: "user-msg",
				Role:      "user",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Hello, how are you?",
					},
				},
			},
			setupMocks: func(llmClient *mocks.FakeLLMClient) {
				llmClient.CreateChatCompletionReturns(&adk.Message{
					Kind:      "message",
					MessageID: "llm-response",
					Role:      "assistant",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "I'm doing well, thank you!",
						},
					},
				}, nil)
			},
			expectError: false,
		},
		{
			name: "LLM client returns error",
			task: &adk.Task{
				ID:        "error-task",
				ContextID: "test-context",
				Status: adk.TaskStatus{
					State: adk.TaskStateSubmitted,
					Message: &adk.Message{
						Kind:      "message",
						MessageID: "user-msg",
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
			setupMocks: func(llmClient *mocks.FakeLLMClient) {
				llmClient.CreateChatCompletionReturns(nil, assert.AnError)
			},
			expectError: true,
		},
		{
			name: "task with nil message",
			task: &adk.Task{
				ID:        "nil-message-task",
				ContextID: "test-context",
				Status: adk.TaskStatus{
					State:   adk.TaskStateSubmitted,
					Message: nil,
				},
			},
			message: nil,
			setupMocks: func(llmClient *mocks.FakeLLMClient) {
				// Should handle nil message gracefully
				llmClient.CreateChatCompletionReturns(&adk.Message{
					Kind:      "message",
					MessageID: "llm-response",
					Role:      "assistant",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "No message provided",
						},
					},
				}, nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockLLMClient := &mocks.FakeLLMClient{}

			tt.setupMocks(mockLLMClient)

			taskHandler := server.NewLLMTaskHandler(logger, mockLLMClient)

			ctx := context.Background()
			result, err := taskHandler.HandleTask(ctx, tt.task, tt.message)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, adk.TaskStateCompleted, result.Status.State)
			}
		})
	}
}

func TestLLMTaskHandler_HandleTask_WithNilLLMClient(t *testing.T) {
	logger := zap.NewNop()
	taskHandler := server.NewLLMTaskHandler(logger, nil)

	task := &adk.Task{
		ID:        "test-task",
		ContextID: "test-context",
		Status: adk.TaskStatus{
			State: adk.TaskStateSubmitted,
		},
	}

	message := &adk.Message{
		Kind:      "message",
		MessageID: "test-msg",
		Role:      "user",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Test message",
			},
		},
	}

	ctx := context.Background()
	result, err := taskHandler.HandleTask(ctx, task, message)

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, adk.TaskStateFailed, result.Status.State)
}

func TestNewLLMTaskHandler(t *testing.T) {
	logger := zap.NewNop()
	llmClient := &mocks.FakeLLMClient{}

	taskHandler := server.NewLLMTaskHandler(logger, llmClient)

	assert.NotNil(t, taskHandler)
}

func TestNewLLMTaskHandler_WithNilParams(t *testing.T) {
	llmClient := &mocks.FakeLLMClient{}

	taskHandler := server.NewLLMTaskHandler(nil, llmClient)
	assert.NotNil(t, taskHandler)

	logger := zap.NewNop()
	taskHandler = server.NewLLMTaskHandler(logger, nil)
	assert.NotNil(t, taskHandler)
}

func TestLLMTaskHandler_Interface(t *testing.T) {
	logger := zap.NewNop()
	llmClient := &mocks.FakeLLMClient{}

	taskHandler := server.NewLLMTaskHandler(logger, llmClient)

	var _ server.TaskHandler = taskHandler
}
