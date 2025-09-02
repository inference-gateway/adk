package server

import (
	"context"
	"fmt"
	"sync"

	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// QueuedTask represents a task in the processing queue
type QueuedTask struct {
	Task      *types.Task
	RequestID any
}

// Storage defines the interface for queue-centric task management
// Tasks carry their complete message history and flow through: Queue -> Processing -> Dead Letter
type Storage interface {
	// Task Queue Management (primary storage for active tasks)
	EnqueueTask(task *types.Task, requestID any) error
	DequeueTask(ctx context.Context) (*QueuedTask, error)
	GetQueueLength() int
	ClearQueue() error

	// Active Task Queries (for tasks currently in queue or being processed)
	GetActiveTask(taskID string) (*types.Task, error)
	CreateActiveTask(task *types.Task) error
	UpdateActiveTask(task *types.Task) error

	// Dead Letter Queue (completed/failed tasks with full history for audit)
	StoreDeadLetterTask(task *types.Task) error
	GetTask(taskID string) (*types.Task, bool)
	GetTaskByContextAndID(contextID, taskID string) (*types.Task, bool)
	DeleteTask(taskID string) error
	ListTasks(filter TaskFilter) ([]*types.Task, error)
	ListTasksByContext(contextID string, filter TaskFilter) ([]*types.Task, error)

	// Context Management (contexts are implicit from tasks)
	GetContexts() []string
	GetContextsWithTasks() []string
	DeleteContext(contextID string) error
	DeleteContextAndTasks(contextID string) error

	// Cleanup Operations
	CleanupCompletedTasks() int
	CleanupTasksWithRetention(maxCompleted, maxFailed int) int

	// Health and Statistics
	GetStats() StorageStats
}

// TaskFilter defines filtering criteria for listing tasks
type TaskFilter struct {
	State     *types.TaskState
	ContextID *string
	Limit     int
	Offset    int
	SortBy    TaskSortField
	SortOrder SortOrder
}

// TaskSortField defines the fields that can be used for sorting tasks
type TaskSortField string

const (
	TaskSortFieldCreatedAt TaskSortField = "created_at"
	TaskSortFieldUpdatedAt TaskSortField = "updated_at"
	TaskSortFieldState     TaskSortField = "state"
	TaskSortFieldContextID TaskSortField = "context_id"
)

// SortOrder defines the sort order
type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

// StorageStats provides statistics about the storage
type StorageStats struct {
	TotalTasks                int            `json:"total_tasks"`
	TasksByState              map[string]int `json:"tasks_by_state"`
	TotalContexts             int            `json:"total_contexts"`
	ContextsWithTasks         int            `json:"contexts_with_tasks"`
	AverageTasksPerContext    float64        `json:"average_tasks_per_context"`
	TotalMessages             int            `json:"total_messages"`
	AverageMessagesPerContext float64        `json:"average_messages_per_context"`
}

// InMemoryStorage implements Storage interface using in-memory storage
type InMemoryStorage struct {
	logger *zap.Logger

	// Active tasks (in queue only - no persistent storage for active tasks)
	activeTasksMetadata map[string]*types.Task
	activeTasksMu       sync.RWMutex

	// Dead letter queue (completed/failed tasks for audit)
	deadLetterTasks map[string]*types.Task
	tasksByContext  map[string][]string
	deadLetterMu    sync.RWMutex

	// Task queue for processing (primary storage for active tasks)
	taskQueue   []*QueuedTask
	queueMu     sync.RWMutex
	queueNotify chan struct{}
}

// NewInMemoryStorage creates a new in-memory storage instance
func NewInMemoryStorage(logger *zap.Logger, maxConversationHistory int) *InMemoryStorage {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &InMemoryStorage{
		logger:              logger,
		activeTasksMetadata: make(map[string]*types.Task),
		deadLetterTasks:     make(map[string]*types.Task),
		tasksByContext:      make(map[string][]string),
		taskQueue:           make([]*QueuedTask, 0),
		queueNotify:         make(chan struct{}, 1000), // Buffered channel for queue notifications
	}
}

// GetActiveTask retrieves an active task by ID (from queue or processing)
func (s *InMemoryStorage) GetActiveTask(taskID string) (*types.Task, error) {
	s.activeTasksMu.RLock()
	defer s.activeTasksMu.RUnlock()

	task, exists := s.activeTasksMetadata[taskID]
	if !exists {
		return nil, fmt.Errorf("active task not found: %s", taskID)
	}

	taskCopy := *task
	return &taskCopy, nil
}

// CreateActiveTask creates a new active task in the active tasks storage
func (s *InMemoryStorage) CreateActiveTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	s.activeTasksMu.Lock()
	defer s.activeTasksMu.Unlock()

	if _, exists := s.activeTasksMetadata[task.ID]; exists {
		return fmt.Errorf("active task already exists: %s", task.ID)
	}

	taskCopy := *task
	s.activeTasksMetadata[task.ID] = &taskCopy

	s.logger.Debug("active task created",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(task.Status.State)))

	return nil
}

// UpdateActiveTask updates an active task's metadata
func (s *InMemoryStorage) UpdateActiveTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	s.activeTasksMu.Lock()
	defer s.activeTasksMu.Unlock()

	if _, exists := s.activeTasksMetadata[task.ID]; !exists {
		return fmt.Errorf("active task not found: %s", task.ID)
	}

	taskCopy := *task
	s.activeTasksMetadata[task.ID] = &taskCopy

	s.logger.Debug("active task updated",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(task.Status.State)))

	return nil
}

// StoreDeadLetterTask stores a completed/failed task in the dead letter queue for audit
func (s *InMemoryStorage) StoreDeadLetterTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	s.deadLetterMu.Lock()
	defer s.deadLetterMu.Unlock()

	taskCopy := *task
	s.deadLetterTasks[task.ID] = &taskCopy

	contextTasks := s.tasksByContext[task.ContextID]

	found := false
	for _, existingTaskID := range contextTasks {
		if existingTaskID == task.ID {
			found = true
			break
		}
	}

	if !found {
		s.tasksByContext[task.ContextID] = append(contextTasks, task.ID)
	}

	s.activeTasksMu.Lock()
	delete(s.activeTasksMetadata, task.ID)
	s.activeTasksMu.Unlock()

	s.logger.Debug("task stored in dead letter queue",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(task.Status.State)))

	return nil
}

// GetTask retrieves a task by ID from dead letter queue
func (s *InMemoryStorage) GetTask(taskID string) (*types.Task, bool) {
	s.deadLetterMu.RLock()
	defer s.deadLetterMu.RUnlock()

	task, exists := s.deadLetterTasks[taskID]
	if !exists {
		return nil, false
	}

	taskCopy := *task
	return &taskCopy, true
}

// GetTaskByContextAndID retrieves a task by context ID and task ID from dead letter queue
func (s *InMemoryStorage) GetTaskByContextAndID(contextID, taskID string) (*types.Task, bool) {
	s.deadLetterMu.RLock()
	defer s.deadLetterMu.RUnlock()

	task, exists := s.deadLetterTasks[taskID]
	if !exists {
		return nil, false
	}

	if task.ContextID != contextID {
		return nil, false
	}

	taskCopy := *task
	return &taskCopy, true
}

// DeleteTask deletes a task from dead letter queue and cleans up context mapping
func (s *InMemoryStorage) DeleteTask(taskID string) error {
	s.deadLetterMu.Lock()
	defer s.deadLetterMu.Unlock()

	task, exists := s.deadLetterTasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	contextID := task.ContextID

	delete(s.deadLetterTasks, taskID)

	contextTasks := s.tasksByContext[contextID]
	for i, existingTaskID := range contextTasks {
		if existingTaskID == taskID {
			s.tasksByContext[contextID] = append(contextTasks[:i], contextTasks[i+1:]...)
			break
		}
	}

	if len(s.tasksByContext[contextID]) == 0 {
		delete(s.tasksByContext, contextID)
	}

	s.logger.Debug("task deleted",
		zap.String("task_id", taskID),
		zap.String("context_id", contextID))

	return nil
}

// ListTasks retrieves a list of tasks based on the provided filter from both active and dead letter queues
func (s *InMemoryStorage) ListTasks(filter TaskFilter) ([]*types.Task, error) {
	var filteredTasks []*types.Task

	s.deadLetterMu.RLock()
	for _, task := range s.deadLetterTasks {
		if filter.State != nil && task.Status.State != *filter.State {
			continue
		}

		if filter.ContextID != nil && task.ContextID != *filter.ContextID {
			continue
		}

		taskCopy := *task
		filteredTasks = append(filteredTasks, &taskCopy)
	}
	s.deadLetterMu.RUnlock()

	queueTaskIDs := make(map[string]bool)
	s.queueMu.RLock()
	for _, queuedTask := range s.taskQueue {
		task := queuedTask.Task
		queueTaskIDs[task.ID] = true

		if filter.State != nil && task.Status.State != *filter.State {
			continue
		}

		if filter.ContextID != nil && task.ContextID != *filter.ContextID {
			continue
		}

		taskCopy := *task
		filteredTasks = append(filteredTasks, &taskCopy)
	}
	s.queueMu.RUnlock()

	s.activeTasksMu.RLock()
	for _, task := range s.activeTasksMetadata {
		task := task
		if filter.State != nil && task.Status.State != *filter.State {
			continue
		}

		if filter.ContextID != nil && task.ContextID != *filter.ContextID {
			continue
		}

		if !queueTaskIDs[task.ID] {
			taskCopy := *task
			filteredTasks = append(filteredTasks, &taskCopy)
		}
	}
	s.activeTasksMu.RUnlock()

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

// ListTasksByContext retrieves tasks for a specific context with filtering from dead letter queue
func (s *InMemoryStorage) ListTasksByContext(contextID string, filter TaskFilter) ([]*types.Task, error) {
	s.deadLetterMu.RLock()
	defer s.deadLetterMu.RUnlock()

	contextTasks, exists := s.tasksByContext[contextID]
	if !exists {
		return []*types.Task{}, nil
	}

	var filteredTasks []*types.Task

	for _, taskID := range contextTasks {
		task, exists := s.deadLetterTasks[taskID]
		if !exists {
			s.logger.Warn("task not found in dead letter tasks but exists in context mapping",
				zap.String("task_id", taskID),
				zap.String("context_id", contextID))
			continue
		}

		if filter.State != nil && task.Status.State != *filter.State {
			continue
		}

		taskCopy := *task
		filteredTasks = append(filteredTasks, &taskCopy)
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

// sortTasks sorts tasks based on the specified field and order
func (s *InMemoryStorage) sortTasks(tasks []*types.Task, sortBy TaskSortField, order SortOrder) {
	if len(tasks) <= 1 {
		return
	}

	for i := 0; i < len(tasks)-1; i++ {
		for j := 0; j < len(tasks)-i-1; j++ {
			var shouldSwap bool

			switch sortBy {
			case TaskSortFieldCreatedAt, TaskSortFieldUpdatedAt:
				var time1, time2 string
				if tasks[j].Status.Timestamp != nil {
					time1 = *tasks[j].Status.Timestamp
				}
				if tasks[j+1].Status.Timestamp != nil {
					time2 = *tasks[j+1].Status.Timestamp
				}

				if order == SortOrderAsc {
					shouldSwap = time1 > time2
				} else {
					shouldSwap = time1 < time2
				}
			case TaskSortFieldState:
				if order == SortOrderAsc {
					shouldSwap = string(tasks[j].Status.State) > string(tasks[j+1].Status.State)
				} else {
					shouldSwap = string(tasks[j].Status.State) < string(tasks[j+1].Status.State)
				}
			case TaskSortFieldContextID:
				if order == SortOrderAsc {
					shouldSwap = tasks[j].ContextID > tasks[j+1].ContextID
				} else {
					shouldSwap = tasks[j].ContextID < tasks[j+1].ContextID
				}
			default:
				if order == SortOrderAsc {
					shouldSwap = tasks[j].ID > tasks[j+1].ID
				} else {
					shouldSwap = tasks[j].ID < tasks[j+1].ID
				}
			}

			if shouldSwap {
				tasks[j], tasks[j+1] = tasks[j+1], tasks[j]
			}
		}
	}
}

// GetContexts returns all context IDs that have tasks (both active and dead letter)
func (s *InMemoryStorage) GetContexts() []string {
	contextSet := make(map[string]bool)

	s.deadLetterMu.RLock()
	for contextID := range s.tasksByContext {
		contextSet[contextID] = true
	}
	s.deadLetterMu.RUnlock()

	s.queueMu.RLock()
	for _, queuedTask := range s.taskQueue {
		contextSet[queuedTask.Task.ContextID] = true
	}
	s.queueMu.RUnlock()

	s.activeTasksMu.RLock()
	for _, task := range s.activeTasksMetadata {
		contextSet[task.ContextID] = true
	}
	s.activeTasksMu.RUnlock()

	contexts := make([]string, 0, len(contextSet))
	for contextID := range contextSet {
		contexts = append(contexts, contextID)
	}

	return contexts
}

// DeleteContext deletes all tasks for a context (not applicable since no conversation history)
func (s *InMemoryStorage) DeleteContext(contextID string) error {
	// Since we don't have separate conversation history,
	// this is equivalent to deleting all tasks for the context
	return s.DeleteContextAndTasks(contextID)
}

// CleanupCompletedTasks removes completed, failed, and canceled tasks
func (s *InMemoryStorage) CleanupCompletedTasks() int {
	s.deadLetterMu.Lock()
	defer s.deadLetterMu.Unlock()

	var toRemove []string
	contextUpdates := make(map[string][]string)

	for taskID, task := range s.deadLetterTasks {
		switch task.Status.State {
		case types.TaskStateCompleted, types.TaskStateFailed, types.TaskStateCanceled:
			toRemove = append(toRemove, taskID)
			contextUpdates[task.ContextID] = append(contextUpdates[task.ContextID], taskID)
		}
	}

	for _, taskID := range toRemove {
		delete(s.deadLetterTasks, taskID)
	}

	for contextID, taskIDsToRemove := range contextUpdates {
		contextTasks := s.tasksByContext[contextID]

		var remainingTasks []string
		for _, existingTaskID := range contextTasks {
			found := false
			for _, removedTaskID := range taskIDsToRemove {
				if existingTaskID == removedTaskID {
					found = true
					break
				}
			}
			if !found {
				remainingTasks = append(remainingTasks, existingTaskID)
			}
		}

		if len(remainingTasks) == 0 {
			delete(s.tasksByContext, contextID)
		} else {
			s.tasksByContext[contextID] = remainingTasks
		}
	}

	return len(toRemove)
}

// CleanupTasksWithRetention removes old completed and failed tasks while keeping the specified number of most recent ones
func (s *InMemoryStorage) CleanupTasksWithRetention(maxCompleted, maxFailed int) int {
	s.deadLetterMu.Lock()
	defer s.deadLetterMu.Unlock()

	completedTasks := make([]*types.Task, 0)
	failedTasks := make([]*types.Task, 0)

	for _, task := range s.deadLetterTasks {
		switch task.Status.State {
		case types.TaskStateCompleted:
			completedTasks = append(completedTasks, task)
		case types.TaskStateFailed:
			failedTasks = append(failedTasks, task)
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

	for _, taskID := range toRemove {
		delete(s.deadLetterTasks, taskID)
	}

	for contextID, taskIDsToRemove := range contextUpdates {
		contextTasks := s.tasksByContext[contextID]

		var remainingTasks []string
		for _, existingTaskID := range contextTasks {
			found := false
			for _, removedTaskID := range taskIDsToRemove {
				if existingTaskID == removedTaskID {
					found = true
					break
				}
			}
			if !found {
				remainingTasks = append(remainingTasks, existingTaskID)
			}
		}

		if len(remainingTasks) == 0 {
			delete(s.tasksByContext, contextID)
		} else {
			s.tasksByContext[contextID] = remainingTasks
		}
	}

	return len(toRemove)
}

// sortTasksByTimestamp sorts tasks by timestamp (newest first if desc=true)
func (s *InMemoryStorage) sortTasksByTimestamp(tasks []*types.Task, desc bool) {
	if len(tasks) <= 1 {
		return
	}

	for i := 0; i < len(tasks)-1; i++ {
		for j := 0; j < len(tasks)-i-1; j++ {
			var time1, time2 string
			if tasks[j].Status.Timestamp != nil {
				time1 = *tasks[j].Status.Timestamp
			}
			if tasks[j+1].Status.Timestamp != nil {
				time2 = *tasks[j+1].Status.Timestamp
			}

			var shouldSwap bool
			if desc {
				shouldSwap = time1 < time2 // For descending (newest first)
			} else {
				shouldSwap = time1 > time2 // For ascending (oldest first)
			}

			if shouldSwap {
				tasks[j], tasks[j+1] = tasks[j+1], tasks[j]
			}
		}
	}
}

// CleanupOldConversations removes conversations older than maxAge (in seconds)
func (s *InMemoryStorage) CleanupOldConversations(maxAge int64) int {
	// For in-memory storage, we don't track timestamps, so this is a no-op
	// In a persistent storage implementation, this would remove old conversations
	return 0
}

// GetContextsWithTasks returns all context IDs that have tasks
func (s *InMemoryStorage) GetContextsWithTasks() []string {
	s.deadLetterMu.RLock()
	defer s.deadLetterMu.RUnlock()

	contexts := make([]string, 0, len(s.tasksByContext))
	for contextID := range s.tasksByContext {
		contexts = append(contexts, contextID)
	}

	return contexts
}

// DeleteContextAndTasks deletes all tasks for a context
func (s *InMemoryStorage) DeleteContextAndTasks(contextID string) error {
	s.queueMu.Lock()
	s.deadLetterMu.Lock()
	defer s.queueMu.Unlock()
	defer s.deadLetterMu.Unlock()

	filteredQueue := make([]*QueuedTask, 0)
	for _, queuedTask := range s.taskQueue {
		if queuedTask.Task.ContextID != contextID {
			filteredQueue = append(filteredQueue, queuedTask)
		} else {
			delete(s.activeTasksMetadata, queuedTask.Task.ID)
		}
	}
	s.taskQueue = filteredQueue

	taskIDs, exists := s.tasksByContext[contextID]
	if exists {
		for _, taskID := range taskIDs {
			delete(s.deadLetterTasks, taskID)
		}
		delete(s.tasksByContext, contextID)
	}

	s.logger.Debug("context and tasks deleted", zap.String("context_id", contextID))
	return nil
}

// GetStats provides statistics about the storage
func (s *InMemoryStorage) GetStats() StorageStats {
	s.deadLetterMu.RLock()
	defer s.deadLetterMu.RUnlock()

	tasksByState := make(map[string]int)
	totalMessages := 0

	for _, task := range s.deadLetterTasks {
		state := string(task.Status.State)
		tasksByState[state]++
		totalMessages += len(task.History)
	}

	contextsWithTasks := len(s.tasksByContext)
	totalContexts := contextsWithTasks

	var avgTasksPerContext float64
	var avgMessagesPerContext float64

	if totalContexts > 0 {
		avgTasksPerContext = float64(len(s.deadLetterTasks)) / float64(totalContexts)
		avgMessagesPerContext = float64(totalMessages) / float64(totalContexts)
	}

	return StorageStats{
		TotalTasks:                len(s.deadLetterTasks),
		TasksByState:              tasksByState,
		TotalContexts:             totalContexts,
		ContextsWithTasks:         contextsWithTasks,
		AverageTasksPerContext:    avgTasksPerContext,
		TotalMessages:             totalMessages,
		AverageMessagesPerContext: avgMessagesPerContext,
	}
}

// EnqueueTask adds a task to the processing queue
func (s *InMemoryStorage) EnqueueTask(task *types.Task, requestID any) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	queuedTask := &QueuedTask{
		Task:      task,
		RequestID: requestID,
	}

	s.queueMu.Lock()
	s.taskQueue = append(s.taskQueue, queuedTask)
	queueLength := len(s.taskQueue)
	s.queueMu.Unlock()

	s.activeTasksMu.Lock()
	taskCopy := *task
	s.activeTasksMetadata[task.ID] = &taskCopy
	s.activeTasksMu.Unlock()

	select {
	case s.queueNotify <- struct{}{}:
	default:
		// Channel is full, but that's OK - multiple notifications for the same state are redundant
	}

	s.logger.Info("task enqueued for processing",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.Int("queue_length", queueLength))

	return nil
}

// DequeueTask retrieves and removes the next task from the processing queue
// Blocks until a task is available or context is cancelled
func (s *InMemoryStorage) DequeueTask(ctx context.Context) (*QueuedTask, error) {
	for {
		s.queueMu.Lock()
		if len(s.taskQueue) > 0 {
			task := s.taskQueue[0]
			s.taskQueue = s.taskQueue[1:]
			remainingQueueLength := len(s.taskQueue)
			s.queueMu.Unlock()

			s.logger.Info("task dequeued for processing",
				zap.String("task_id", task.Task.ID),
				zap.String("context_id", task.Task.ContextID),
				zap.Int("remaining_queue_length", remainingQueueLength))

			return task, nil
		}
		s.queueMu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.queueNotify:
			continue
		}
	}
}

// GetQueueLength returns the current number of tasks in the queue
func (s *InMemoryStorage) GetQueueLength() int {
	s.queueMu.RLock()
	defer s.queueMu.RUnlock()
	return len(s.taskQueue)
}

// ClearQueue removes all tasks from the queue
func (s *InMemoryStorage) ClearQueue() error {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()

	queueLength := len(s.taskQueue)
	s.taskQueue = make([]*QueuedTask, 0)

	s.logger.Info("task queue cleared", zap.Int("removed_tasks", queueLength))
	return nil
}
