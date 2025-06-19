package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	uuid "github.com/google/uuid"
	adk "github.com/inference-gateway/a2a/adk"
	utils "github.com/inference-gateway/a2a/adk/server/utils"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// StreamResponse represents a streaming response chunk
type StreamResponse struct {
	Kind    string      `json:"kind"`
	TaskID  string      `json:"task_id,omitempty"`
	ChunkID int         `json:"chunk_id,omitempty"`
	Content string      `json:"content,omitempty"`
	Partial bool        `json:"partial,omitempty"`
	Status  string      `json:"status,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// MessageHandler defines how to handle different types of A2A messages
type MessageHandler interface {
	// HandleMessageSend processes message/send requests
	HandleMessageSend(ctx context.Context, params adk.MessageSendParams) (*adk.Task, error)

	// HandleMessageStream processes message/stream requests (for streaming responses)
	HandleMessageStream(ctx context.Context, params adk.MessageSendParams, responseChan chan<- StreamResponse) error
}

// DefaultMessageHandler implements the MessageHandler interface
type DefaultMessageHandler struct {
	logger      *zap.Logger
	taskManager TaskManager
	agent       OpenAICompatibleAgent
}

// NewDefaultMessageHandler creates a new default message handler
func NewDefaultMessageHandler(logger *zap.Logger, taskManager TaskManager) *DefaultMessageHandler {
	return &DefaultMessageHandler{
		logger:      logger,
		taskManager: taskManager,
		agent:       nil,
	}
}

// NewDefaultMessageHandlerWithAgent creates a new default message handler with an agent for streaming
func NewDefaultMessageHandlerWithAgent(logger *zap.Logger, taskManager TaskManager, agent OpenAICompatibleAgent) *DefaultMessageHandler {
	return &DefaultMessageHandler{
		logger:      logger,
		taskManager: taskManager,
		agent:       agent,
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
func (mh *DefaultMessageHandler) HandleMessageStream(ctx context.Context, params adk.MessageSendParams, responseChan chan<- StreamResponse) error {
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
	case responseChan <- StreamResponse{
		Kind:   "task_started",
		TaskID: task.ID,
		Data:   task,
	}:
	case <-ctx.Done():
		return ctx.Err()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if err := mh.taskManager.UpdateTask(task.ID, adk.TaskStateCompleted, &params.Message); err != nil {
				mh.logger.Error("failed to update streaming task", zap.Error(err))
			}
		}()

		if mh.agent != nil {
			mh.handleLLMStreaming(ctx, task, &params.Message, responseChan)
		} else {
			mh.handleMockStreaming(ctx, task, responseChan)
		}
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// handleLLMStreaming processes the message using actual LLM streaming with iterative approach and tool calling
func (mh *DefaultMessageHandler) handleLLMStreaming(ctx context.Context, task *adk.Task, message *adk.Message, responseChan chan<- StreamResponse) {
	agent := mh.getAgentFromHandler()
	if agent == nil {
		mh.logger.Error("no agent available for streaming")
		mh.handleMockStreaming(ctx, task, responseChan)
		return
	}

	messages := mh.prepareMessages(agent, task, message)
	llmClient := mh.getLLMClientFromAgent()
	if llmClient == nil {
		mh.logger.Error("no LLM client available for streaming")
		mh.handleMockStreaming(ctx, task, responseChan)
		return
	}

	converter := mh.getMessageConverter()
	tools := mh.getToolsFromAgent(agent)
	maxIterations := mh.getMaxIterationsFromAgent(agent)

	mh.processIterativeStreaming(ctx, task, messages, llmClient, converter, tools, maxIterations, responseChan)
}

// processIterativeStreaming handles the iterative streaming process with tool calling support
func (mh *DefaultMessageHandler) processIterativeStreaming(ctx context.Context, task *adk.Task,
	messages []adk.Message, llmClient LLMClient, converter *utils.OptimizedMessageConverter,
	tools []sdk.ChatCompletionTool, maxIterations int, responseChan chan<- StreamResponse) {

	currentMessages := messages
	iteration := 0
	chunkID := 1

	for iteration < maxIterations {
		iteration++
		mh.logger.Debug("starting streaming iteration",
			zap.Int("iteration", iteration),
			zap.Int("max_iterations", maxIterations),
			zap.String("task_id", task.ID))

		select {
		case responseChan <- StreamResponse{
			Kind:    "iteration_started",
			TaskID:  task.ID,
			ChunkID: chunkID,
			Data: map[string]interface{}{
				"iteration":      iteration,
				"max_iterations": maxIterations,
			},
		}:
			chunkID++
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

		streamResponseChan, streamErrorChan = llmClient.CreateStreamingChatCompletion(ctx, sdkMessages)

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
			case responseChan <- StreamResponse{
				Kind:    "tool_calls_started",
				TaskID:  task.ID,
				ChunkID: chunkID,
				Data: map[string]interface{}{
					"tool_calls": toolCalls,
					"count":      len(toolCalls),
				},
			}:
				chunkID++
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

			toolResults, err := mh.executeToolsForStreaming(ctx, task, toolCalls, &chunkID, responseChan)
			if err != nil {
				mh.logger.Error("tool execution failed in streaming", zap.Error(err))
				mh.sendErrorResponse(ctx, task, fmt.Sprintf("Tool execution failed: %v", err), responseChan)
				return
			}

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

		mh.logger.Info("streaming task completed successfully",
			zap.String("task_id", task.ID),
			zap.Int("iterations", iteration))

		select {
		case responseChan <- StreamResponse{
			Kind:   "task_completed",
			TaskID: task.ID,
			Status: "completed",
			Data: map[string]interface{}{
				"iterations": iteration,
				"message":    finalMessage,
			},
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
func (mh *DefaultMessageHandler) handleMockStreaming(ctx context.Context, task *adk.Task, responseChan chan<- StreamResponse) {
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
			case responseChan <- StreamResponse{
				Kind:    "message_chunk",
				TaskID:  task.ID,
				ChunkID: i + 1,
				Content: chunk,
				Partial: i < len(chunks)-1,
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
		case responseChan <- StreamResponse{
			Kind:   "task_completed",
			TaskID: task.ID,
			Status: "completed",
		}:
		case <-ctx.Done():
			return
		}
	}
}

// sendErrorResponse sends an error response through the stream
func (mh *DefaultMessageHandler) sendErrorResponse(ctx context.Context, task *adk.Task, errorMsg string, responseChan chan<- StreamResponse) {
	select {
	case responseChan <- StreamResponse{
		Kind:    "message_chunk",
		TaskID:  task.ID,
		ChunkID: 1,
		Content: errorMsg,
		Partial: false,
	}:
	case <-ctx.Done():
		return
	}

	select {
	case responseChan <- StreamResponse{
		Kind:   "task_completed",
		TaskID: task.ID,
		Status: "failed",
	}:
	case <-ctx.Done():
		return
	}
}

// getLLMClientFromAgent extracts LLM client from agent (helper method)
func (mh *DefaultMessageHandler) getLLMClientFromAgent() LLMClient {
	if mh.agent == nil {
		return nil
	}

	// Use the getter method to access the LLM client
	if defaultAgent, ok := mh.agent.(*DefaultOpenAICompatibleAgent); ok {
		return defaultAgent.GetLLMClient()
	}

	return nil
}

// getMessageConverter gets the message converter for A2A to SDK conversion
func (mh *DefaultMessageHandler) getMessageConverter() *utils.OptimizedMessageConverter {
	return utils.NewOptimizedMessageConverter(mh.logger)
}

// getAgentFromHandler extracts agent from message handler
func (mh *DefaultMessageHandler) getAgentFromHandler() OpenAICompatibleAgent {
	return mh.agent
}

// prepareMessages prepares the message chain for LLM processing
func (mh *DefaultMessageHandler) prepareMessages(agent OpenAICompatibleAgent, task *adk.Task, message *adk.Message) []adk.Message {
	messages := make([]adk.Message, 0)

	if agent != nil {
		if defaultAgent, ok := agent.(*DefaultOpenAICompatibleAgent); ok && defaultAgent.config.SystemPrompt != "" {
			systemMessage := adk.Message{
				Kind:      "message",
				MessageID: "system-prompt",
				Role:      "system",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": defaultAgent.config.SystemPrompt,
					},
				},
			}
			messages = append(messages, systemMessage)
		}
	}

	if len(messages) == 0 {
		systemMessage := adk.Message{
			Kind:      "message",
			MessageID: "system-prompt",
			Role:      "system",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "You are a helpful AI assistant. Provide clear and concise responses.",
				},
			},
		}
		messages = append(messages, systemMessage)
	}

	messages = append(messages, task.History...)
	messages = append(messages, *message)

	return messages
}

// getToolsFromAgent extracts tools from agent
func (mh *DefaultMessageHandler) getToolsFromAgent(agent OpenAICompatibleAgent) []sdk.ChatCompletionTool {
	if agent == nil {
		return nil
	}

	if defaultAgent, ok := agent.(*DefaultOpenAICompatibleAgent); ok && defaultAgent.toolBox != nil {
		return defaultAgent.toolBox.GetTools()
	}

	return nil
}

// getMaxIterationsFromAgent extracts max iterations from agent config
func (mh *DefaultMessageHandler) getMaxIterationsFromAgent(agent OpenAICompatibleAgent) int {
	if agent == nil {
		return 10
	}

	if defaultAgent, ok := agent.(*DefaultOpenAICompatibleAgent); ok {
		return defaultAgent.config.MaxChatCompletionIterations
	}

	return 10
}

// processStreamIteration processes a single streaming iteration and returns content, tool calls, and completion status
func (mh *DefaultMessageHandler) processStreamIteration(
	ctx context.Context,
	task *adk.Task,
	streamResponseChan <-chan *sdk.CreateChatCompletionStreamResponse,
	streamErrorChan <-chan error,
	iteration int,
	chunkID *int,
	responseChan chan<- StreamResponse,
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
					case responseChan <- StreamResponse{
						Kind:    "message_chunk",
						TaskID:  task.ID,
						ChunkID: *chunkID,
						Content: content,
						Partial: true,
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
					case responseChan <- StreamResponse{
						Kind:    "message_chunk",
						TaskID:  task.ID,
						ChunkID: *chunkID,
						Content: fullContent,
						Partial: false,
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
	toolCalls []sdk.ChatCompletionMessageToolCall, chunkID *int, responseChan chan<- StreamResponse) ([]adk.Message, error) {

	agent := mh.getAgentFromHandler()
	if agent == nil {
		return nil, fmt.Errorf("no agent available for tool execution")
	}

	defaultAgent, ok := agent.(*DefaultOpenAICompatibleAgent)
	if !ok || defaultAgent.toolBox == nil {
		return nil, fmt.Errorf("agent does not support tool execution")
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
		case responseChan <- StreamResponse{
			Kind:    "tool_execution_started",
			TaskID:  task.ID,
			ChunkID: *chunkID,
			Data: map[string]interface{}{
				"tool_name": function.Name,
				"tool_id":   toolCall.Id,
			},
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

		result, err := defaultAgent.toolBox.ExecuteTool(ctx, function.Name, args)
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
		case responseChan <- StreamResponse{
			Kind:    "tool_execution_completed",
			TaskID:  task.ID,
			ChunkID: *chunkID,
			Data: map[string]interface{}{
				"tool_name": function.Name,
				"tool_id":   toolCall.Id,
				"result":    result,
			},
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
		task.History = append(task.History, toolResultMessage)
	}

	return toolResults, nil
}
