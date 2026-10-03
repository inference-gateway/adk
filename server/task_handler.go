package server

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	gin "github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	zap "go.uber.org/zap"

	types "github.com/inference-gateway/adk/types"
)

// Context keys for injecting dependencies into tool execution
type ContextKey string

const (
	TaskContextKey            ContextKey = "task"
	ArtifactServiceContextKey ContextKey = "artifactService"
	UsageTrackerContextKey    ContextKey = "usageTracker"
)

const taskPollInterval = 50 * time.Millisecond

// taskInputMessage returns the task's status message, or an empty user
// placeholder when the task has none, so handlers always get a message.
func taskInputMessage(task *types.Task) *types.Message {
	if task.Status.Message != nil {
		return task.Status.Message
	}
	return &types.Message{
		MessageID: uuid.New().String(),
		Role:      types.RoleUser,
		Parts:     []types.Part{},
	}
}

// A2AProtocolHandler defines the interface for handling A2A protocol requests
type A2AProtocolHandler interface {
	// HandleMessageSend processes SendMessage requests
	HandleMessageSend(c *gin.Context, req types.JSONRPCRequest)

	// HandleMessageStream processes SendStreamingMessage requests
	HandleMessageStream(c *gin.Context, req types.JSONRPCRequest, streamingHandler StreamableTaskHandler)

	// HandleTaskGet processes GetTask requests
	HandleTaskGet(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskList processes ListTasks requests
	HandleTaskList(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskCancel processes CancelTask requests
	HandleTaskCancel(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskPushNotificationConfigSet processes CreateTaskPushNotificationConfig requests
	HandleTaskPushNotificationConfigSet(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskPushNotificationConfigGet processes GetTaskPushNotificationConfig requests
	HandleTaskPushNotificationConfigGet(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskPushNotificationConfigList processes ListTaskPushNotificationConfigs requests
	HandleTaskPushNotificationConfigList(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskPushNotificationConfigDelete processes DeleteTaskPushNotificationConfig requests
	HandleTaskPushNotificationConfigDelete(c *gin.Context, req types.JSONRPCRequest)

	// HandleTaskResubscribe processes SubscribeToTask requests, re-attaching a streaming
	// subscription for an existing task and emitting its current state via SSE.
	HandleTaskResubscribe(c *gin.Context, req types.JSONRPCRequest, streamingHandler StreamableTaskHandler)

	// HandleGetAuthenticatedExtendedCard processes GetExtendedAgentCard requests.
	// It enforces the spec section 3.3.4 error contract using the served public card and the
	// optional extended card: ErrUnsupportedOperation (-32004) when the public card does not
	// declare supportsExtendedAgentCard, ErrExtendedAgentCardNotConfigured (-32007) when it
	// does but no extended card is configured, otherwise the extended card is returned.
	HandleGetAuthenticatedExtendedCard(c *gin.Context, req types.JSONRPCRequest, publicCard *types.AgentCard, extendedCard *types.AgentCard)
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
	logger              *zap.Logger
	agent               OpenAICompatibleAgent
	artifactService     ArtifactService
	enableUsageMetadata bool
}

// NewDefaultBackgroundTaskHandler creates a new default background task handler
func NewDefaultBackgroundTaskHandler(logger *zap.Logger, agent OpenAICompatibleAgent) *DefaultBackgroundTaskHandler {
	return &DefaultBackgroundTaskHandler{
		logger:              logger,
		agent:               agent,
		enableUsageMetadata: true,
	}
}

// NewDefaultBackgroundTaskHandlerWithAgent creates a new default background task handler with an agent
//
// Deprecated: Use NewDefaultBackgroundTaskHandler.
func NewDefaultBackgroundTaskHandlerWithAgent(logger *zap.Logger, agent OpenAICompatibleAgent) *DefaultBackgroundTaskHandler {
	return NewDefaultBackgroundTaskHandler(logger, agent)
}

// SetAgent sets the agent for the task handler
func (bth *DefaultBackgroundTaskHandler) SetAgent(agent OpenAICompatibleAgent) {
	bth.agent = agent
}

// GetAgent returns the configured agent
func (bth *DefaultBackgroundTaskHandler) GetAgent() OpenAICompatibleAgent {
	return bth.agent
}

// SetEnableUsageMetadata toggles whether usage metadata (token counts and
// execution statistics) is attached to the task's metadata when the task
// reaches a terminal state.
func (bth *DefaultBackgroundTaskHandler) SetEnableUsageMetadata(enabled bool) {
	bth.enableUsageMetadata = enabled
}

// IsUsageMetadataEnabled reports whether the handler will attach usage
// metadata to completed tasks.
func (bth *DefaultBackgroundTaskHandler) IsUsageMetadataEnabled() bool {
	return bth.enableUsageMetadata
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

	usageTracker := NewUsageTracker()

	toolCtx := newToolContext(ctx, task, usageTracker, bth.artifactService)

	eventChan, err := bth.agent.RunWithStream(toolCtx, messages)
	if err != nil {
		bth.logger.Error("agent streaming failed to start", zap.Error(err))

		task.Status.State = types.TaskStateFailed
		task.Status.Message = &types.Message{
			MessageID: fmt.Sprintf("error-%s", task.ID),
			Role:      types.RoleAgent,
			TaskID:    &task.ID,
			ContextID: task.ContextID,
			Parts: []types.Part{
				types.CreateTextPart(fmt.Sprintf("Failed to start agent: %s", err.Error())),
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

				if statusData.State.IsTerminal() {
					bth.populateTaskMetadata(task, usageTracker)
					return task, nil
				}
			}

		case types.EventIterationCompleted:
			var iterationMessage types.Message
			if err := event.DataAs(&iterationMessage); err == nil {
				finalMessage = &iterationMessage
				bth.logger.Debug("captured iteration message",
					zap.String("task_id", task.ID),
					zap.String("message_id", iterationMessage.MessageID))
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

		bth.populateTaskMetadata(task, usageTracker)
		return task, nil
	}

	bth.logger.Warn("background task completed but no final message received",
		zap.String("task_id", task.ID))

	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &types.Message{
		MessageID: fmt.Sprintf("empty-response-%s", task.ID),
		Role:      types.RoleAgent,
		TaskID:    &task.ID,
		ContextID: task.ContextID,
		Parts: []types.Part{
			types.CreateTextPart("Task completed"),
		},
	}

	bth.populateTaskMetadata(task, usageTracker)
	return task, nil
}

// processWithoutAgentBackground processes a task without agent capabilities for background
func (bth *DefaultBackgroundTaskHandler) processWithoutAgentBackground(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	bth.logger.Info("processing background task without agent",
		zap.String("task_id", task.ID))

	response := &types.Message{
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      types.RoleAgent,
		TaskID:    &task.ID,
		ContextID: task.ContextID,
		Parts: []types.Part{
			types.CreateTextPart("I received your message. I'm a default polling task handler without AI capabilities. To enable AI responses with automatic input-required pausing, configure an OpenAI-compatible agent."),
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
	logger              *zap.Logger
	agent               OpenAICompatibleAgent
	artifactService     ArtifactService
	enableUsageMetadata bool
}

// NewDefaultStreamingTaskHandler creates a new default streaming task handler
func NewDefaultStreamingTaskHandler(logger *zap.Logger, agent OpenAICompatibleAgent) *DefaultStreamingTaskHandler {
	return &DefaultStreamingTaskHandler{
		logger:              logger,
		agent:               agent,
		enableUsageMetadata: true,
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

// SetEnableUsageMetadata toggles whether usage metadata (token counts and
// execution statistics) is attached to the task's metadata when the task
// reaches a terminal state.
func (sth *DefaultStreamingTaskHandler) SetEnableUsageMetadata(enabled bool) {
	sth.enableUsageMetadata = enabled
}

// IsUsageMetadataEnabled reports whether the handler will attach usage
// metadata to completed tasks.
func (sth *DefaultStreamingTaskHandler) IsUsageMetadataEnabled() bool {
	return sth.enableUsageMetadata
}

// HandleStreamingTask processes a task and returns a channel of CloudEvents
// It forwards events from the agent directly without conversion
func (sth *DefaultStreamingTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error) {
	sth.logger.Info("processing streaming task",
		zap.String("task_id", task.ID),
		zap.Stringp("context_id", task.ContextID),
		zap.Bool("has_agent", sth.agent != nil))

	if sth.agent == nil {
		return nil, fmt.Errorf("streaming task handler requires an agent to be configured - use SetAgent() to configure an OpenAI-compatible agent for streaming support")
	}

	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

	usageTracker := NewUsageTracker()

	toolCtx := newToolContext(ctx, task, usageTracker, sth.artifactService)

	eventChan, err := sth.agent.RunWithStream(toolCtx, messages)
	if err != nil {
		return nil, err
	}

	wrappedChan := make(chan cloudevents.Event, 100)
	go func() {
		defer close(wrappedChan)
		for event := range eventChan {
			if event.Type() == types.EventTaskStatusChanged {
				var statusData types.TaskStatus
				if err := event.DataAs(&statusData); err == nil && statusData.State.IsTerminal() {
					sth.populateTaskMetadata(task, usageTracker)
				}
			}
			wrappedChan <- event
		}
	}()

	return wrappedChan, nil
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
func (h *DefaultA2AProtocolHandler) CreateTaskFromMessage(ctx context.Context, params types.SendMessageRequest) (*types.Task, error) {
	if len(params.Message.Parts) == 0 {
		return nil, fmt.Errorf("empty message parts not allowed")
	}

	enrichedMessage := params.Message
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
			zap.Stringp("context_id", task.ContextID))

		return task, h.registerPushConfig(params, task.ID)
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
			zap.Stringp("context_id", task.ContextID))
	} else {
		h.logger.Error("failed to create task - task manager returned nil")
		return nil, fmt.Errorf("failed to create task")
	}
	return task, h.registerPushConfig(params, task.ID)
}

// registerPushConfig stores the push notification config sent inline in the message
// configuration, if any, for the task (spec section 3.2.2).
func (h *DefaultA2AProtocolHandler) registerPushConfig(params types.SendMessageRequest, taskID string) error {
	if params.Configuration == nil || params.Configuration.TaskPushNotificationConfig == nil {
		return nil
	}
	config := *params.Configuration.TaskPushNotificationConfig
	config.TaskID = &taskID
	_, err := h.taskManager.SetTaskPushNotificationConfig(config)
	return err
}

// HandleMessageSend processes SendMessage requests
func (h *DefaultA2AProtocolHandler) HandleMessageSend(c *gin.Context, req types.JSONRPCRequest) {
	params, err := decodeParams[types.SendMessageRequest](req.Params)
	if err != nil {
		h.logger.Error("failed to parse SendMessage request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	task, err := h.CreateTaskFromMessage(c.Request.Context(), params)
	if err != nil {
		h.logger.Error("failed to create task", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(a2aErrorCode(err, ErrInternalError)), err.Error())
		return
	}

	workerCopy := *task
	err = h.storage.EnqueueTask(c.Request.Context(), &workerCopy, req.ID)
	if err != nil {
		h.logger.Error("failed to enqueue task", zap.Error(err))
		err := h.taskManager.UpdateError(task.ID, &types.Message{
			MessageID: uuid.New().String(),
			Role:      types.RoleAgent,
			TaskID:    &task.ID,
			ContextID: task.ContextID,
			Parts: []types.Part{
				types.CreateTextPart("Failed to queue task for processing. Please try again later."),
			},
		})
		if err != nil {
			h.logger.Error("failed to update task to failed state due to enqueue failure",
				zap.Error(err),
				zap.String("task_id", task.ID),
				zap.Stringp("context_id", task.ContextID))
		}
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), "Failed to queue task")
		return
	}

	config := params.Configuration
	if config == nil {
		config = &types.SendMessageConfiguration{}
	}
	if config.ReturnImmediately == nil || !*config.ReturnImmediately {
		task = h.pollTask(c.Request.Context(), task, isTerminalOrInterrupted, nil)
	}

	response := task.WithHistoryLength(config.HistoryLength)
	h.responseSender.SendSuccess(c, req.ID, types.SendMessageResponse{Task: &response})
}

// pollTask re-reads the task from the task store until done holds for its state or ctx ends,
// calling onChange (when non-nil) on every state change, and returns the latest snapshot.
// ponytail: polling works with every storage backend and remote workers; replace with storage
// change notifications if the polling load ever shows up.
func (h *DefaultA2AProtocolHandler) pollTask(ctx context.Context, task *types.Task, done func(types.TaskState) bool, onChange func(*types.Task) error) *types.Task {
	ticker := time.NewTicker(taskPollInterval)
	defer ticker.Stop()
	for !done(task.Status.State) {
		select {
		case <-ctx.Done():
			return task
		case <-ticker.C:
		}
		latest, ok := h.taskManager.GetTask(task.ID)
		if !ok || latest.Status.State == task.Status.State {
			continue
		}
		task = latest
		if onChange != nil && onChange(task) != nil {
			return task
		}
	}
	return task
}

// isTerminalOrInterrupted reports whether a blocking SendMessage may return (spec section 3.2.2).
func isTerminalOrInterrupted(state types.TaskState) bool {
	return state.IsTerminal() || state.IsInterrupted()
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

// writeArtifactUpdate records an EventTaskArtifactUpdated (a TaskArtifactUpdateEvent payload) on
// the task and streams it as an artifactUpdate response. Undecodable events are skipped.
func (h *DefaultA2AProtocolHandler) writeArtifactUpdate(c *gin.Context, id *types.Value, task *types.Task, event cloudevents.Event) error {
	var update types.TaskArtifactUpdateEvent
	if err := event.DataAs(&update); err != nil {
		h.logger.Warn("skipping undecodable artifact update", zap.String("task_id", task.ID), zap.Error(err))
		return nil
	}
	update.TaskID = task.ID
	update.ContextID = task.GetContextID()
	task.ApplyArtifactUpdate(update)
	return h.writeStreamingResponse(c, &types.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  types.StreamResponse{ArtifactUpdate: &update},
	})
}

// writeStatusUpdate streams the task's current status as a statusUpdate response.
func (h *DefaultA2AProtocolHandler) writeStatusUpdate(c *gin.Context, id *types.Value, task *types.Task) error {
	return h.writeStreamingResponse(c, &types.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: types.StreamResponse{StatusUpdate: &types.TaskStatusUpdateEvent{
			TaskID:    task.ID,
			ContextID: task.GetContextID(),
			Status:    task.Status,
		}},
	})
}

// HandleMessageStream processes SendStreamingMessage requests
func (h *DefaultA2AProtocolHandler) HandleMessageStream(c *gin.Context, req types.JSONRPCRequest, streamingHandler StreamableTaskHandler) {
	params, err := decodeParams[types.SendMessageRequest](req.Params)
	if err != nil {
		h.logger.Error("failed to parse SendStreamingMessage request", zap.Error(err))
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
			Error:   newJSONRPCError(int(ErrInternalError), err.Error()),
		}
		if writeErr := h.writeStreamingErrorResponse(c, &errorResponse); writeErr != nil {
			h.logger.Error("failed to write streaming error response", zap.Error(writeErr))
		}
		return
	}

	h.logger.Info("processing streaming task",
		zap.String("task_id", task.ID),
		zap.Stringp("context_id", task.ContextID))

	err = h.taskManager.UpdateState(task.ID, types.TaskStateWorking)
	if err != nil {
		h.logger.Error("failed to update streaming task state", zap.Error(err))
		return
	}

	var historyLength *int
	if params.Configuration != nil {
		historyLength = params.Configuration.HistoryLength
	}
	initialTask := task.WithHistoryLength(historyLength)
	initialResponse := types.JSONRPCSuccessResponse{JSONRPC: "2.0", ID: req.ID, Result: types.StreamResponse{Task: &initialTask}}
	if err := h.writeStreamingResponse(c, &initialResponse); err != nil {
		h.logger.Error("failed to write initial task", zap.Error(err))
		return
	}

	message := taskInputMessage(task)

	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if defaultTM, ok := h.taskManager.(*DefaultTaskManager); ok {
		defaultTM.RegisterTaskCancelFunc(task.ID, cancel)
	}

	eventsChan, err := streamingHandler.HandleStreamingTask(taskCtx, task, message)
	if err != nil {
		h.logger.Error("failed to start streaming task",
			zap.Error(err),
			zap.String("task_id", task.ID),
			zap.Stringp("context_id", task.ContextID))

		errorResponse := types.JSONRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   newJSONRPCError(int(ErrInternalError), err.Error()),
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
					if part.Text != nil {
						accumulatedText += *part.Text
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
					Result:  types.StreamResponse{StatusUpdate: &types.TaskStatusUpdateEvent{TaskID: task.ID, ContextID: task.GetContextID(), Status: task.Status}},
				}

				if err := h.writeStreamingResponse(c, &deltaResponse); err != nil {
					h.logger.Error("failed to write delta", zap.Error(err))
					return
				}
			}

		case types.EventTaskArtifactUpdated:
			if err := h.writeArtifactUpdate(c, req.ID, task, event); err != nil {
				h.logger.Error("failed to write artifact update", zap.Error(err))
				return
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
					TaskID:    task.ID,
					ContextID: task.GetContextID(),
					Status:    statusData,
				}

				statusResponse := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  types.StreamResponse{StatusUpdate: &statusUpdate},
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
					zap.Stringp("context_id", task.ContextID))

				statusUpdate := types.TaskStatusUpdateEvent{
					TaskID:    task.ID,
					ContextID: task.GetContextID(),
					Status: types.TaskStatus{
						State:   types.TaskStateInputRequired,
						Message: &inputMessage,
					},
				}

				statusResponse := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  types.StreamResponse{StatusUpdate: &statusUpdate},
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
					zap.Stringp("context_id", task.ContextID))

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
					Error:   newJSONRPCError(int(ErrInternalError), "streaming failed"),
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

	h.logger.Info("streaming task processed successfully",
		zap.String("task_id", task.ID),
		zap.Stringp("context_id", task.ContextID))
}

// HandleTaskGet processes GetTask requests
func (h *DefaultA2AProtocolHandler) HandleTaskGet(c *gin.Context, req types.JSONRPCRequest) {
	params, err := decodeParams[types.GetTaskRequest](req.Params)
	if err != nil {
		h.logger.Error("failed to parse GetTask request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("retrieving task", zap.String("task_id", params.ID))

	task, exists := h.taskManager.GetTask(params.ID)
	if !exists {
		h.logger.Error("task not found", zap.String("task_id", params.ID))
		h.responseSender.SendError(c, req.ID, int(ErrTaskNotFound), "task not found")
		return
	}

	h.logger.Info("task retrieved successfully",
		zap.String("task_id", params.ID),
		zap.Stringp("context_id", task.ContextID),
		zap.String("status", string(task.Status.State)))
	h.responseSender.SendSuccess(c, req.ID, task.WithHistoryLength(params.HistoryLength))
}

// HandleTaskCancel processes CancelTask requests
func (h *DefaultA2AProtocolHandler) HandleTaskCancel(c *gin.Context, req types.JSONRPCRequest) {
	params, err := decodeParams[types.CancelTaskRequest](req.Params)
	if err != nil {
		h.logger.Error("failed to parse CancelTask request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("canceling task", zap.String("task_id", params.ID))

	err = h.taskManager.CancelTask(params.ID)
	if err != nil {
		h.logger.Error("failed to cancel task",
			zap.Error(err),
			zap.String("task_id", params.ID))
		h.responseSender.SendError(c, req.ID, int(a2aErrorCode(err, ErrInvalidParams)), err.Error())
		return
	}

	task, _ := h.taskManager.GetTask(params.ID)
	h.responseSender.SendSuccess(c, req.ID, *task)
}

// HandleTaskList processes ListTasks requests
func (h *DefaultA2AProtocolHandler) HandleTaskList(c *gin.Context, req types.JSONRPCRequest) {
	params, err := decodeParams[types.ListTasksRequest](req.Params)
	if err != nil {
		h.logger.Error("failed to parse ListTasks request", zap.Error(err))
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

	h.logger.Info("tasks listed successfully", zap.Int("count", len(taskList.Tasks)), zap.Int("total", taskList.TotalSize))
	h.responseSender.SendSuccess(c, req.ID, taskList)
}

// HandleTaskPushNotificationConfigSet processes CreateTaskPushNotificationConfig requests
func (h *DefaultA2AProtocolHandler) HandleTaskPushNotificationConfigSet(c *gin.Context, req types.JSONRPCRequest) {
	params, err := decodeParams[types.TaskPushNotificationConfig](req.Params)
	if err != nil {
		h.logger.Error("failed to parse CreateTaskPushNotificationConfig request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("setting push notification config for task",
		zap.Stringp("task_id", params.TaskID),
		zap.String("url", params.URL))

	config, err := h.taskManager.SetTaskPushNotificationConfig(params)
	if err != nil {
		h.logger.Error("failed to set push notification config", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	h.logger.Info("push notification config set successfully", zap.Stringp("task_id", params.TaskID))
	h.responseSender.SendSuccess(c, req.ID, config)
}

// HandleTaskPushNotificationConfigGet processes GetTaskPushNotificationConfig requests
func (h *DefaultA2AProtocolHandler) HandleTaskPushNotificationConfigGet(c *gin.Context, req types.JSONRPCRequest) {
	params, err := decodeParams[types.GetTaskPushNotificationConfigRequest](req.Params)
	if err != nil {
		h.logger.Error("failed to parse GetTaskPushNotificationConfig request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("getting push notification config for task", zap.String("task_id", params.TaskID))

	config, err := h.taskManager.GetTaskPushNotificationConfig(params)
	if err != nil {
		h.logger.Error("failed to get push notification config", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	h.logger.Info("push notification config retrieved successfully", zap.String("task_id", params.TaskID))
	h.responseSender.SendSuccess(c, req.ID, config)
}

// HandleTaskPushNotificationConfigList processes ListTaskPushNotificationConfigs requests
func (h *DefaultA2AProtocolHandler) HandleTaskPushNotificationConfigList(c *gin.Context, req types.JSONRPCRequest) {
	params, err := decodeParams[types.ListTaskPushNotificationConfigsRequest](req.Params)
	if err != nil {
		h.logger.Error("failed to parse ListTaskPushNotificationConfigs request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("listing push notification configs for task", zap.String("task_id", params.TaskID))

	configs, err := h.taskManager.ListTaskPushNotificationConfigs(params)
	if err != nil {
		h.logger.Error("failed to list push notification configs", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	h.logger.Info("push notification configs listed successfully",
		zap.String("task_id", params.TaskID),
		zap.Int("count", len(configs)))
	h.responseSender.SendSuccess(c, req.ID, configs)
}

// HandleTaskPushNotificationConfigDelete processes DeleteTaskPushNotificationConfig requests
func (h *DefaultA2AProtocolHandler) HandleTaskPushNotificationConfigDelete(c *gin.Context, req types.JSONRPCRequest) {
	params, err := decodeParams[types.DeleteTaskPushNotificationConfigRequest](req.Params)
	if err != nil {
		h.logger.Error("failed to parse DeleteTaskPushNotificationConfig request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	h.logger.Info("deleting push notification config",
		zap.String("task_id", params.TaskID))

	err = h.taskManager.DeleteTaskPushNotificationConfig(params)
	if err != nil {
		h.logger.Error("failed to delete push notification config", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	h.logger.Info("push notification config deleted successfully",
		zap.String("task_id", params.TaskID))
	h.responseSender.SendSuccess(c, req.ID, nil)
}

// HandleTaskResubscribe processes SubscribeToTask requests.
//
// The request body is a `SubscribeToTaskRequest` carrying the task name (ID).
// If the task does not exist, a TaskNotFound error is returned. If it does, the
// current Task is the first stream event, and the stream closes once the task is
// done. When the task is still in a working state, the streaming handler is invoked
// to continue delivering live events for the task.
func (h *DefaultA2AProtocolHandler) HandleTaskResubscribe(c *gin.Context, req types.JSONRPCRequest, streamingHandler StreamableTaskHandler) {
	params, err := decodeParams[types.SubscribeToTaskRequest](req.Params)
	if err != nil {
		h.logger.Error("failed to parse SubscribeToTask request", zap.Error(err))
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	if params.ID == "" {
		h.logger.Error("SubscribeToTask missing task name")
		h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "task id is required")
		return
	}

	task, exists := h.taskManager.GetTask(params.ID)
	if !exists {
		h.logger.Error("task not found for resubscribe", zap.String("task_id", params.ID))
		h.responseSender.SendError(c, req.ID, int(ErrTaskNotFound), "task not found")
		return
	}

	if task.Status.State.IsTerminal() {
		h.responseSender.SendError(c, req.ID, int(ErrUnsupportedOperation), "cannot subscribe to a task in a terminal state")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	h.logger.Info("resubscribing to task",
		zap.String("task_id", task.ID),
		zap.Stringp("context_id", task.ContextID),
		zap.String("state", string(task.Status.State)))

	initialResponse := types.JSONRPCSuccessResponse{JSONRPC: "2.0", ID: req.ID, Result: types.StreamResponse{Task: task}}

	if err := h.writeStreamingResponse(c, &initialResponse); err != nil {
		h.logger.Error("failed to write initial resubscribe status", zap.Error(err))
		return
	}

	if task.Status.State.IsInterrupted() {
		h.pollTask(c.Request.Context(), task, types.TaskState.IsTerminal, func(latest *types.Task) error {
			return h.writeStatusUpdate(c, req.ID, latest)
		})
		return
	}

	if task.Status.State != types.TaskStateWorking && task.Status.State != types.TaskStateSubmitted {
		return
	}

	if streamingHandler == nil {
		h.logger.Warn("no streaming handler configured; resubscribe will end after sending current state",
			zap.String("task_id", task.ID))
		return
	}

	message := taskInputMessage(task)

	ctx := c.Request.Context()
	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if defaultTM, ok := h.taskManager.(*DefaultTaskManager); ok {
		defaultTM.RegisterTaskCancelFunc(task.ID, cancel)
	}

	eventsChan, err := streamingHandler.HandleStreamingTask(taskCtx, task, message)
	if err != nil {
		h.logger.Error("failed to resume streaming task",
			zap.Error(err),
			zap.String("task_id", task.ID))
		errorResponse := types.JSONRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   newJSONRPCError(int(ErrInternalError), err.Error()),
		}
		if writeErr := h.writeStreamingErrorResponse(c, &errorResponse); writeErr != nil {
			h.logger.Error("failed to write streaming error response", zap.Error(writeErr))
		}
		return
	}

	for event := range eventsChan {
		switch event.Type() {
		case types.EventDelta:
			var deltaMessage types.Message
			if err := event.DataAs(&deltaMessage); err == nil {
				task.Status.Message = &deltaMessage
				task.Status.State = types.TaskStateWorking
				deltaResponse := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  types.StreamResponse{StatusUpdate: &types.TaskStatusUpdateEvent{TaskID: task.ID, ContextID: task.GetContextID(), Status: task.Status}},
				}
				if err := h.writeStreamingResponse(c, &deltaResponse); err != nil {
					h.logger.Error("failed to write delta", zap.Error(err))
					return
				}
			}

		case types.EventTaskArtifactUpdated:
			if err := h.writeArtifactUpdate(c, req.ID, task, event); err != nil {
				h.logger.Error("failed to write artifact update", zap.Error(err))
				return
			}

		case types.EventTaskStatusChanged:
			var statusData types.TaskStatus
			if err := event.DataAs(&statusData); err == nil {
				task.Status.State = statusData.State
				statusEvent := types.TaskStatusUpdateEvent{
					TaskID:    task.ID,
					ContextID: task.GetContextID(),
					Status:    statusData,
				}
				statusResponse := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  types.StreamResponse{StatusUpdate: &statusEvent},
				}
				if err := h.writeStreamingResponse(c, &statusResponse); err != nil {
					h.logger.Error("failed to write status change", zap.Error(err))
					return
				}
			}
		}
	}

	h.logger.Info("task resubscribe completed",
		zap.String("task_id", task.ID),
		zap.Stringp("context_id", task.ContextID))
}

// HandleGetAuthenticatedExtendedCard processes GetExtendedAgentCard requests.
//
// Access to this endpoint is gated by the JSON-RPC route (protected by the configured
// authentication middleware when enabled), so reaching this method implies the caller has
// successfully authenticated. It enforces the spec section 3.3.4 error contract:
//   - public card does not declare supportsExtendedAgentCard -> ErrUnsupportedOperation (-32004)
//   - declared but no extended card configured -> ErrExtendedAgentCardNotConfigured (-32007)
//   - otherwise the extended card is returned.
func (h *DefaultA2AProtocolHandler) HandleGetAuthenticatedExtendedCard(c *gin.Context, req types.JSONRPCRequest, publicCard *types.AgentCard, extendedCard *types.AgentCard) {
	if publicCard == nil {
		h.logger.Error("no agent card configured for GetExtendedAgentCard")
		h.responseSender.SendError(c, req.ID, int(ErrInternalError), "agent card not configured")
		return
	}

	if publicCard.Capabilities.ExtendedAgentCard == nil || !*publicCard.Capabilities.ExtendedAgentCard {
		h.logger.Info("extended agent card not supported by this agent")
		h.responseSender.SendError(c, req.ID, int(ErrUnsupportedOperation), "extended agent card is not supported")
		return
	}

	if extendedCard == nil {
		h.logger.Info("extended agent card supported but not configured")
		h.responseSender.SendError(c, req.ID, int(ErrExtendedAgentCardNotConfigured), "extended agent card is not configured")
		return
	}

	if req.Params != nil {
		params, err := decodeParams[types.GetExtendedAgentCardRequest](req.Params)
		if err != nil {
			h.logger.Error("failed to parse GetExtendedAgentCard request", zap.Error(err))
			h.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
			return
		}
		h.logger.Info("returning authenticated extended agent card", zap.Stringp("tenant", params.Tenant))
	} else {
		h.logger.Info("returning authenticated extended agent card")
	}

	h.responseSender.SendSuccess(c, req.ID, *extendedCard)
}

// populateTaskMetadata merges the tracked usage statistics into the task metadata
func populateTaskMetadata(logger *zap.Logger, task *types.Task, usageTracker *UsageTracker) {
	if usageTracker == nil || !usageTracker.HasUsage() {
		return
	}

	if task.Metadata == nil {
		m := make(map[string]any)
		task.Metadata = &m
	}

	metadata := usageTracker.GetMetadata()
	maps.Copy(*task.Metadata, metadata)

	logger.Debug("populated task metadata with usage statistics",
		zap.String("task_id", task.ID),
		zap.Any("metadata", metadata))
}

func (bth *DefaultBackgroundTaskHandler) populateTaskMetadata(task *types.Task, usageTracker *UsageTracker) {
	if !bth.enableUsageMetadata {
		return
	}
	populateTaskMetadata(bth.logger, task, usageTracker)
}

func (sth *DefaultStreamingTaskHandler) populateTaskMetadata(task *types.Task, usageTracker *UsageTracker) {
	if !sth.enableUsageMetadata {
		return
	}
	populateTaskMetadata(sth.logger, task, usageTracker)
}

// newToolContext injects the task, usage tracker and artifact service for tool execution
func newToolContext(ctx context.Context, task *types.Task, usageTracker *UsageTracker, artifactService ArtifactService) context.Context {
	toolCtx := context.WithValue(ctx, TaskContextKey, task)
	toolCtx = context.WithValue(toolCtx, UsageTrackerContextKey, usageTracker)
	if artifactService != nil {
		toolCtx = context.WithValue(toolCtx, ArtifactServiceContextKey, artifactService)
	}
	return toolCtx
}
