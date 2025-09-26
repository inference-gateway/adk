package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// A2AProtocolHandler defines the interface for handling A2A protocol requests
type A2AProtocolHandler interface {
	// HandleMessageSend processes message/send requests
	HandleMessageSend(c *gin.Context, req types.JSONRPCRequest)

	// HandleMessageStream processes message/stream requests
	HandleMessageStream(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskGet processes tasks/get requests
	HandleTaskGet(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskList processes tasks/list requests
	HandleTaskList(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskCancel processes tasks/cancel requests
	HandleTaskCancel(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskPushNotificationConfigSet processes tasks/pushNotificationConfig/set requests
	HandleTaskPushNotificationConfigSet(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskPushNotificationConfigGet processes tasks/pushNotificationConfig/get requests
	HandleTaskPushNotificationConfigGet(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskPushNotificationConfigList processes tasks/pushNotificationConfig/list requests
	HandleTaskPushNotificationConfigList(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskPushNotificationConfigDelete processes tasks/pushNotificationConfig/delete requests
	HandleTaskPushNotificationConfigDelete(c *gin.Context, req types.JSONRPCRequest)
}

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
}

// StreamableTaskHandler defines how to handle streaming task processing
// This interface should be implemented by streaming task handlers that need to return real-time data
type StreamableTaskHandler interface {
	// HandleStreamingTask processes a task and returns a channel of streaming events
	// The channel should be closed when streaming is complete
	HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan StreamEvent, error)

	// SetAgent sets the OpenAI-compatible agent for the task handler
	SetAgent(agent OpenAICompatibleAgent)

	// GetAgent returns the configured OpenAI-compatible agent
	GetAgent() OpenAICompatibleAgent
}

// StreamEvent represents a streaming event that can be sent to clients
type StreamEvent interface {
	// GetEventType returns the type of the streaming event (delta, status, error, etc.)
	GetEventType() string

	// GetData returns the event data
	GetData() interface{}
}

// DeltaStreamEvent represents a delta streaming event
type DeltaStreamEvent struct {
	Data interface{}
}

func (e *DeltaStreamEvent) GetEventType() string { return "delta" }
func (e *DeltaStreamEvent) GetData() interface{} { return e.Data }

// StatusStreamEvent represents a status update streaming event
type StatusStreamEvent struct {
	Status interface{}
}

func (e *StatusStreamEvent) GetEventType() string { return "status" }
func (e *StatusStreamEvent) GetData() interface{} { return e.Status }

// ErrorStreamEvent represents an error streaming event
type ErrorStreamEvent struct {
	ErrorMessage string
}

func (e *ErrorStreamEvent) GetEventType() string { return "error" }
func (e *ErrorStreamEvent) GetData() interface{} { return e.ErrorMessage }

// TaskCompleteStreamEvent represents a task completion streaming event
type TaskCompleteStreamEvent struct {
	Task *types.Task
}

func (e *TaskCompleteStreamEvent) GetEventType() string { return "task_complete" }
func (e *TaskCompleteStreamEvent) GetData() interface{} { return e.Task }

// TaskInterruptedStreamEvent represents a task interruption streaming event
type TaskInterruptedStreamEvent struct {
	Task   *types.Task
	Reason string
}

func (e *TaskInterruptedStreamEvent) GetEventType() string { return "task_interrupted" }
func (e *TaskInterruptedStreamEvent) GetData() interface{} { return e.Task }

// ArtifactUpdateStreamEvent represents an artifact update streaming event
type ArtifactUpdateStreamEvent struct {
	Event types.TaskArtifactUpdateEvent
}

func (e *ArtifactUpdateStreamEvent) GetEventType() string { return "artifact_update" }
func (e *ArtifactUpdateStreamEvent) GetData() interface{} { return e.Event }

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
				map[string]any{
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
			map[string]any{
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
				map[string]any{
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
				if textPart, ok := lastMessage.Parts[0].(map[string]any); ok {
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
			map[string]any{
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

	message := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("input-request-%d", time.Now().Unix()),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": inputMessage,
			},
		},
	}

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

// HandleStreamingTask processes a task and returns a channel of streaming events
func (sth *DefaultStreamingTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan StreamEvent, error) {
	eventsChan := make(chan StreamEvent, 100)

	go func() {
		defer close(eventsChan)

		sth.logger.Info("processing streaming task internally",
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID),
			zap.Bool("has_agent", sth.agent != nil))

		var updatedTask *types.Task
		var err error

		if sth.agent == nil {
			eventsChan <- &ErrorStreamEvent{ErrorMessage: "streaming task handler requires an agent to be configured - use SetAgent() to configure an OpenAI-compatible agent for streaming support"}
			return
		}

		sth.logger.Info("processing streaming task with agent capabilities",
			zap.String("task_id", task.ID))

		eventsChan <- &StatusStreamEvent{Status: "starting"}

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
					map[string]any{
						"kind": "text",
						"text": errorText,
					},
				},
			}
			eventsChan <- &ErrorStreamEvent{ErrorMessage: err.Error()}
			return
		}

		for event := range streamChan {
			if event.Type() == "adk.agent.delta" {
				var deltaMessage types.Message
				if err := event.DataAs(&deltaMessage); err == nil {
					eventsChan <- &DeltaStreamEvent{Data: deltaMessage}
				}
			}

			if event.Type() == "adk.agent.iteration.completed" {
				var finalMessage types.Message
				if err := event.DataAs(&finalMessage); err != nil {
					sth.logger.Error("failed to parse iteration completed event data", zap.Error(err))
					continue
				}

				task.History = append(task.History, finalMessage)
				sth.logger.Debug("stored iteration completed message to history",
					zap.String("task_id", task.ID),
					zap.String("message_id", finalMessage.MessageID),
					zap.String("event_id", event.ID()))
			}

			if event.Type() == "adk.agent.tool.result" {
				var toolResultMessage types.Message
				if err := event.DataAs(&toolResultMessage); err != nil {
					sth.logger.Error("failed to parse tool result event data", zap.Error(err))
					continue
				}

				task.History = append(task.History, toolResultMessage)
				sth.logger.Debug("stored tool result message to history",
					zap.String("task_id", task.ID),
					zap.String("message_id", toolResultMessage.MessageID),
					zap.String("event_id", event.ID()))
			}

			if event.Type() == "adk.agent.input.required" {
				var inputMessage types.Message
				if err := event.DataAs(&inputMessage); err != nil {
					sth.logger.Error("failed to parse input required event data", zap.Error(err))
					continue
				}

				if inputMessage.Kind == "input_required" {
					inputText := "Please provide more information to continue streaming."
					if len(inputMessage.Parts) > 0 {
						if textPart, ok := inputMessage.Parts[0].(map[string]any); ok {
							if text, exists := textPart["text"].(string); exists && text != "" {
								inputText = text
							}
						}
					}

					task.History = append(task.History, inputMessage)
					updatedTask = sth.pauseTaskForStreamingInput(task, inputText)
					eventsChan <- &TaskCompleteStreamEvent{Task: updatedTask}
					return
				}
			}

			if event.Type() == "adk.agent.stream.failed" {
				var errorMessage types.Message
				if err := event.DataAs(&errorMessage); err != nil {
					sth.logger.Error("failed to parse stream failed event data", zap.Error(err))
					continue
				}

				sth.logger.Error("streaming failed",
					zap.String("task_id", task.ID),
					zap.String("context_id", task.ContextID),
					zap.String("event_id", event.ID()))

				task.History = append(task.History, errorMessage)

				task.Status.State = types.TaskStateFailed
				task.Status.Message = &errorMessage

				eventsChan <- &TaskCompleteStreamEvent{Task: task}
				return
			}

			if event.Type() == "adk.agent.task.interrupted" {
				var interruptMessage types.Message
				if err := event.DataAs(&interruptMessage); err != nil {
					sth.logger.Error("failed to parse task interrupted event data", zap.Error(err))
					continue
				}

				sth.logger.Info("streaming task was interrupted by agent",
					zap.String("task_id", task.ID),
					zap.String("context_id", task.ContextID),
					zap.Int("preserved_history_count", len(task.History)))

				task.Status.State = types.TaskStateWorking
				if len(task.History) > 0 {
					task.Status.Message = &task.History[len(task.History)-1]
				}
				eventsChan <- &TaskInterruptedStreamEvent{Task: task, Reason: "context_cancelled"}
				return
			}
		}

		task.Status.State = types.TaskStateCompleted
		if len(task.History) > 0 {
			task.Status.Message = &task.History[len(task.History)-1]
		} else {
			task.Status.Message = nil
		}

		eventsChan <- &StatusStreamEvent{Status: "completed"}
		eventsChan <- &TaskCompleteStreamEvent{Task: task}
	}()

	return eventsChan, nil
}

// pauseTaskForStreamingInput updates a task to input-required state optimized for streaming scenarios
func (sth *DefaultStreamingTaskHandler) pauseTaskForStreamingInput(task *types.Task, inputMessage string) *types.Task {
	sth.logger.Info("pausing streaming task for user input",
		zap.String("task_id", task.ID),
		zap.String("input_message", inputMessage))

	message := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("stream-input-request-%d", time.Now().Unix()),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": inputMessage,
			},
		},
	}

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

// DefaultA2AProtocolHandler implements the A2AProtocolHandler interface
type DefaultA2AProtocolHandler struct {
	logger                *zap.Logger
	storage               Storage
	taskManager           TaskManager
	responseSender        ResponseSender
	backgroundTaskHandler TaskHandler
	streamingTaskHandler  StreamableTaskHandler
}

// NewDefaultA2AProtocolHandler creates a new default A2A protocol handler
func NewDefaultA2AProtocolHandler(
	logger *zap.Logger,
	storage Storage,
	taskManager TaskManager,
	responseSender ResponseSender,
	backgroundTaskHandler TaskHandler,
	streamingTaskHandler StreamableTaskHandler,
) *DefaultA2AProtocolHandler {
	return &DefaultA2AProtocolHandler{
		logger:                logger,
		storage:               storage,
		taskManager:           taskManager,
		responseSender:        responseSender,
		backgroundTaskHandler: backgroundTaskHandler,
		streamingTaskHandler:  streamingTaskHandler,
	}
}

// createTaskFromMessage creates a task directly from message parameters
func (h *DefaultA2AProtocolHandler) createTaskFromMessage(ctx context.Context, params types.MessageSendParams) (*types.Task, error) {
	if len(params.Message.Parts) == 0 {
		return nil, fmt.Errorf("empty message parts not allowed")
	}

	if params.Message.TaskID != nil {
		taskID := *params.Message.TaskID

		err := h.taskManager.ResumeTaskWithInput(taskID, &params.Message)
		if err != nil {
			h.logger.Error("failed to resume task with input",
				zap.String("task_id", taskID),
				zap.Error(err))
			return nil, fmt.Errorf("failed to resume task: %w", err)
		}

		task, exists := h.taskManager.GetTask(taskID)
		if !exists {
			h.logger.Error("failed to get resumed task",
				zap.String("task_id", taskID))
			return nil, fmt.Errorf("resumed task not found: %s", taskID)
		}

		h.logger.Info("task resumed with user input",
			zap.String("task_id", taskID),
			zap.String("context_id", task.ContextID))

		return task, nil
	}

	originalContextID := params.Message.ContextID

	contextID := params.Message.ContextID
	if contextID == nil {
		newContextID := uuid.New().String()
		contextID = &newContextID
	}

	var task *types.Task
	if originalContextID != nil {
		conversationHistory := h.taskManager.GetConversationHistory(*contextID)

		if len(conversationHistory) > 0 {
			h.logger.Info("creating task with existing conversation history",
				zap.String("context_id", *contextID),
				zap.Int("history_count", len(conversationHistory)))
			task = h.taskManager.CreateTaskWithHistory(*contextID, types.TaskStateSubmitted, &params.Message, conversationHistory)
		} else {
			h.logger.Info("creating new task without history for existing context",
				zap.String("context_id", *contextID))
			task = h.taskManager.CreateTask(*contextID, types.TaskStateSubmitted, &params.Message)
		}
	} else {
		h.logger.Info("creating new task without history for new context",
			zap.String("context_id", *contextID))
		task = h.taskManager.CreateTask(*contextID, types.TaskStateSubmitted, &params.Message)
	}

	if task != nil {
		h.logger.Info("task created for processing",
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID))
	} else {
		h.logger.Error("failed to create task - task manager returned nil")
		return nil, fmt.Errorf("failed to create task")
	}
	return task, nil
}

// HandleMessageSend processes message/send requests
func (h *DefaultA2AProtocolHandler) HandleMessageSend(c *gin.Context, req types.JSONRPCRequest) {
	var params types.MessageSendParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		h.logger.Error("failed to marshal params", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		h.logger.Error("failed to parse message/send request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	task, err := h.createTaskFromMessage(c.Request.Context(), params)
	if err != nil {
		h.logger.Error("failed to create task", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	err = h.storage.EnqueueTask(task, req.ID)
	if err != nil {
		h.logger.Error("failed to enqueue task", zap.Error(err))
		err := h.taskManager.UpdateError(task.ID, &types.Message{
			Kind:      "message",
			MessageID: uuid.New().String(),
			Role:      "assistant",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Failed to queue task for processing. Please try again later.",
				},
			},
		})
		if err != nil {
			h.logger.Error("failed to update task to failed state due to enqueue failure",
				zap.Error(err),
				zap.String("task_id", task.ID),
				zap.String("context_id", task.ContextID))
		}
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), "Failed to queue task")
		return
	}

	h.responseSender.SendSuccess(c, req.ID, *task)
}

// writeStreamingResponse writes a JSON-RPC response to the streaming connection in SSE format
func (h *DefaultA2AProtocolHandler) writeStreamingResponse(c *gin.Context, response *types.JSONRPCSuccessResponse) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if _, err := c.Writer.Write([]byte("data: ")); err != nil {
		return fmt.Errorf("failed to write data prefix: %w", err)
	}

	if _, err := c.Writer.Write(responseBytes); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
		return fmt.Errorf("failed to write SSE terminator: %w", err)
	}

	c.Writer.Flush()
	return nil
}

// writeStreamingErrorResponse writes a JSON-RPC error response to the streaming connection in SSE format
func (h *DefaultA2AProtocolHandler) writeStreamingErrorResponse(c *gin.Context, response *types.JSONRPCErrorResponse) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal error response: %w", err)
	}

	if _, err := c.Writer.Write([]byte("data: ")); err != nil {
		return fmt.Errorf("failed to write data prefix: %w", err)
	}

	if _, err := c.Writer.Write(responseBytes); err != nil {
		return fmt.Errorf("failed to write error response: %w", err)
	}

	if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
		return fmt.Errorf("failed to write SSE terminator: %w", err)
	}

	c.Writer.Flush()
	return nil
}

// HandleMessageStream processes message/stream requests
func (h *DefaultA2AProtocolHandler) HandleMessageStream(c *gin.Context, req types.JSONRPCRequest) {
	var params types.MessageSendParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		h.logger.Error("failed to marshal params", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		h.logger.Error("failed to parse message/stream request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	ctx := c.Request.Context()

	task, err := h.createTaskFromMessage(ctx, params)
	if err != nil {
		h.logger.Error("failed to create streaming task", zap.Error(err))
		errorResponse := types.JSONRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &types.JSONRPCError{
				Code:    int(ErrInternalError),
				Message: err.Error(),
			},
		}
		if writeErr := h.writeStreamingErrorResponse(c, &errorResponse); writeErr != nil {
			h.logger.Error("failed to write streaming error response", zap.Error(writeErr))
		}
		return
	}

	h.logger.Info("processing streaming task",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID))

	err = h.taskManager.UpdateState(task.ID, types.TaskStateWorking)
	if err != nil {
		h.logger.Error("failed to update streaming task state", zap.Error(err))
		return
	}

	var message *types.Message
	if task.Status.Message != nil {
		message = task.Status.Message
	} else {
		message = &types.Message{
			Kind:      "message",
			MessageID: uuid.New().String(),
			Role:      "user",
			Parts:     []types.Part{},
		}
	}

	eventsChan, err := h.streamingTaskHandler.HandleStreamingTask(ctx, task, message)
	if err != nil {
		h.logger.Error("failed to start streaming task",
			zap.Error(err),
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID))

		errorResponse := types.JSONRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &types.JSONRPCError{
				Code:    int(ErrInternalError),
				Message: err.Error(),
			},
		}
		if writeErr := h.writeStreamingErrorResponse(c, &errorResponse); writeErr != nil {
			h.logger.Error("failed to write streaming error response", zap.Error(writeErr))
		}
		return
	}

	var finalTask *types.Task
	for event := range eventsChan {
		switch event.GetEventType() {
		case "delta":
			deltaData := event.GetData()
			var deltaMessage *types.Message

			switch msg := deltaData.(type) {
			case types.Message:
				deltaMessage = &msg
			case *types.Message:
				deltaMessage = msg
			}

			if deltaMessage != nil {
				statusUpdate := types.TaskStatusUpdateEvent{
					Kind:      "status-update",
					TaskID:    task.ID,
					ContextID: task.ContextID,
					Status: types.TaskStatus{
						State:   "working",
						Message: deltaMessage,
					},
					Final: false,
				}

				deltaResponse := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  statusUpdate,
				}
				if err := h.writeStreamingResponse(c, &deltaResponse); err != nil {
					h.logger.Error("failed to write streaming delta", zap.Error(err))
					return
				}
			}
		case "artifact_update":
			if artifactEvent, ok := event.GetData().(types.TaskArtifactUpdateEvent); ok {
				h.logger.Debug("received artifact update",
					zap.String("artifact_id", artifactEvent.Artifact.ArtifactID),
					zap.String("task_id", artifactEvent.TaskID))

				artifactResponse := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  artifactEvent,
				}
				if err := h.writeStreamingResponse(c, &artifactResponse); err != nil {
					h.logger.Error("failed to write streaming artifact update", zap.Error(err))
					return
				}
			}
		case "status":
			h.logger.Debug("received status update", zap.Any("status", event.GetData()))
		case "task_complete":
			if taskData, ok := event.GetData().(*types.Task); ok {
				finalTask = taskData
			}
		case "task_interrupted":
			if taskData, ok := event.GetData().(*types.Task); ok {
				h.logger.Info("streaming task was interrupted, saving task with preserved history",
					zap.String("task_id", taskData.ID),
					zap.String("context_id", taskData.ContextID),
					zap.Int("history_count", len(taskData.History)))

				if err := h.taskManager.UpdateTask(taskData); err != nil {
					h.logger.Error("failed to save interrupted task to storage",
						zap.String("task_id", taskData.ID),
						zap.Error(err))
				} else {
					h.logger.Info("successfully saved interrupted task with history to storage",
						zap.String("task_id", taskData.ID),
						zap.Int("history_count", len(taskData.History)))
				}
				return
			}
		case "error":
			h.logger.Error("streaming task error", zap.Any("error", event.GetData()))

			if taskData, ok := event.GetData().(*types.Task); ok && taskData != nil {
				h.logger.Info("saving failed streaming task with error in history",
					zap.String("task_id", taskData.ID),
					zap.String("context_id", taskData.ContextID),
					zap.Int("history_count", len(taskData.History)))

				if err := h.taskManager.UpdateTask(taskData); err != nil {
					h.logger.Error("failed to save failed streaming task to storage",
						zap.String("task_id", taskData.ID),
						zap.Error(err))
				}
			}

			errorResponse := types.JSONRPCErrorResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &types.JSONRPCError{
					Code:    int(ErrInternalError),
					Message: fmt.Sprintf("streaming error: %v", event.GetData()),
				},
			}
			if writeErr := h.writeStreamingErrorResponse(c, &errorResponse); writeErr != nil {
				h.logger.Error("failed to write streaming error response", zap.Error(writeErr))
			}
			return
		}
	}

	if finalTask != nil {
		finalResponse := types.TaskStatusUpdateEvent{
			Kind:      "status-update",
			TaskID:    finalTask.ID,
			ContextID: finalTask.ContextID,
			Status:    finalTask.Status,
			Final:     true,
		}

		jsonRPCResponse := types.JSONRPCSuccessResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  finalResponse,
		}

		if err := h.writeStreamingResponse(c, &jsonRPCResponse); err != nil {
			h.logger.Error("failed to write final streaming response", zap.Error(err))
		} else {
			if err := h.taskManager.UpdateTask(finalTask); err != nil {
				h.logger.Error("failed to update streaming task",
					zap.Error(err),
					zap.String("task_id", finalTask.ID),
					zap.String("context_id", finalTask.ContextID))
			}
		}
	}

	if _, err := c.Writer.Write([]byte("data: [DONE]\n\n")); err != nil {
		h.logger.Error("failed to write stream termination signal", zap.Error(err))
	} else {
		c.Writer.Flush()
		h.logger.Debug("sent stream termination signal [DONE]")
	}

	h.logger.Info("streaming task processed successfully",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID))
}

// HandleTaskGet processes tasks/get requests
func (h *DefaultA2AProtocolHandler) HandleTaskGet(c *gin.Context, req types.JSONRPCRequest) {
	var params types.TaskQueryParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		h.logger.Error("failed to marshal params", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		h.logger.Error("failed to parse tasks/get request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("retrieving task", zap.String("task_id", params.ID))

	task, exists := h.taskManager.GetTask(params.ID)
	if !exists {
		h.logger.Error("task not found", zap.String("task_id", params.ID))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "task not found")
		return
	}

	h.logger.Info("task retrieved successfully",
		zap.String("task_id", params.ID),
		zap.String("context_id", task.ContextID),
		zap.String("status", string(task.Status.State)))
	h.responseSender.SendSuccess(c, req.ID, *task)
}

// HandleTaskCancel processes tasks/cancel requests
func (h *DefaultA2AProtocolHandler) HandleTaskCancel(c *gin.Context, req types.JSONRPCRequest) {
	var params types.TaskIdParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		h.logger.Error("failed to marshal params", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		h.logger.Error("failed to parse tasks/cancel request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("canceling task", zap.String("task_id", params.ID))

	err = h.taskManager.CancelTask(params.ID)
	if err != nil {
		h.logger.Error("failed to cancel task",
			zap.Error(err),
			zap.String("task_id", params.ID))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), err.Error())
		return
	}

	task, _ := h.taskManager.GetTask(params.ID)
	h.responseSender.SendSuccess(c, req.ID, *task)
}

// HandleTaskList processes tasks/list requests
func (h *DefaultA2AProtocolHandler) HandleTaskList(c *gin.Context, req types.JSONRPCRequest) {
	var params types.TaskListParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		h.logger.Error("failed to marshal params", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		h.logger.Error("failed to parse tasks/list request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("listing tasks")

	taskList, err := h.taskManager.ListTasks(params)
	if err != nil {
		h.logger.Error("failed to list tasks", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	h.logger.Info("tasks listed successfully", zap.Int("count", len(taskList.Tasks)), zap.Int("total", taskList.Total))
	h.responseSender.SendSuccess(c, req.ID, taskList)
}

// HandleTaskPushNotificationConfigSet processes tasks/pushNotificationConfig/set requests
func (h *DefaultA2AProtocolHandler) HandleTaskPushNotificationConfigSet(c *gin.Context, req types.JSONRPCRequest) {
	var params types.TaskPushNotificationConfig
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		h.logger.Error("failed to marshal params", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		h.logger.Error("failed to parse tasks/pushNotificationConfig/set request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("setting push notification config for task",
		zap.String("task_id", params.TaskID),
		zap.String("url", params.PushNotificationConfig.URL))

	config, err := h.taskManager.SetTaskPushNotificationConfig(params)
	if err != nil {
		h.logger.Error("failed to set push notification config", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	h.logger.Info("push notification config set successfully", zap.String("task_id", params.TaskID))
	h.responseSender.SendSuccess(c, req.ID, config)
}

// HandleTaskPushNotificationConfigGet processes tasks/pushNotificationConfig/get requests
func (h *DefaultA2AProtocolHandler) HandleTaskPushNotificationConfigGet(c *gin.Context, req types.JSONRPCRequest) {
	var params types.GetTaskPushNotificationConfigParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		h.logger.Error("failed to marshal params", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		h.logger.Error("failed to parse tasks/pushNotificationConfig/get request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("getting push notification config for task", zap.String("task_id", params.ID))

	config, err := h.taskManager.GetTaskPushNotificationConfig(params)
	if err != nil {
		h.logger.Error("failed to get push notification config", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	h.logger.Info("push notification config retrieved successfully", zap.String("task_id", params.ID))
	h.responseSender.SendSuccess(c, req.ID, config)
}

// HandleTaskPushNotificationConfigList processes tasks/pushNotificationConfig/list requests
func (h *DefaultA2AProtocolHandler) HandleTaskPushNotificationConfigList(c *gin.Context, req types.JSONRPCRequest) {
	var params types.ListTaskPushNotificationConfigParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		h.logger.Error("failed to marshal params", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		h.logger.Error("failed to parse tasks/pushNotificationConfig/list request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("listing push notification configs for task", zap.String("task_id", params.ID))

	configs, err := h.taskManager.ListTaskPushNotificationConfigs(params)
	if err != nil {
		h.logger.Error("failed to list push notification configs", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	h.logger.Info("push notification configs listed successfully",
		zap.String("task_id", params.ID),
		zap.Int("count", len(configs)))
	h.responseSender.SendSuccess(c, req.ID, configs)
}

// HandleTaskPushNotificationConfigDelete processes tasks/pushNotificationConfig/delete requests
func (h *DefaultA2AProtocolHandler) HandleTaskPushNotificationConfigDelete(c *gin.Context, req types.JSONRPCRequest) {
	var params types.DeleteTaskPushNotificationConfigParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		h.logger.Error("failed to marshal params", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		h.logger.Error("failed to parse tasks/pushNotificationConfig/delete request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("deleting push notification config",
		zap.String("task_id", params.ID),
		zap.String("config_id", params.PushNotificationConfigID))

	err = h.taskManager.DeleteTaskPushNotificationConfig(params)
	if err != nil {
		h.logger.Error("failed to delete push notification config", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	h.logger.Info("push notification config deleted successfully",
		zap.String("task_id", params.ID),
		zap.String("config_id", params.PushNotificationConfigID))
	h.responseSender.SendSuccess(c, req.ID, nil)
}
