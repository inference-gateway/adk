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
			Status:    types.TaskStatus{State: string(types.TaskStateSubmitted)},
			History:   []types.Message{},
		}

		err := storage.EnqueueTask(task, "request-1")
		assert.NoError(t, err)

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
			Status:    types.TaskStatus{State: string(types.TaskStateWorking)},
			History:   []types.Message{},
		}

		err := storage.EnqueueTask(task, "request-2")
		assert.NoError(t, err)

		retrievedTask, err := storage.GetActiveTask("task-2")
		assert.NoError(t, err)
		assert.Equal(t, "task-2", retrievedTask.ID)

		task.Status.State = string(types.TaskStateInputRequired)
		err = storage.UpdateActiveTask(task)
		assert.NoError(t, err)

		updatedTask, err := storage.GetActiveTask("task-2")
		assert.NoError(t, err)
		assert.Equal(t, string(types.TaskStateInputRequired), updatedTask.Status.State)
	})

	t.Run("dead letter queue functionality", func(t *testing.T) {
		storage := server.NewInMemoryStorage(logger, 10)
		task := &types.Task{
			ID:        "task-3",
			ContextID: "context-3",
			Status:    types.TaskStatus{State: string(types.TaskStateCompleted)},
			History:   []types.Message{},
		}

		err := storage.StoreDeadLetterTask(task)
		assert.NoError(t, err)

		_, err = storage.GetActiveTask("task-3")
		assert.Error(t, err)

		retrievedTask, found := storage.GetTask("task-3")
		assert.True(t, found)
		assert.Equal(t, "task-3", retrievedTask.ID)
		assert.Equal(t, string(types.TaskStateCompleted), retrievedTask.Status.State)
	})

	t.Run("list tasks with filtering", func(t *testing.T) {
		storage := server.NewInMemoryStorage(logger, 10)
		workingTask := &types.Task{
			ID:        "working-task",
			ContextID: "context-working",
			Status:    types.TaskStatus{State: string(types.TaskStateWorking)},
			History:   []types.Message{},
		}

		completedTask := &types.Task{
			ID:        "completed-task",
			ContextID: "context-completed",
			Status:    types.TaskStatus{State: string(types.TaskStateCompleted)},
			History:   []types.Message{},
		}

		err := storage.EnqueueTask(workingTask, "request-working")
		assert.NoError(t, err)

		err = storage.StoreDeadLetterTask(completedTask)
		assert.NoError(t, err)

		allTasks, err := storage.ListTasks(server.TaskFilter{})
		assert.NoError(t, err)
		assert.Len(t, allTasks, 2)

		workingState := types.TaskStateWorking
		workingTasks, err := storage.ListTasks(server.TaskFilter{State: &workingState})
		assert.NoError(t, err)
		assert.Len(t, workingTasks, 1)
		assert.Equal(t, "working-task", workingTasks[0].ID)

		completedState := types.TaskStateCompleted
		completedTasks, err := storage.ListTasks(server.TaskFilter{State: &completedState})
		assert.NoError(t, err)
		assert.Len(t, completedTasks, 1)
		assert.Equal(t, "completed-task", completedTasks[0].ID)
	})

	t.Run("queue management", func(t *testing.T) {
		freshStorage := server.NewInMemoryStorage(logger, 10)

		for i := 0; i < 3; i++ {
			task := &types.Task{
				ID:        fmt.Sprintf("queue-task-%d", i),
				ContextID: fmt.Sprintf("context-%d", i),
				Status:    types.TaskStatus{State: string(types.TaskStateSubmitted)},
				History:   []types.Message{},
			}

			err := freshStorage.EnqueueTask(task, fmt.Sprintf("request-%d", i))
			assert.NoError(t, err)
		}

		size := freshStorage.GetQueueLength()
		assert.Equal(t, 3, size)

		ctx := context.Background()
		ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		for i := 0; i < 3; i++ {
			queuedTask, err := freshStorage.DequeueTask(ctxTimeout)
			assert.NoError(t, err)
			assert.NotNil(t, queuedTask)
		}

		_, err := freshStorage.DequeueTask(ctxTimeout)
		assert.Error(t, err)
	})

	t.Run("context management", func(t *testing.T) {
		storage := server.NewInMemoryStorage(logger, 10)
		task1 := &types.Task{
			ID:        "ctx-task-1",
			ContextID: "context-alpha",
			Status:    types.TaskStatus{State: string(types.TaskStateWorking)},
			History:   []types.Message{},
		}

		task2 := &types.Task{
			ID:        "ctx-task-2",
			ContextID: "context-beta",
			Status:    types.TaskStatus{State: string(types.TaskStateCompleted)},
			History:   []types.Message{},
		}

		err := storage.EnqueueTask(task1, "request-alpha")
		assert.NoError(t, err)

		err = storage.StoreDeadLetterTask(task2)
		assert.NoError(t, err)

		contexts := storage.GetContexts()
		assert.Contains(t, contexts, "context-alpha")
		assert.Contains(t, contexts, "context-beta")
		assert.GreaterOrEqual(t, len(contexts), 2)
	})
}
