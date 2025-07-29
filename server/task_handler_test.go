package server_test

import (
	"context"
	"testing"

	server "github.com/inference-gateway/adk/server"
	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
	zap "go.uber.org/zap"
)

func TestDefaultTaskHandler_HandleTask(t *testing.T) {
	tests := []struct {
		name        string
		task        *types.Task
		expectError bool
		expectedMsg string
	}{
		{
			name: "default handler throws error - task with message",
			task: &types.Task{
				ID:        "test-task-1",
				ContextID: "test-context",
				Status: types.TaskStatus{
					State: types.TaskStateSubmitted,
					Message: &types.Message{
						Kind:      "message",
						MessageID: "test-msg",
						Role:      "user",
						Parts: []types.Part{
							map[string]interface{}{
								"kind": "text",
								"text": "Hello from task",
							},
						},
					},
				},
			},
			expectError: true,
			expectedMsg: "no task handler configured",
		},
		{
			name: "default handler throws error - task with nil message",
			task: &types.Task{
				ID:        "test-task-2",
				ContextID: "test-context",
				Status: types.TaskStatus{
					State:   types.TaskStateSubmitted,
					Message: nil,
				},
			},
			expectError: true,
			expectedMsg: "no task handler configured",
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
				assert.Contains(t, err.Error(), tt.expectedMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
