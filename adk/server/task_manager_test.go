package server_test

import (
	"testing"
	"time"

	"github.com/inference-gateway/a2a/adk"
	"github.com/inference-gateway/a2a/adk/server"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultTaskManager_CreateTask(t *testing.T) {
	tests := []struct {
		name      string
		contextID string
		state     adk.TaskState
		message   *adk.Message
	}{
		{
			name:      "create task with submitted state",
			contextID: "context-1",
			state:     adk.TaskStateSubmitted,
			message: &adk.Message{
				Kind:      "message",
				MessageID: "msg-1",
				Role:      "user",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Test message",
					},
				},
			},
		},
		{
			name:      "create task with working state",
			contextID: "context-2",
			state:     adk.TaskStateWorking,
			message: &adk.Message{
				Kind:      "message",
				MessageID: "msg-2",
				Role:      "assistant",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Processing...",
					},
				},
			},
		},
		{
			name:      "create task with completed state",
			contextID: "context-3",
			state:     adk.TaskStateCompleted,
			message: &adk.Message{
				Kind:      "message",
				MessageID: "msg-3",
				Role:      "assistant",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Task completed",
					},
				},
			},
		},
		{
			name:      "create task with failed state",
			contextID: "context-4",
			state:     adk.TaskStateFailed,
			message: &adk.Message{
				Kind:      "message",
				MessageID: "msg-4",
				Role:      "assistant",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Task failed",
					},
				},
			},
		},
		{
			name:      "create task with nil message",
			contextID: "context-5",
			state:     adk.TaskStateSubmitted,
			message:   nil,
		},
		{
			name:      "create task with empty context",
			contextID: "",
			state:     adk.TaskStateSubmitted,
			message: &adk.Message{
				Kind:      "message",
				MessageID: "msg-6",
				Role:      "user",
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

			if task.Status.Timestamp != nil {
				timestamp, err := time.Parse(time.RFC3339Nano, *task.Status.Timestamp)
				assert.NoError(t, err)
				assert.WithinDuration(t, time.Now(), timestamp, time.Second)
			}
		})
	}
}

func TestDefaultTaskManager_GetTask(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger)

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

	createdTask := taskManager.CreateTask("test-context", adk.TaskStateSubmitted, message)

	retrievedTask, exists := taskManager.GetTask(createdTask.ID)
	assert.True(t, exists)
	assert.NotNil(t, retrievedTask)
	assert.Equal(t, createdTask.ID, retrievedTask.ID)
	assert.Equal(t, createdTask.ContextID, retrievedTask.ContextID)
	assert.Equal(t, createdTask.Status.State, retrievedTask.Status.State)

	nonExistentTask, exists := taskManager.GetTask("non-existent-id")
	assert.False(t, exists)
	assert.Nil(t, nonExistentTask)

	emptyTask, exists := taskManager.GetTask("")
	assert.False(t, exists)
	assert.Nil(t, emptyTask)
}

func TestDefaultTaskManager_UpdateTask(t *testing.T) {
	tests := []struct {
		name        string
		newState    adk.TaskState
		newMessage  *adk.Message
		expectError bool
	}{
		{
			name:     "update to working state",
			newState: adk.TaskStateWorking,
			newMessage: &adk.Message{
				Kind:      "message",
				MessageID: "updated-msg-1",
				Role:      "assistant",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Now working on the task",
					},
				},
			},
			expectError: false,
		},
		{
			name:     "update to completed state",
			newState: adk.TaskStateCompleted,
			newMessage: &adk.Message{
				Kind:      "message",
				MessageID: "updated-msg-2",
				Role:      "assistant",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Task completed successfully",
					},
				},
			},
			expectError: false,
		},
		{
			name:     "update to failed state",
			newState: adk.TaskStateFailed,
			newMessage: &adk.Message{
				Kind:      "message",
				MessageID: "updated-msg-3",
				Role:      "assistant",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Task failed with error",
					},
				},
			},
			expectError: false,
		},
		{
			name:        "update with nil message",
			newState:    adk.TaskStateWorking,
			newMessage:  nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			taskManager := server.NewDefaultTaskManager(logger)

			originalMessage := &adk.Message{
				Kind:      "message",
				MessageID: "original-msg",
				Role:      "user",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Original message",
					},
				},
			}

			task := taskManager.CreateTask("test-context", adk.TaskStateSubmitted, originalMessage)
			originalTimestamp := task.Status.Timestamp

			err := taskManager.UpdateTask(task.ID, tt.newState, tt.newMessage)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				updatedTask, exists := taskManager.GetTask(task.ID)
				assert.True(t, exists)
				assert.Equal(t, tt.newState, updatedTask.Status.State)
				assert.Equal(t, tt.newMessage, updatedTask.Status.Message)

				if originalTimestamp != nil && updatedTask.Status.Timestamp != nil {
					originalTime, err := time.Parse(time.RFC3339Nano, *originalTimestamp)
					assert.NoError(t, err)
					updatedTime, err := time.Parse(time.RFC3339Nano, *updatedTask.Status.Timestamp)
					assert.NoError(t, err)
					assert.True(t, updatedTime.After(originalTime))
				}
			}
		})
	}
}

func TestDefaultTaskManager_UpdateNonExistentTask(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger)

	message := &adk.Message{
		Kind:      "message",
		MessageID: "test-msg",
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Update message",
			},
		},
	}

	err := taskManager.UpdateTask("non-existent-id", adk.TaskStateCompleted, message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")
}

func TestDefaultTaskManager_CleanupCompletedTasks(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger)

	submittedTask := taskManager.CreateTask("context-1", adk.TaskStateSubmitted, nil)
	workingTask := taskManager.CreateTask("context-2", adk.TaskStateWorking, nil)
	completedTask := taskManager.CreateTask("context-3", adk.TaskStateCompleted, nil)
	failedTask := taskManager.CreateTask("context-4", adk.TaskStateFailed, nil)

	_, exists := taskManager.GetTask(submittedTask.ID)
	assert.True(t, exists)
	_, exists = taskManager.GetTask(workingTask.ID)
	assert.True(t, exists)
	_, exists = taskManager.GetTask(completedTask.ID)
	assert.True(t, exists)
	_, exists = taskManager.GetTask(failedTask.ID)
	assert.True(t, exists)

	taskManager.CleanupCompletedTasks()

	_, exists = taskManager.GetTask(submittedTask.ID)
	assert.True(t, exists, "submitted task should remain")
	_, exists = taskManager.GetTask(workingTask.ID)
	assert.True(t, exists, "working task should remain")
	_, exists = taskManager.GetTask(completedTask.ID)
	assert.False(t, exists, "completed task should be cleaned up")
	_, exists = taskManager.GetTask(failedTask.ID)
	assert.False(t, exists, "failed task should be cleaned up")
}

func TestDefaultTaskManager_ConcurrentAccess(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger)

	numGoroutines := 10
	tasksChan := make(chan *adk.Task, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			message := &adk.Message{
				Kind:      "message",
				MessageID: "concurrent-msg",
				Role:      "user",
			}
			task := taskManager.CreateTask("concurrent-context", adk.TaskStateSubmitted, message)
			tasksChan <- task
		}(i)
	}

	createdTasks := make([]*adk.Task, 0, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		task := <-tasksChan
		createdTasks = append(createdTasks, task)
	}

	taskIDs := make(map[string]bool)
	for _, task := range createdTasks {
		assert.NotEmpty(t, task.ID)
		assert.False(t, taskIDs[task.ID], "Task ID should be unique: %s", task.ID)
		taskIDs[task.ID] = true
	}

	assert.Len(t, taskIDs, numGoroutines, "All tasks should have unique IDs")
}

func TestNewDefaultTaskManager(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger)

	assert.NotNil(t, taskManager)
}

func TestNewDefaultTaskManager_WithNilLogger(t *testing.T) {
	assert.NotPanics(t, func() {
		taskManager := server.NewDefaultTaskManager(nil)
		assert.NotNil(t, taskManager)
	})
}
