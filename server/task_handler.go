package server

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// DefaultTaskHandler implements the TaskHandler interface for basic scenarios
// For optimized polling or streaming with automatic input-required pausing, 
// use DefaultPollingTaskHandler or DefaultStreamingTaskHandler instead
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

	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

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

// DefaultPollingTaskHandler implements the TaskHandler interface optimized for polling scenarios
// This handler automatically handles input-required pausing without requiring custom implementation
type DefaultPollingTaskHandler struct {
	logger *zap.Logger
}

// NewDefaultPollingTaskHandler creates a new default polling task handler
func NewDefaultPollingTaskHandler(logger *zap.Logger) *DefaultPollingTaskHandler {
	return &DefaultPollingTaskHandler{
		logger: logger,
	}
}

// HandleTask processes a task optimized for polling scenarios with automatic input pausing
func (pth *DefaultPollingTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message, agent OpenAICompatibleAgent) (*types.Task, error) {
	pth.logger.Info("processing task with default polling task handler",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.Bool("has_agent", agent != nil))

	if agent != nil {
		return pth.processWithAgentPolling(ctx, task, message, agent)
	}

	return pth.processWithoutAgent(ctx, task, message)
}

// processWithAgentPolling processes a task using agent capabilities with automatic input-required handling
func (pth *DefaultPollingTaskHandler) processWithAgentPolling(ctx context.Context, task *types.Task, message *types.Message, agent OpenAICompatibleAgent) (*types.Task, error) {
	pth.logger.Info("processing polling task with agent capabilities",
		zap.String("task_id", task.ID))

	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

	agentResponse, err := agent.Run(ctx, messages)
	if err != nil {
		pth.logger.Error("agent processing failed", zap.Error(err))

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

	// Check if the response requires input pausing (similar to pausedtask example)
	if agentResponse.Response != nil {
		lastMessage := agentResponse.Response
		if lastMessage.Kind == "input_required" {
			inputMessage := "Please provide more information to continue."
			if len(lastMessage.Parts) > 0 {
				if textPart, ok := lastMessage.Parts[0].(map[string]interface{}); ok {
					if text, exists := textPart["text"].(string); exists && text != "" {
						inputMessage = text
					}
				}
			}
			// Add the agent's response to history
			task.History = append(task.History, *agentResponse.Response)
			return pth.pauseTaskForInput(task, inputMessage), nil
		}
	}

	// Add additional messages and response to history
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

// processWithoutAgent processes a task without agent capabilities for polling
func (pth *DefaultPollingTaskHandler) processWithoutAgent(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	pth.logger.Info("processing polling task without agent",
		zap.String("task_id", task.ID))

	response := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "I received your message. I'm a default polling task handler without AI capabilities. To enable AI responses with automatic input-required pausing, configure an OpenAI-compatible agent.",
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

// pauseTaskForInput updates a task to input-required state with the given message
func (pth *DefaultPollingTaskHandler) pauseTaskForInput(task *types.Task, inputMessage string) *types.Task {
	pth.logger.Info("pausing polling task for user input",
		zap.String("task_id", task.ID),
		zap.String("input_message", inputMessage))

	// Create the input request message for the user
	message := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("input-request-%d", time.Now().Unix()),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": inputMessage,
			},
		},
	}

	// Add the assistant's input request message to conversation history
	if task.History == nil {
		task.History = []types.Message{}
	}
	task.History = append(task.History, *message)

	// Update task state to input-required
	task.Status.State = types.TaskStateInputRequired
	task.Status.Message = message

	pth.logger.Info("polling task paused for user input",
		zap.String("task_id", task.ID),
		zap.String("state", string(task.Status.State)),
		zap.Int("conversation_history_count", len(task.History)))

	return task
}

// DefaultStreamingTaskHandler implements the TaskHandler interface optimized for streaming scenarios
// This handler automatically handles input-required pausing with streaming-aware behavior
type DefaultStreamingTaskHandler struct {
	logger *zap.Logger
}

// NewDefaultStreamingTaskHandler creates a new default streaming task handler
func NewDefaultStreamingTaskHandler(logger *zap.Logger) *DefaultStreamingTaskHandler {
	return &DefaultStreamingTaskHandler{
		logger: logger,
	}
}

// HandleTask processes a task optimized for streaming scenarios with automatic input pausing
func (sth *DefaultStreamingTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message, agent OpenAICompatibleAgent) (*types.Task, error) {
	sth.logger.Info("processing task with default streaming task handler",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.Bool("has_agent", agent != nil))

	if agent != nil {
		return sth.processWithAgentStreaming(ctx, task, message, agent)
	}

	return sth.processWithoutAgent(ctx, task, message)
}

// processWithAgentStreaming processes a task using agent capabilities optimized for streaming with automatic input-required handling
func (sth *DefaultStreamingTaskHandler) processWithAgentStreaming(ctx context.Context, task *types.Task, message *types.Message, agent OpenAICompatibleAgent) (*types.Task, error) {
	sth.logger.Info("processing streaming task with agent capabilities",
		zap.String("task_id", task.ID))

	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

	// For streaming scenarios, we use the agent's Run method but with streaming-aware logic
	agentResponse, err := agent.Run(ctx, messages)
	if err != nil {
		sth.logger.Error("agent processing failed in streaming context", zap.Error(err))

		task.Status.State = types.TaskStateFailed

		errorText := err.Error()
		if strings.Contains(errorText, "failed to create chat completion") {
			errorText = "LLM streaming request failed: " + err.Error()
		}

		task.Status.Message = &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("stream-error-%s", task.ID),
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

	// Check if the response requires input pausing in streaming context
	if agentResponse.Response != nil {
		lastMessage := agentResponse.Response
		if lastMessage.Kind == "input_required" {
			inputMessage := "Please provide more information to continue streaming."
			if len(lastMessage.Parts) > 0 {
				if textPart, ok := lastMessage.Parts[0].(map[string]interface{}); ok {
					if text, exists := textPart["text"].(string); exists && text != "" {
						inputMessage = text
					}
				}
			}
			// Add the agent's response to history
			task.History = append(task.History, *agentResponse.Response)
			return sth.pauseTaskForStreamingInput(task, inputMessage), nil
		}
	}

	// Add additional messages and response to history
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

// processWithoutAgent processes a task without agent capabilities for streaming
func (sth *DefaultStreamingTaskHandler) processWithoutAgent(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	sth.logger.Info("processing streaming task without agent",
		zap.String("task_id", task.ID))

	response := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("stream-response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "I received your message in streaming context. I'm a default streaming task handler without AI capabilities. To enable AI responses with automatic streaming input-required pausing, configure an OpenAI-compatible agent.",
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

// pauseTaskForStreamingInput updates a task to input-required state optimized for streaming scenarios
func (sth *DefaultStreamingTaskHandler) pauseTaskForStreamingInput(task *types.Task, inputMessage string) *types.Task {
	sth.logger.Info("pausing streaming task for user input",
		zap.String("task_id", task.ID),
		zap.String("input_message", inputMessage))

	// Create the input request message for the user with streaming context
	message := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("stream-input-request-%d", time.Now().Unix()),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": inputMessage,
			},
		},
	}

	// Add the assistant's input request message to conversation history
	if task.History == nil {
		task.History = []types.Message{}
	}
	task.History = append(task.History, *message)

	// Update task state to input-required
	task.Status.State = types.TaskStateInputRequired
	task.Status.Message = message

	sth.logger.Info("streaming task paused for user input",
		zap.String("task_id", task.ID),
		zap.String("state", string(task.Status.State)),
		zap.Int("conversation_history_count", len(task.History)))

	return task
}
