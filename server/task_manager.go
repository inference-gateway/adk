package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	uuid "github.com/google/uuid"
	"github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// TaskManager defines task lifecycle management
type TaskManager interface {
	// CreateTask creates a new task and stores it
	CreateTask(contextID string, state types.TaskState, message *types.Message) *types.Task

	// CreateTaskWithHistory creates a new task with existing conversation history
	CreateTaskWithHistory(contextID string, state types.TaskState, message *types.Message, history []types.Message) *types.Task

	// UpdateState updates a task's state
	UpdateState(taskID string, state types.TaskState) error

	// UpdateTask updates a complete task (including history, state, and message)
	UpdateTask(task *types.Task) error

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

	// SetRetentionConfig sets the task retention configuration and starts automatic cleanup
	SetRetentionConfig(retentionConfig config.TaskRetentionConfig)

	// StopCleanup stops the automatic cleanup process
	StopCleanup()
}

// DefaultTaskManager implements the TaskManager interface
type DefaultTaskManager struct {
	logger                    *zap.Logger
	storage                   Storage
	pushNotificationConfigs   map[string]map[string]*types.TaskPushNotificationConfig
	notificationSender        PushNotificationSender
	pushNotificationConfigsMu sync.RWMutex
	retentionConfig           config.TaskRetentionConfig
	cleanupTicker             *time.Ticker
	stopCleanup               chan struct{}
}

// NewDefaultTaskManager creates a new default task manager
func NewDefaultTaskManager(logger *zap.Logger) *DefaultTaskManager {
	return &DefaultTaskManager{
		logger:                  logger,
		storage:                 NewInMemoryStorage(logger, 0),
		pushNotificationConfigs: make(map[string]map[string]*types.TaskPushNotificationConfig),
		notificationSender:      nil,
	}
}

// NewDefaultTaskManagerWithStorage creates a new default task manager with custom storage
func NewDefaultTaskManagerWithStorage(logger *zap.Logger, storage Storage) *DefaultTaskManager {
	return &DefaultTaskManager{
		logger:                  logger,
		storage:                 storage,
		pushNotificationConfigs: make(map[string]map[string]*types.TaskPushNotificationConfig),
		notificationSender:      nil,
	}
}

// NewDefaultTaskManagerWithNotifications creates a new default task manager with push notification support
func NewDefaultTaskManagerWithNotifications(logger *zap.Logger, notificationSender PushNotificationSender) *DefaultTaskManager {
	return &DefaultTaskManager{
		logger:                  logger,
		storage:                 NewInMemoryStorage(logger, 0),
		pushNotificationConfigs: make(map[string]map[string]*types.TaskPushNotificationConfig),
		notificationSender:      notificationSender,
	}
}

// SetNotificationSender sets the push notification sender
func (tm *DefaultTaskManager) SetNotificationSender(sender PushNotificationSender) {
	tm.notificationSender = sender
}

// GetStorage returns the storage interface used by this task manager
func (tm *DefaultTaskManager) GetStorage() Storage {
	return tm.storage
}

// CreateTask creates a new task with message history managed within the task
func (tm *DefaultTaskManager) CreateTask(contextID string, state types.TaskState, message *types.Message) *types.Task {
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)

	var history []types.Message

	if message != nil {
		history = append(history, *message)
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

	switch state {
	case types.TaskStateCompleted, types.TaskStateFailed, types.TaskStateCanceled, types.TaskStateRejected:
		err := tm.storage.StoreDeadLetterTask(task)
		if err != nil {
			tm.logger.Error("failed to store task in dead letter queue", zap.Error(err))
		}
	default:
		err := tm.storage.CreateActiveTask(task)
		if err != nil {
			tm.logger.Error("failed to store created task", zap.Error(err))
		}
	}

	tm.logger.Debug("task created and stored",
		zap.String("task_id", task.ID),
		zap.String("context_id", contextID),
		zap.String("state", string(state)),
		zap.Int("history_count", len(history)))

	return task
}

// CreateTaskWithHistory creates a new task with existing conversation history
func (tm *DefaultTaskManager) CreateTaskWithHistory(contextID string, state types.TaskState, message *types.Message, history []types.Message) *types.Task {
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)

	taskHistory := make([]types.Message, len(history))
	copy(taskHistory, history)

	if message != nil {
		taskHistory = append(taskHistory, *message)
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
		History:   taskHistory,
	}

	switch state {
	case types.TaskStateCompleted, types.TaskStateFailed, types.TaskStateCanceled, types.TaskStateRejected:
		err := tm.storage.StoreDeadLetterTask(task)
		if err != nil {
			tm.logger.Error("failed to store task in dead letter queue", zap.Error(err))
		}
	default:
		err := tm.storage.CreateActiveTask(task)
		if err != nil {
			tm.logger.Error("failed to store created task", zap.Error(err))
		}
	}

	tm.logger.Debug("task created with history and stored",
		zap.String("task_id", task.ID),
		zap.String("context_id", contextID),
		zap.String("state", string(state)),
		zap.Int("history_count", len(taskHistory)))

	return task
}

// UpdateState updates a task's state
func (tm *DefaultTaskManager) UpdateState(taskID string, state types.TaskState) error {
	task, err := tm.storage.GetActiveTask(taskID)
	if err != nil {
		contexts := tm.storage.GetContexts()
		var found bool
		for _, contextID := range contexts {
			tasks, err := tm.storage.ListTasksByContext(contextID, TaskFilter{})
			if err != nil {
				continue
			}
			for _, deadTask := range tasks {
				if deadTask.ID == taskID {
					task = deadTask
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return NewTaskNotFoundError(taskID)
		}
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task.Status.State = state
	task.Status.Timestamp = &timestamp

	if tm.isTaskFinalState(state) {
		err := tm.storage.StoreDeadLetterTask(task)
		if err != nil {
			tm.logger.Error("failed to store task in dead letter queue", zap.Error(err))
			return err
		}
	} else {
		err := tm.storage.UpdateActiveTask(task)
		if err != nil {
			tm.logger.Error("failed to update active task", zap.Error(err))
			return err
		}
	}

	tm.logger.Debug("task state updated",
		zap.String("task_id", taskID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(state)))

	if tm.notificationSender != nil {
		go tm.sendPushNotifications(taskID, task)
	}

	return nil
}

// UpdateTask updates a complete task (including history, state, and message)
func (tm *DefaultTaskManager) UpdateTask(task *types.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task.Status.Timestamp = &timestamp

	if tm.isTaskFinalState(task.Status.State) {
		err := tm.storage.StoreDeadLetterTask(task)
		if err != nil {
			tm.logger.Error("failed to store task in dead letter queue", zap.Error(err))
			return err
		}
	} else {
		err := tm.storage.UpdateActiveTask(task)
		if err != nil {
			tm.logger.Error("failed to update active task", zap.Error(err))
			return err
		}
	}

	tm.logger.Debug("task updated completely",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("state", string(task.Status.State)),
		zap.Int("history_count", len(task.History)))

	if tm.notificationSender != nil {
		go tm.sendPushNotifications(task.ID, task)
	}

	return nil
}

// UpdateError updates a task to failed state with an error message
func (tm *DefaultTaskManager) UpdateError(taskID string, message *types.Message) error {
	task, exists := tm.GetTask(taskID)
	if !exists {
		return NewTaskNotFoundError(taskID)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task.Status.State = types.TaskStateFailed
	task.Status.Message = message
	task.Status.Timestamp = &timestamp

	err := tm.storage.StoreDeadLetterTask(task)
	if err != nil {
		tm.logger.Error("failed to store failed task in dead letter queue", zap.Error(err))
		return err
	}
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
	task, err := tm.storage.GetActiveTask(taskID)
	if err == nil {
		return task, true
	}

	task, found := tm.storage.GetTask(taskID)
	if found {
		return task, true
	}

	return nil, false
}

// ListTasks retrieves a list of tasks based on the provided parameters
func (tm *DefaultTaskManager) ListTasks(params types.TaskListParams) (*types.TaskList, error) {
	filter := TaskFilter{
		State:     params.State,
		ContextID: params.ContextID,
		Limit:     params.Limit,
		Offset:    params.Offset,
	}

	if filter.Limit <= 0 {
		filter.Limit = 50
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}

	allTasks, err := tm.storage.ListTasks(filter)
	if err != nil {
		tm.logger.Error("failed to list tasks", zap.Error(err))
		return nil, err
	}

	totalFilter := filter
	totalFilter.Limit = 0
	totalFilter.Offset = 0
	totalTasks, err := tm.storage.ListTasks(totalFilter)
	if err != nil {
		tm.logger.Error("failed to count total tasks", zap.Error(err))
		return nil, err
	}

	var resultTasks []types.Task
	for _, taskPtr := range allTasks {
		resultTasks = append(resultTasks, *taskPtr)
	}

	result := &types.TaskList{
		Tasks:  resultTasks,
		Total:  len(totalTasks),
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}

	tm.logger.Debug("listed tasks",
		zap.Int("total_count", len(totalTasks)),
		zap.Int("returned_count", len(resultTasks)),
		zap.Int("offset", filter.Offset),
		zap.Int("limit", filter.Limit))

	return result, nil
}

// CancelTask cancels a task
func (tm *DefaultTaskManager) CancelTask(taskID string) error {
	task, exists := tm.GetTask(taskID)
	if !exists {
		return NewTaskNotFoundError(taskID)
	}

	if !tm.isTaskCancelable(task.Status.State) {
		return NewTaskNotCancelableError(taskID, task.Status.State)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task.Status.State = types.TaskStateCanceled
	task.Status.Timestamp = &timestamp

	err := tm.storage.StoreDeadLetterTask(task)
	if err != nil {
		tm.logger.Error("failed to store canceled task in dead letter queue", zap.Error(err))
		return err
	}

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
		return false
	case types.TaskStateSubmitted, types.TaskStateWorking, types.TaskStateInputRequired, types.TaskStateAuthRequired, types.TaskStateUnknown:
		return true
	default:
		return false
	}
}

// isTaskFinalState determines if a task state is final and should move to dead letter queue
func (tm *DefaultTaskManager) isTaskFinalState(state types.TaskState) bool {
	switch state {
	case types.TaskStateCompleted, types.TaskStateFailed, types.TaskStateCanceled, types.TaskStateRejected:
		return true
	default:
		return false
	}
}

// CleanupCompletedTasks removes old completed tasks from memory
func (tm *DefaultTaskManager) CleanupCompletedTasks() {
	removedCount := tm.storage.CleanupCompletedTasks()
	if removedCount > 0 {
		tm.logger.Info("cleaned up completed tasks", zap.Int("count", removedCount))
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
	var allMessages []types.Message

	tm.logger.Debug("getting conversation history from task-based storage",
		zap.String("context_id", contextID))

	filter := TaskFilter{
		ContextID: &contextID,
		SortBy:    TaskSortFieldCreatedAt,
		SortOrder: SortOrderDesc,
	}

	existingTasks, err := tm.storage.ListTasksByContext(contextID, filter)
	if err == nil && len(existingTasks) > 0 {
		allMessages = make([]types.Message, len(existingTasks[0].History))
		copy(allMessages, existingTasks[0].History)
	} else {
		allTasks, err := tm.storage.ListTasks(filter)
		if err == nil {
			for _, task := range allTasks {
				if task.ContextID == contextID {
					allMessages = make([]types.Message, len(task.History))
					copy(allMessages, task.History)
					break
				}
			}
		}
	}

	return allMessages
}

// UpdateConversationHistory updates conversation history for a context ID
func (tm *DefaultTaskManager) UpdateConversationHistory(contextID string, messages []types.Message) {
	tm.logger.Debug("updating conversation history in task-based storage",
		zap.String("context_id", contextID),
		zap.Int("message_count", len(messages)))

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)

	historyCopy := make([]types.Message, len(messages))
	copy(historyCopy, messages)

	task := &types.Task{
		ID:   uuid.New().String(),
		Kind: "task",
		Status: types.TaskStatus{
			State:     types.TaskStateCompleted,
			Message:   nil,
			Timestamp: &timestamp,
		},
		ContextID: contextID,
		History:   historyCopy,
	}

	err := tm.storage.StoreDeadLetterTask(task)
	if err != nil {
		tm.logger.Error("failed to store conversation history task", zap.Error(err))
	}
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
	task, exists := tm.GetTask(taskID)
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

	err := tm.storage.UpdateActiveTask(task)
	if err != nil {
		tm.logger.Error("failed to update paused task in storage", zap.Error(err))
		return err
	}

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
	task, exists := tm.GetTask(taskID)
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

	err := tm.storage.UpdateActiveTask(task)
	if err != nil {
		tm.logger.Error("failed to update resumed task in storage", zap.Error(err))
		return err
	}

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
	task, exists := tm.GetTask(taskID)
	if !exists {
		return false, NewTaskNotFoundError(taskID)
	}

	return task.Status.State == types.TaskStateInputRequired, nil
}

// SetRetentionConfig sets the task retention configuration and starts automatic cleanup
func (tm *DefaultTaskManager) SetRetentionConfig(retentionConfig config.TaskRetentionConfig) {
	tm.retentionConfig = retentionConfig

	// Stop existing cleanup if running
	tm.StopCleanup()

	// Start automatic cleanup if interval is configured
	if retentionConfig.CleanupInterval > 0 {
		tm.stopCleanup = make(chan struct{})
		tm.cleanupTicker = time.NewTicker(retentionConfig.CleanupInterval)

		go func() {
			for {
				select {
				case <-tm.cleanupTicker.C:
					tm.cleanupWithRetention()
				case <-tm.stopCleanup:
					return
				}
			}
		}()

		tm.logger.Info("started automatic task retention cleanup",
			zap.Int("max_completed_tasks", retentionConfig.MaxCompletedTasks),
			zap.Int("max_failed_tasks", retentionConfig.MaxFailedTasks),
			zap.Duration("cleanup_interval", retentionConfig.CleanupInterval))
	}
}

// StopCleanup stops the automatic cleanup process
func (tm *DefaultTaskManager) StopCleanup() {
	if tm.cleanupTicker != nil {
		tm.cleanupTicker.Stop()
		tm.cleanupTicker = nil
	}

	if tm.stopCleanup != nil {
		close(tm.stopCleanup)
		tm.stopCleanup = nil
	}
}

// cleanupWithRetention removes tasks based on retention configuration
func (tm *DefaultTaskManager) cleanupWithRetention() {
	if tm.retentionConfig.MaxCompletedTasks <= 0 && tm.retentionConfig.MaxFailedTasks <= 0 {
		return // No retention limits configured
	}

	removedCount := tm.storage.CleanupTasksWithRetention(
		tm.retentionConfig.MaxCompletedTasks,
		tm.retentionConfig.MaxFailedTasks,
	)

	if removedCount > 0 {
		tm.logger.Info("cleaned up tasks based on retention policy",
			zap.Int("removed_count", removedCount),
			zap.Int("max_completed_tasks", tm.retentionConfig.MaxCompletedTasks),
			zap.Int("max_failed_tasks", tm.retentionConfig.MaxFailedTasks))
	}
}
