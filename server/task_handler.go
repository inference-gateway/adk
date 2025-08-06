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
	HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error)

	// SetAgent sets the OpenAI-compatible agent for the task handler
	SetAgent(agent OpenAICompatibleAgent)

	// GetAgent returns the configured OpenAI-compatible agent
	GetAgent() OpenAICompatibleAgent

	// RequestInput creates an input-required message with the specified prompt
	// This method should be used by task handlers to request additional input from the user
	// Returns a Message with kind "input_required" that can be used to pause the task
	RequestInput(message string) *types.Message
}

// DefaultTaskHandler implements the TaskHandler interface for basic scenarios
// For optimized background or streaming with automatic input-required pausing,
// use DefaultBackgroundTaskHandler or DefaultStreamingTaskHandler instead
type DefaultTaskHandler struct {
	logger *zap.Logger
	agent  OpenAICompatibleAgent
}

// NewDefaultTaskHandler creates a new default task handler
func NewDefaultTaskHandler(logger *zap.Logger) *DefaultTaskHandler {
	return &DefaultTaskHandler{
		logger: logger,
	}
}

// NewDefaultTaskHandlerWithAgent creates a new default task handler with an agent
func NewDefaultTaskHandlerWithAgent(logger *zap.Logger, agent OpenAICompatibleAgent) *DefaultTaskHandler {
	return &DefaultTaskHandler{
		logger: logger,
		agent:  agent,
	}
}

// SetAgent sets the OpenAI-compatible agent for the task handler
func (th *DefaultTaskHandler) SetAgent(agent OpenAICompatibleAgent) {
	th.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (th *DefaultTaskHandler) GetAgent() OpenAICompatibleAgent {
	return th.agent
}

// RequestInput creates an input-required message with the specified prompt
func (th *DefaultTaskHandler) RequestInput(message string) *types.Message {
	return &types.Message{
		Kind:      "input_required",
		MessageID: fmt.Sprintf("input-required-%d", time.Now().UnixNano()),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": message,
			},
		},
	}
}

// HandleTask processes a task and returns the updated task
// If an agent is configured, it will use the agent's capabilities, otherwise it will provide a simple response
func (th *DefaultTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	th.logger.Info("processing task with default task handler",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.Bool("has_agent", th.agent != nil))

	if th.agent != nil {
		return th.processWithAgentBackground(ctx, task, message)
	}

	return th.processWithoutAgentBackground(ctx, task, message)
}

// processWithAgentBackground processes a task using the configured agent's capabilities
func (th *DefaultTaskHandler) processWithAgentBackground(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	th.logger.Info("processing task with agent capabilities",
		zap.String("task_id", task.ID))

	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

	agentResponse, err := th.agent.Run(ctx, messages)
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

// processWithoutAgentBackground processes a task without any agent capabilities
func (th *DefaultTaskHandler) processWithoutAgentBackground(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
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

// DefaultBackgroundTaskHandler implements the TaskHandler interface optimized for background scenarios
// This handler automatically handles input-required pausing without requiring custom implementation
type DefaultBackgroundTaskHandler struct {
	logger *zap.Logger
	agent  OpenAICompatibleAgent
}

// NewDefaultBackgroundTaskHandler creates a new default background task handler
func NewDefaultBackgroundTaskHandler(logger *zap.Logger, agent OpenAICompatibleAgent) *DefaultBackgroundTaskHandler {
	return &DefaultBackgroundTaskHandler{
		logger: logger,
		agent:  agent,
	}
}

// NewDefaultBackgroundTaskHandlerWithAgent creates a new default background task handler with an agent
func NewDefaultBackgroundTaskHandlerWithAgent(logger *zap.Logger, agent OpenAICompatibleAgent) *DefaultBackgroundTaskHandler {
	return &DefaultBackgroundTaskHandler{
		logger: logger,
		agent:  agent,
	}
}

// SetAgent sets the agent for the task handler
func (bth *DefaultBackgroundTaskHandler) SetAgent(agent OpenAICompatibleAgent) {
	bth.agent = agent
}

// GetAgent returns the configured agent
func (bth *DefaultBackgroundTaskHandler) GetAgent() OpenAICompatibleAgent {
	return bth.agent
}

// RequestInput creates an input-required message with the specified prompt
func (bth *DefaultBackgroundTaskHandler) RequestInput(message string) *types.Message {
	return &types.Message{
		Kind:      "input_required",
		MessageID: fmt.Sprintf("input-required-%d", time.Now().UnixNano()),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": message,
			},
		},
	}
}

// HandleTask processes a task with optimized logic for background scenarios
func (bth *DefaultBackgroundTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	if bth.agent != nil {
		return bth.processWithAgentBackground(ctx, task, message)
	}
	return bth.processWithoutAgentBackground(ctx, task, message)
}

// processWithAgentBackground processes a task using agent capabilities with automatic input-required handling
func (bth *DefaultBackgroundTaskHandler) processWithAgentBackground(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	bth.logger.Info("processing background task with agent capabilities",
		zap.String("task_id", task.ID))

	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

	agentResponse, err := bth.agent.Run(ctx, messages)
	if err != nil {
		bth.logger.Error("agent processing failed", zap.Error(err))

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
			task.History = append(task.History, *agentResponse.Response)
			return bth.pauseTaskForInput(task, inputMessage), nil
		}
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

// processWithoutAgentBackground processes a task without agent capabilities for background
func (bth *DefaultBackgroundTaskHandler) processWithoutAgentBackground(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	bth.logger.Info("processing background task without agent",
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
func (bth *DefaultBackgroundTaskHandler) pauseTaskForInput(task *types.Task, inputMessage string) *types.Task {
	bth.logger.Info("pausing background task for user input",
		zap.String("task_id", task.ID),
		zap.String("input_message", inputMessage))

	message := bth.RequestInput(inputMessage)

	if task.History == nil {
		task.History = []types.Message{}
	}
	task.History = append(task.History, *message)

	task.Status.State = types.TaskStateInputRequired
	task.Status.Message = message

	bth.logger.Info("background task paused for user input",
		zap.String("task_id", task.ID),
		zap.String("state", string(task.Status.State)),
		zap.Int("conversation_history_count", len(task.History)))

	return task
}

// DefaultStreamingTaskHandler implements the TaskHandler interface optimized for streaming scenarios
// This handler automatically handles input-required pausing with streaming-aware behavior
type DefaultStreamingTaskHandler struct {
	logger *zap.Logger
	agent  OpenAICompatibleAgent
}

// NewDefaultStreamingTaskHandler creates a new default streaming task handler
func NewDefaultStreamingTaskHandler(logger *zap.Logger, agent OpenAICompatibleAgent) *DefaultStreamingTaskHandler {
	var agentInstance OpenAICompatibleAgent
	if agent != nil {
		agentInstance = agent
	}

	return &DefaultStreamingTaskHandler{
		logger: logger,
		agent:  agentInstance,
	}
}

// SetAgent sets the agent for the task handler
func (sth *DefaultStreamingTaskHandler) SetAgent(agent OpenAICompatibleAgent) {
	sth.agent = agent
}

// GetAgent returns the configured agent
func (sth *DefaultStreamingTaskHandler) GetAgent() OpenAICompatibleAgent {
	return sth.agent
}

// RequestInput creates an input-required message with the specified prompt
func (sth *DefaultStreamingTaskHandler) RequestInput(message string) *types.Message {
	return &types.Message{
		Kind:      "input_required",
		MessageID: fmt.Sprintf("input-required-%d", time.Now().UnixNano()),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": message,
			},
		},
	}
}

// HandleTask processes a task optimized for streaming scenarios with automatic input pausing
func (sth *DefaultStreamingTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	sth.logger.Info("processing task with default streaming task handler",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.Bool("has_agent", sth.agent != nil))

	if sth.agent != nil {
		return sth.processWithAgentStreaming(ctx, task, message)
	}

	return sth.processWithoutAgentBackground(ctx, task, message)
}

// processWithAgentStreaming processes a task using agent capabilities optimized for streaming with automatic input-required handling
func (sth *DefaultStreamingTaskHandler) processWithAgentStreaming(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	sth.logger.Info("processing streaming task with agent capabilities",
		zap.String("task_id", task.ID))

	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

	streamChan, err := sth.agent.RunWithStream(ctx, messages)
	if err != nil {
		sth.logger.Error("agent streaming failed", zap.Error(err))

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

	// Collect all streaming messages
	var additionalMessages []types.Message
	var lastMessage *types.Message

	for streamMessage := range streamChan {
		if streamMessage != nil {
			additionalMessages = append(additionalMessages, *streamMessage)
			lastMessage = streamMessage
		}
	}

	// Check for input required in the last message
	if lastMessage != nil && lastMessage.Kind == "input_required" {
		inputMessage := "Please provide more information to continue streaming."
		if len(lastMessage.Parts) > 0 {
			if textPart, ok := lastMessage.Parts[0].(map[string]interface{}); ok {
				if text, exists := textPart["text"].(string); exists && text != "" {
					inputMessage = text
				}
			}
		}
		task.History = append(task.History, additionalMessages...)
		return sth.pauseTaskForStreamingInput(task, inputMessage), nil
	}

	// Add all messages to history
	if len(additionalMessages) > 0 {
		task.History = append(task.History, additionalMessages...)
	}

	task.Status.State = types.TaskStateCompleted
	task.Status.Message = lastMessage

	return task, nil
}

// processWithoutAgentBackground processes a task without agent capabilities for streaming
func (sth *DefaultStreamingTaskHandler) processWithoutAgentBackground(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
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

	message := sth.RequestInput(inputMessage)

	if task.History == nil {
		task.History = []types.Message{}
	}
	task.History = append(task.History, *message)

	task.Status.State = types.TaskStateInputRequired
	task.Status.Message = message

	sth.logger.Info("streaming task paused for user input",
		zap.String("task_id", task.ID),
		zap.String("state", string(task.Status.State)),
		zap.Int("conversation_history_count", len(task.History)))

	return task
}

// PauseTaskForInput is a utility function that pauses a task by setting it to input-required state
// This function can be used by any task handler to request input from the user
// It uses the handler's RequestInput method to create a consistent input-required message
func PauseTaskForInput(handler TaskHandler, task *types.Task, inputMessage string) *types.Task {
	message := handler.RequestInput(inputMessage)

	if task.History == nil {
		task.History = []types.Message{}
	}
	task.History = append(task.History, *message)

	task.Status.State = types.TaskStateInputRequired
	task.Status.Message = message

	return task
}
