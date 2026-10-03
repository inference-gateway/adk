package server

import (
	"testing"

	assert "github.com/stretchr/testify/assert"

	types "github.com/inference-gateway/adk/types"
)

func TestTaskInputMessage(t *testing.T) {
	statusMessage := &types.Message{
		MessageID: "existing-message",
		Role:      types.RoleUser,
		Parts:     []types.Part{types.CreateTextPart("hello")},
	}

	tests := []struct {
		name string
		task *types.Task
		want *types.Message
	}{
		{
			name: "returns the status message when present",
			task: &types.Task{ID: "task-1", Status: types.TaskStatus{Message: statusMessage}},
			want: statusMessage,
		},
		{
			name: "returns a placeholder when the task has no status message",
			task: &types.Task{ID: "task-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := taskInputMessage(tt.task)

			if tt.want != nil {
				assert.Same(t, tt.want, message)
				return
			}

			assert.NotEmpty(t, message.MessageID)
			assert.Empty(t, message.Parts)
			assert.Equal(t, types.RoleUser, message.Role)
			assert.True(t, message.Role.Valid())
		})
	}
}
