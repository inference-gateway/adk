package server_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	server "github.com/inference-gateway/adk/server"
	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
	zap "go.uber.org/zap"
)

func TestInMemoryStorage_QueueCentricOperations(t *testing.T) {
	logger := zap.NewNop()

	t.Run("enqueue and dequeue tasks", func(t *testing.T) {
		storage := server.NewInMemoryStorage(logger, 10)
		task := &types.Task{
			ID:        "task-1",
			ContextID: "context-1",
			Kind:      "task",
			Status:    types.TaskStatus{State: types.TaskStateSubmitted},
			History:   []types.Message{},
		}

		// Enqueue task
		err := storage.EnqueueTask(task, "request-1")
		assert.NoError(t, err)

		// Dequeue task
		ctx := context.Background()
		queuedTask, err := storage.DequeueTask(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, queuedTask)
		assert.Equal(t, "task-1", queuedTask.Task.ID)
		assert.Equal(t, "request-1", queuedTask.RequestID)
	})

	t.Run("get and update active tasks", func(t *testing.T) {
		storage := server.NewInMemoryStorage(logger, 10)
		task := &types.Task{
			ID:        "task-2",
			ContextID: "context-2",
			Kind:      "task",
			Status:    types.TaskStatus{State: types.TaskStateWorking},
			History:   []types.Message{},
		}

		// Enqueue task (makes it active)
		err := storage.EnqueueTask(task, "request-2")
		assert.NoError(t, err)

		// Get active task
		retrievedTask, err := storage.GetActiveTask("task-2")
		assert.NoError(t, err)
		assert.Equal(t, "task-2", retrievedTask.ID)

		// Update active task
		task.Status.State = types.TaskStateInputRequired
		err = storage.UpdateActiveTask(task)
		assert.NoError(t, err)

		// Verify update
		updatedTask, err := storage.GetActiveTask("task-2")
		assert.NoError(t, err)
		assert.Equal(t, types.TaskStateInputRequired, updatedTask.Status.State)
	})

	t.Run("dead letter queue functionality", func(t *testing.T) {
		storage := server.NewInMemoryStorage(logger, 10)
		task := &types.Task{
			ID:        "task-3",
			ContextID: "context-3",
			Kind:      "task",
			Status:    types.TaskStatus{State: types.TaskStateCompleted},
			History:   []types.Message{},
		}

		// Store task in dead letter queue
		err := storage.StoreDeadLetterTask(task)
		assert.NoError(t, err)

		// Task should not be in active tasks
		_, err = storage.GetActiveTask("task-3")
		assert.Error(t, err) // Should return error because task is not active

		// Task should be retrievable via GetTask (which checks both active and dead letter)
		retrievedTask, found := storage.GetTask("task-3")
		assert.True(t, found)
		assert.Equal(t, "task-3", retrievedTask.ID)
		assert.Equal(t, types.TaskStateCompleted, retrievedTask.Status.State)
	})

	t.Run("list tasks with filtering", func(t *testing.T) {
		storage := server.NewInMemoryStorage(logger, 10)
		// Create tasks with different states
		workingTask := &types.Task{
			ID:        "working-task",
			ContextID: "context-working",
			Kind:      "task",
			Status:    types.TaskStatus{State: types.TaskStateWorking},
			History:   []types.Message{},
		}

		completedTask := &types.Task{
			ID:        "completed-task",
			ContextID: "context-completed",
			Kind:      "task",
			Status:    types.TaskStatus{State: types.TaskStateCompleted},
			History:   []types.Message{},
		}

		// Enqueue working task
		err := storage.EnqueueTask(workingTask, "request-working")
		assert.NoError(t, err)

		// Store completed task in dead letter queue
		err = storage.StoreDeadLetterTask(completedTask)
		assert.NoError(t, err)

		// List all tasks
		allTasks, err := storage.ListTasks(server.TaskFilter{})
		assert.NoError(t, err)
		assert.Len(t, allTasks, 2)

		// List working tasks
		workingState := types.TaskStateWorking
		workingTasks, err := storage.ListTasks(server.TaskFilter{State: &workingState})
		assert.NoError(t, err)
		assert.Len(t, workingTasks, 1)
		assert.Equal(t, "working-task", workingTasks[0].ID)

		// List completed tasks
		completedState := types.TaskStateCompleted
		completedTasks, err := storage.ListTasks(server.TaskFilter{State: &completedState})
		assert.NoError(t, err)
		assert.Len(t, completedTasks, 1)
		assert.Equal(t, "completed-task", completedTasks[0].ID)
	})

	t.Run("queue management", func(t *testing.T) {
		freshStorage := server.NewInMemoryStorage(logger, 10)

		// Test basic queue operations
		for i := 0; i < 3; i++ {
			task := &types.Task{
				ID:        fmt.Sprintf("queue-task-%d", i),
				ContextID: fmt.Sprintf("context-%d", i),
				Kind:      "task",
				Status:    types.TaskStatus{State: types.TaskStateSubmitted},
				History:   []types.Message{},
			}

			err := freshStorage.EnqueueTask(task, fmt.Sprintf("request-%d", i))
			assert.NoError(t, err)
		}

		// Test queue size
		size := freshStorage.GetQueueLength()
		assert.Equal(t, 3, size)

		// Test dequeue operations
		ctx := context.Background()
		ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		
		// Should dequeue successfully while tasks exist
		for i := 0; i < 3; i++ {
			queuedTask, err := freshStorage.DequeueTask(ctxTimeout)
			assert.NoError(t, err)
			assert.NotNil(t, queuedTask)
		}

		// Should timeout when trying to dequeue from empty queue
		_, err := freshStorage.DequeueTask(ctxTimeout)
		assert.Error(t, err) // Should return timeout error
	})

	t.Run("context management", func(t *testing.T) {
		storage := server.NewInMemoryStorage(logger, 10)
		// Create tasks with different contexts
		task1 := &types.Task{
			ID:        "ctx-task-1",
			ContextID: "context-alpha",
			Kind:      "task",
			Status:    types.TaskStatus{State: types.TaskStateWorking},
			History:   []types.Message{},
		}

		task2 := &types.Task{
			ID:        "ctx-task-2",
			ContextID: "context-beta",
			Kind:      "task",
			Status:    types.TaskStatus{State: types.TaskStateCompleted},
			History:   []types.Message{},
		}

		// Enqueue and store tasks
		err := storage.EnqueueTask(task1, "request-alpha")
		assert.NoError(t, err)

		err = storage.StoreDeadLetterTask(task2)
		assert.NoError(t, err)

		// Get contexts
		contexts := storage.GetContexts()
		assert.Contains(t, contexts, "context-alpha")
		assert.Contains(t, contexts, "context-beta")
		assert.GreaterOrEqual(t, len(contexts), 2)
	})
}
