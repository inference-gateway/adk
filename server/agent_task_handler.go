package server

import (
	"context"

	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// AgentTaskHandler is a TaskHandler that delegates to an OpenAICompatibleAgent
type AgentTaskHandler struct {
	logger *zap.Logger
	agent  OpenAICompatibleAgent
}

// NewAgentTaskHandler creates a new task handler that uses an OpenAI-compatible agent
func NewAgentTaskHandler(logger *zap.Logger, agent OpenAICompatibleAgent) *AgentTaskHandler {
	return &AgentTaskHandler{
		logger: logger,
		agent:  agent,
	}
}

// HandleTask processes a task by delegating to the OpenAI-compatible agent
func (h *AgentTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	h.logger.Info("processing task with openai-compatible agent",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID))

	if h.agent == nil {
		h.logger.Error("agent not configured")
		return h.handleError(task, "Agent not configured"), nil
	}

	return h.agent.ProcessTask(ctx, task, message)
}

// handleError creates an error response task
func (h *AgentTaskHandler) handleError(task *types.Task, errorMsg string) *types.Task {
	task.Status.State = types.TaskStateFailed

	errorMessage := &types.Message{
		Kind:      "message",
		MessageID: "error-" + task.ID,
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": errorMsg,
			},
		},
	}

	if task.History == nil {
		task.History = []types.Message{}
	}
	task.History = append(task.History, *errorMessage)
	task.Status.Message = errorMessage

	return task
}
