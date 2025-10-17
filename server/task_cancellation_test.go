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

		// Register cancel function
		taskManager.RegisterTaskCancelFunc(taskID, cancel)

		// Verify that cancel function is registered by checking it exists
		taskManager.runningTasksMu.RLock()
		_, exists := taskManager.runningTasks[taskID]
		taskManager.runningTasksMu.RUnlock()
		assert.True(t, exists, "Cancel function should be registered")

		// Unregister cancel function
		taskManager.UnregisterTaskCancelFunc(taskID)

		// Verify that cancel function is removed
		taskManager.runningTasksMu.RLock()
		_, exists = taskManager.runningTasks[taskID]
		taskManager.runningTasksMu.RUnlock()
		assert.False(t, exists, "Cancel function should be unregistered")
	})

	t.Run("CancelTaskActuallyCancelsContext", func(t *testing.T) {
		// Create a task
		task := taskManager.CreateTask("test-context", types.TaskStateWorking, &types.Message{
			Kind:      "message",
			MessageID: "test-message",
			Role:      "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Test message",
				},
			},
		})

		// Create a cancellable context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Register the cancel function
		taskManager.RegisterTaskCancelFunc(task.ID, cancel)

		// Start a goroutine that waits for context cancellation
		var contextCancelled bool
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				contextCancelled = true
			case <-time.After(5 * time.Second):
				// Timeout - context was not cancelled
			}
		}()

		// Give the goroutine a moment to start
		time.Sleep(10 * time.Millisecond)

		// Cancel the task
		err := taskManager.CancelTask(task.ID)
		assert.NoError(t, err, "CancelTask should succeed")

		// Wait for the goroutine to complete
		wg.Wait()

		// Verify that the context was actually cancelled
		assert.True(t, contextCancelled, "Context should have been cancelled")

		// Verify that the task state is cancelled
		updatedTask, exists := taskManager.GetTask(task.ID)
		assert.True(t, exists, "Task should exist")
		assert.Equal(t, types.TaskStateCanceled, updatedTask.Status.State, "Task state should be cancelled")

		// Verify that cancel function was cleaned up
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
		// Create a completed task
		task := taskManager.CreateTask("test-context", types.TaskStateCompleted, &types.Message{
			Kind:      "message",
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
		// Create a working task without registering a cancel function
		task := taskManager.CreateTask("test-context", types.TaskStateWorking, &types.Message{
			Kind:      "message",
			MessageID: "test-message",
			Role:      "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Test message",
				},
			},
		})

		// This should succeed even without a registered cancel function
		// (handles the case where a task is in the database but not actively running)
		err := taskManager.CancelTask(task.ID)
		assert.NoError(t, err, "CancelTask should succeed even without registered cancel function")

		// Verify that the task state is cancelled
		updatedTask, exists := taskManager.GetTask(task.ID)
		assert.True(t, exists, "Task should exist")
		assert.Equal(t, types.TaskStateCanceled, updatedTask.Status.State, "Task state should be cancelled")
	})
}