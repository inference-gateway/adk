package server

import (
	"fmt"
	"sync"

	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// Storage defines the interface for managing conversation history and task data
// The storage is designed to support multiple tasks per context ID and efficient retrieval
type Storage interface {
	// Task Management
	StoreTask(task *types.Task) error
	GetTask(taskID string) (*types.Task, bool)
	GetTaskByContextAndID(contextID, taskID string) (*types.Task, bool)
	UpdateTask(task *types.Task) error
	DeleteTask(taskID string) error
	ListTasks(filter TaskFilter) ([]*types.Task, error)
	ListTasksByContext(contextID string, filter TaskFilter) ([]*types.Task, error)

	// Conversation History Management
	GetConversationHistory(contextID string) []types.Message
	UpdateConversationHistory(contextID string, messages []types.Message)
	AddMessageToConversation(contextID string, message types.Message) error
	TrimConversationHistory(contextID string, maxMessages int) error

	// Context Management
	GetContexts() []string
	GetContextsWithTasks() []string
	DeleteContext(contextID string) error
	DeleteContextAndTasks(contextID string) error

	// Cleanup Operations
	CleanupCompletedTasks() int
	CleanupOldConversations(maxAge int64) int

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
	logger                 *zap.Logger
	tasks                  map[string]*types.Task
	tasksByContext         map[string][]string
	conversationHistory    map[string][]types.Message
	maxConversationHistory int
	tasksMu                sync.RWMutex
	conversationMu         sync.RWMutex
}

// NewInMemoryStorage creates a new in-memory storage instance
func NewInMemoryStorage(logger *zap.Logger, maxConversationHistory int) *InMemoryStorage {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &InMemoryStorage{
		logger:                 logger,
		tasks:                  make(map[string]*types.Task),
		tasksByContext:         make(map[string][]string),
		conversationHistory:    make(map[string][]types.Message),
		maxConversationHistory: maxConversationHistory,
	}
}

// GetConversationHistory retrieves conversation history for a context ID
func (s *InMemoryStorage) GetConversationHistory(contextID string) []types.Message {
	s.conversationMu.RLock()
	defer s.conversationMu.RUnlock()

	if history, exists := s.conversationHistory[contextID]; exists {
		result := make([]types.Message, len(history))
		copy(result, history)
		return result
	}

	return []types.Message{}
}

// UpdateConversationHistory updates conversation history for a context ID
func (s *InMemoryStorage) UpdateConversationHistory(contextID string, messages []types.Message) {
	s.conversationMu.Lock()
	defer s.conversationMu.Unlock()

	history := make([]types.Message, len(messages))
	copy(history, messages)

	trimmedHistory := s.trimConversationHistoryInternal(history)
	s.conversationHistory[contextID] = trimmedHistory

	s.logger.Debug("conversation history updated",
		zap.String("context_id", contextID),
		zap.Int("message_count", len(trimmedHistory)),
		zap.Int("max_history", s.maxConversationHistory))
}

// AddMessageToConversation adds a single message to conversation history
func (s *InMemoryStorage) AddMessageToConversation(contextID string, message types.Message) error {
	s.conversationMu.Lock()
	defer s.conversationMu.Unlock()

	history := s.conversationHistory[contextID]
	if history == nil {
		history = make([]types.Message, 0, 1)
	}

	for _, existingMsg := range history {
		if existingMsg.MessageID == message.MessageID {
			s.logger.Warn("attempted to add duplicate message to conversation",
				zap.String("context_id", contextID),
				zap.String("message_id", message.MessageID))
			return nil
		}
	}

	history = append(history, message)
	trimmedHistory := s.trimConversationHistoryInternal(history)
	s.conversationHistory[contextID] = trimmedHistory

	s.logger.Debug("message added to conversation history",
		zap.String("context_id", contextID),
		zap.String("message_id", message.MessageID),
		zap.String("role", message.Role),
		zap.Int("total_messages", len(trimmedHistory)))

	return nil
}

// TrimConversationHistory trims conversation history to max messages
func (s *InMemoryStorage) TrimConversationHistory(contextID string, maxMessages int) error {
	s.conversationMu.Lock()
	defer s.conversationMu.Unlock()

	history := s.conversationHistory[contextID]
	if history == nil || len(history) <= maxMessages {
		return nil
	}

	startIndex := len(history) - maxMessages
	trimmed := make([]types.Message, maxMessages)
	copy(trimmed, history[startIndex:])
	s.conversationHistory[contextID] = trimmed

	s.logger.Debug("conversation history trimmed",
		zap.String("context_id", contextID),
		zap.Int("original_length", len(history)),
		zap.Int("trimmed_length", len(trimmed)))

	return nil
}

// trimConversationHistoryInternal ensures conversation history doesn't exceed the maximum allowed size
func (s *InMemoryStorage) trimConversationHistoryInternal(history []types.Message) []types.Message {
	if s.maxConversationHistory <= 0 {
		return []types.Message{}
	}

	if len(history) <= s.maxConversationHistory {
		return history
	}

	startIndex := len(history) - s.maxConversationHistory
	trimmed := make([]types.Message, s.maxConversationHistory)
	copy(trimmed, history[startIndex:])

	return trimmed
}

// StoreTask stores a task and maintains context->tasks mapping
func (s *InMemoryStorage) StoreTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	taskCopy := *task
	s.tasks[task.ID] = &taskCopy

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

	s.logger.Debug("task stored",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(task.Status.State)))

	return nil
}

// GetTask retrieves a task by ID
func (s *InMemoryStorage) GetTask(taskID string) (*types.Task, bool) {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, false
	}

	taskCopy := *task
	return &taskCopy, true
}

// GetTaskByContextAndID retrieves a task by context ID and task ID
func (s *InMemoryStorage) GetTaskByContextAndID(contextID, taskID string) (*types.Task, bool) {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, false
	}

	if task.ContextID != contextID {
		return nil, false
	}

	taskCopy := *task
	return &taskCopy, true
}

// UpdateTask updates an existing task
func (s *InMemoryStorage) UpdateTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	if _, exists := s.tasks[task.ID]; !exists {
		return fmt.Errorf("task not found: %s", task.ID)
	}

	taskCopy := *task
	s.tasks[task.ID] = &taskCopy

	s.logger.Debug("task updated",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(task.Status.State)))

	return nil
}

// DeleteTask deletes a task and cleans up context mapping
func (s *InMemoryStorage) DeleteTask(taskID string) error {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	contextID := task.ContextID

	delete(s.tasks, taskID)

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

// ListTasks retrieves a list of tasks based on the provided filter
func (s *InMemoryStorage) ListTasks(filter TaskFilter) ([]*types.Task, error) {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	var filteredTasks []*types.Task

	for _, task := range s.tasks {
		if filter.State != nil && task.Status.State != *filter.State {
			continue
		}

		if filter.ContextID != nil && task.ContextID != *filter.ContextID {
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

// ListTasksByContext retrieves tasks for a specific context with filtering
func (s *InMemoryStorage) ListTasksByContext(contextID string, filter TaskFilter) ([]*types.Task, error) {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	contextTasks, exists := s.tasksByContext[contextID]
	if !exists {
		return []*types.Task{}, nil
	}

	var filteredTasks []*types.Task

	for _, taskID := range contextTasks {
		task, exists := s.tasks[taskID]
		if !exists {
			s.logger.Warn("task not found in tasks map but exists in context mapping",
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

// GetContexts returns all context IDs that have conversation history
func (s *InMemoryStorage) GetContexts() []string {
	s.conversationMu.RLock()
	defer s.conversationMu.RUnlock()

	contexts := make([]string, 0, len(s.conversationHistory))
	for contextID := range s.conversationHistory {
		contexts = append(contexts, contextID)
	}

	return contexts
}

// DeleteContext deletes all conversation history for a context
func (s *InMemoryStorage) DeleteContext(contextID string) error {
	s.conversationMu.Lock()
	defer s.conversationMu.Unlock()

	if _, exists := s.conversationHistory[contextID]; !exists {
		return fmt.Errorf("context not found: %s", contextID)
	}

	delete(s.conversationHistory, contextID)
	s.logger.Debug("context deleted", zap.String("context_id", contextID))

	return nil
}

// CleanupCompletedTasks removes completed, failed, and canceled tasks
func (s *InMemoryStorage) CleanupCompletedTasks() int {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	var toRemove []string
	contextUpdates := make(map[string][]string) // contextID -> taskIDs to remove

	for taskID, task := range s.tasks {
		switch task.Status.State {
		case types.TaskStateCompleted, types.TaskStateFailed, types.TaskStateCanceled:
			toRemove = append(toRemove, taskID)
			contextUpdates[task.ContextID] = append(contextUpdates[task.ContextID], taskID)
		}
	}

	for _, taskID := range toRemove {
		delete(s.tasks, taskID)
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

	if len(toRemove) > 0 {
		s.logger.Info("cleaned up completed tasks", zap.Int("count", len(toRemove)))
	}

	return len(toRemove)
}

// CleanupOldConversations removes conversations older than maxAge (in seconds)
func (s *InMemoryStorage) CleanupOldConversations(maxAge int64) int {
	// For in-memory storage, we don't track timestamps, so this is a no-op
	// In a persistent storage implementation, this would remove old conversations
	return 0
}

// GetContextsWithTasks returns all context IDs that have tasks
func (s *InMemoryStorage) GetContextsWithTasks() []string {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	contexts := make([]string, 0, len(s.tasksByContext))
	for contextID := range s.tasksByContext {
		contexts = append(contexts, contextID)
	}

	return contexts
}

// DeleteContextAndTasks deletes all conversation history and tasks for a context
func (s *InMemoryStorage) DeleteContextAndTasks(contextID string) error {
	s.conversationMu.Lock()
	delete(s.conversationHistory, contextID)
	s.conversationMu.Unlock()

	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	taskIDs, exists := s.tasksByContext[contextID]
	if exists {
		for _, taskID := range taskIDs {
			delete(s.tasks, taskID)
		}
		delete(s.tasksByContext, contextID)
	}

	s.logger.Debug("context and tasks deleted", zap.String("context_id", contextID))
	return nil
}

// GetStats provides statistics about the storage
func (s *InMemoryStorage) GetStats() StorageStats {
	s.tasksMu.RLock()
	s.conversationMu.RLock()
	defer s.tasksMu.RUnlock()
	defer s.conversationMu.RUnlock()

	tasksByState := make(map[string]int)
	for _, task := range s.tasks {
		state := string(task.Status.State)
		tasksByState[state]++
	}

	totalMessages := 0
	for _, messages := range s.conversationHistory {
		totalMessages += len(messages)
	}

	contextsWithTasks := len(s.tasksByContext)
	totalContexts := len(s.conversationHistory)

	for contextID := range s.tasksByContext {
		if _, exists := s.conversationHistory[contextID]; !exists {
			totalContexts++
		}
	}

	var avgTasksPerContext float64
	var avgMessagesPerContext float64

	if totalContexts > 0 {
		avgTasksPerContext = float64(len(s.tasks)) / float64(totalContexts)
		avgMessagesPerContext = float64(totalMessages) / float64(len(s.conversationHistory))
	}

	return StorageStats{
		TotalTasks:                len(s.tasks),
		TasksByState:              tasksByState,
		TotalContexts:             totalContexts,
		ContextsWithTasks:         contextsWithTasks,
		AverageTasksPerContext:    avgTasksPerContext,
		TotalMessages:             totalMessages,
		AverageMessagesPerContext: avgMessagesPerContext,
	}
}
