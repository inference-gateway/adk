package server

import (
	"context"
	"fmt"
	"strings"

	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// TaskHandler defines how to handle task processing
// This interface should be implemented by domain-specific task handlers
type TaskHandler interface {
	// HandleTask processes a task and returns the updated task
	// This is where the main business logic should be implemented
	// The agent parameter is optional and will be nil if no OpenAI-compatible agent is configured
	HandleTask(ctx context.Context, task *types.Task, message *types.Message, agent OpenAICompatibleAgent) (*types.Task, error)
}

// DefaultTaskHandler implements the TaskHandler interface
// This handler throws an error to enforce using proper handlers like AgentTaskHandler
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
// If an agent is provided, it will use the agent's capabilities, otherwise it will provide a simple response
func (th *DefaultTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message, agent OpenAICompatibleAgent) (*types.Task, error) {
	th.logger.Info("processing task with default task handler",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.Bool("has_agent", agent != nil))

	if agent != nil {
		return th.processWithAgent(ctx, task, message, agent)
	}

	return th.processWithoutAgent(ctx, task, message)
}

// processWithAgent processes a task using the provided agent's capabilities
func (th *DefaultTaskHandler) processWithAgent(ctx context.Context, task *types.Task, message *types.Message, agent OpenAICompatibleAgent) (*types.Task, error) {
	th.logger.Info("processing task with agent capabilities",
		zap.String("task_id", task.ID))

	messages := make([]types.Message, 0, len(task.History)+1)

	messages = append(messages, task.History...)

	if message != nil {
		messages = append(messages, *message)
	}

	agentResponse, err := agent.Run(ctx, messages)
	if err != nil {
		th.logger.Error("agent processing failed", zap.Error(err))

		task.Status.State = types.TaskStateFailed

		errorText := err.Error()
		if strings.Contains(errorText, "failed to create chat completion") {
			errorText = "LLM request failed: " + err.Error()
		}

		task.Status.Message = &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("error-%s", task.ID),
			Role:      "assistant",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": errorText,
				},
			},
		}
		return task, nil
	}

	if message != nil {
		task.History = append(task.History, *message)
	}

	if len(agentResponse.AdditionalMessages) > 0 {
		task.History = append(task.History, agentResponse.AdditionalMessages...)
	}

	if agentResponse.Response != nil {
		task.History = append(task.History, *agentResponse.Response)
	}

	task.Status.State = types.TaskStateCompleted
	task.Status.Message = agentResponse.Response

	return task, nil
}

// processWithoutAgent processes a task without any agent capabilities
func (th *DefaultTaskHandler) processWithoutAgent(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	th.logger.Info("processing task without agent",
		zap.String("task_id", task.ID))

	response := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "I received your message. I'm a basic task handler without AI capabilities. To enable AI responses, configure an OpenAI-compatible agent.",
			},
		},
	}

	if task.History == nil {
		task.History = []types.Message{}
	}
	task.History = append(task.History, *response)
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = response

	return task, nil
}
