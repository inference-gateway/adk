package server

import (
	"context"
	"fmt"

	adk "github.com/inference-gateway/a2a/adk"
	zap "go.uber.org/zap"
)

// TaskHandler defines how to handle task processing
// This interface should be implemented by domain-specific task handlers
type TaskHandler interface {
	// HandleTask processes a task and returns the updated task
	// This is where the main business logic should be implemented
	HandleTask(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error)
}

// DefaultTaskHandler implements the TaskHandler interface
type DefaultTaskHandler struct {
	logger    *zap.Logger
	llmClient LLMClient
}

// NewDefaultTaskHandler creates a new default task handler
func NewDefaultTaskHandler(logger *zap.Logger) *DefaultTaskHandler {
	return &DefaultTaskHandler{
		logger:    logger,
		llmClient: nil,
	}
}

// NewDefaultTaskHandlerWithLLM creates a new default task handler with optional LLM support
func NewDefaultTaskHandlerWithLLM(logger *zap.Logger, llmClient LLMClient) *DefaultTaskHandler {
	return &DefaultTaskHandler{
		logger:    logger,
		llmClient: llmClient,
	}
}

// SetLLMClient sets the optional LLM client
func (th *DefaultTaskHandler) SetLLMClient(client LLMClient) {
	th.llmClient = client
}

// HandleTask processes a task and returns the updated task
// This is the main entry point for business logic - override this for custom behavior
func (th *DefaultTaskHandler) HandleTask(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error) {
	th.logger.Info("processing task with default handler", zap.String("task_id", task.ID))

	// Try to use LLM if available, otherwise fall back to default behavior
	if th.llmClient != nil {
		th.logger.Info("using llm client for task processing", zap.String("task_id", task.ID))
		return th.handleTaskWithLLM(ctx, task, message)
	}

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

	if message != nil {
		task.History = append(task.History, *message)
	}

	task.History = append(task.History, *responseMessage)

	th.logger.Info("task processed by default handler", zap.String("task_id", task.ID))
	return task, nil
}

// handleTaskWithLLM processes a task using the available LLM client
func (th *DefaultTaskHandler) handleTaskWithLLM(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error) {
	messages := th.prepareMessages(task, message)

	systemPrompt := &adk.Message{
		Kind:      "message",
		MessageID: "system-prompt",
		Role:      "system",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "You are a helpful AI assistant. Process the user's request and provide a useful response.",
			},
		},
	}

	allMessages := append([]adk.Message{*systemPrompt}, messages...)

	response, err := th.llmClient.CreateChatCompletion(ctx, allMessages)
	if err != nil {
		th.logger.Error("failed to create chat completion", zap.Error(err))
		return th.handleError(task, "Failed to process with LLM: "+err.Error())
	}

	responseMessage, ok := response.(*adk.Message)
	if !ok {
		th.logger.Error("unexpected response type from llm client")
		return th.handleError(task, "Unexpected response type from LLM")
	}

	task.Status.State = adk.TaskStateCompleted

	if task.History == nil {
		task.History = []adk.Message{}
	}

	if message != nil {
		task.History = append(task.History, *message)
	}
	task.History = append(task.History, *responseMessage)

	th.logger.Info("task processed with llm", zap.String("task_id", task.ID))
	return task, nil
}

// prepareMessages prepares messages for LLM processing
func (th *DefaultTaskHandler) prepareMessages(task *adk.Task, message *adk.Message) []adk.Message {
	var messages []adk.Message

	if task.History != nil {
		messages = append(messages, task.History...)
	}

	if message != nil {
		messages = append(messages, *message)
	}

	return messages
}

// handleError creates an error response for a task
func (th *DefaultTaskHandler) handleError(task *adk.Task, errorMsg string) (*adk.Task, error) {
	task.Status.State = adk.TaskStateFailed

	errorMessage := &adk.Message{
		Kind:      "message",
		MessageID: "error-" + task.ID,
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": errorMsg,
			},
		},
	}

	if task.History == nil {
		task.History = []adk.Message{}
	}
	task.History = append(task.History, *errorMessage)

	return task, fmt.Errorf("%s", errorMsg)
}
