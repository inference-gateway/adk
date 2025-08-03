package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	uuid "github.com/google/uuid"
	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// TaskManager defines task lifecycle management
type TaskManager interface {
	// CreateTask creates a new task and stores it
	CreateTask(contextID string, state types.TaskState, message *types.Message) *types.Task

	// UpdateState updates a task's state
	UpdateState(taskID string, state types.TaskState) error

	// UpdateError updates a task to failed state with an error message
	UpdateError(taskID string, message *types.Message) error

	// GetTask retrieves a task by ID
	GetTask(taskID string) (*types.Task, bool)

	// ListTasks retrieves a list of tasks based on the provided parameters
	ListTasks(params types.TaskListParams) (*types.TaskList, error)

	// CancelTask cancels a task
	CancelTask(taskID string) error

	// CleanupCompletedTasks removes old completed tasks from memory
	CleanupCompletedTasks()

	// PollTaskStatus periodically checks the status of a task until it is completed or failed
	PollTaskStatus(taskID string, interval time.Duration, timeout time.Duration) (*types.Task, error)

	// GetConversationHistory retrieves conversation history for a context ID
	GetConversationHistory(contextID string) []types.Message

	// UpdateConversationHistory updates conversation history for a context ID
	UpdateConversationHistory(contextID string, messages []types.Message)

	// SetTaskPushNotificationConfig sets push notification configuration for a task
	SetTaskPushNotificationConfig(config types.TaskPushNotificationConfig) (*types.TaskPushNotificationConfig, error)

	// GetTaskPushNotificationConfig gets push notification configuration for a task
	GetTaskPushNotificationConfig(params types.GetTaskPushNotificationConfigParams) (*types.TaskPushNotificationConfig, error)

	// ListTaskPushNotificationConfigs lists all push notification configurations for a task
	ListTaskPushNotificationConfigs(params types.ListTaskPushNotificationConfigParams) ([]types.TaskPushNotificationConfig, error)

	// DeleteTaskPushNotificationConfig deletes a push notification configuration
	DeleteTaskPushNotificationConfig(params types.DeleteTaskPushNotificationConfigParams) error

	// PauseTaskForInput pauses a task waiting for additional input from the client
	PauseTaskForInput(taskID string, message *types.Message) error

	// ResumeTaskWithInput resumes a paused task with new input from the client
	ResumeTaskWithInput(taskID string, message *types.Message) error

	// IsTaskPaused checks if a task is currently paused (in input-required state)
	IsTaskPaused(taskID string) (bool, error)
}

// DefaultTaskManager implements the TaskManager interface
type DefaultTaskManager struct {
	logger                    *zap.Logger
	tasks                     map[string]*types.Task
	pushNotificationConfigs   map[string]map[string]*types.TaskPushNotificationConfig // taskID -> configID -> config
	conversationHistory       map[string][]types.Message                              // contextID -> conversation history
	maxConversationHistory    int                                                     // maximum number of messages to keep in history
	notificationSender        PushNotificationSender                                  // for sending push notifications
	tasksMu                   sync.RWMutex
	pushNotificationConfigsMu sync.RWMutex
	conversationMu            sync.RWMutex
}

// NewDefaultTaskManager creates a new default task manager
func NewDefaultTaskManager(logger *zap.Logger, maxConversationHistory int) *DefaultTaskManager {
	return &DefaultTaskManager{
		logger:                  logger,
		tasks:                   make(map[string]*types.Task),
		pushNotificationConfigs: make(map[string]map[string]*types.TaskPushNotificationConfig),
		conversationHistory:     make(map[string][]types.Message),
		maxConversationHistory:  maxConversationHistory,
		notificationSender:      nil, // Can be set later with SetNotificationSender
	}
}

// NewDefaultTaskManagerWithNotifications creates a new default task manager with push notification support
func NewDefaultTaskManagerWithNotifications(logger *zap.Logger, maxConversationHistory int, notificationSender PushNotificationSender) *DefaultTaskManager {
	return &DefaultTaskManager{
		logger:                  logger,
		tasks:                   make(map[string]*types.Task),
		pushNotificationConfigs: make(map[string]map[string]*types.TaskPushNotificationConfig),
		conversationHistory:     make(map[string][]types.Message),
		maxConversationHistory:  maxConversationHistory,
		notificationSender:      notificationSender,
	}
}

// SetNotificationSender sets the push notification sender
func (tm *DefaultTaskManager) SetNotificationSender(sender PushNotificationSender) {
	tm.notificationSender = sender
}

// CreateTask creates a new task and stores it
func (tm *DefaultTaskManager) CreateTask(contextID string, state types.TaskState, message *types.Message) *types.Task {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)

	history := tm.GetConversationHistory(contextID)

	if message != nil {
		history = append(history, *message)
		tm.UpdateConversationHistory(contextID, history)
		history = tm.GetConversationHistory(contextID)
	}

	task := &types.Task{
		ID:   uuid.New().String(),
		Kind: "task",
		Status: types.TaskStatus{
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

// UpdateState updates a task's state
func (tm *DefaultTaskManager) UpdateState(taskID string, state types.TaskState) error {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return NewTaskNotFoundError(taskID)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task.Status.State = state
	task.Status.Timestamp = &timestamp

	// Update conversation history for terminal states (history is managed elsewhere during processing)
	if (state == types.TaskStateCompleted || state == types.TaskStateInputRequired) && task.ContextID != "" {
		tm.UpdateConversationHistory(task.ContextID, task.History)
	}

	tm.tasks[taskID] = task
	tm.logger.Debug("task state updated",
		zap.String("task_id", taskID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(state)))

	if tm.notificationSender != nil {
		go tm.sendPushNotifications(taskID, task)
	}

	return nil
}

// UpdateError updates a task to failed state with an error message
func (tm *DefaultTaskManager) UpdateError(taskID string, message *types.Message) error {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return NewTaskNotFoundError(taskID)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task.Status.State = types.TaskStateFailed
	task.Status.Message = message
	task.Status.Timestamp = &timestamp

	tm.tasks[taskID] = task
	tm.logger.Debug("task error updated",
		zap.String("task_id", taskID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(types.TaskStateFailed)),
		zap.Int("history_count", len(task.History)))

	if tm.notificationSender != nil {
		go tm.sendPushNotifications(taskID, task)
	}

	return nil
}

// sendPushNotifications sends push notifications for a task update
func (tm *DefaultTaskManager) sendPushNotifications(taskID string, task *types.Task) {
	configs, err := tm.ListTaskPushNotificationConfigs(types.ListTaskPushNotificationConfigParams{
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
func (tm *DefaultTaskManager) GetTask(taskID string) (*types.Task, bool) {
	tm.tasksMu.RLock()
	defer tm.tasksMu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, false
	}

	taskCopy := *task
	return &taskCopy, true
}

// ListTasks retrieves a list of tasks based on the provided parameters
func (tm *DefaultTaskManager) ListTasks(params types.TaskListParams) (*types.TaskList, error) {
	tm.tasksMu.RLock()
	defer tm.tasksMu.RUnlock()

	var allTasks []*types.Task

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

	var paginatedTasks []*types.Task
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		paginatedTasks = allTasks[offset:end]
	}

	var resultTasks []types.Task
	for _, taskPtr := range paginatedTasks {
		resultTasks = append(resultTasks, *taskPtr)
	}

	result := &types.TaskList{
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

	// Check if task can be canceled based on its current state
	if !tm.isTaskCancelable(task.Status.State) {
		return NewTaskNotCancelableError(taskID, task.Status.State)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task.Status.State = types.TaskStateCanceled
	task.Status.Timestamp = &timestamp
	tm.tasks[taskID] = task
	tm.logger.Info("task canceled", zap.String("task_id", taskID))

	if tm.notificationSender != nil {
		go tm.sendPushNotifications(taskID, task)
	}

	return nil
}

// isTaskCancelable determines if a task can be canceled based on its current state
func (tm *DefaultTaskManager) isTaskCancelable(state types.TaskState) bool {
	switch state {
	case types.TaskStateCompleted, types.TaskStateFailed, types.TaskStateCanceled, types.TaskStateRejected:
		// Cannot cancel tasks that are already in a final state
		return false
	case types.TaskStateSubmitted, types.TaskStateWorking, types.TaskStateInputRequired, types.TaskStateAuthRequired, types.TaskStateUnknown:
		// Can cancel tasks that are still in progress or waiting
		return true
	default:
		// Unknown states cannot be canceled for safety
		return false
	}
}

// CleanupCompletedTasks removes old completed tasks from memory
func (tm *DefaultTaskManager) CleanupCompletedTasks() {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	var toRemove []string

	for taskID, task := range tm.tasks {
		switch task.Status.State {
		case types.TaskStateCompleted, types.TaskStateFailed, types.TaskStateCanceled:
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
func (tm *DefaultTaskManager) PollTaskStatus(taskID string, interval time.Duration, timeout time.Duration) (*types.Task, error) {
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

			switch task.Status.State {
			case types.TaskStateCompleted, types.TaskStateFailed, types.TaskStateCanceled, types.TaskStateRejected:
				return task, nil
			case types.TaskStateInputRequired:
				return task, nil
			}

		case <-timeoutTimer.C:
			return nil, fmt.Errorf("polling timed out for task %s", taskID)
		}
	}
}

// GetConversationHistory retrieves conversation history for a context ID
func (tm *DefaultTaskManager) GetConversationHistory(contextID string) []types.Message {
	tm.conversationMu.RLock()
	defer tm.conversationMu.RUnlock()

	if history, exists := tm.conversationHistory[contextID]; exists {
		result := make([]types.Message, len(history))
		copy(result, history)
		return result
	}

	return []types.Message{}
}

// UpdateConversationHistory updates conversation history for a context ID
func (tm *DefaultTaskManager) UpdateConversationHistory(contextID string, messages []types.Message) {
	tm.conversationMu.Lock()
	defer tm.conversationMu.Unlock()

	history := make([]types.Message, len(messages))
	copy(history, messages)

	trimmedHistory := tm.trimConversationHistory(history)
	tm.conversationHistory[contextID] = trimmedHistory

	tm.logger.Debug("conversation history updated",
		zap.String("context_id", contextID),
		zap.Int("message_count", len(trimmedHistory)),
		zap.Int("max_history", tm.maxConversationHistory))
}

// SetTaskPushNotificationConfig sets push notification configuration for a task
func (tm *DefaultTaskManager) SetTaskPushNotificationConfig(config types.TaskPushNotificationConfig) (*types.TaskPushNotificationConfig, error) {
	tm.pushNotificationConfigsMu.Lock()
	defer tm.pushNotificationConfigsMu.Unlock()

	if _, ok := tm.pushNotificationConfigs[config.TaskID]; !ok {
		tm.pushNotificationConfigs[config.TaskID] = make(map[string]*types.TaskPushNotificationConfig)
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
func (tm *DefaultTaskManager) GetTaskPushNotificationConfig(params types.GetTaskPushNotificationConfigParams) (*types.TaskPushNotificationConfig, error) {
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
func (tm *DefaultTaskManager) ListTaskPushNotificationConfigs(params types.ListTaskPushNotificationConfigParams) ([]types.TaskPushNotificationConfig, error) {
	tm.pushNotificationConfigsMu.RLock()
	defer tm.pushNotificationConfigsMu.RUnlock()

	if configs, ok := tm.pushNotificationConfigs[params.ID]; ok {
		var result []types.TaskPushNotificationConfig
		for _, config := range configs {
			result = append(result, *config)
		}
		return result, nil
	}

	return []types.TaskPushNotificationConfig{}, nil
}

// DeleteTaskPushNotificationConfig deletes a push notification configuration
func (tm *DefaultTaskManager) DeleteTaskPushNotificationConfig(params types.DeleteTaskPushNotificationConfigParams) error {
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
func (tm *DefaultTaskManager) trimConversationHistory(history []types.Message) []types.Message {
	if tm.maxConversationHistory <= 0 {
		return []types.Message{}
	}

	if len(history) <= tm.maxConversationHistory {
		return history
	}

	startIndex := len(history) - tm.maxConversationHistory
	trimmed := make([]types.Message, tm.maxConversationHistory)
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

// TaskNotCancelableError represents an error when a task cannot be canceled due to its current state
type TaskNotCancelableError struct {
	TaskID string
	State  types.TaskState
}

func (e *TaskNotCancelableError) Error() string {
	return fmt.Sprintf("task %s cannot be canceled: current state is %s", e.TaskID, e.State)
}

// NewTaskNotCancelableError creates a new TaskNotCancelableError
func NewTaskNotCancelableError(taskID string, state types.TaskState) error {
	return &TaskNotCancelableError{TaskID: taskID, State: state}
}

// PauseTaskForInput pauses a task waiting for additional input from the client
func (tm *DefaultTaskManager) PauseTaskForInput(taskID string, message *types.Message) error {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return NewTaskNotFoundError(taskID)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task.Status.State = types.TaskStateInputRequired
	task.Status.Message = message
	task.Status.Timestamp = &timestamp

	if message != nil {
		task.History = append(task.History, *message)
		tm.UpdateConversationHistory(task.ContextID, task.History)
	}

	tm.tasks[taskID] = task
	tm.logger.Info("task paused for input",
		zap.String("task_id", taskID),
		zap.String("context_id", task.ContextID))

	if tm.notificationSender != nil {
		go tm.sendPushNotifications(taskID, task)
	}

	return nil
}

// ResumeTaskWithInput resumes a paused task with new input from the client
func (tm *DefaultTaskManager) ResumeTaskWithInput(taskID string, message *types.Message) error {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return NewTaskNotFoundError(taskID)
	}

	if task.Status.State != types.TaskStateInputRequired {
		return fmt.Errorf("task %s is not in input-required state, current state: %s", taskID, task.Status.State)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task.Status.State = types.TaskStateWorking
	task.Status.Message = message
	task.Status.Timestamp = &timestamp

	if message != nil {
		task.History = append(task.History, *message)
		tm.UpdateConversationHistory(task.ContextID, task.History)
	}

	tm.tasks[taskID] = task
	tm.logger.Info("task resumed with input",
		zap.String("task_id", taskID),
		zap.String("context_id", task.ContextID))

	if tm.notificationSender != nil {
		go tm.sendPushNotifications(taskID, task)
	}

	return nil
}

// IsTaskPaused checks if a task is currently paused (in input-required state)
func (tm *DefaultTaskManager) IsTaskPaused(taskID string) (bool, error) {
	tm.tasksMu.RLock()
	defer tm.tasksMu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return false, NewTaskNotFoundError(taskID)
	}

	return task.Status.State == types.TaskStateInputRequired, nil
}
