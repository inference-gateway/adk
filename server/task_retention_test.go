package server

import (
	"context"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	zap "go.uber.org/zap"

	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

// TestStartTaskProcessor_AppliesTaskRetention verifies that finished tasks are
// only trimmed according to TASK_RETENTION_* and not purged wholesale on every
// QUEUE_CLEANUP_INTERVAL tick (issue #317).
func TestStartTaskProcessor_AppliesTaskRetention(t *testing.T) {
	cfg := &serverConfig.Config{
		QueueConfig: serverConfig.QueueConfig{
			MaxSize:         10,
			CleanupInterval: 10 * time.Millisecond,
		},
		TaskRetentionConfig: serverConfig.TaskRetentionConfig{
			MaxCompletedTasks: 2,
			MaxFailedTasks:    50,
			CleanupInterval:   10 * time.Millisecond,
		},
	}

	srv := NewA2AServer(cfg, zap.NewNop(), nil)

	for _, id := range []string{"task-1", "task-2", "task-3"} {
		now := time.Now()
		assert.NoError(t, srv.storage.StoreDeadLetterTask(&types.Task{
			ID:        id,
			ContextID: "ctx-1",
			Status: types.TaskStatus{
				State:     types.TaskStateCompleted,
				Timestamp: &now,
			},
		}))
		time.Sleep(time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.StartTaskProcessor(ctx)

	time.Sleep(150 * time.Millisecond)

	tasks, err := srv.storage.ListTasks(TaskFilter{})
	assert.NoError(t, err)
	assert.Len(t, tasks, cfg.TaskRetentionConfig.MaxCompletedTasks, "retention should keep the most recent completed tasks")

	_, exists := srv.storage.GetTask("task-3")
	assert.True(t, exists, "most recent completed task must stay retrievable")
}
