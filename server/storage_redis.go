package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/types"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisStorageFactory implements StorageFactory for Redis storage
type RedisStorageFactory struct{}

// SupportedProvider returns the provider name
func (f *RedisStorageFactory) SupportedProvider() string {
	return "redis"
}

// ValidateConfig validates the configuration for Redis storage
func (f *RedisStorageFactory) ValidateConfig(config config.QueueConfig) error {
	if config.URL == "" {
		return fmt.Errorf("URL is required for Redis storage provider")
	}
	return nil
}

// CreateStorage creates a Redis storage instance
func (f *RedisStorageFactory) CreateStorage(ctx context.Context, config config.QueueConfig, logger *zap.Logger) (Storage, error) {
	opt, err := redis.ParseURL(config.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	if dbStr, exists := config.Options["db"]; exists {
		if db, err := strconv.Atoi(dbStr); err == nil {
			opt.DB = db
		}
	}

	if maxRetriesStr, exists := config.Options["max_retries"]; exists {
		if maxRetries, err := strconv.Atoi(maxRetriesStr); err == nil {
			opt.MaxRetries = maxRetries
		}
	}

	if timeoutStr, exists := config.Options["timeout"]; exists {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			opt.DialTimeout = timeout
			opt.ReadTimeout = timeout
			opt.WriteTimeout = timeout
		}
	}

	if username, exists := config.Credentials["username"]; exists {
		opt.Username = username
	}
	if password, exists := config.Credentials["password"]; exists {
		opt.Password = password
	}

	client := redis.NewClient(opt)

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("connected to Redis",
		zap.String("addr", opt.Addr),
		zap.Int("db", opt.DB))

	return &RedisStorage{
		client: client,
		logger: logger,
		config: config,
	}, nil
}

// RedisStorage implements Storage interface using Redis
type RedisStorage struct {
	client *redis.Client
	logger *zap.Logger
	config config.QueueConfig
}

var _ Storage = (*RedisStorage)(nil)

const (
	taskQueueKey        = "a2a:queue"
	activeTaskKeyPrefix = "a2a:active:"
	deadLetterKeyPrefix = "a2a:deadletter:"
	contextTasksPrefix  = "a2a:context:"
	queueNotifyChannel  = "a2a:queue:notify"
)

// EnqueueTask adds a task to the processing queue
func (s *RedisStorage) EnqueueTask(task *types.Task, requestID any) error {
	ctx := context.Background()

	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	queuedTask := &QueuedTask{
		Task:      task,
		RequestID: requestID,
	}

	data, err := json.Marshal(queuedTask)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	pipe := s.client.Pipeline()

	pipe.LPush(ctx, taskQueueKey, data)

	activeKey := activeTaskKeyPrefix + task.ID
	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to serialize active task: %w", err)
	}
	pipe.Set(ctx, activeKey, taskData, 0)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	s.client.Publish(ctx, queueNotifyChannel, "task_added")

	queueLength := s.GetQueueLength()
	s.logger.Info("task enqueued for processing",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.Int("queue_length", queueLength))

	return nil
}

// DequeueTask retrieves and removes the next task from the processing queue
func (s *RedisStorage) DequeueTask(ctx context.Context) (*QueuedTask, error) {
	result, err := s.client.BRPop(ctx, 0, taskQueueKey).Result()
	if err != nil {
		if err == redis.Nil || strings.Contains(err.Error(), "context") {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("failed to dequeue task: %w", err)
	}

	if len(result) != 2 {
		return nil, fmt.Errorf("unexpected BRPOP result format")
	}

	var queuedTask QueuedTask
	if err := json.Unmarshal([]byte(result[1]), &queuedTask); err != nil {
		return nil, fmt.Errorf("failed to deserialize queued task: %w", err)
	}

	remainingQueueLength := s.GetQueueLength()
	s.logger.Info("task dequeued for processing",
		zap.String("task_id", queuedTask.Task.ID),
		zap.String("context_id", queuedTask.Task.ContextID),
		zap.Int("remaining_queue_length", remainingQueueLength))

	return &queuedTask, nil
}

// GetQueueLength returns the current number of tasks in the queue
func (s *RedisStorage) GetQueueLength() int {
	ctx := context.Background()
	length, err := s.client.LLen(ctx, taskQueueKey).Result()
	if err != nil {
		s.logger.Error("failed to get queue length", zap.Error(err))
		return 0
	}
	return int(length)
}

// ClearQueue removes all tasks from the queue
func (s *RedisStorage) ClearQueue() error {
	ctx := context.Background()
	queueLength := s.GetQueueLength()

	if err := s.client.Del(ctx, taskQueueKey).Err(); err != nil {
		return fmt.Errorf("failed to clear queue: %w", err)
	}

	s.logger.Info("task queue cleared", zap.Int("removed_tasks", queueLength))
	return nil
}

// GetActiveTask retrieves an active task by ID
func (s *RedisStorage) GetActiveTask(taskID string) (*types.Task, error) {
	ctx := context.Background()
	activeKey := activeTaskKeyPrefix + taskID

	data, err := s.client.Get(ctx, activeKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("active task not found: %s", taskID)
		}
		return nil, fmt.Errorf("failed to get active task: %w", err)
	}

	var task types.Task
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return nil, fmt.Errorf("failed to deserialize active task: %w", err)
	}

	return &task, nil
}

// CreateActiveTask creates a new active task
func (s *RedisStorage) CreateActiveTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	ctx := context.Background()
	activeKey := activeTaskKeyPrefix + task.ID

	exists, err := s.client.Exists(ctx, activeKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check task existence: %w", err)
	}
	if exists > 0 {
		return fmt.Errorf("active task already exists: %s", task.ID)
	}

	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	if err := s.client.Set(ctx, activeKey, taskData, 0).Err(); err != nil {
		return fmt.Errorf("failed to create active task: %w", err)
	}

	s.logger.Debug("active task created",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(task.Status.State)))

	return nil
}

// UpdateActiveTask updates an active task's metadata
func (s *RedisStorage) UpdateActiveTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	ctx := context.Background()
	activeKey := activeTaskKeyPrefix + task.ID

	exists, err := s.client.Exists(ctx, activeKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check task existence: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("active task not found: %s", task.ID)
	}

	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	if err := s.client.Set(ctx, activeKey, taskData, 0).Err(); err != nil {
		return fmt.Errorf("failed to update active task: %w", err)
	}

	s.logger.Debug("active task updated",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(task.Status.State)))

	return nil
}

// StoreDeadLetterTask stores a completed/failed task in the dead letter queue
func (s *RedisStorage) StoreDeadLetterTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	ctx := context.Background()
	deadLetterKey := deadLetterKeyPrefix + task.ID
	contextKey := contextTasksPrefix + task.ContextID
	activeKey := activeTaskKeyPrefix + task.ID

	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	pipe := s.client.Pipeline()

	pipe.Set(ctx, deadLetterKey, taskData, 0)

	pipe.SAdd(ctx, contextKey, task.ID)

	pipe.Del(ctx, activeKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to store dead letter task: %w", err)
	}

	s.logger.Debug("task stored in dead letter queue",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(task.Status.State)))

	return nil
}

// GetTask retrieves a task by ID from dead letter queue
func (s *RedisStorage) GetTask(taskID string) (*types.Task, bool) {
	ctx := context.Background()
	deadLetterKey := deadLetterKeyPrefix + taskID

	data, err := s.client.Get(ctx, deadLetterKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false
		}
		s.logger.Error("failed to get task from dead letter queue", zap.String("task_id", taskID), zap.Error(err))
		return nil, false
	}

	var task types.Task
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		s.logger.Error("failed to deserialize dead letter task", zap.String("task_id", taskID), zap.Error(err))
		return nil, false
	}

	return &task, true
}

// GetTaskByContextAndID retrieves a task by context ID and task ID
func (s *RedisStorage) GetTaskByContextAndID(contextID, taskID string) (*types.Task, bool) {
	task, exists := s.GetTask(taskID)
	if !exists {
		return nil, false
	}

	if task.ContextID != contextID {
		return nil, false
	}

	return task, true
}

// DeleteTask deletes a task from dead letter queue and cleans up context mapping
func (s *RedisStorage) DeleteTask(taskID string) error {
	ctx := context.Background()

	task, exists := s.GetTask(taskID)
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	deadLetterKey := deadLetterKeyPrefix + taskID
	contextKey := contextTasksPrefix + task.ContextID

	pipe := s.client.Pipeline()

	pipe.Del(ctx, deadLetterKey)

	pipe.SRem(ctx, contextKey, taskID)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	s.logger.Debug("task deleted",
		zap.String("task_id", taskID),
		zap.String("context_id", task.ContextID))

	return nil
}

// ListTasks retrieves a list of tasks based on the provided filter
func (s *RedisStorage) ListTasks(filter TaskFilter) ([]*types.Task, error) {
	ctx := context.Background()
	var allTasks []*types.Task

	deadLetterPattern := deadLetterKeyPrefix + "*"
	deadLetterKeys, err := s.client.Keys(ctx, deadLetterPattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get dead letter tasks: %w", err)
	}

	for _, key := range deadLetterKeys {
		taskID := strings.TrimPrefix(key, deadLetterKeyPrefix)
		if task, exists := s.GetTask(taskID); exists {
			if s.matchesFilter(task, filter) {
				allTasks = append(allTasks, task)
			}
		}
	}

	activePattern := activeTaskKeyPrefix + "*"
	activeKeys, err := s.client.Keys(ctx, activePattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get active tasks: %w", err)
	}

	for _, key := range activeKeys {
		taskID := strings.TrimPrefix(key, activeTaskKeyPrefix)
		if task, err := s.GetActiveTask(taskID); err == nil {
			if s.matchesFilter(task, filter) {
				allTasks = append(allTasks, task)
			}
		}
	}

	s.sortTasks(allTasks, filter.SortBy, filter.SortOrder)

	total := len(allTasks)
	start := filter.Offset
	if start >= total {
		return []*types.Task{}, nil
	}

	end := start + filter.Limit
	if filter.Limit <= 0 || end > total {
		end = total
	}

	return allTasks[start:end], nil
}

// ListTasksByContext retrieves tasks for a specific context
func (s *RedisStorage) ListTasksByContext(contextID string, filter TaskFilter) ([]*types.Task, error) {
	ctx := context.Background()
	contextKey := contextTasksPrefix + contextID

	taskIDs, err := s.client.SMembers(ctx, contextKey).Result()
	if err != nil {
		if err == redis.Nil {
			return []*types.Task{}, nil
		}
		return nil, fmt.Errorf("failed to get context tasks: %w", err)
	}

	var filteredTasks []*types.Task

	for _, taskID := range taskIDs {
		if task, exists := s.GetTask(taskID); exists {
			if s.matchesFilter(task, filter) {
				filteredTasks = append(filteredTasks, task)
			}
		}
	}

	s.sortTasks(filteredTasks, filter.SortBy, filter.SortOrder)

	total := len(filteredTasks)
	start := filter.Offset
	if start >= total {
		return []*types.Task{}, nil
	}

	end := start + filter.Limit
	if filter.Limit <= 0 || end > total {
		end = total
	}

	return filteredTasks[start:end], nil
}

// Helper method to check if a task matches the filter
func (s *RedisStorage) matchesFilter(task *types.Task, filter TaskFilter) bool {
	if filter.State != nil && task.Status.State != *filter.State {
		return false
	}

	if filter.ContextID != nil && task.ContextID != *filter.ContextID {
		return false
	}

	return true
}

// sortTasks sorts tasks based on the specified field and order
func (s *RedisStorage) sortTasks(tasks []*types.Task, sortBy TaskSortField, order SortOrder) {
	if len(tasks) <= 1 {
		return
	}

	sort.Slice(tasks, func(i, j int) bool {
		var compareResult int

		switch sortBy {
		case TaskSortFieldCreatedAt, TaskSortFieldUpdatedAt:
			// Timestamp is an empty struct, so we can't sort by it
			// Maintain current order
			compareResult = 0
		case TaskSortFieldState:
			compareResult = strings.Compare(string(tasks[i].Status.State), string(tasks[j].Status.State))
		case TaskSortFieldContextID:
			compareResult = strings.Compare(tasks[i].ContextID, tasks[j].ContextID)
		default:
			compareResult = strings.Compare(tasks[i].ID, tasks[j].ID)
		}

		if order == SortOrderAsc {
			return compareResult < 0
		}
		return compareResult > 0
	})
}

// GetContexts returns all context IDs that have tasks
func (s *RedisStorage) GetContexts() []string {
	ctx := context.Background()
	contextPattern := contextTasksPrefix + "*"

	contextKeys, err := s.client.Keys(ctx, contextPattern).Result()
	if err != nil {
		s.logger.Error("failed to get contexts", zap.Error(err))
		return []string{}
	}

	contexts := make([]string, len(contextKeys))
	for i, key := range contextKeys {
		contexts[i] = strings.TrimPrefix(key, contextTasksPrefix)
	}

	return contexts
}

// GetContextsWithTasks returns all context IDs that have tasks
func (s *RedisStorage) GetContextsWithTasks() []string {
	return s.GetContexts()
}

// DeleteContext deletes all tasks for a context
func (s *RedisStorage) DeleteContext(contextID string) error {
	return s.DeleteContextAndTasks(contextID)
}

// DeleteContextAndTasks deletes all tasks for a context
func (s *RedisStorage) DeleteContextAndTasks(contextID string) error {
	ctx := context.Background()
	contextKey := contextTasksPrefix + contextID

	taskIDs, err := s.client.SMembers(ctx, contextKey).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get context tasks: %w", err)
	}

	if len(taskIDs) == 0 {
		return nil
	}

	pipe := s.client.Pipeline()

	for _, taskID := range taskIDs {
		deadLetterKey := deadLetterKeyPrefix + taskID
		activeKey := activeTaskKeyPrefix + taskID
		pipe.Del(ctx, deadLetterKey)
		pipe.Del(ctx, activeKey)
	}

	pipe.Del(ctx, contextKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete context and tasks: %w", err)
	}

	s.logger.Debug("context and tasks deleted", zap.String("context_id", contextID))
	return nil
}

// CleanupCompletedTasks removes completed, failed, and canceled tasks
func (s *RedisStorage) CleanupCompletedTasks() int {
	ctx := context.Background()
	deadLetterPattern := deadLetterKeyPrefix + "*"

	deadLetterKeys, err := s.client.Keys(ctx, deadLetterPattern).Result()
	if err != nil {
		s.logger.Error("failed to get tasks for cleanup", zap.Error(err))
		return 0
	}

	var toRemove []string
	contextUpdates := make(map[string][]string)

	for _, key := range deadLetterKeys {
		taskID := strings.TrimPrefix(key, deadLetterKeyPrefix)
		if task, exists := s.GetTask(taskID); exists {
			switch task.Status.State {
			case types.TaskStateCompleted, types.TaskStateFailed, types.TaskStateCancelled:
				toRemove = append(toRemove, taskID)
				contextUpdates[task.ContextID] = append(contextUpdates[task.ContextID], taskID)
			}
		}
	}

	if len(toRemove) == 0 {
		return 0
	}

	pipe := s.client.Pipeline()

	for _, taskID := range toRemove {
		deadLetterKey := deadLetterKeyPrefix + taskID
		pipe.Del(ctx, deadLetterKey)
	}

	for contextID, taskIDs := range contextUpdates {
		contextKey := contextTasksPrefix + contextID
		for _, taskID := range taskIDs {
			pipe.SRem(ctx, contextKey, taskID)
		}
	}

	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.Error("failed to cleanup completed tasks", zap.Error(err))
		return 0
	}

	return len(toRemove)
}

// CleanupTasksWithRetention removes old completed and failed tasks while keeping specified number
func (s *RedisStorage) CleanupTasksWithRetention(maxCompleted, maxFailed int) int {
	ctx := context.Background()
	deadLetterPattern := deadLetterKeyPrefix + "*"

	deadLetterKeys, err := s.client.Keys(ctx, deadLetterPattern).Result()
	if err != nil {
		s.logger.Error("failed to get tasks for retention cleanup", zap.Error(err))
		return 0
	}

	var completedTasks, failedTasks []*types.Task

	for _, key := range deadLetterKeys {
		taskID := strings.TrimPrefix(key, deadLetterKeyPrefix)
		if task, exists := s.GetTask(taskID); exists {
			switch task.Status.State {
			case types.TaskStateCompleted:
				completedTasks = append(completedTasks, task)
			case types.TaskStateFailed:
				failedTasks = append(failedTasks, task)
			}
		}
	}

	s.sortTasksByTimestamp(completedTasks, true)
	s.sortTasksByTimestamp(failedTasks, true)

	var toRemove []string
	contextUpdates := make(map[string][]string)

	if len(completedTasks) > maxCompleted {
		for i := maxCompleted; i < len(completedTasks); i++ {
			task := completedTasks[i]
			toRemove = append(toRemove, task.ID)
			contextUpdates[task.ContextID] = append(contextUpdates[task.ContextID], task.ID)
		}
	}

	if len(failedTasks) > maxFailed {
		for i := maxFailed; i < len(failedTasks); i++ {
			task := failedTasks[i]
			toRemove = append(toRemove, task.ID)
			contextUpdates[task.ContextID] = append(contextUpdates[task.ContextID], task.ID)
		}
	}

	if len(toRemove) == 0 {
		return 0
	}

	pipe := s.client.Pipeline()

	for _, taskID := range toRemove {
		deadLetterKey := deadLetterKeyPrefix + taskID
		pipe.Del(ctx, deadLetterKey)
	}

	for contextID, taskIDs := range contextUpdates {
		contextKey := contextTasksPrefix + contextID
		for _, taskID := range taskIDs {
			pipe.SRem(ctx, contextKey, taskID)
		}
	}

	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.Error("failed to cleanup tasks with retention", zap.Error(err))
		return 0
	}

	return len(toRemove)
}

// sortTasksByTimestamp sorts tasks by timestamp (newest first if desc=true)
func (s *RedisStorage) sortTasksByTimestamp(tasks []*types.Task, desc bool) {
	if len(tasks) <= 1 {
		return
	}

	for i := 0; i < len(tasks)-1; i++ {
		for j := 0; j < len(tasks)-i-1; j++ {
			var shouldSwap bool

			ts1 := tasks[j].Status.Timestamp
			ts2 := tasks[j+1].Status.Timestamp

			if ts1 != nil && ts2 != nil {
				if desc {
					shouldSwap = ts1.Before(*ts2)
				} else {
					shouldSwap = ts1.After(*ts2)
				}
			}

			if shouldSwap {
				tasks[j], tasks[j+1] = tasks[j+1], tasks[j]
			}
		}
	}
}

// GetStats provides statistics about the storage
func (s *RedisStorage) GetStats() StorageStats {
	ctx := context.Background()

	deadLetterPattern := deadLetterKeyPrefix + "*"
	deadLetterKeys, err := s.client.Keys(ctx, deadLetterPattern).Result()
	if err != nil {
		s.logger.Error("failed to get stats", zap.Error(err))
		return StorageStats{}
	}

	tasksByState := make(map[string]int)
	totalMessages := 0

	for _, key := range deadLetterKeys {
		taskID := strings.TrimPrefix(key, deadLetterKeyPrefix)
		if task, exists := s.GetTask(taskID); exists {
			state := string(task.Status.State)
			tasksByState[state]++
			totalMessages += len(task.History)
		}
	}

	contextPattern := contextTasksPrefix + "*"
	contextKeys, err := s.client.Keys(ctx, contextPattern).Result()
	if err != nil {
		s.logger.Error("failed to get context stats", zap.Error(err))
		return StorageStats{}
	}

	contextsWithTasks := len(contextKeys)
	totalContexts := contextsWithTasks

	var avgTasksPerContext float64
	var avgMessagesPerContext float64

	if totalContexts > 0 {
		avgTasksPerContext = float64(len(deadLetterKeys)) / float64(totalContexts)
		avgMessagesPerContext = float64(totalMessages) / float64(totalContexts)
	}

	return StorageStats{
		TotalTasks:                len(deadLetterKeys),
		TasksByState:              tasksByState,
		TotalContexts:             totalContexts,
		ContextsWithTasks:         contextsWithTasks,
		AverageTasksPerContext:    avgTasksPerContext,
		TotalMessages:             totalMessages,
		AverageMessagesPerContext: avgMessagesPerContext,
	}
}

// init registers the Redis storage provider
func init() {
	RegisterStorageProvider("redis", &RedisStorageFactory{})
}
