package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	uuid "github.com/google/uuid"
	adk "github.com/inference-gateway/a2a/adk"
	config "github.com/inference-gateway/a2a/adk/server/config"
	utils "github.com/inference-gateway/a2a/adk/server/utils"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// MessageHandler defines how to handle different types of A2A messages
type MessageHandler interface {
	// HandleMessageSend processes message/send requests
	HandleMessageSend(ctx context.Context, params adk.MessageSendParams) (*adk.Task, error)

	// HandleMessageStream processes message/stream requests (for streaming responses)
	HandleMessageStream(ctx context.Context, params adk.MessageSendParams, responseChan chan<- adk.SendStreamingMessageResponse) error
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
func (mh *DefaultMessageHandler) HandleMessageSend(ctx context.Context, params adk.MessageSendParams) (*adk.Task, error) {
	if len(params.Message.Parts) == 0 {
		return nil, NewEmptyMessagePartsError()
	}

	contextID := params.Message.ContextID
	if contextID == nil {
		newContextID := uuid.New().String()
		contextID = &newContextID
	}

	task := mh.taskManager.CreateTask(*contextID, adk.TaskStateSubmitted, &params.Message)

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
func (mh *DefaultMessageHandler) HandleMessageStream(ctx context.Context, params adk.MessageSendParams, responseChan chan<- adk.SendStreamingMessageResponse) error {
	if len(params.Message.Parts) == 0 {
		return NewEmptyMessagePartsError()
	}

	contextID := params.Message.ContextID
	if contextID == nil {
		newContextID := uuid.New().String()
		contextID = &newContextID
	}

	task := mh.taskManager.CreateTask(*contextID, adk.TaskStateWorking, &params.Message)
	if task == nil {
		mh.logger.Error("failed to create streaming task - task manager returned nil")
		return fmt.Errorf("failed to create streaming task")
	}

	mh.logger.Info("message stream started",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID))

	select {
	case responseChan <- adk.TaskStatusUpdateEvent{
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
			if err := mh.taskManager.UpdateTask(task.ID, adk.TaskStateCompleted, nil); err != nil {
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

		mh.handleLLMStreaming(ctx, task, &params.Message, responseChan)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// handleLLMStreaming processes the message using actual LLM streaming with iterative approach and tool calling
func (mh *DefaultMessageHandler) handleLLMStreaming(ctx context.Context, task *adk.Task, message *adk.Message, responseChan chan<- adk.SendStreamingMessageResponse) {
	messages := make([]adk.Message, 0)

	var systemPrompt string
	if mh.agent != nil {
		systemPrompt = mh.agent.GetSystemPrompt()
	}
	if systemPrompt == "" && mh.config != nil {
		systemPrompt = mh.config.AgentConfig.SystemPrompt
	}

	if systemPrompt != "" {
		systemMessage := adk.Message{
			Kind:      "message",
			MessageID: "system-prompt",
			Role:      "system",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": systemPrompt,
				},
			},
		}
		messages = append(messages, systemMessage)
	}

	messages = append(messages, task.History...)

	var tools []sdk.ChatCompletionTool
	if mh.toolBox != nil {
		tools = mh.toolBox.GetTools()
	}

	mh.processIterativeStreaming(ctx, task, messages, mh.llmClient, mh.converter, tools, mh.maxIterations, responseChan)
}

// processIterativeStreaming handles the iterative streaming process with tool calling support
func (mh *DefaultMessageHandler) processIterativeStreaming(ctx context.Context, task *adk.Task,
	messages []adk.Message, llmClient LLMClient, converter *utils.OptimizedMessageConverter,
	tools []sdk.ChatCompletionTool, maxIterations int, responseChan chan<- adk.SendStreamingMessageResponse) {

	currentMessages := messages
	iteration := 0
	chunkID := 1

	for iteration < maxIterations {
		iteration++
		mh.logger.Debug("starting streaming iteration",
			zap.Int("iteration", iteration),
			zap.Int("max_iterations", maxIterations),
			zap.String("task_id", task.ID))

		timestamp := mh.getCurrentTimestamp()
		statusUpdate := adk.TaskStatusUpdateEvent{
			Kind:      "status-update",
			TaskID:    task.ID,
			ContextID: task.ContextID,
			Status: adk.TaskStatus{
				State: adk.TaskStateWorking,
				Message: &adk.Message{
					Kind:      "message",
					MessageID: fmt.Sprintf("status-%s-%d", task.ID, iteration),
					Role:      "agent",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": fmt.Sprintf("Starting iteration %d of %d", iteration, maxIterations),
						},
					},
					TaskID:    &task.ID,
					ContextID: &task.ContextID,
				},
				Timestamp: &timestamp,
			},
			Final: false,
		}

		select {
		case responseChan <- statusUpdate:
		case <-ctx.Done():
			return
		}

		sdkMessages, err := converter.ConvertToSDK(currentMessages)
		if err != nil {
			mh.logger.Error("failed to convert messages for streaming", zap.Error(err))
			mh.sendErrorResponse(ctx, task, fmt.Sprintf("Message conversion failed: %v", err), responseChan)
			return
		}

		var streamResponseChan <-chan *sdk.CreateChatCompletionStreamResponse
		var streamErrorChan <-chan error

		streamResponseChan, streamErrorChan = llmClient.CreateStreamingChatCompletion(ctx, sdkMessages, tools...)

		fullContent, toolCalls, finished, err := mh.processStreamIteration(ctx, task, streamResponseChan, streamErrorChan, iteration, &chunkID, responseChan)
		if err != nil {
			mh.logger.Error("streaming iteration failed", zap.Error(err), zap.Int("iteration", iteration))
			mh.sendErrorResponse(ctx, task, fmt.Sprintf("Streaming iteration %d failed: %v", iteration, err), responseChan)
			return
		}

		if !finished {
			return
		}

		if len(toolCalls) > 0 {
			mh.logger.Info("processing tool calls in streaming",
				zap.Int("count", len(toolCalls)),
				zap.Int("iteration", iteration))

			select {
			case responseChan <- adk.TaskStatusUpdateEvent{
				Kind:      "status-update",
				TaskID:    task.ID,
				ContextID: task.ContextID,
				Status: adk.TaskStatus{
					State: adk.TaskStateWorking,
					Message: &adk.Message{
						Kind:      "message",
						MessageID: fmt.Sprintf("tool-calls-%s-%d", task.ID, iteration),
						Role:      "agent",
						Parts: []adk.Part{
							map[string]interface{}{
								"kind": "data",
								"data": map[string]interface{}{
									"tool_calls": toolCalls,
									"count":      len(toolCalls),
								},
							},
						},
						TaskID:    &task.ID,
						ContextID: &task.ContextID,
					},
				},
				Final: false,
			}:
			case <-ctx.Done():
				return
			}

			assistantMessage := &adk.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("assistant-%s-%d", task.ID, iteration),
				Role:      "assistant",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "data",
						"data": map[string]interface{}{
							"tool_calls": toolCalls,
							"content":    fullContent,
						},
					},
				},
			}
			task.History = append(task.History, *assistantMessage)
			currentMessages = append(currentMessages, *assistantMessage)

			mh.taskManager.UpdateConversationHistory(task.ContextID, task.History)

			toolResults, err := mh.executeToolsForStreaming(ctx, task, toolCalls, &chunkID, responseChan)
			if err != nil {
				mh.logger.Error("tool execution failed in streaming", zap.Error(err))
				mh.sendErrorResponse(ctx, task, fmt.Sprintf("Tool execution failed: %v", err), responseChan)
				return
			}

			task.History = append(task.History, toolResults...)
			mh.taskManager.UpdateConversationHistory(task.ContextID, task.History)

			currentMessages = append(currentMessages, toolResults...)
			continue
		}

		finalMessage := &adk.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("response-%s-%d", task.ID, iteration),
			Role:      "assistant",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": fullContent,
				},
			},
		}

		task.History = append(task.History, *finalMessage)
		task.Status.Message = finalMessage

		mh.taskManager.UpdateConversationHistory(task.ContextID, task.History)

		mh.logger.Info("streaming task completed successfully",
			zap.String("task_id", task.ID),
			zap.Int("iterations", iteration))

		select {
		case responseChan <- adk.TaskStatusUpdateEvent{
			Kind:      "status-update",
			TaskID:    task.ID,
			ContextID: task.ContextID,
			Status: adk.TaskStatus{
				State:   adk.TaskStateCompleted,
				Message: finalMessage,
			},
			Final: true,
		}:
		case <-ctx.Done():
		}
		return
	}

	mh.logger.Warn("max streaming iterations reached",
		zap.String("task_id", task.ID),
		zap.Int("max_iterations", maxIterations))
	mh.sendErrorResponse(ctx, task, fmt.Sprintf("Maximum iterations (%d) reached without completion", maxIterations), responseChan)
}

// handleMockStreaming provides fallback mock streaming for backward compatibility
func (mh *DefaultMessageHandler) handleMockStreaming(ctx context.Context, task *adk.Task, responseChan chan<- adk.SendStreamingMessageResponse) {
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
			case responseChan <- adk.TaskStatusUpdateEvent{
				Kind:      "status-update",
				TaskID:    task.ID,
				ContextID: task.ContextID,
				Status: adk.TaskStatus{
					State: adk.TaskStateWorking,
					Message: &adk.Message{
						Kind:      "message",
						MessageID: fmt.Sprintf("mock-progress-%s-%d", task.ID, i+1),
						Role:      "agent",
						Parts: []adk.Part{
							map[string]interface{}{
								"kind": "text",
								"text": chunk,
							},
						},
						TaskID:    &task.ID,
						ContextID: &task.ContextID,
					},
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
		case responseChan <- adk.TaskStatusUpdateEvent{
			Kind:      "status-update",
			TaskID:    task.ID,
			ContextID: task.ContextID,
			Status: adk.TaskStatus{
				State: adk.TaskStateCompleted,
			},
			Final: true,
		}:
		case <-ctx.Done():
			return
		}
	}
}

// sendErrorResponse sends an error response through the stream
func (mh *DefaultMessageHandler) sendErrorResponse(ctx context.Context, task *adk.Task, errorMsg string, responseChan chan<- adk.SendStreamingMessageResponse) {
	select {
	case responseChan <- adk.TaskStatusUpdateEvent{
		Kind:      "status-update",
		TaskID:    task.ID,
		ContextID: task.ContextID,
		Status: adk.TaskStatus{
			State: adk.TaskStateFailed,
			Message: &adk.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("error-%s", task.ID),
				Role:      "assistant",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": errorMsg,
					},
				},
				TaskID:    &task.ID,
				ContextID: &task.ContextID,
			},
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

// processStreamIteration processes a single streaming iteration and returns content, tool calls, and completion status
func (mh *DefaultMessageHandler) processStreamIteration(
	ctx context.Context,
	task *adk.Task,
	streamResponseChan <-chan *sdk.CreateChatCompletionStreamResponse,
	streamErrorChan <-chan error,
	iteration int,
	chunkID *int,
	responseChan chan<- adk.SendStreamingMessageResponse,
) (string, []sdk.ChatCompletionMessageToolCall, bool, error) {

	var fullContent string
	var toolCalls []sdk.ChatCompletionMessageToolCall

	for {
		select {
		case <-ctx.Done():
			return "", nil, false, ctx.Err()
		case streamErr := <-streamErrorChan:
			if streamErr != nil {
				return "", nil, false, streamErr
			}
		case streamResp, ok := <-streamResponseChan:
			if !ok {
				return fullContent, toolCalls, true, nil
			}

			if streamResp != nil && len(streamResp.Choices) > 0 {
				choice := streamResp.Choices[0]

				content := choice.Delta.Content
				if content != "" {
					fullContent += content

					select {
					case responseChan <- adk.TaskStatusUpdateEvent{
						Kind:      "status-update",
						TaskID:    task.ID,
						ContextID: task.ContextID,
						Status: adk.TaskStatus{
							State: adk.TaskStateWorking,
							Message: &adk.Message{
								Kind:      "message",
								MessageID: fmt.Sprintf("content-chunk-%s-%d", task.ID, *chunkID),
								Role:      "assistant",
								Parts: []adk.Part{
									map[string]interface{}{
										"kind": "text",
										"text": content,
									},
								},
								TaskID:    &task.ID,
								ContextID: &task.ContextID,
							},
						},
						Final: false,
					}:
						*chunkID++
					case <-ctx.Done():
						return "", nil, false, ctx.Err()
					}
				}

				if len(choice.Delta.ToolCalls) > 0 {
					for _, toolCallChunk := range choice.Delta.ToolCalls {
						if toolCallChunk.Function.Name != "" && toolCallChunk.Function.Arguments != "" {
							toolCall := sdk.ChatCompletionMessageToolCall{
								Id:   toolCallChunk.ID,
								Type: "function",
								Function: sdk.ChatCompletionMessageToolCallFunction{
									Name:      toolCallChunk.Function.Name,
									Arguments: toolCallChunk.Function.Arguments,
								},
							}
							toolCalls = append(toolCalls, toolCall)
						}
					}
				}

				if choice.FinishReason != "" {
					select {
					case responseChan <- adk.TaskStatusUpdateEvent{
						Kind:      "status-update",
						TaskID:    task.ID,
						ContextID: task.ContextID,
						Status: adk.TaskStatus{
							State: adk.TaskStateWorking,
							Message: &adk.Message{
								Kind:      "message",
								MessageID: fmt.Sprintf("final-content-%s-%d", task.ID, *chunkID),
								Role:      "assistant",
								Parts: []adk.Part{
									map[string]interface{}{
										"kind": "text",
										"text": fullContent,
									},
								},
								TaskID:    &task.ID,
								ContextID: &task.ContextID,
							},
						},
						Final: false,
					}:
						*chunkID++
					case <-ctx.Done():
						return "", nil, false, ctx.Err()
					}

					return fullContent, toolCalls, true, nil
				}
			}
		}
	}
}

// executeToolsForStreaming executes tools and sends streaming updates
func (mh *DefaultMessageHandler) executeToolsForStreaming(ctx context.Context, task *adk.Task,
	toolCalls []sdk.ChatCompletionMessageToolCall, chunkID *int, responseChan chan<- adk.SendStreamingMessageResponse) ([]adk.Message, error) {

	if mh.toolBox == nil {
		return nil, fmt.Errorf("no tool box available for tool execution")
	}

	toolResults := make([]adk.Message, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		if toolCall.Type != "function" {
			continue
		}

		function := toolCall.Function
		if function.Name == "" {
			continue
		}

		select {
		case responseChan <- adk.TaskStatusUpdateEvent{
			Kind:      "status-update",
			TaskID:    task.ID,
			ContextID: task.ContextID,
			Status: adk.TaskStatus{
				State: adk.TaskStateWorking,
				Message: &adk.Message{
					Kind:      "message",
					MessageID: fmt.Sprintf("tool-start-%s-%d", task.ID, *chunkID),
					Role:      "agent",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "data",
							"data": map[string]interface{}{
								"tool_name": function.Name,
								"tool_id":   toolCall.Id,
								"status":    "started",
							},
						},
					},
					TaskID:    &task.ID,
					ContextID: &task.ContextID,
				},
			},
			Final: false,
		}:
			*chunkID++
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		var args map[string]interface{}
		if function.Arguments != "" {
			if err := json.Unmarshal([]byte(function.Arguments), &args); err != nil {
				mh.logger.Error("failed to parse tool arguments",
					zap.String("tool", function.Name),
					zap.Error(err))
				continue
			}
		}

		result, err := mh.toolBox.ExecuteTool(ctx, function.Name, args)
		if err != nil {
			result = fmt.Sprintf("Error executing tool: %v", err)
			mh.logger.Error("tool execution failed",
				zap.String("tool", function.Name),
				zap.Error(err))
		} else {
			mh.logger.Info("tool executed successfully",
				zap.String("tool", function.Name))
		}

		select {
		case responseChan <- adk.TaskStatusUpdateEvent{
			Kind:      "status-update",
			TaskID:    task.ID,
			ContextID: task.ContextID,
			Status: adk.TaskStatus{
				State: adk.TaskStateWorking,
				Message: &adk.Message{
					Kind:      "message",
					MessageID: fmt.Sprintf("tool-completed-%s-%d", task.ID, *chunkID),
					Role:      "agent",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "data",
							"data": map[string]interface{}{
								"tool_name": function.Name,
								"tool_id":   toolCall.Id,
								"result":    result,
								"status":    "completed",
							},
						},
					},
					TaskID:    &task.ID,
					ContextID: &task.ContextID,
				},
			},
			Final: false,
		}:
			*chunkID++
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		toolResultMessage := adk.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("tool-result-%s", toolCall.Id),
			Role:      "tool",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "data",
					"data": map[string]interface{}{
						"tool_call_id": toolCall.Id,
						"tool_name":    function.Name,
						"result":       result,
					},
				},
			},
		}

		toolResults = append(toolResults, toolResultMessage)
	}

	return toolResults, nil
}
