package server

import (
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

	// CancelTask cancels a task
	CancelTask(taskID string) error

	// CleanupCompletedTasks removes old completed tasks from memory
	CleanupCompletedTasks()

	// PollTaskStatus periodically checks the status of a task until it is completed or failed
	PollTaskStatus(taskID string, interval time.Duration, timeout time.Duration) (*adk.Task, error)
}

// DefaultTaskManager implements the TaskManager interface
type DefaultTaskManager struct {
	logger  *zap.Logger
	tasks   map[string]*adk.Task
	tasksMu sync.RWMutex
}

// NewDefaultTaskManager creates a new default task manager
func NewDefaultTaskManager(logger *zap.Logger) *DefaultTaskManager {
	return &DefaultTaskManager{
		logger: logger,
		tasks:  make(map[string]*adk.Task),
	}
}

// CreateTask creates a new task and stores it
func (tm *DefaultTaskManager) CreateTask(contextID string, state adk.TaskState, message *adk.Message) *adk.Task {
	tm.tasksMu.Lock()
	defer tm.tasksMu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	task := &adk.Task{
		ID: uuid.New().String(),
		Status: adk.TaskStatus{
			State:     state,
			Message:   message,
			Timestamp: &timestamp,
		},
		ContextID: contextID,
	}

	tm.tasks[task.ID] = task
	tm.logger.Debug("task created", zap.String("task_id", task.ID), zap.String("state", string(state)))

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

	tm.tasks[taskID] = task
	tm.logger.Debug("task updated", zap.String("task_id", taskID), zap.String("state", string(state)))

	return nil
}

// GetTask retrieves a task by ID
func (tm *DefaultTaskManager) GetTask(taskID string) (*adk.Task, bool) {
	tm.tasksMu.RLock()
	defer tm.tasksMu.RUnlock()

	task, exists := tm.tasks[taskID]
	return task, exists
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
