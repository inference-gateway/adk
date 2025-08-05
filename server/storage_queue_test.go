package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestInMemoryStorage_QueueOperations(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewInMemoryStorage(logger, 10)

	// Create a test task
	task := &types.Task{
		ID:        "test-task-1",
		ContextID: "test-context",
		Status: types.TaskStatus{
			State: types.TaskStateSubmitted,
		},
	}

	t.Run("Enqueue and Dequeue Task", func(t *testing.T) {
		err := storage.EnqueueTask(task, "request-123")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if length := storage.GetQueueLength(); length != 1 {
			t.Fatalf("expected queue length 1, got %d", length)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		queuedTask, err := storage.DequeueTask(ctx)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if queuedTask.Task.ID != task.ID {
			t.Fatalf("expected task ID %s, got %s", task.ID, queuedTask.Task.ID)
		}

		if queuedTask.RequestID != "request-123" {
			t.Fatalf("expected request ID 'request-123', got %v", queuedTask.RequestID)
		}

		if length := storage.GetQueueLength(); length != 0 {
			t.Fatalf("expected queue length 0, got %d", length)
		}
	})

	t.Run("FIFO Ordering", func(t *testing.T) {
		task1 := &types.Task{ID: "task-1", ContextID: "ctx-1", Status: types.TaskStatus{State: types.TaskStateSubmitted}}
		task2 := &types.Task{ID: "task-2", ContextID: "ctx-2", Status: types.TaskStatus{State: types.TaskStateSubmitted}}
		task3 := &types.Task{ID: "task-3", ContextID: "ctx-3", Status: types.TaskStatus{State: types.TaskStateSubmitted}}

		err := storage.EnqueueTask(task1, "req-1")
		require.NoError(t, err)
		err = storage.EnqueueTask(task2, "req-2")
		require.NoError(t, err)
		err = storage.EnqueueTask(task3, "req-3")
		require.NoError(t, err)

		if length := storage.GetQueueLength(); length != 3 {
			t.Fatalf("expected queue length 3, got %d", length)
		}

		ctx := context.Background()

		queuedTask1, _ := storage.DequeueTask(ctx)
		queuedTask2, _ := storage.DequeueTask(ctx)
		queuedTask3, _ := storage.DequeueTask(ctx)

		if queuedTask1.Task.ID != "task-1" {
			t.Fatalf("expected first task to be 'task-1', got %s", queuedTask1.Task.ID)
		}
		if queuedTask2.Task.ID != "task-2" {
			t.Fatalf("expected second task to be 'task-2', got %s", queuedTask2.Task.ID)
		}
		if queuedTask3.Task.ID != "task-3" {
			t.Fatalf("expected third task to be 'task-3', got %s", queuedTask3.Task.ID)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := storage.DequeueTask(ctx)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatalf("expected context cancellation error, got nil")
		}

		if elapsed < 90*time.Millisecond || elapsed > 200*time.Millisecond {
			t.Fatalf("expected timeout around 100ms, got %v", elapsed)
		}
	})

	t.Run("Clear Queue", func(t *testing.T) {
		task1 := &types.Task{ID: "clear-task-1", ContextID: "ctx", Status: types.TaskStatus{State: types.TaskStateSubmitted}}
		task2 := &types.Task{ID: "clear-task-2", ContextID: "ctx", Status: types.TaskStatus{State: types.TaskStateSubmitted}}

		err := storage.EnqueueTask(task1, "req-1")
		require.NoError(t, err)
		err = storage.EnqueueTask(task2, "req-2")
		require.NoError(t, err)

		if length := storage.GetQueueLength(); length != 2 {
			t.Fatalf("expected queue length 2, got %d", length)
		}

		err = storage.ClearQueue()
		require.NoError(t, err)

		if length := storage.GetQueueLength(); length != 0 {
			t.Fatalf("expected queue length 0 after clear, got %d", length)
		}
	})

	t.Run("Enqueue Nil Task", func(t *testing.T) {
		err := storage.EnqueueTask(nil, "req-123")
		if err == nil {
			t.Fatalf("expected error for nil task, got nil")
		}
	})
}

func TestInMemoryStorage_ConcurrentQueueOperations(t *testing.T) {
	logger := zaptest.NewLogger(t)
	storage := NewInMemoryStorage(logger, 10)

	t.Run("Concurrent Enqueue and Dequeue", func(t *testing.T) {
		const numWorkers = 5
		const tasksPerWorker = 10

		results := make(chan *QueuedTask, numWorkers*tasksPerWorker)
		ctx := context.Background()

		for i := 0; i < numWorkers; i++ {
			go func() {
				for j := 0; j < tasksPerWorker; j++ {
					queuedTask, err := storage.DequeueTask(ctx)
					if err != nil {
						t.Errorf("dequeue error: %v", err)
						return
					}
					results <- queuedTask
				}
			}()
		}

		go func() {
			for i := 0; i < numWorkers; i++ {
				for j := 0; j < tasksPerWorker; j++ {
					task := &types.Task{
						ID:        fmt.Sprintf("task-%d-%d", i, j),
						ContextID: "concurrent-test",
						Status:    types.TaskStatus{State: types.TaskStateSubmitted},
					}
					err := storage.EnqueueTask(task, fmt.Sprintf("req-%d-%d", i, j))
					if err != nil {
						t.Errorf("enqueue error: %v", err)
						return
					}
				}
			}
		}()

		receivedTasks := make(map[string]bool)
		for i := 0; i < numWorkers*tasksPerWorker; i++ {
			select {
			case queuedTask := <-results:
				receivedTasks[queuedTask.Task.ID] = true
			case <-time.After(5 * time.Second):
				t.Fatalf("timeout waiting for task %d", i)
			}
		}

		if len(receivedTasks) != numWorkers*tasksPerWorker {
			t.Fatalf("expected %d unique tasks, got %d", numWorkers*tasksPerWorker, len(receivedTasks))
		}

		if length := storage.GetQueueLength(); length != 0 {
			t.Fatalf("expected queue length 0, got %d", length)
		}
	})
}
