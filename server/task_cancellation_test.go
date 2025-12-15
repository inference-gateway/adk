package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestTaskCancellation(t *testing.T) {
	logger := zap.NewNop()
	taskManager := NewDefaultTaskManager(logger)

	t.Run("RegisterAndUnregisterTaskCancelFunc", func(t *testing.T) {
		taskID := "test-task-id"
		_, cancel := context.WithCancel(context.Background())

		taskManager.RegisterTaskCancelFunc(taskID, cancel)

		taskManager.runningTasksMu.RLock()
		_, exists := taskManager.runningTasks[taskID]
		taskManager.runningTasksMu.RUnlock()
		assert.True(t, exists, "Cancel function should be registered")

		taskManager.UnregisterTaskCancelFunc(taskID)

		taskManager.runningTasksMu.RLock()
		_, exists = taskManager.runningTasks[taskID]
		taskManager.runningTasksMu.RUnlock()
		assert.False(t, exists, "Cancel function should be unregistered")
	})

	t.Run("CancelTaskActuallyCancelsContext", func(t *testing.T) {
		task := taskManager.CreateTask("test-context", types.TaskStateWorking, &types.Message{
			MessageID: "test-message",
			Role:      "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Test message",
				},
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		taskManager.RegisterTaskCancelFunc(task.ID, cancel)

		var contextCancelled bool
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				contextCancelled = true
			case <-time.After(5 * time.Second):
			}
		}()

		time.Sleep(10 * time.Millisecond)

		err := taskManager.CancelTask(task.ID)
		assert.NoError(t, err, "CancelTask should succeed")

		wg.Wait()

		assert.True(t, contextCancelled, "Context should have been cancelled")

		updatedTask, exists := taskManager.GetTask(task.ID)
		assert.True(t, exists, "Task should exist")
		assert.Equal(t, types.TaskStateCanceled, updatedTask.Status.State, "Task state should be cancelled")

		taskManager.runningTasksMu.RLock()
		_, exists = taskManager.runningTasks[task.ID]
		taskManager.runningTasksMu.RUnlock()
		assert.False(t, exists, "Cancel function should be cleaned up after cancellation")
	})

	t.Run("CancelNonExistentTask", func(t *testing.T) {
		err := taskManager.CancelTask("non-existent-task")
		assert.Error(t, err, "Should return error for non-existent task")
		assert.Contains(t, err.Error(), "task not found", "Error should indicate task not found")
	})

	t.Run("CancelAlreadyCompletedTask", func(t *testing.T) {
		task := taskManager.CreateTask("test-context", types.TaskStateCompleted, &types.Message{
			MessageID: "test-message",
			Role:      "assistant",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Task completed",
				},
			},
		})

		err := taskManager.CancelTask(task.ID)
		assert.Error(t, err, "Should return error when trying to cancel completed task")
		assert.Contains(t, err.Error(), "cannot be canceled", "Error should indicate task cannot be cancelled")
	})

	t.Run("CancelTaskWithoutRegisteredCancelFunc", func(t *testing.T) {
		task := taskManager.CreateTask("test-context", types.TaskStateWorking, &types.Message{
			MessageID: "test-message",
			Role:      "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Test message",
				},
			},
		})

		err := taskManager.CancelTask(task.ID)
		assert.NoError(t, err, "CancelTask should succeed even without registered cancel function")

		updatedTask, exists := taskManager.GetTask(task.ID)
		assert.True(t, exists, "Task should exist")
		assert.Equal(t, types.TaskStateCanceled, updatedTask.Status.State, "Task state should be cancelled")
	})
}
