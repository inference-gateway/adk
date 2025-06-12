package server

import (
	"context"

	adk "github.com/inference-gateway/a2a/adk"
	zap "go.uber.org/zap"
)

// DefaultTaskHandler implements the TaskHandler interface
type DefaultTaskHandler struct {
	logger *zap.Logger
}

// NewDefaultTaskHandler creates a new default task handler
func NewDefaultTaskHandler(logger *zap.Logger) *DefaultTaskHandler {
	return &DefaultTaskHandler{
		logger: logger,
	}
}

// HandleTask processes a task and returns the updated task
// This is the main entry point for business logic - override this for custom behavior
func (th *DefaultTaskHandler) HandleTask(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error) {
	th.logger.Info("processing task with default handler", zap.String("task_id", task.ID))

	// Default implementation: just mark the task as completed
	// In a real implementation, this would:
	// 1. Extract message content
	// 2. Convert to SDK messages
	// 3. Call LLM with tools
	// 4. Process tool calls
	// 5. Generate response
	// 6. Update task with result

	task.Status.State = adk.TaskStateCompleted

	responseMessage := &adk.Message{
		Kind:      "message",
		MessageID: "response-" + task.ID,
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Task processed successfully by default handler",
			},
		},
	}

	if task.History == nil {
		task.History = []adk.Message{}
	}
	task.History = append(task.History, *message)

	task.History = append(task.History, *responseMessage)

	th.logger.Info("task processed by default handler", zap.String("task_id", task.ID))
	return task, nil
}
