package server

import (
	"context"
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	gin "github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// Context keys for injecting Task and ArtifactHelper into tool execution
type ContextKey string

const (
	TaskContextKey           ContextKey = "task"
	ArtifactHelperContextKey ContextKey = "artifactHelper"
)

// A2AProtocolHandler defines the interface for handling A2A protocol requests
type A2AProtocolHandler interface {
	// HandleMessageSend processes message/send requests
	HandleMessageSend(c *gin.Context, req types.JSONRPCRequest)

	// HandleMessageStream processes message/stream requests
	HandleMessageStream(c *gin.Context, req types.JSONRPCRequest, streamingHandler StreamableTaskHandler)

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
	// HandleStreamingTask processes a task and returns a channel of CloudEvents
	// The channel should be closed when streaming is complete
	// Event flow: agent → handler → protocol handler → client
	HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error)

	// SetAgent sets the OpenAI-compatible agent for the task handler
	SetAgent(agent OpenAICompatibleAgent)

	// GetAgent returns the configured OpenAI-compatible agent
	GetAgent() OpenAICompatibleAgent
}

// DefaultBackgroundTaskHandler implements the TaskHandler interface optimized for background scenarios
// This handler automatically handles input-required pausing without requiring custom implementation
type DefaultBackgroundTaskHandler struct {
	logger  *zap.Logger
	agent   OpenAICompatibleAgent
	storage ArtifactStorageProvider
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

	artifactHelper := NewArtifactHelper()
	if bth.storage != nil {
		artifactHelper.SetStorage(bth.storage)
	}
	toolCtx := context.WithValue(ctx, TaskContextKey, task)
	toolCtx = context.WithValue(toolCtx, ArtifactHelperContextKey, artifactHelper)

	eventChan, err := bth.agent.RunWithStream(toolCtx, messages)
	if err != nil {
		bth.logger.Error("agent streaming failed to start", zap.Error(err))

		task.Status.State = types.TaskStateFailed
		task.Status.Message = &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("error-%s", task.ID),
			Role:      "assistant",
			TaskID:    &task.ID,
			ContextID: &task.ContextID,
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": fmt.Sprintf("Failed to start agent: %s", err.Error()),
				},
			},
		}
		return task, nil
	}

	var finalMessage *types.Message

	for event := range eventChan {
		eventType := event.Type()
		bth.logger.Debug("background handler received event",
			zap.String("task_id", task.ID),
			zap.String("event_type", eventType))

		switch eventType {
		case types.EventTaskStatusChanged:
			var statusData types.TaskStatus
			if err := event.DataAs(&statusData); err == nil {
				task.Status.State = statusData.State
				if statusData.Message != nil {
					task.Status.Message = statusData.Message
				}

				bth.logger.Info("background task status changed",
					zap.String("task_id", task.ID),
					zap.String("state", string(statusData.State)))

				if statusData.State == types.TaskStateCompleted ||
					statusData.State == types.TaskStateFailed ||
					statusData.State == types.TaskStateCanceled {
					return task, nil
				}
			}

		case types.EventIterationCompleted:
			var iterationMessage types.Message
			if err := event.DataAs(&iterationMessage); err == nil {
				finalMessage = &iterationMessage
				bth.logger.Debug("captured iteration message",
					zap.String("task_id", task.ID),
					zap.String("message_kind", iterationMessage.Kind))
			}

		case types.EventInputRequired:
			var inputMessage types.Message
			if err := event.DataAs(&inputMessage); err == nil {
				if task.History == nil {
					task.History = []types.Message{}
				}
				task.History = append(task.History, inputMessage)

				task.Status.State = types.TaskStateInputRequired
				task.Status.Message = &inputMessage

				bth.logger.Info("background task paused for user input",
					zap.String("task_id", task.ID),
					zap.String("state", string(task.Status.State)))

				return task, nil
			}

		case types.EventDelta:
			continue

		case types.EventToolStarted, types.EventToolCompleted, types.EventToolFailed, types.EventToolResult:
			bth.logger.Debug("tool event in background task",
				zap.String("task_id", task.ID),
				zap.String("event_type", eventType))
		}
	}

	if finalMessage != nil {
		task.Status.State = types.TaskStateCompleted
		task.Status.Message = finalMessage

		bth.logger.Info("background task completed successfully",
			zap.String("task_id", task.ID))

		return task, nil
	}

	bth.logger.Warn("background task completed but no final message received",
		zap.String("task_id", task.ID))

	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("empty-response-%s", task.ID),
		Role:      "assistant",
		TaskID:    &task.ID,
		ContextID: &task.ContextID,
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Task completed",
			},
		},
	}

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
		TaskID:    &task.ID,
		ContextID: &task.ContextID,
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

// DefaultStreamingTaskHandler implements the TaskHandler interface optimized for streaming scenarios
// This handler automatically handles input-required pausing with streaming-aware behavior
type DefaultStreamingTaskHandler struct {
	logger  *zap.Logger
	agent   OpenAICompatibleAgent
	storage ArtifactStorageProvider
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

// HandleStreamingTask processes a task and returns a channel of CloudEvents
// It forwards events from the agent directly without conversion
func (sth *DefaultStreamingTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error) {
	sth.logger.Info("processing streaming task",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.Bool("has_agent", sth.agent != nil))

	if sth.agent == nil {
		return nil, fmt.Errorf("streaming task handler requires an agent to be configured - use SetAgent() to configure an OpenAI-compatible agent for streaming support")
	}

	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

	artifactHelper := NewArtifactHelper()
	if sth.storage != nil {
		artifactHelper.SetStorage(sth.storage)
	}
	toolCtx := context.WithValue(ctx, TaskContextKey, task)
	toolCtx = context.WithValue(toolCtx, ArtifactHelperContextKey, artifactHelper)

	return sth.agent.RunWithStream(toolCtx, messages)
}

// DefaultA2AProtocolHandler implements the A2AProtocolHandler interface
type DefaultA2AProtocolHandler struct {
	logger         *zap.Logger
	storage        Storage
	taskManager    TaskManager
	responseSender ResponseSender
}

// NewDefaultA2AProtocolHandler creates a new default A2A protocol handler
func NewDefaultA2AProtocolHandler(
	logger *zap.Logger,
	storage Storage,
	taskManager TaskManager,
	responseSender ResponseSender,
) *DefaultA2AProtocolHandler {
	return &DefaultA2AProtocolHandler{
		logger:         logger,
		storage:        storage,
		taskManager:    taskManager,
		responseSender: responseSender,
	}
}

// CreateTaskFromMessage creates a task directly from message parameters
func (h *DefaultA2AProtocolHandler) CreateTaskFromMessage(ctx context.Context, params types.MessageSendParams) (*types.Task, error) {
	if len(params.Message.Parts) == 0 {
		return nil, fmt.Errorf("empty message parts not allowed")
	}

	enrichedMessage := params.Message
	if enrichedMessage.Kind == "" {
		enrichedMessage.Kind = "message"
	}
	if enrichedMessage.MessageID == "" {
		enrichedMessage.MessageID = uuid.New().String()
	}

	if params.Message.TaskID != nil {
		taskID := *params.Message.TaskID

		err := h.taskManager.ResumeTaskWithInput(taskID, &enrichedMessage)
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
			task = h.taskManager.CreateTaskWithHistory(*contextID, types.TaskStateSubmitted, &enrichedMessage, conversationHistory)
		} else {
			h.logger.Info("creating new task without history for existing context",
				zap.String("context_id", *contextID))
			task = h.taskManager.CreateTask(*contextID, types.TaskStateSubmitted, &enrichedMessage)
		}
	} else {
		h.logger.Info("creating new task without history for new context",
			zap.String("context_id", *contextID))
		task = h.taskManager.CreateTask(*contextID, types.TaskStateSubmitted, &enrichedMessage)
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

	task, err := h.CreateTaskFromMessage(c.Request.Context(), params)
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
			TaskID:    &task.ID,
			ContextID: &task.ContextID,
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
func (h *DefaultA2AProtocolHandler) HandleMessageStream(c *gin.Context, req types.JSONRPCRequest, streamingHandler StreamableTaskHandler) {
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

	task, err := h.CreateTaskFromMessage(ctx, params)
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

	eventsChan, err := streamingHandler.HandleStreamingTask(ctx, task, message)
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

	var accumulatedText string

	for event := range eventsChan {
		switch event.Type() {
		case types.EventDelta:
			var deltaMessage types.Message
			if err := event.DataAs(&deltaMessage); err == nil {
				for _, part := range deltaMessage.Parts {
					if textPart, ok := part.(types.TextPart); ok {
						accumulatedText += textPart.Text
					}
				}
				h.logger.Debug("accumulated delta text",
					zap.String("task_id", task.ID),
					zap.Int("total_length", len(accumulatedText)))

				task.Status.Message = &deltaMessage
				task.Status.State = types.TaskStateWorking

				deltaResponse := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  *task,
				}

				if err := h.writeStreamingResponse(c, &deltaResponse); err != nil {
					h.logger.Error("failed to write delta", zap.Error(err))
					return
				}
			}

		case types.EventIterationCompleted:
			var iterationMessage types.Message
			if err := event.DataAs(&iterationMessage); err == nil {
				task.History = append(task.History, iterationMessage)
				h.logger.Debug("stored iteration completed message to history",
					zap.String("task_id", task.ID),
					zap.String("message_id", iterationMessage.MessageID),
					zap.Int("history_size", len(task.History)))
			}

		case types.EventTaskStatusChanged:
			var statusData types.TaskStatus
			if err := event.DataAs(&statusData); err == nil {
				h.logger.Info("task state changed",
					zap.String("task_id", task.ID),
					zap.String("new_state", string(statusData.State)))

				task.Status.State = statusData.State

				statusUpdate := types.TaskStatusUpdateEvent{
					Kind:      "status-update",
					TaskID:    task.ID,
					ContextID: task.ContextID,
					Status:    statusData,
					Final:     statusData.State == types.TaskStateCompleted || statusData.State == types.TaskStateFailed || statusData.State == types.TaskStateCanceled,
				}

				statusResponse := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  statusUpdate,
				}

				if err := h.writeStreamingResponse(c, &statusResponse); err != nil {
					h.logger.Error("failed to write status change", zap.Error(err))
					return
				}
			}

		case types.EventInputRequired:
			var inputMessage types.Message
			if err := event.DataAs(&inputMessage); err == nil {
				task.History = append(task.History, inputMessage)
				task.Status.State = types.TaskStateInputRequired
				task.Status.Message = &inputMessage

				h.logger.Info("streaming task paused for user input",
					zap.String("task_id", task.ID),
					zap.String("context_id", task.ContextID))

				statusUpdate := types.TaskStatusUpdateEvent{
					Kind:      "status-update",
					TaskID:    task.ID,
					ContextID: task.ContextID,
					Status: types.TaskStatus{
						State:   types.TaskStateInputRequired,
						Message: &inputMessage,
					},
					Final: false,
				}

				statusResponse := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  statusUpdate,
				}

				if err := h.writeStreamingResponse(c, &statusResponse); err != nil {
					h.logger.Error("failed to write input-required status", zap.Error(err))
					return
				}

				if err := h.taskManager.UpdateTask(task); err != nil {
					h.logger.Error("failed to save input-required task",
						zap.String("task_id", task.ID),
						zap.Error(err))
				}
				return
			}

		case types.EventTaskInterrupted:
			var interruptMessage types.Message
			if err := event.DataAs(&interruptMessage); err == nil {
				task.History = append(task.History, interruptMessage)
				task.Status.State = types.TaskStateCanceled

				h.logger.Info("streaming task was interrupted",
					zap.String("task_id", task.ID),
					zap.String("context_id", task.ContextID))

				if err := h.taskManager.UpdateTask(task); err != nil {
					h.logger.Error("failed to save interrupted task",
						zap.String("task_id", task.ID),
						zap.Error(err))
				}
				return
			}

		case types.EventStreamFailed:
			var errorMessage types.Message
			if err := event.DataAs(&errorMessage); err == nil {
				task.History = append(task.History, errorMessage)
				task.Status.State = types.TaskStateFailed
				task.Status.Message = &errorMessage

				h.logger.Error("streaming task failed",
					zap.String("task_id", task.ID))

				if err := h.taskManager.UpdateTask(task); err != nil {
					h.logger.Error("failed to save failed task",
						zap.String("task_id", task.ID),
						zap.Error(err))
				}

				errorResponse := types.JSONRPCErrorResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &types.JSONRPCError{
						Code:    int(ErrInternalError),
						Message: "streaming failed",
					},
				}
				if writeErr := h.writeStreamingErrorResponse(c, &errorResponse); writeErr != nil {
					h.logger.Error("failed to write error response", zap.Error(writeErr))
				}
				return
			}
		}
	}

	if len(task.History) > 0 {
		task.Status.State = types.TaskStateCompleted
		task.Status.Message = &task.History[len(task.History)-1]

		if err := h.taskManager.UpdateTask(task); err != nil {
			h.logger.Error("failed to update completed task",
				zap.Error(err),
				zap.String("task_id", task.ID))
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
