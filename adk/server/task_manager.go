package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	uuid "github.com/google/uuid"
	adk "github.com/inference-gateway/a2a/adk"
	zap "go.uber.org/zap"
)

// TaskManager defines task lifecycle management
type TaskManager interface {
	// CreateTask creates a new task and stores it
	CreateTask(contextID string, state adk.TaskState, message *adk.Message) *adk.Task

	// UpdateTask updates an existing task
	UpdateTask(taskID string, state adk.TaskState, message *adk.Message) error

	// GetTask retrieves a task by ID
	GetTask(taskID string) (*adk.Task, bool)

	// ListTasks retrieves a list of tasks based on the provided parameters
	ListTasks(params adk.TaskListParams) (*adk.TaskList, error)

	// CancelTask cancels a task
	CancelTask(taskID string) error

	// CleanupCompletedTasks removes old completed tasks from memory
	CleanupCompletedTasks()

	// PollTaskStatus periodically checks the status of a task until it is completed or failed
	PollTaskStatus(taskID string, interval time.Duration, timeout time.Duration) (*adk.Task, error)

	// GetConversationHistory retrieves conversation history for a context ID
	GetConversationHistory(contextID string) []adk.Message

	// UpdateConversationHistory updates conversation history for a context ID
	UpdateConversationHistory(contextID string, messages []adk.Message)

	// SetTaskPushNotificationConfig sets push notification configuration for a task
	SetTaskPushNotificationConfig(config adk.TaskPushNotificationConfig) (*adk.TaskPushNotificationConfig, error)

	// GetTaskPushNotificationConfig gets push notification configuration for a task
	GetTaskPushNotificationConfig(params adk.GetTaskPushNotificationConfigParams) (*adk.TaskPushNotificationConfig, error)

	// ListTaskPushNotificationConfigs lists all push notification configurations for a task
	ListTaskPushNotificationConfigs(params adk.ListTaskPushNotificationConfigParams) ([]adk.TaskPushNotificationConfig, error)

	// DeleteTaskPushNotificationConfig deletes a push notification configuration
	DeleteTaskPushNotificationConfig(params adk.DeleteTaskPushNotificationConfigParams) error
}

// DefaultTaskManager implements the TaskManager interface
type DefaultTaskManager struct {
	logger                    *zap.Logger
	tasks                     map[string]*adk.Task
	pushNotificationConfigs   map[string]map[string]*adk.TaskPushNotificationConfig // taskID -> configID -> config
	conversationHistory       map[string][]adk.Message                              // contextID -> conversation history
	maxConversationHistory    int                                                   // maximum number of messages to keep in history
	notificationSender        PushNotificationSender                                // for sending push notifications
	tasksMu                   sync.RWMutex
	pushNotificationConfigsMu sync.RWMutex
	conversationMu            sync.RWMutex
}

// NewDefaultTaskManager creates a new default task manager
func NewDefaultTaskManager(logger *zap.Logger, maxConversationHistory int) *DefaultTaskManager {
	return &DefaultTaskManager{
		logger:                  logger,
		tasks:                   make(map[string]*adk.Task),
		pushNotificationConfigs: make(map[string]map[string]*adk.TaskPushNotificationConfig),
		conversationHistory:     make(map[string][]adk.Message),
		maxConversationHistory:  maxConversationHistory,
		notificationSender:      nil, // Can be set later with SetNotificationSender
	}
}

// NewDefaultTaskManagerWithNotifications creates a new default task manager with push notification support
func NewDefaultTaskManagerWithNotifications(logger *zap.Logger, maxConversationHistory int, notificationSender PushNotificationSender) *DefaultTaskManager {
	return &DefaultTaskManager{
		logger:                  logger,
		tasks:                   make(map[string]*adk.Task),
		pushNotificationConfigs: make(map[string]map[string]*adk.TaskPushNotificationConfig),
		conversationHistory:     make(map[string][]adk.Message),
		maxConversationHistory:  maxConversationHistory,
		notificationSender:      notificationSender,
	}
}

// SetNotificationSender sets the push notification sender
func (tm *DefaultTaskManager) SetNotificationSender(sender PushNotificationSender) {
	tm.notificationSender = sender
}

// CreateTask creates a new task and stores it
func (tm *DefaultTaskManager) CreateTask(contextID string, state adk.TaskState, message *adk.Message) *adk.Task {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)

	history := tm.GetConversationHistory(contextID)

	if message != nil {
		history = append(history, *message)
		tm.UpdateConversationHistory(contextID, history)
		history = tm.GetConversationHistory(contextID)
	}

	task := &adk.Task{
		ID:   uuid.New().String(),
		Kind: "task",
		Status: adk.TaskStatus{
			State:     state,
			Message:   message,
			Timestamp: &timestamp,
		},
		ContextID: contextID,
		History:   history,
	}

	tm.tasks[task.ID] = task
	tm.logger.Debug("task created",
		zap.String("task_id", task.ID),
		zap.String("context_id", contextID),
		zap.String("state", string(state)),
		zap.Int("history_count", len(history)))

	return task
}

// UpdateTask updates an existing task
func (tm *DefaultTaskManager) UpdateTask(taskID string, state adk.TaskState, message *adk.Message) error {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return NewTaskNotFoundError(taskID)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task.Status.State = state
	task.Status.Message = message
	task.Status.Timestamp = &timestamp

	if state == adk.TaskStateCompleted && message != nil && task.ContextID != "" {
		task.History = append(task.History, *message)
		tm.UpdateConversationHistory(task.ContextID, task.History)
	}

	tm.tasks[taskID] = task
	tm.logger.Debug("task updated",
		zap.String("task_id", taskID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(state)),
		zap.Int("history_count", len(task.History)))

	if tm.notificationSender != nil {
		go tm.sendPushNotifications(taskID, task)
	}

	return nil
}

// sendPushNotifications sends push notifications for a task update
func (tm *DefaultTaskManager) sendPushNotifications(taskID string, task *adk.Task) {
	configs, err := tm.ListTaskPushNotificationConfigs(adk.ListTaskPushNotificationConfigParams{
		ID: taskID,
	})
	if err != nil {
		tm.logger.Error("failed to retrieve push notification configs",
			zap.String("task_id", taskID),
			zap.Error(err))
		return
	}

	if len(configs) == 0 {
		tm.logger.Debug("no push notification configs found for task",
			zap.String("task_id", taskID))
		return
	}

	ctx := context.Background()
	for _, config := range configs {
		if err := tm.notificationSender.SendTaskUpdate(ctx, config.PushNotificationConfig, task); err != nil {
			tm.logger.Error("failed to send push notification",
				zap.String("task_id", taskID),
				zap.String("webhook_url", config.PushNotificationConfig.URL),
				zap.Error(err))
		} else {
			tm.logger.Debug("push notification sent successfully",
				zap.String("task_id", taskID),
				zap.String("webhook_url", config.PushNotificationConfig.URL),
				zap.String("state", string(task.Status.State)))
		}
	}
}

// GetTask retrieves a task by ID
func (tm *DefaultTaskManager) GetTask(taskID string) (*adk.Task, bool) {
	tm.tasksMu.RLock()
	defer tm.tasksMu.RUnlock()

	task, exists := tm.tasks[taskID]
	return task, exists
}

// ListTasks retrieves a list of tasks based on the provided parameters
func (tm *DefaultTaskManager) ListTasks(params adk.TaskListParams) (*adk.TaskList, error) {
	tm.tasksMu.RLock()
	defer tm.tasksMu.RUnlock()

	var allTasks []*adk.Task

	for _, task := range tm.tasks {
		if params.State != nil && task.Status.State != *params.State {
			continue
		}

		if params.ContextID != nil && task.ContextID != *params.ContextID {
			continue
		}

		taskCopy := *task

		allTasks = append(allTasks, &taskCopy)
	}

	limit := 50
	if params.Limit > 0 {
		if params.Limit > 100 {
			limit = 100
		} else {
			limit = params.Limit
		}
	}

	offset := 0
	if params.Offset > 0 {
		offset = params.Offset
	}

	total := len(allTasks)

	var paginatedTasks []*adk.Task
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		paginatedTasks = allTasks[offset:end]
	}

	var resultTasks []adk.Task
	for _, taskPtr := range paginatedTasks {
		resultTasks = append(resultTasks, *taskPtr)
	}

	result := &adk.TaskList{
		Tasks:  resultTasks,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	tm.logger.Debug("listed tasks",
		zap.Int("total_count", len(tm.tasks)),
		zap.Int("filtered_count", total),
		zap.Int("returned_count", len(resultTasks)),
		zap.Int("offset", offset),
		zap.Int("limit", limit))

	return result, nil
}

// CancelTask cancels a task
func (tm *DefaultTaskManager) CancelTask(taskID string) error {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return NewTaskNotFoundError(taskID)
	}

	task.Status.State = adk.TaskStateCanceled
	tm.tasks[taskID] = task
	tm.logger.Info("task canceled", zap.String("task_id", taskID))

	return nil
}

// CleanupCompletedTasks removes old completed tasks from memory
func (tm *DefaultTaskManager) CleanupCompletedTasks() {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	var toRemove []string

	for taskID, task := range tm.tasks {
		switch task.Status.State {
		case adk.TaskStateCompleted, adk.TaskStateFailed, adk.TaskStateCanceled:
			toRemove = append(toRemove, taskID)
		}
	}

	for _, taskID := range toRemove {
		delete(tm.tasks, taskID)
	}

	if len(toRemove) > 0 {
		tm.logger.Info("cleaned up completed tasks", zap.Int("count", len(toRemove)))
	}
}

// PollTaskStatus periodically checks the status of a task until it is completed or failed
func (tm *DefaultTaskManager) PollTaskStatus(taskID string, interval time.Duration, timeout time.Duration) (*adk.Task, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-ticker.C:
			task, exists := tm.GetTask(taskID)
			if !exists {
				return nil, NewTaskNotFoundError(taskID)
			}

			if task.Status.State == adk.TaskStateCompleted || task.Status.State == adk.TaskStateFailed {
				return task, nil
			}

		case <-timeoutTimer.C:
			return nil, fmt.Errorf("polling timed out for task %s", taskID)
		}
	}
}

// GetConversationHistory retrieves conversation history for a context ID
func (tm *DefaultTaskManager) GetConversationHistory(contextID string) []adk.Message {
	tm.conversationMu.RLock()
	defer tm.conversationMu.RUnlock()

	if history, exists := tm.conversationHistory[contextID]; exists {
		result := make([]adk.Message, len(history))
		copy(result, history)
		return result
	}

	return []adk.Message{}
}

// UpdateConversationHistory updates conversation history for a context ID
func (tm *DefaultTaskManager) UpdateConversationHistory(contextID string, messages []adk.Message) {
	tm.conversationMu.Lock()
	defer tm.conversationMu.Unlock()

	history := make([]adk.Message, len(messages))
	copy(history, messages)

	trimmedHistory := tm.trimConversationHistory(history)
	tm.conversationHistory[contextID] = trimmedHistory

	tm.logger.Debug("conversation history updated",
		zap.String("context_id", contextID),
		zap.Int("message_count", len(trimmedHistory)),
		zap.Int("max_history", tm.maxConversationHistory))
}

// SetTaskPushNotificationConfig sets push notification configuration for a task
func (tm *DefaultTaskManager) SetTaskPushNotificationConfig(config adk.TaskPushNotificationConfig) (*adk.TaskPushNotificationConfig, error) {
	tm.pushNotificationConfigsMu.Lock()
	defer tm.pushNotificationConfigsMu.Unlock()

	if _, ok := tm.pushNotificationConfigs[config.TaskID]; !ok {
		tm.pushNotificationConfigs[config.TaskID] = make(map[string]*adk.TaskPushNotificationConfig)
	}

	configID := config.PushNotificationConfig.ID
	if configID == nil || *configID == "" {
		id := uuid.New().String()
		config.PushNotificationConfig.ID = &id
		configID = &id
	}

	tm.pushNotificationConfigs[config.TaskID][*configID] = &config

	tm.logger.Debug("push notification config set",
		zap.String("task_id", config.TaskID),
		zap.String("config_id", *configID))

	return &config, nil
}

// GetTaskPushNotificationConfig gets push notification configuration for a task
func (tm *DefaultTaskManager) GetTaskPushNotificationConfig(params adk.GetTaskPushNotificationConfigParams) (*adk.TaskPushNotificationConfig, error) {
	tm.pushNotificationConfigsMu.RLock()
	defer tm.pushNotificationConfigsMu.RUnlock()

	if configs, ok := tm.pushNotificationConfigs[params.ID]; ok {
		if params.PushNotificationConfigID != nil {
			if config, ok := configs[*params.PushNotificationConfigID]; ok {
				return config, nil
			}
			return nil, fmt.Errorf("push notification config not found for task %s, config %s", params.ID, *params.PushNotificationConfigID)
		}

		for _, config := range configs {
			return config, nil
		}
	}

	return nil, fmt.Errorf("no push notification configs found for task %s", params.ID)
}

// ListTaskPushNotificationConfigs lists all push notification configurations for a task
func (tm *DefaultTaskManager) ListTaskPushNotificationConfigs(params adk.ListTaskPushNotificationConfigParams) ([]adk.TaskPushNotificationConfig, error) {
	tm.pushNotificationConfigsMu.RLock()
	defer tm.pushNotificationConfigsMu.RUnlock()

	if configs, ok := tm.pushNotificationConfigs[params.ID]; ok {
		var result []adk.TaskPushNotificationConfig
		for _, config := range configs {
			result = append(result, *config)
		}
		return result, nil
	}

	return []adk.TaskPushNotificationConfig{}, nil
}

// DeleteTaskPushNotificationConfig deletes a push notification configuration
func (tm *DefaultTaskManager) DeleteTaskPushNotificationConfig(params adk.DeleteTaskPushNotificationConfigParams) error {
	tm.pushNotificationConfigsMu.Lock()
	defer tm.pushNotificationConfigsMu.Unlock()

	if configs, ok := tm.pushNotificationConfigs[params.ID]; ok {
		if _, ok := configs[params.PushNotificationConfigID]; ok {
			delete(configs, params.PushNotificationConfigID)
			tm.logger.Info("push notification config deleted",
				zap.String("task_id", params.ID),
				zap.String("config_id", params.PushNotificationConfigID))
			return nil
		}
	}

	return fmt.Errorf("push notification config not found for task %s, config %s", params.ID, params.PushNotificationConfigID)
}

// trimConversationHistory ensures conversation history doesn't exceed the maximum allowed size
// It keeps the most recent messages and removes the oldest ones
func (tm *DefaultTaskManager) trimConversationHistory(history []adk.Message) []adk.Message {
	if tm.maxConversationHistory <= 0 {
		return []adk.Message{}
	}

	if len(history) <= tm.maxConversationHistory {
		return history
	}

	startIndex := len(history) - tm.maxConversationHistory
	trimmed := make([]adk.Message, tm.maxConversationHistory)
	copy(trimmed, history[startIndex:])

	tm.logger.Debug("conversation history trimmed",
		zap.Int("original_length", len(history)),
		zap.Int("trimmed_length", len(trimmed)),
		zap.Int("max_history", tm.maxConversationHistory))

	return trimmed
}

// TaskNotFoundError represents an error when a task is not found
type TaskNotFoundError struct {
	TaskID string
}

func (e *TaskNotFoundError) Error() string {
	return "task not found: " + e.TaskID
}

// NewTaskNotFoundError creates a new TaskNotFoundError
func NewTaskNotFoundError(taskID string) error {
	return &TaskNotFoundError{TaskID: taskID}
}
