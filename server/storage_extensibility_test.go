package server

import (
	"context"
	"testing"

	"github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestQueueCentricOperations(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "EnqueueTask and GetActiveTask",
			test: func(t *testing.T) {
				logger := zap.NewNop()
				storage := NewInMemoryStorage(logger, 100)
				testCtx := "test-context-enqueue"

				task := &types.Task{
					ID:        "task-1",
					ContextID: testCtx,
					Kind:      "task",
					Status: types.TaskStatus{
						State: types.TaskStateSubmitted,
					},
				}

				err := storage.EnqueueTask(task, "request-1")
				require.NoError(t, err)

				err = storage.UpdateActiveTask(task)
				require.NoError(t, err)

				retrievedTask, err := storage.GetActiveTask(task.ID)
				require.NoError(t, err)
				assert.Equal(t, task.ID, retrievedTask.ID)
				assert.Equal(t, task.Status.State, retrievedTask.Status.State)
			},
		},
		{
			name: "StoreDeadLetterTask",
			test: func(t *testing.T) {
				logger := zap.NewNop()
				storage := NewInMemoryStorage(logger, 100)
				testCtx := "test-context-dead"

				task := &types.Task{
					ID:        "task-2",
					ContextID: testCtx,
					Kind:      "task",
					Status: types.TaskStatus{
						State: types.TaskStateCompleted,
					},
				}

				err := storage.StoreDeadLetterTask(task)
				require.NoError(t, err)

				// Verify task is stored in dead letter
				tasks, err := storage.ListTasksByContext(task.ContextID, TaskFilter{})
				require.NoError(t, err)
				assert.Len(t, tasks, 1)
				assert.Equal(t, task.ID, tasks[0].ID)
			},
		},
		{
			name: "DequeueTask",
			test: func(t *testing.T) {
				logger := zap.NewNop()
				storage := NewInMemoryStorage(logger, 100)
				ctx := context.Background()
				testContext := "test-context-dequeue"

				task := &types.Task{
					ID:        "task-3",
					ContextID: testContext,
					Kind:      "task",
					Status: types.TaskStatus{
						State: types.TaskStateSubmitted,
					},
				}

				err := storage.EnqueueTask(task, "request-3")
				require.NoError(t, err)

				err = storage.UpdateActiveTask(task)
				require.NoError(t, err)

				queuedTask, err := storage.DequeueTask(ctx)
				require.NoError(t, err)
				assert.Equal(t, task.ID, queuedTask.Task.ID)

				// Verify task is no longer active (queue should be empty)
				length := storage.GetQueueLength()
				assert.Equal(t, 0, length)
			},
		},
		{
			name: "GetQueueLength",
			test: func(t *testing.T) {
				logger := zap.NewNop()
				storage := NewInMemoryStorage(logger, 100)

				length := storage.GetQueueLength()
				assert.Equal(t, 0, length)

				task := &types.Task{
					ID:        "task-4",
					ContextID: "test-context-length",
					Kind:      "task",
					Status: types.TaskStatus{
						State: types.TaskStateSubmitted,
					},
				}

				err := storage.EnqueueTask(task, "request-4")
				require.NoError(t, err)

				length = storage.GetQueueLength()
				assert.Equal(t, 1, length)
			},
		},
		{
			name: "ListTasksByContext",
			test: func(t *testing.T) {
				logger := zap.NewNop()
				storage := NewInMemoryStorage(logger, 100)
				testContext := "test-context-list"

				task1 := &types.Task{
					ID:        "task-5",
					ContextID: testContext,
					Kind:      "task",
					Status: types.TaskStatus{
						State: types.TaskStateCompleted,
					},
				}
				task2 := &types.Task{
					ID:        "task-6",
					ContextID: testContext,
					Kind:      "task",
					Status: types.TaskStatus{
						State: types.TaskStateCompleted,
					},
				}

				err := storage.StoreDeadLetterTask(task1)
				require.NoError(t, err)

				err = storage.StoreDeadLetterTask(task2)
				require.NoError(t, err)

				tasks, err := storage.ListTasksByContext(testContext, TaskFilter{})
				require.NoError(t, err)
				assert.Len(t, tasks, 2)

				taskIDs := []string{tasks[0].ID, tasks[1].ID}
				assert.Contains(t, taskIDs, task1.ID)
				assert.Contains(t, taskIDs, task2.ID)
			},
		},
		{
			name: "DeleteContextAndTasks",
			test: func(t *testing.T) {
				logger := zap.NewNop()
				storage := NewInMemoryStorage(logger, 100)
				testContext := "test-context-delete"

				// Create active task
				activeTask := &types.Task{
					ID:        "active-task",
					ContextID: testContext,
					Kind:      "task",
					Status: types.TaskStatus{
						State: types.TaskStateSubmitted,
					},
				}
				err := storage.EnqueueTask(activeTask, "request-active")
				require.NoError(t, err)
				err = storage.UpdateActiveTask(activeTask)
				require.NoError(t, err)

				// Create dead letter task
				deadTask := &types.Task{
					ID:        "dead-task",
					ContextID: testContext,
					Kind:      "task",
					Status: types.TaskStatus{
						State: types.TaskStateCompleted,
					},
				}
				err = storage.StoreDeadLetterTask(deadTask)
				require.NoError(t, err)

				// Verify both tasks exist
				_, err = storage.GetActiveTask(activeTask.ID)
				assert.NoError(t, err)

				tasks, err := storage.ListTasksByContext(testContext, TaskFilter{})
				require.NoError(t, err)
				assert.Len(t, tasks, 1)

				// Delete context and tasks
				err = storage.DeleteContextAndTasks(testContext)
				require.NoError(t, err)

				// Verify both tasks are deleted
				_, err = storage.GetActiveTask(activeTask.ID)
				assert.Error(t, err)

				tasks, err = storage.ListTasksByContext(testContext, TaskFilter{})
				require.NoError(t, err)
				assert.Len(t, tasks, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}
