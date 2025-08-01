package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	uuid "github.com/google/uuid"
	config "github.com/inference-gateway/adk/server/config"
	utils "github.com/inference-gateway/adk/server/utils"
	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// MessageHandler defines how to handle different types of A2A messages
type MessageHandler interface {
	// HandleMessageSend processes message/send requests
	HandleMessageSend(ctx context.Context, params types.MessageSendParams) (*types.Task, error)

	// HandleMessageStream processes message/stream requests (for streaming responses)
	HandleMessageStream(ctx context.Context, params types.MessageSendParams, responseChan chan<- types.SendStreamingMessageResponse) error
}

// DefaultMessageHandler implements the MessageHandler interface
type DefaultMessageHandler struct {
	logger        *zap.Logger
	taskManager   TaskManager
	agent         OpenAICompatibleAgent
	config        *config.Config
	llmClient     LLMClient
	converter     *utils.OptimizedMessageConverter
	toolBox       ToolBox
	maxIterations int
}

// NewDefaultMessageHandler creates a new default message handler
func NewDefaultMessageHandler(logger *zap.Logger, taskManager TaskManager, cfg *config.Config) *DefaultMessageHandler {
	if cfg == nil {
		logger.Fatal("config is required but was nil")
	}

	return &DefaultMessageHandler{
		logger:        logger,
		taskManager:   taskManager,
		agent:         nil,
		config:        cfg,
		llmClient:     nil,
		converter:     utils.NewOptimizedMessageConverter(logger),
		toolBox:       nil,
		maxIterations: cfg.AgentConfig.MaxChatCompletionIterations,
	}
}

// NewDefaultMessageHandlerWithAgent creates a new default message handler with an agent for streaming
func NewDefaultMessageHandlerWithAgent(logger *zap.Logger, taskManager TaskManager, agent OpenAICompatibleAgent, cfg *config.Config) *DefaultMessageHandler {
	if cfg == nil {
		logger.Fatal("config is required but was nil")
	}

	var llmClient LLMClient
	var toolBox ToolBox

	if agent != nil {
		llmClient = agent.GetLLMClient()
		toolBox = agent.GetToolBox()
	}

	return &DefaultMessageHandler{
		logger:        logger,
		taskManager:   taskManager,
		agent:         agent,
		config:        cfg,
		llmClient:     llmClient,
		converter:     utils.NewOptimizedMessageConverter(logger),
		toolBox:       toolBox,
		maxIterations: cfg.AgentConfig.MaxChatCompletionIterations,
	}
}

// HandleMessageSend processes message/send requests
func (mh *DefaultMessageHandler) HandleMessageSend(ctx context.Context, params types.MessageSendParams) (*types.Task, error) {
	if len(params.Message.Parts) == 0 {
		return nil, NewEmptyMessagePartsError()
	}

	contextID := params.Message.ContextID
	if contextID == nil {
		newContextID := uuid.New().String()
		contextID = &newContextID
	}

	task := mh.taskManager.CreateTask(*contextID, types.TaskStateSubmitted, &params.Message)

	if task != nil {
		mh.logger.Info("message send handled",
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID))
	} else {
		mh.logger.Error("failed to create task - task manager returned nil")
		return nil, fmt.Errorf("failed to create task")
	}
	return task, nil
}

// HandleMessageStream processes message/stream requests (for streaming responses)
func (mh *DefaultMessageHandler) HandleMessageStream(ctx context.Context, params types.MessageSendParams, responseChan chan<- types.SendStreamingMessageResponse) error {
	if len(params.Message.Parts) == 0 {
		return NewEmptyMessagePartsError()
	}

	contextID := params.Message.ContextID
	if contextID == nil {
		newContextID := uuid.New().String()
		contextID = &newContextID
	}

	task := mh.taskManager.CreateTask(*contextID, types.TaskStateWorking, &params.Message)
	if task == nil {
		mh.logger.Error("failed to create streaming task - task manager returned nil")
		return fmt.Errorf("failed to create streaming task")
	}

	mh.logger.Info("message stream started",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID))

	select {
	case responseChan <- types.TaskStatusUpdateEvent{
		Kind:      "status-update",
		TaskID:    task.ID,
		ContextID: task.ContextID,
		Status:    task.Status,
		Final:     false,
	}:
	case <-ctx.Done():
		return ctx.Err()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if err := mh.taskManager.UpdateTask(task.ID, types.TaskStateCompleted, nil); err != nil {
				mh.logger.Error("failed to update streaming task", zap.Error(err))
			}
		}()

		if mh.llmClient == nil {
			mh.logger.Error("no LLM client available for streaming")
			mh.handleMockStreaming(ctx, task, responseChan)
			return
		}

		if mh.agent == nil {
			mh.logger.Error("no agent available for streaming")
			mh.handleMockStreaming(ctx, task, responseChan)
			return
		}

		mh.handleIterativeStreaming(ctx, task, &params.Message, responseChan)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// handleIterativeStreaming handles the iterative streaming process with tool calling support
func (mh *DefaultMessageHandler) handleIterativeStreaming(
	ctx context.Context,
	task *types.Task,
	message *types.Message,
	responseChan chan<- types.SendStreamingMessageResponse,
) {
	messages := make([]types.Message, 0)

	systemMessage := types.Message{
		Kind:      "message",
		MessageID: "system-prompt",
		Role:      "system",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": mh.agent.GetSystemPrompt(),
			},
		},
	}
	messages = append(messages, systemMessage)

	if message != nil {
		messages = append(messages, *message)
	}

	messages = append(messages, task.History...)

	var tools []sdk.ChatCompletionTool
	if mh.toolBox != nil {
		tools = mh.toolBox.GetTools()
	}

	for iteration := 1; iteration <= mh.maxIterations; iteration++ {
		mh.logger.Debug("starting streaming iteration",
			zap.Int("iteration", iteration),
			zap.String("task_id", task.ID))

		sdkMessages, err := mh.converter.ConvertToSDK(messages)
		if err != nil {
			mh.logger.Error("failed to convert messages", zap.Error(err))
			mh.sendErrorResponse(ctx, task, fmt.Sprintf("Message conversion failed: %v", err), responseChan)
			return
		}

		streamResponseChan, streamErrorChan := mh.llmClient.CreateStreamingChatCompletion(ctx, sdkMessages, tools...)
		toolCallsExecuted, assistantMessage, toolResultMessages := mh.processStream(ctx, task, iteration, streamResponseChan, streamErrorChan, responseChan, &messages)

		if assistantMessage != nil {
			messages = append(messages, *assistantMessage)
			task.History = append(task.History, *assistantMessage)
		}

		for _, toolResultMsg := range toolResultMessages {
			messages = append(messages, toolResultMsg)
			task.History = append(task.History, toolResultMsg)
		}

		if !toolCallsExecuted {
			finalMessage := assistantMessage
			if finalMessage != nil {
				task.Status.Message = finalMessage
				mh.taskManager.UpdateConversationHistory(task.ContextID, task.History)
			}

			mh.logger.Info("streaming task completed successfully",
				zap.String("task_id", task.ID),
				zap.Int("iterations", iteration))

			select {
			case responseChan <- types.TaskStatusUpdateEvent{
				Kind:      "status-update",
				TaskID:    task.ID,
				ContextID: task.ContextID,
				Status: types.TaskStatus{
					State:     types.TaskStateCompleted,
					Message:   finalMessage,
					Timestamp: StringPtr(mh.getCurrentTimestamp()),
				},
				Final: true,
			}:
			case <-ctx.Done():
			}
			return
		}

		mh.logger.Debug("tool calls executed, continuing to next iteration",
			zap.Int("iteration", iteration),
			zap.String("task_id", task.ID))

		mh.taskManager.UpdateConversationHistory(task.ContextID, task.History)
	}

	mh.logger.Warn("max streaming iterations reached",
		zap.String("task_id", task.ID),
		zap.Int("max_iterations", mh.maxIterations))
	mh.sendErrorResponse(ctx, task, fmt.Sprintf("Maximum iterations (%d) reached without completion", mh.maxIterations), responseChan)
}

// processStream handles the streaming response and tool execution
func (mh *DefaultMessageHandler) processStream(
	ctx context.Context,
	task *types.Task,
	iteration int,
	streamResponseChan <-chan *sdk.CreateChatCompletionStreamResponse,
	streamErrorChan <-chan error,
	responseChan chan<- types.SendStreamingMessageResponse,
	messages *[]types.Message,
) (toolCallsExecuted bool, assistantMessage *types.Message, toolResultMessages []types.Message) {
	var fullContent string
	toolCallAccumulator := make(map[int]*sdk.ChatCompletionMessageToolCall)
	toolResultMessages = make([]types.Message, 0)

	for {
		select {
		case <-ctx.Done():
			return false, nil, nil
		case streamErr := <-streamErrorChan:
			if streamErr != nil {
				mh.logger.Error("streaming failed", zap.Error(streamErr))
				mh.sendErrorResponse(ctx, task, fmt.Sprintf("Streaming failed: %v", streamErr), responseChan)
				return false, nil, nil
			}
		case streamResp, ok := <-streamResponseChan:
			if !ok {
				break
			}

			if streamResp == nil || len(streamResp.Choices) == 0 {
				continue
			}

			choice := streamResp.Choices[0]

			if choice.Delta.Content != "" {
				fullContent += choice.Delta.Content
				mh.sendContentChunk(ctx, task, choice.Delta.Content, responseChan)
			}

			for _, toolCallChunk := range choice.Delta.ToolCalls {
				if toolCallAccumulator[toolCallChunk.Index] == nil {
					toolCallAccumulator[toolCallChunk.Index] = &sdk.ChatCompletionMessageToolCall{
						Type:     "function",
						Function: sdk.ChatCompletionMessageToolCallFunction{},
					}
				}

				toolCall := toolCallAccumulator[toolCallChunk.Index]
				if toolCallChunk.ID != "" {
					toolCall.Id = toolCallChunk.ID
				}
				if toolCallChunk.Function.Name != "" {
					toolCall.Function.Name = toolCallChunk.Function.Name
				}
				if toolCallChunk.Function.Arguments != "" {
					toolCall.Function.Arguments += toolCallChunk.Function.Arguments
				}
			}

			if choice.FinishReason == "" {
				continue
			}

			assistantMessage = &types.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("assistant-%s-%d", task.ID, iteration),
				Role:      "assistant",
				Parts:     make([]types.Part, 0),
				TaskID:    &task.ID,
				ContextID: &task.ContextID,
			}

			if fullContent != "" {
				assistantMessage.Parts = append(assistantMessage.Parts, map[string]interface{}{
					"kind": "text",
					"text": fullContent,
				})
			}

			if len(toolCallAccumulator) == 0 {
				return false, assistantMessage, toolResultMessages
			}

			toolCalls := make([]sdk.ChatCompletionMessageToolCall, 0, len(toolCallAccumulator))
			for _, toolCall := range toolCallAccumulator {
				toolCalls = append(toolCalls, *toolCall)
			}
			assistantMessage.Parts = append(assistantMessage.Parts, map[string]interface{}{
				"kind": "data",
				"data": map[string]interface{}{
					"tool_calls": toolCalls,
				},
			})

			for _, toolCall := range toolCalls {
				if toolCall.Function.Name == "" || toolCall.Id == "" {
					continue
				}

				mh.sendToolExecutionEvent(ctx, task, toolCall.Function.Name, "started", responseChan)

				if mh.toolBox == nil {
					mh.logger.Debug("no toolbox available for tool execution", zap.String("function", toolCall.Function.Name))
					continue
				}

				var argsMap map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap); err != nil {
					mh.logger.Error("failed to parse tool arguments", zap.Error(err), zap.String("function", toolCall.Function.Name))
					mh.sendToolExecutionEvent(ctx, task, toolCall.Function.Name, "failed", responseChan)
					continue
				}

				toolResult, err := mh.toolBox.ExecuteTool(ctx, toolCall.Function.Name, argsMap)
				if err != nil {
					mh.logger.Error("tool execution failed", zap.Error(err), zap.String("function", toolCall.Function.Name))
					mh.sendToolExecutionEvent(ctx, task, toolCall.Function.Name, "failed", responseChan)
					continue
				}

				mh.logger.Info("tool executed successfully", zap.String("function", toolCall.Function.Name))

				mh.sendToolExecutionEvent(ctx, task, toolCall.Function.Name, "completed", responseChan)

				toolResultMessage := types.Message{
					Kind:      "message",
					MessageID: fmt.Sprintf("tool-result-%s", toolCall.Id),
					Role:      "tool",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "data",
							"data": map[string]interface{}{
								"tool_call_id": toolCall.Id,
								"result":       toolResult,
							},
						},
					},
					TaskID:    &task.ID,
					ContextID: &task.ContextID,
				}

				select {
				case responseChan <- types.TaskStatusUpdateEvent{
					Kind:      "status-update",
					TaskID:    task.ID,
					ContextID: task.ContextID,
					Status: types.TaskStatus{
						State:     types.TaskStateWorking,
						Message:   &toolResultMessage,
						Timestamp: StringPtr(mh.getCurrentTimestamp()),
					},
					Final: false,
				}:
				case <-ctx.Done():
				}

				toolResultMessages = append(toolResultMessages, toolResultMessage)
			}

			return true, assistantMessage, toolResultMessages
		}
	}
}

// sendContentChunk sends a content chunk through the response channel
func (mh *DefaultMessageHandler) sendContentChunk(
	ctx context.Context,
	task *types.Task,
	content string,
	responseChan chan<- types.SendStreamingMessageResponse,
) {
	select {
	case responseChan <- types.TaskStatusUpdateEvent{
		Kind:      "status-update",
		TaskID:    task.ID,
		ContextID: task.ContextID,
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
			Message: &types.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("stream-chunk-%s", task.ID),
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": content,
					},
				},
				TaskID:    &task.ID,
				ContextID: &task.ContextID,
			},
			Timestamp: StringPtr(mh.getCurrentTimestamp()),
		},
		Final: false,
	}:
	case <-ctx.Done():
	}
}

// sendToolExecutionEvent sends tool execution status events
func (mh *DefaultMessageHandler) sendToolExecutionEvent(
	ctx context.Context,
	task *types.Task,
	toolName string,
	status string,
	responseChan chan<- types.SendStreamingMessageResponse,
) {
	select {
	case responseChan <- types.TaskStatusUpdateEvent{
		Kind:      "status-update",
		TaskID:    task.ID,
		ContextID: task.ContextID,
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
			Message: &types.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("tool-status-%s-%s", task.ID, status),
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "data",
						"data": map[string]interface{}{
							"tool_name": toolName,
							"status":    status,
						},
					},
				},
				TaskID:    &task.ID,
				ContextID: &task.ContextID,
			},
			Timestamp: StringPtr(mh.getCurrentTimestamp()),
		},
		Final: false,
	}:
	case <-ctx.Done():
	}
}

// handleMockStreaming provides fallback mock streaming for backward compatibility
func (mh *DefaultMessageHandler) handleMockStreaming(ctx context.Context, task *types.Task, responseChan chan<- types.SendStreamingMessageResponse) {
	mh.logger.Debug("using mock streaming - no LLM agent configured",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID))

	time.Sleep(100 * time.Millisecond)

	chunks := []string{
		"Starting to process your request...",
		"Analyzing the message content...",
		"Generating response...",
		"Response completed.",
	}

	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			return
		default:
			select {
			case responseChan <- types.TaskStatusUpdateEvent{
				Kind:      "status-update",
				TaskID:    task.ID,
				ContextID: task.ContextID,
				Status: types.TaskStatus{
					State: types.TaskStateWorking,
					Message: &types.Message{
						Kind:      "message",
						MessageID: fmt.Sprintf("mock-progress-%s-%d", task.ID, i+1),
						Role:      "assistant",
						Parts: []types.Part{
							map[string]interface{}{
								"kind": "text",
								"text": chunk,
							},
						},
						TaskID:    &task.ID,
						ContextID: &task.ContextID,
					},
					Timestamp: StringPtr(mh.getCurrentTimestamp()),
				},
				Final: false,
			}:
				mh.logger.Debug("mock streaming chunk sent",
					zap.String("task_id", task.ID),
					zap.Int("chunk_id", i+1),
					zap.String("content", chunk))
				time.Sleep(100 * time.Millisecond)
			case <-ctx.Done():
				return
			}
		}
	}

	select {
	case <-ctx.Done():
		return
	default:
		select {
		case responseChan <- types.TaskStatusUpdateEvent{
			Kind:      "status-update",
			TaskID:    task.ID,
			ContextID: task.ContextID,
			Status: types.TaskStatus{
				State:     types.TaskStateCompleted,
				Timestamp: StringPtr(mh.getCurrentTimestamp()),
			},
			Final: true,
		}:
		case <-ctx.Done():
			return
		}
	}
}

// sendErrorResponse sends an error response through the stream
func (mh *DefaultMessageHandler) sendErrorResponse(ctx context.Context, task *types.Task, errorMsg string, responseChan chan<- types.SendStreamingMessageResponse) {
	select {
	case responseChan <- types.TaskStatusUpdateEvent{
		Kind:      "status-update",
		TaskID:    task.ID,
		ContextID: task.ContextID,
		Status: types.TaskStatus{
			State: types.TaskStateFailed,
			Message: &types.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("error-%s", task.ID),
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": errorMsg,
					},
				},
				TaskID:    &task.ID,
				ContextID: &task.ContextID,
			},
			Timestamp: StringPtr(mh.getCurrentTimestamp()),
		},
		Final: true,
	}:
	case <-ctx.Done():
		return
	}
}

// getCurrentTimestamp returns the current timestamp in the configured timezone
func (h *DefaultMessageHandler) getCurrentTimestamp() string {
	if h.config == nil {
		return time.Now().UTC().Format(time.RFC3339)
	}

	currentTime, err := h.config.GetCurrentTime()
	if err != nil {
		h.logger.Warn("failed to get current time with configured timezone, falling back to UTC",
			zap.String("timezone", h.config.Timezone),
			zap.Error(err))
		return time.Now().UTC().Format(time.RFC3339)
	}

	return currentTime.Format(time.RFC3339)
}
