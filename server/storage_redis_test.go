package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	redis "github.com/redis/go-redis/v9"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zap "go.uber.org/zap"
	zaptest "go.uber.org/zap/zaptest"

	mocks "github.com/inference-gateway/adk/server/mocks"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

// Mirror the (unexported) key constants from storage_redis.go.
// Tests pin the wire format; renaming a prefix in production is a
// breaking change and should fail these tests.
const (
	testTaskQueueKey        = "a2a:queue"
	testActiveTaskKeyPrefix = "a2a:active:"
	testDeadLetterKeyPrefix = "a2a:deadletter:"
	testContextTasksPrefix  = "a2a:context:"
	testQueueNotifyChannel  = "a2a:queue:notify"
)

func newTestRedisStorage(t *testing.T) (*server.RedisStorage, *mocks.FakeRedisClient, *mocks.FakeRedisPipeliner) {
	t.Helper()
	fakeClient := &mocks.FakeRedisClient{}
	fakePipe := &mocks.FakeRedisPipeliner{}
	fakeClient.PipelineReturns(fakePipe)
	storage := server.NewRedisStorageForTest(fakeClient, zaptest.NewLogger(t, zaptest.Level(zap.WarnLevel)), config.QueueConfig{URL: "redis://fake"})
	return storage, fakeClient, fakePipe
}

func TestRedisStorageFactory(t *testing.T) {
	factory := &server.RedisStorageFactory{}

	assert.Equal(t, "redis", factory.SupportedProvider())

	cfg := config.QueueConfig{Provider: "redis"}
	err := factory.ValidateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "URL is required")

	cfg.URL = "redis://localhost:6379"
	assert.NoError(t, factory.ValidateConfig(cfg))
}

func TestRedisStorageFactoryWithInvalidURL(t *testing.T) {
	factory := &server.RedisStorageFactory{}
	logger := zaptest.NewLogger(t)

	storage, err := factory.CreateStorage(context.Background(), config.QueueConfig{
		Provider: "redis",
		URL:      "invalid-url",
	}, logger)
	assert.Error(t, err)
	assert.Nil(t, storage)
	assert.Contains(t, err.Error(), "invalid Redis URL")
}

func TestRedisStorageEnqueueTask(t *testing.T) {
	storage, fakeClient, fakePipe := newTestRedisStorage(t)

	task := &types.Task{
		ID:        "test-task-1",
		ContextID: "test-context",
		Status:    types.TaskStatus{State: types.TaskStateSubmitted},
		History:   []types.Message{},
	}

	fakePipe.ExecReturns(nil, nil)
	fakeClient.PublishReturns(redis.NewIntResult(1, nil))
	fakeClient.LLenReturns(redis.NewIntResult(1, nil))

	require.NoError(t, storage.EnqueueTask(task, "request-123"))

	require.Equal(t, 1, fakePipe.LPushCallCount())
	_, gotQueueKey, gotQueueValues := fakePipe.LPushArgsForCall(0)
	assert.Equal(t, testTaskQueueKey, gotQueueKey)
	require.Len(t, gotQueueValues, 1)

	var enqueued server.QueuedTask
	require.NoError(t, json.Unmarshal(gotQueueValues[0].([]byte), &enqueued))
	assert.Equal(t, task.ID, enqueued.Task.ID)
	assert.Equal(t, "request-123", enqueued.RequestID)

	require.Equal(t, 1, fakePipe.SetCallCount())
	_, gotActiveKey, gotActiveValue, _ := fakePipe.SetArgsForCall(0)
	assert.Equal(t, testActiveTaskKeyPrefix+task.ID, gotActiveKey)

	var storedTask types.Task
	require.NoError(t, json.Unmarshal(gotActiveValue.([]byte), &storedTask))
	assert.Equal(t, task.ID, storedTask.ID)

	require.Equal(t, 1, fakePipe.ExecCallCount())

	require.Equal(t, 1, fakeClient.PublishCallCount())
	_, gotChannel, gotMessage := fakeClient.PublishArgsForCall(0)
	assert.Equal(t, testQueueNotifyChannel, gotChannel)
	assert.Equal(t, "task_added", gotMessage)
}

func TestRedisStorageEnqueueNilTask(t *testing.T) {
	storage, _, _ := newTestRedisStorage(t)
	err := storage.EnqueueTask(nil, "request-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task cannot be nil")
}

func TestRedisStorageEnqueueTaskPipelineExecError(t *testing.T) {
	storage, _, fakePipe := newTestRedisStorage(t)
	fakePipe.ExecReturns(nil, errors.New("boom"))

	err := storage.EnqueueTask(&types.Task{ID: "t", ContextID: "c"}, "r")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to enqueue task")
}

func TestRedisStorageDequeueTask(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)

	queued := server.QueuedTask{
		Task: &types.Task{
			ID:        "test-task-1",
			ContextID: "test-context",
			Status:    types.TaskStatus{State: types.TaskStateSubmitted},
		},
		RequestID: "request-123",
	}
	data, err := json.Marshal(queued)
	require.NoError(t, err)

	fakeClient.BRPopReturns(redis.NewStringSliceResult([]string{testTaskQueueKey, string(data)}, nil))
	fakeClient.LLenReturns(redis.NewIntResult(0, nil))

	got, err := storage.DequeueTask(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test-task-1", got.Task.ID)
	assert.Equal(t, "request-123", got.RequestID)

	require.Equal(t, 1, fakeClient.BRPopCallCount())
	_, _, gotKeys := fakeClient.BRPopArgsForCall(0)
	assert.Equal(t, []string{testTaskQueueKey}, gotKeys)
}

func TestRedisStorageDequeueTaskContextCancelled(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.BRPopReturns(redis.NewStringSliceResult(nil, redis.Nil))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := storage.DequeueTask(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRedisStorageGetQueueLength(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.LLenReturns(redis.NewIntResult(7, nil))

	assert.Equal(t, 7, storage.GetQueueLength())

	require.Equal(t, 1, fakeClient.LLenCallCount())
	_, key := fakeClient.LLenArgsForCall(0)
	assert.Equal(t, testTaskQueueKey, key)
}

func TestRedisStorageClearQueue(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.LLenReturns(redis.NewIntResult(3, nil))
	fakeClient.DelReturns(redis.NewIntResult(1, nil))

	require.NoError(t, storage.ClearQueue())

	require.Equal(t, 1, fakeClient.DelCallCount())
	_, keys := fakeClient.DelArgsForCall(0)
	assert.Equal(t, []string{testTaskQueueKey}, keys)
}

func TestRedisStorageCreateActiveTask(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.ExistsReturns(redis.NewIntResult(0, nil))
	fakeClient.SetReturns(redis.NewStatusResult("OK", nil))

	task := &types.Task{ID: "t1", ContextID: "c1", Status: types.TaskStatus{State: types.TaskStateSubmitted}}
	require.NoError(t, storage.CreateActiveTask(task))

	require.Equal(t, 1, fakeClient.ExistsCallCount())
	_, existsKeys := fakeClient.ExistsArgsForCall(0)
	assert.Equal(t, []string{testActiveTaskKeyPrefix + "t1"}, existsKeys)

	require.Equal(t, 1, fakeClient.SetCallCount())
	_, setKey, setValue, _ := fakeClient.SetArgsForCall(0)
	assert.Equal(t, testActiveTaskKeyPrefix+"t1", setKey)

	var stored types.Task
	require.NoError(t, json.Unmarshal(setValue.([]byte), &stored))
	assert.Equal(t, "t1", stored.ID)
}

func TestRedisStorageCreateActiveTaskAlreadyExists(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.ExistsReturns(redis.NewIntResult(1, nil))

	err := storage.CreateActiveTask(&types.Task{ID: "t1", ContextID: "c1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Equal(t, 0, fakeClient.SetCallCount())
}

func TestRedisStorageCreateActiveTaskNil(t *testing.T) {
	storage, _, _ := newTestRedisStorage(t)
	err := storage.CreateActiveTask(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task cannot be nil")
}

func TestRedisStorageUpdateActiveTask(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.ExistsReturns(redis.NewIntResult(1, nil))
	fakeClient.SetReturns(redis.NewStatusResult("OK", nil))

	task := &types.Task{ID: "t1", ContextID: "c1", Status: types.TaskStatus{State: types.TaskStateWorking}}
	require.NoError(t, storage.UpdateActiveTask(task))

	require.Equal(t, 1, fakeClient.SetCallCount())
	_, setKey, _, _ := fakeClient.SetArgsForCall(0)
	assert.Equal(t, testActiveTaskKeyPrefix+"t1", setKey)
}

func TestRedisStorageUpdateActiveTaskNotFound(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.ExistsReturns(redis.NewIntResult(0, nil))

	err := storage.UpdateActiveTask(&types.Task{ID: "t1", ContextID: "c1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Equal(t, 0, fakeClient.SetCallCount())
}

func TestRedisStorageGetActiveTask(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	task := &types.Task{ID: "t1", ContextID: "c1", Status: types.TaskStatus{State: types.TaskStateWorking}}
	data, err := json.Marshal(task)
	require.NoError(t, err)
	fakeClient.GetReturns(redis.NewStringResult(string(data), nil))

	got, err := storage.GetActiveTask("t1")
	require.NoError(t, err)
	assert.Equal(t, "t1", got.ID)
	assert.Equal(t, types.TaskStateWorking, got.Status.State)

	require.Equal(t, 1, fakeClient.GetCallCount())
	_, key := fakeClient.GetArgsForCall(0)
	assert.Equal(t, testActiveTaskKeyPrefix+"t1", key)
}

func TestRedisStorageGetActiveTaskNotFound(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.GetReturns(redis.NewStringResult("", redis.Nil))

	_, err := storage.GetActiveTask("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRedisStorageStoreDeadLetterTask(t *testing.T) {
	storage, _, fakePipe := newTestRedisStorage(t)
	fakePipe.ExecReturns(nil, nil)

	task := &types.Task{
		ID:        "t1",
		ContextID: "c1",
		Status:    types.TaskStatus{State: types.TaskStateCompleted},
	}
	require.NoError(t, storage.StoreDeadLetterTask(task))

	require.Equal(t, 1, fakePipe.SetCallCount())
	_, setKey, _, _ := fakePipe.SetArgsForCall(0)
	assert.Equal(t, testDeadLetterKeyPrefix+"t1", setKey)

	require.Equal(t, 1, fakePipe.SAddCallCount())
	_, sAddKey, sAddMembers := fakePipe.SAddArgsForCall(0)
	assert.Equal(t, testContextTasksPrefix+"c1", sAddKey)
	assert.Equal(t, []any{"t1"}, sAddMembers)

	require.Equal(t, 1, fakePipe.DelCallCount())
	_, delKeys := fakePipe.DelArgsForCall(0)
	assert.Equal(t, []string{testActiveTaskKeyPrefix + "t1"}, delKeys)

	require.Equal(t, 1, fakePipe.ExecCallCount())
}

func TestRedisStorageStoreDeadLetterTaskNil(t *testing.T) {
	storage, _, _ := newTestRedisStorage(t)
	err := storage.StoreDeadLetterTask(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task cannot be nil")
}

func TestRedisStorageGetTask(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	task := &types.Task{ID: "t1", ContextID: "c1", Status: types.TaskStatus{State: types.TaskStateCompleted}}
	data, err := json.Marshal(task)
	require.NoError(t, err)
	fakeClient.GetReturns(redis.NewStringResult(string(data), nil))

	got, ok := storage.GetTask("t1")
	require.True(t, ok)
	assert.Equal(t, "t1", got.ID)

	_, key := fakeClient.GetArgsForCall(0)
	assert.Equal(t, testDeadLetterKeyPrefix+"t1", key)
}

func TestRedisStorageGetTaskMiss(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.GetReturns(redis.NewStringResult("", redis.Nil))

	_, ok := storage.GetTask("missing")
	assert.False(t, ok)
}

func TestRedisStorageGetTaskByContextAndID(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	task := &types.Task{ID: "t1", ContextID: "c1", Status: types.TaskStatus{State: types.TaskStateCompleted}}
	data, err := json.Marshal(task)
	require.NoError(t, err)
	fakeClient.GetReturns(redis.NewStringResult(string(data), nil))

	got, ok := storage.GetTaskByContextAndID("c1", "t1")
	require.True(t, ok)
	assert.Equal(t, "t1", got.ID)

	_, ok = storage.GetTaskByContextAndID("wrong-context", "t1")
	assert.False(t, ok)
}

func TestRedisStorageDeleteTask(t *testing.T) {
	storage, fakeClient, fakePipe := newTestRedisStorage(t)

	task := &types.Task{ID: "t1", ContextID: "c1", Status: types.TaskStatus{State: types.TaskStateCompleted}}
	data, err := json.Marshal(task)
	require.NoError(t, err)
	fakeClient.GetReturns(redis.NewStringResult(string(data), nil))
	fakePipe.ExecReturns(nil, nil)

	require.NoError(t, storage.DeleteTask("t1"))

	require.Equal(t, 1, fakePipe.DelCallCount())
	_, delKeys := fakePipe.DelArgsForCall(0)
	assert.Equal(t, []string{testDeadLetterKeyPrefix + "t1"}, delKeys)

	require.Equal(t, 1, fakePipe.SRemCallCount())
	_, sRemKey, sRemMembers := fakePipe.SRemArgsForCall(0)
	assert.Equal(t, testContextTasksPrefix+"c1", sRemKey)
	assert.Equal(t, []any{"t1"}, sRemMembers)
}

func TestRedisStorageDeleteTaskNotFound(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.GetReturns(redis.NewStringResult("", redis.Nil))

	err := storage.DeleteTask("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRedisStorageListTasks(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)

	tasks := []*types.Task{
		{ID: "t1", ContextID: "c1", Status: types.TaskStatus{State: types.TaskStateCompleted}},
		{ID: "t2", ContextID: "c1", Status: types.TaskStatus{State: types.TaskStateFailed}},
		{ID: "t3", ContextID: "c2", Status: types.TaskStatus{State: types.TaskStateCompleted}},
	}

	deadLetterKeys := make([]string, 0, len(tasks))
	taskByDeadLetterKey := map[string]*types.Task{}
	for _, task := range tasks {
		key := testDeadLetterKeyPrefix + task.ID
		deadLetterKeys = append(deadLetterKeys, key)
		taskByDeadLetterKey[key] = task
	}

	fakeClient.KeysStub = func(_ context.Context, pattern string) *redis.StringSliceCmd {
		switch pattern {
		case testDeadLetterKeyPrefix + "*":
			return redis.NewStringSliceResult(deadLetterKeys, nil)
		case testActiveTaskKeyPrefix + "*":
			return redis.NewStringSliceResult(nil, nil)
		}
		return redis.NewStringSliceResult(nil, nil)
	}

	fakeClient.GetStub = func(_ context.Context, key string) *redis.StringCmd {
		if task, ok := taskByDeadLetterKey[key]; ok {
			data, err := json.Marshal(task)
			require.NoError(t, err)
			return redis.NewStringResult(string(data), nil)
		}
		return redis.NewStringResult("", redis.Nil)
	}

	all, err := storage.ListTasks(server.TaskFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 3)

	completedState := types.TaskStateCompleted
	completed, err := storage.ListTasks(server.TaskFilter{State: &completedState})
	require.NoError(t, err)
	assert.Len(t, completed, 2)

	context1 := "c1"
	context1Tasks, err := storage.ListTasks(server.TaskFilter{ContextID: &context1})
	require.NoError(t, err)
	assert.Len(t, context1Tasks, 2)

	page1, err := storage.ListTasks(server.TaskFilter{Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	page2, err := storage.ListTasks(server.TaskFilter{Limit: 2, Offset: 2})
	require.NoError(t, err)
	assert.Len(t, page2, 1)
}

func TestRedisStorageListTasksByContext(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)

	fakeClient.SMembersReturns(redis.NewStringSliceResult([]string{"t1", "t2"}, nil))

	taskByID := map[string]*types.Task{
		"t1": {ID: "t1", ContextID: "c1", Status: types.TaskStatus{State: types.TaskStateCompleted}},
		"t2": {ID: "t2", ContextID: "c1", Status: types.TaskStatus{State: types.TaskStateFailed}},
	}
	fakeClient.GetStub = func(_ context.Context, key string) *redis.StringCmd {
		taskID := key[len(testDeadLetterKeyPrefix):]
		if task, ok := taskByID[taskID]; ok {
			data, err := json.Marshal(task)
			require.NoError(t, err)
			return redis.NewStringResult(string(data), nil)
		}
		return redis.NewStringResult("", redis.Nil)
	}

	got, err := storage.ListTasksByContext("c1", server.TaskFilter{})
	require.NoError(t, err)
	assert.Len(t, got, 2)

	_, sMembersKey := fakeClient.SMembersArgsForCall(0)
	assert.Equal(t, testContextTasksPrefix+"c1", sMembersKey)
}

func TestRedisStorageGetContexts(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)
	fakeClient.KeysReturns(redis.NewStringSliceResult([]string{
		testContextTasksPrefix + "c1",
		testContextTasksPrefix + "c2",
	}, nil))

	contexts := storage.GetContexts()
	assert.ElementsMatch(t, []string{"c1", "c2"}, contexts)

	_, pattern := fakeClient.KeysArgsForCall(0)
	assert.Equal(t, testContextTasksPrefix+"*", pattern)
}

func TestRedisStorageDeleteContextAndTasks(t *testing.T) {
	storage, fakeClient, fakePipe := newTestRedisStorage(t)

	fakeClient.SMembersReturns(redis.NewStringSliceResult([]string{"t1", "t2"}, nil))
	fakePipe.ExecReturns(nil, nil)

	require.NoError(t, storage.DeleteContextAndTasks("c1"))

	// Expect 2 dead-letter Del + 2 active Del + 1 context Del = 5
	require.Equal(t, 5, fakePipe.DelCallCount())

	var deletedKeys []string
	for i := 0; i < fakePipe.DelCallCount(); i++ {
		_, keys := fakePipe.DelArgsForCall(i)
		deletedKeys = append(deletedKeys, keys...)
	}
	assert.ElementsMatch(t, []string{
		testDeadLetterKeyPrefix + "t1",
		testActiveTaskKeyPrefix + "t1",
		testDeadLetterKeyPrefix + "t2",
		testActiveTaskKeyPrefix + "t2",
		testContextTasksPrefix + "c1",
	}, deletedKeys)

	require.Equal(t, 1, fakePipe.ExecCallCount())
}

func TestRedisStorageDeleteContextNoTasks(t *testing.T) {
	storage, fakeClient, fakePipe := newTestRedisStorage(t)
	fakeClient.SMembersReturns(redis.NewStringSliceResult(nil, nil))

	require.NoError(t, storage.DeleteContextAndTasks("c1"))
	assert.Equal(t, 0, fakePipe.ExecCallCount())
}

func TestRedisStorageGetStats(t *testing.T) {
	storage, fakeClient, _ := newTestRedisStorage(t)

	completedTask := &types.Task{
		ID:        "completed-task",
		ContextID: "c1",
		Status:    types.TaskStatus{State: types.TaskStateCompleted},
		History:   []types.Message{{Role: "user", MessageID: "m1", Parts: []types.Part{types.CreateTextPart("hi")}}},
	}
	failedTask := &types.Task{
		ID:        "failed-task",
		ContextID: "c2",
		Status:    types.TaskStatus{State: types.TaskStateFailed},
		History:   []types.Message{{Role: "user", MessageID: "m2", Parts: []types.Part{types.CreateTextPart("oh no")}}},
	}

	deadLetterKeys := []string{
		testDeadLetterKeyPrefix + "completed-task",
		testDeadLetterKeyPrefix + "failed-task",
	}
	tasksByKey := map[string]*types.Task{
		deadLetterKeys[0]: completedTask,
		deadLetterKeys[1]: failedTask,
	}

	fakeClient.KeysStub = func(_ context.Context, pattern string) *redis.StringSliceCmd {
		switch pattern {
		case testDeadLetterKeyPrefix + "*":
			return redis.NewStringSliceResult(deadLetterKeys, nil)
		case testContextTasksPrefix + "*":
			return redis.NewStringSliceResult([]string{
				testContextTasksPrefix + "c1",
				testContextTasksPrefix + "c2",
			}, nil)
		}
		return redis.NewStringSliceResult(nil, nil)
	}

	fakeClient.GetStub = func(_ context.Context, key string) *redis.StringCmd {
		if task, ok := tasksByKey[key]; ok {
			data, err := json.Marshal(task)
			require.NoError(t, err)
			return redis.NewStringResult(string(data), nil)
		}
		return redis.NewStringResult("", redis.Nil)
	}

	stats := storage.GetStats()
	assert.Equal(t, 2, stats.TotalTasks)
	assert.Equal(t, 1, stats.TasksByState[string(types.TaskStateCompleted)])
	assert.Equal(t, 1, stats.TasksByState[string(types.TaskStateFailed)])
	assert.Equal(t, 2, stats.TotalContexts)
	assert.Equal(t, 2, stats.ContextsWithTasks)
	assert.Equal(t, float64(1), stats.AverageTasksPerContext)
	assert.Equal(t, 2, stats.TotalMessages)
	assert.Equal(t, float64(1), stats.AverageMessagesPerContext)
}
