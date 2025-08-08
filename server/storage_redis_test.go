package server

import (
	"context"
	"testing"
	"time"

	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/types"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Helper function to get Redis URL for testing
func getTestRedisURL() string {
	testURLs := []string{
		"redis://localhost:6379/15",
		"redis://127.0.0.1:6379/15",
	}

	for _, url := range testURLs {
		opt, err := redis.ParseURL(url)
		if err != nil {
			continue
		}
		client := redis.NewClient(opt)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err = client.Ping(ctx).Err()
		cancel()
		_ = client.Close()
		if err == nil {
			return url
		}
	}

	return ""
}

// Helper function to skip test if Redis is not available
func requireRedis(t *testing.T) string {
	url := getTestRedisURL()
	if url == "" {
		t.Skip("Redis not available for integration tests")
	}
	return url
}

// Helper function to clean up Redis test data
func cleanupRedisTestData(t *testing.T, url string) {
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)

	client := redis.NewClient(opt)
	defer func() { _ = client.Close() }()

	err = client.FlushDB(context.Background()).Err()
	require.NoError(t, err)
}

func TestRedisStorageFactory(t *testing.T) {
	factory := &RedisStorageFactory{}

	assert.Equal(t, "redis", factory.SupportedProvider())

	cfg := config.QueueConfig{
		Provider: "redis",
	}
	err := factory.ValidateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "URL is required")

	cfg.URL = "redis://localhost:6379"
	err = factory.ValidateConfig(cfg)
	assert.NoError(t, err)
}

func TestRedisStorageFactoryCreateStorage(t *testing.T) {
	url := requireRedis(t)
	defer cleanupRedisTestData(t, url)

	factory := &RedisStorageFactory{}
	logger := zaptest.NewLogger(t)

	cfg := config.QueueConfig{
		Provider: "redis",
		URL:      url,
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, storage)

	redisStorage, ok := storage.(*RedisStorage)
	assert.True(t, ok)
	assert.NotNil(t, redisStorage.client)
}

func TestRedisStorageFactoryWithOptions(t *testing.T) {
	url := requireRedis(t)
	defer cleanupRedisTestData(t, url)

	factory := &RedisStorageFactory{}
	logger := zaptest.NewLogger(t)

	cfg := config.QueueConfig{
		Provider: "redis",
		URL:      url,
		Options: map[string]string{
			"db":          "15",
			"max_retries": "5",
			"timeout":     "10s",
		},
		Credentials: map[string]string{
			"username": "test",
			"password": "test123",
		},
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	if err != nil {
		t.Logf("Expected auth error for test Redis: %v", err)
		return
	}

	require.NoError(t, err)
	assert.NotNil(t, storage)
}

func TestRedisStorageFactoryWithInvalidURL(t *testing.T) {
	factory := &RedisStorageFactory{}
	logger := zaptest.NewLogger(t)

	cfg := config.QueueConfig{
		Provider: "redis",
		URL:      "invalid-url",
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	assert.Error(t, err)
	assert.Nil(t, storage)
	assert.Contains(t, err.Error(), "invalid Redis URL")
}

func TestRedisStorageBasicOperations(t *testing.T) {
	url := requireRedis(t)
	defer cleanupRedisTestData(t, url)

	logger := zaptest.NewLogger(t)
	factory := &RedisStorageFactory{}

	cfg := config.QueueConfig{
		Provider: "redis",
		URL:      url,
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)

	task := &types.Task{
		ID:        "test-task-1",
		ContextID: "test-context",
		Kind:      "task",
		Status: types.TaskStatus{
			State: types.TaskStateSubmitted,
		},
		History: []types.Message{},
	}

	err = storage.EnqueueTask(task, "request-123")
	require.NoError(t, err)

	assert.Equal(t, 1, storage.GetQueueLength())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	queuedTask, err := storage.DequeueTask(ctx)
	require.NoError(t, err)
	assert.Equal(t, task.ID, queuedTask.Task.ID)
	assert.Equal(t, "request-123", queuedTask.RequestID)

	assert.Equal(t, 0, storage.GetQueueLength())
}

func TestRedisStorageActiveTaskOperations(t *testing.T) {
	url := requireRedis(t)
	defer cleanupRedisTestData(t, url)

	logger := zaptest.NewLogger(t)
	factory := &RedisStorageFactory{}

	cfg := config.QueueConfig{
		Provider: "redis",
		URL:      url,
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)

	task := &types.Task{
		ID:        "test-task-2",
		ContextID: "test-context",
		Kind:      "task",
		Status: types.TaskStatus{
			State: types.TaskStateSubmitted,
		},
		History: []types.Message{},
	}

	err = storage.CreateActiveTask(task)
	require.NoError(t, err)

	retrieved, err := storage.GetActiveTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, retrieved.ID)
	assert.Equal(t, task.ContextID, retrieved.ContextID)

	task.Status.State = types.TaskStateWorking
	err = storage.UpdateActiveTask(task)
	require.NoError(t, err)

	retrieved, err = storage.GetActiveTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, types.TaskStateWorking, retrieved.Status.State)

	err = storage.CreateActiveTask(task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	_, err = storage.GetActiveTask("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRedisStorageDeadLetterOperations(t *testing.T) {
	url := requireRedis(t)
	defer cleanupRedisTestData(t, url)

	logger := zaptest.NewLogger(t)
	factory := &RedisStorageFactory{}

	cfg := config.QueueConfig{
		Provider: "redis",
		URL:      url,
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)

	task := &types.Task{
		ID:        "test-task-3",
		ContextID: "test-context",
		Kind:      "task",
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
		},
		History: []types.Message{
			{
				Role:      "user",
				Kind:      "message",
				MessageID: "msg-1",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "test message",
					},
				},
			},
		},
	}

	err = storage.StoreDeadLetterTask(task)
	require.NoError(t, err)

	retrieved, exists := storage.GetTask(task.ID)
	assert.True(t, exists)
	assert.Equal(t, task.ID, retrieved.ID)
	assert.Equal(t, task.ContextID, retrieved.ContextID)
	assert.Equal(t, types.TaskStateCompleted, retrieved.Status.State)
	assert.Len(t, retrieved.History, 1)

	retrieved, exists = storage.GetTaskByContextAndID(task.ContextID, task.ID)
	assert.True(t, exists)
	assert.Equal(t, task.ID, retrieved.ID)

	_, exists = storage.GetTaskByContextAndID("wrong-context", task.ID)
	assert.False(t, exists)

	err = storage.DeleteTask(task.ID)
	require.NoError(t, err)

	_, exists = storage.GetTask(task.ID)
	assert.False(t, exists)
}

func TestRedisStorageListOperations(t *testing.T) {
	url := requireRedis(t)
	defer cleanupRedisTestData(t, url)

	logger := zaptest.NewLogger(t)
	factory := &RedisStorageFactory{}

	cfg := config.QueueConfig{
		Provider: "redis",
		URL:      url,
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)

	tasks := []*types.Task{
		{
			ID:        "task-1",
			ContextID: "context-1",
			Kind:      "task",
			Status:    types.TaskStatus{State: types.TaskStateCompleted},
			History:   []types.Message{{Role: "user", Kind: "message", MessageID: "msg1", Parts: []types.Part{map[string]interface{}{"kind": "text", "text": "msg1"}}}},
		},
		{
			ID:        "task-2",
			ContextID: "context-1",
			Kind:      "task",
			Status:    types.TaskStatus{State: types.TaskStateFailed},
			History:   []types.Message{{Role: "user", Kind: "message", MessageID: "msg2", Parts: []types.Part{map[string]interface{}{"kind": "text", "text": "msg2"}}}},
		},
		{
			ID:        "task-3",
			ContextID: "context-2",
			Kind:      "task",
			Status:    types.TaskStatus{State: types.TaskStateCompleted},
			History:   []types.Message{{Role: "user", Kind: "message", MessageID: "msg3", Parts: []types.Part{map[string]interface{}{"kind": "text", "text": "msg3"}}}},
		},
	}

	for _, task := range tasks {
		err = storage.StoreDeadLetterTask(task)
		require.NoError(t, err)
	}

	allTasks, err := storage.ListTasks(TaskFilter{})
	require.NoError(t, err)
	assert.Len(t, allTasks, 3)

	completedState := types.TaskStateCompleted
	completedTasks, err := storage.ListTasks(TaskFilter{State: &completedState})
	require.NoError(t, err)
	assert.Len(t, completedTasks, 2)

	context1 := "context-1"
	context1Tasks, err := storage.ListTasks(TaskFilter{ContextID: &context1})
	require.NoError(t, err)
	assert.Len(t, context1Tasks, 2)

	context1TasksByContext, err := storage.ListTasksByContext("context-1", TaskFilter{})
	require.NoError(t, err)
	assert.Len(t, context1TasksByContext, 2)

	context2TasksByContext, err := storage.ListTasksByContext("context-2", TaskFilter{})
	require.NoError(t, err)
	assert.Len(t, context2TasksByContext, 1)

	paginatedTasks, err := storage.ListTasks(TaskFilter{Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, paginatedTasks, 2)

	paginatedTasks, err = storage.ListTasks(TaskFilter{Limit: 2, Offset: 2})
	require.NoError(t, err)
	assert.Len(t, paginatedTasks, 1)
}

func TestRedisStorageContextOperations(t *testing.T) {
	url := requireRedis(t)
	defer cleanupRedisTestData(t, url)

	logger := zaptest.NewLogger(t)
	factory := &RedisStorageFactory{}

	cfg := config.QueueConfig{
		Provider: "redis",
		URL:      url,
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)

	task1 := &types.Task{
		ID:        "task-1",
		ContextID: "context-1",
		Kind:      "task",
		Status:    types.TaskStatus{State: types.TaskStateCompleted},
	}
	task2 := &types.Task{
		ID:        "task-2",
		ContextID: "context-2",
		Kind:      "task",
		Status:    types.TaskStatus{State: types.TaskStateCompleted},
	}

	err = storage.StoreDeadLetterTask(task1)
	require.NoError(t, err)
	err = storage.StoreDeadLetterTask(task2)
	require.NoError(t, err)

	contexts := storage.GetContexts()
	assert.Len(t, contexts, 2)
	assert.Contains(t, contexts, "context-1")
	assert.Contains(t, contexts, "context-2")

	contextsWithTasks := storage.GetContextsWithTasks()
	assert.Equal(t, contexts, contextsWithTasks)

	err = storage.DeleteContextAndTasks("context-1")
	require.NoError(t, err)

	contexts = storage.GetContexts()
	assert.Len(t, contexts, 1)
	assert.Contains(t, contexts, "context-2")

	_, exists := storage.GetTask("task-1")
	assert.False(t, exists)

	_, exists = storage.GetTask("task-2")
	assert.True(t, exists)
}

func TestRedisStorageClearQueue(t *testing.T) {
	url := requireRedis(t)
	defer cleanupRedisTestData(t, url)

	logger := zaptest.NewLogger(t)
	factory := &RedisStorageFactory{}

	cfg := config.QueueConfig{
		Provider: "redis",
		URL:      url,
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)

	task := &types.Task{
		ID:        "test-task",
		ContextID: "test-context",
		Kind:      "task",
		Status:    types.TaskStatus{State: types.TaskStateSubmitted},
	}

	err = storage.EnqueueTask(task, "request-1")
	require.NoError(t, err)
	assert.Equal(t, 1, storage.GetQueueLength())

	err = storage.ClearQueue()
	require.NoError(t, err)
	assert.Equal(t, 0, storage.GetQueueLength())
}

func TestRedisStorageGetStats(t *testing.T) {
	url := requireRedis(t)
	defer cleanupRedisTestData(t, url)

	logger := zaptest.NewLogger(t)
	factory := &RedisStorageFactory{}

	cfg := config.QueueConfig{
		Provider: "redis",
		URL:      url,
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)

	completedTask := &types.Task{
		ID:        "completed-task",
		ContextID: "context-1",
		Kind:      "task",
		Status:    types.TaskStatus{State: types.TaskStateCompleted},
		History:   []types.Message{{Role: "user", Kind: "message", MessageID: "msg1", Parts: []types.Part{map[string]interface{}{"kind": "text", "text": "msg1"}}}},
	}
	failedTask := &types.Task{
		ID:        "failed-task",
		ContextID: "context-2",
		Kind:      "task",
		Status:    types.TaskStatus{State: types.TaskStateFailed},
		History:   []types.Message{{Role: "user", Kind: "message", MessageID: "msg2", Parts: []types.Part{map[string]interface{}{"kind": "text", "text": "msg2"}}}},
	}

	err = storage.StoreDeadLetterTask(completedTask)
	require.NoError(t, err)
	err = storage.StoreDeadLetterTask(failedTask)
	require.NoError(t, err)

	stats := storage.GetStats()
	assert.Equal(t, 2, stats.TotalTasks)
	assert.Equal(t, 1, stats.TasksByState["completed"])
	assert.Equal(t, 1, stats.TasksByState["failed"])
	assert.Equal(t, 2, stats.TotalContexts)
	assert.Equal(t, 2, stats.ContextsWithTasks)
	assert.Equal(t, float64(1), stats.AverageTasksPerContext)
	assert.Equal(t, 2, stats.TotalMessages)
	assert.Equal(t, float64(1), stats.AverageMessagesPerContext)
}
