package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	config "github.com/inference-gateway/adk/server/config"
	utils "github.com/inference-gateway/adk/server/utils"
	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// AgentResponse contains the response and any additional messages generated during agent execution
type AgentResponse struct {
	// Response is the main assistant response message
	Response *types.Message
	// AdditionalMessages contains any tool calls, tool responses, or intermediate messages
	// that should be added to the conversation history
	AdditionalMessages []types.Message
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . OpenAICompatibleAgent

// OpenAICompatibleAgent represents an agent that can interact with OpenAI-compatible LLM APIs and execute tools
// The agent is stateless and does not maintain conversation history
// Tools are configured during agent creation via the toolbox
type OpenAICompatibleAgent interface {
	// Run processes a conversation and returns the assistant's response along with any additional messages
	// Uses the agent's configured toolbox for tool execution
	Run(ctx context.Context, messages []types.Message) (*AgentResponse, error)

	// RunWithStream processes a conversation and returns a streaming response
	// Uses the agent's configured toolbox for tool execution
	RunWithStream(ctx context.Context, messages []types.Message) (<-chan *types.Message, error)
}

// OpenAICompatibleAgentImpl is the implementation of OpenAICompatibleAgent
// This implementation is stateless and does not maintain conversation history
type OpenAICompatibleAgentImpl struct {
	logger    *zap.Logger
	llmClient LLMClient
	toolBox   ToolBox
	converter utils.MessageConverter
	config    *config.AgentConfig
}

// NewOpenAICompatibleAgent creates a new OpenAICompatibleAgentImpl
func NewOpenAICompatibleAgent(logger *zap.Logger) *OpenAICompatibleAgentImpl {
	defaultConfig := &config.AgentConfig{
		MaxChatCompletionIterations: 10,
		SystemPrompt:                "You are a helpful AI assistant.",
	}
	return &OpenAICompatibleAgentImpl{
		logger:    logger,
		converter: utils.NewOptimizedMessageConverter(logger),
		config:    defaultConfig,
	}
}

// NewOpenAICompatibleAgentWithConfig creates a new OpenAICompatibleAgentImpl with configuration
func NewOpenAICompatibleAgentWithConfig(logger *zap.Logger, cfg *config.AgentConfig) *OpenAICompatibleAgentImpl {
	return &OpenAICompatibleAgentImpl{
		logger:    logger,
		converter: utils.NewOptimizedMessageConverter(logger),
		config:    cfg,
	}
}

// NewOpenAICompatibleAgentWithLLM creates a new agent with an LLM client
func NewOpenAICompatibleAgentWithLLM(logger *zap.Logger, llmClient LLMClient) *OpenAICompatibleAgentImpl {
	agent := NewOpenAICompatibleAgent(logger)
	agent.llmClient = llmClient
	return agent
}

// NewOpenAICompatibleAgentWithLLMConfig creates a new agent with LLM configuration
func NewOpenAICompatibleAgentWithLLMConfig(logger *zap.Logger, config *config.AgentConfig) (*OpenAICompatibleAgentImpl, error) {
	client, err := NewOpenAICompatibleLLMClient(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create llm client: %w", err)
	}

	agent := NewOpenAICompatibleAgentWithConfig(logger, config)
	agent.llmClient = client
	return agent, nil
}

// SetLLMClient sets the LLM client for the agent
func (a *OpenAICompatibleAgentImpl) SetLLMClient(client LLMClient) {
	a.llmClient = client
}

// SetToolBox sets the tool box for the agent
func (a *OpenAICompatibleAgentImpl) SetToolBox(toolBox ToolBox) {
	a.toolBox = toolBox
}

// Run processes a conversation and returns the assistant's response along with additional messages
func (a *OpenAICompatibleAgentImpl) Run(ctx context.Context, messages []types.Message) (*AgentResponse, error) {
	if a.llmClient == nil {
		return nil, fmt.Errorf("no LLM client configured for agent")
	}

	conversation, err := a.converter.ConvertToSDK(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages to SDK format: %w", err)
	}

	if a.config != nil && a.config.SystemPrompt != "" {
		systemMessage := sdk.Message{
			Role:    sdk.System,
			Content: a.config.SystemPrompt,
		}
		conversation = append([]sdk.Message{systemMessage}, conversation...)
	}

	maxIterations := 10
	if a.config != nil && a.config.MaxChatCompletionIterations > 0 {
		maxIterations = a.config.MaxChatCompletionIterations
	}

	var tools []sdk.ChatCompletionTool
	if a.toolBox != nil {
		tools = a.toolBox.GetTools()
	}

	var additionalMessages []types.Message

	for iteration := 0; iteration < maxIterations; iteration++ {
		response, err := a.llmClient.CreateChatCompletion(ctx, conversation, tools...)
		if err != nil {
			return nil, fmt.Errorf("failed to create chat completion: %w", err)
		}

		if len(response.Choices) == 0 {
			return nil, fmt.Errorf("no choices returned from LLM")
		}

		assistantMessage := response.Choices[0].Message

		conversation = append(conversation, assistantMessage)

		assistantA2A, err := a.converter.ConvertFromSDK(assistantMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to convert assistant message to A2A format: %w", err)
		}

		if assistantMessage.ToolCalls == nil || len(*assistantMessage.ToolCalls) == 0 || a.toolBox == nil {
			return &AgentResponse{
				Response:           assistantA2A,
				AdditionalMessages: additionalMessages,
			}, nil
		}

		additionalMessages = append(additionalMessages, *assistantA2A)

		for _, toolCall := range *assistantMessage.ToolCalls {
			var args map[string]interface{}
			var result string
			var toolErr error

			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				a.logger.Error("failed to parse tool arguments", zap.String("tool", toolCall.Function.Name), zap.Error(err))
				return &AgentResponse{
					Response: &types.Message{
						Kind:      "message",
						MessageID: fmt.Sprintf("tool-error-%s", toolCall.Id),
						Role:      "tool",
						Parts: []types.Part{
							map[string]interface{}{
								"kind": "text",
								"text": fmt.Sprintf("Error parsing tool arguments: %s", err.Error()),
							},
						},
					},
				}, err
			}

			if toolCall.Function.Name == "input_required" {
				a.logger.Debug("input_required tool called",
					zap.String("tool_call_id", toolCall.Id),
					zap.String("message", toolCall.Function.Arguments))
				inputMessage := args["message"].(string)
				return &AgentResponse{
					Response: &types.Message{
						Kind:      "input_required",
						MessageID: fmt.Sprintf("input-required-%s", toolCall.Id),
						Role:      "assistant",
						Parts: []types.Part{
							map[string]interface{}{
								"kind": "text",
								"text": inputMessage,
							},
						},
					},
				}, nil

			}

			result, toolErr = a.toolBox.ExecuteTool(ctx, toolCall.Function.Name, args)
			if toolErr != nil {
				a.logger.Error("failed to execute tool", zap.String("tool", toolCall.Function.Name), zap.Error(toolErr))
				result = fmt.Sprintf("Tool execution failed: %s", toolErr.Error())
			}

			toolMessage := sdk.Message{
				Role:       sdk.Tool,
				Content:    result,
				ToolCallId: &toolCall.Id,
			}
			conversation = append(conversation, toolMessage)

			toolA2A := &types.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("tool-%s-%d", toolCall.Function.Name, time.Now().UnixNano()),
				Role:      "tool",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "data",
						"data": map[string]interface{}{
							"tool_call_id": toolCall.Id,
							"tool_name":    toolCall.Function.Name,
							"result":       result,
							"error":        toolErr != nil,
						},
					},
				},
			}
			additionalMessages = append(additionalMessages, *toolA2A)
		}
	}

	return nil, fmt.Errorf("maximum iterations (%d) reached without final response", maxIterations)
}

// RunWithStream processes a conversation and returns a streaming response with iterative tool calling support
func (a *OpenAICompatibleAgentImpl) RunWithStream(ctx context.Context, messages []types.Message) (<-chan *types.Message, error) {
	if a.llmClient == nil {
		return nil, fmt.Errorf("no LLM client configured for agent")
	}

	var tools []sdk.ChatCompletionTool
	if a.toolBox != nil {
		tools = a.toolBox.GetTools()
	}

	outputChan := make(chan *types.Message, 100)

	go func() {
		defer close(outputChan)

		currentMessages := make([]types.Message, len(messages))
		copy(currentMessages, messages)

		for iteration := 1; iteration <= a.config.MaxChatCompletionIterations; iteration++ {
			a.logger.Debug("starting streaming iteration",
				zap.Int("iteration", iteration),
				zap.Int("message_count", len(currentMessages)))

			sdkMessages, err := a.converter.ConvertToSDK(currentMessages)
			if err != nil {
				a.logger.Error("failed to convert messages to SDK format", zap.Error(err))
				return
			}

			if a.config != nil && a.config.SystemPrompt != "" {
				systemMessage := sdk.Message{
					Role:    sdk.System,
					Content: a.config.SystemPrompt,
				}
				sdkMessages = append([]sdk.Message{systemMessage}, sdkMessages...)
			}

			streamResponseChan, streamErrorChan := a.llmClient.CreateStreamingChatCompletion(ctx, sdkMessages, tools...)

			toolCallsExecuted, assistantMessage, toolResultMessages := a.processStreamIteration(ctx, iteration, streamResponseChan, streamErrorChan, outputChan)

			if assistantMessage != nil {
				currentMessages = append(currentMessages, *assistantMessage)
			}

			currentMessages = append(currentMessages, toolResultMessages...)

			if !toolCallsExecuted {
				a.logger.Debug("streaming completed - no tool calls executed",
					zap.Int("iteration", iteration),
					zap.Int("final_message_count", len(currentMessages)))
				return
			}

			a.logger.Debug("tool calls executed, continuing to next iteration",
				zap.Int("iteration", iteration),
				zap.Int("message_count", len(currentMessages)))
		}

		a.logger.Warn("max streaming iterations reached", zap.Int("max_iterations", a.config.MaxChatCompletionIterations))
	}()

	return outputChan, nil
}

// processStreamIteration processes a single streaming iteration and returns whether tool calls were executed
func (a *OpenAICompatibleAgentImpl) processStreamIteration(
	ctx context.Context,
	iteration int,
	streamResponseChan <-chan *sdk.CreateChatCompletionStreamResponse,
	streamErrorChan <-chan error,
	outputChan chan<- *types.Message,
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
				a.logger.Error("streaming failed", zap.Error(streamErr))
			}
			return false, nil, nil
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

				chunkMessage := &types.Message{
					Kind:      "message",
					MessageID: fmt.Sprintf("chunk-%d-%d", iteration, len(fullContent)),
					Role:      "assistant",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": choice.Delta.Content,
						},
					},
				}

				select {
				case outputChan <- chunkMessage:
				case <-ctx.Done():
					return false, nil, nil
				}
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

			if choice.FinishReason != "" {
				assistantMessage = &types.Message{
					Kind:      "message",
					MessageID: fmt.Sprintf("assistant-stream-%d", iteration),
					Role:      "assistant",
					Parts:     make([]types.Part, 0),
				}

				if fullContent != "" {
					assistantMessage.Parts = append(assistantMessage.Parts, map[string]interface{}{
						"kind": "text",
						"text": fullContent,
					})
				}

				if len(toolCallAccumulator) > 0 && a.toolBox != nil {
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

					select {
					case outputChan <- assistantMessage:
					case <-ctx.Done():
						return false, nil, nil
					}

					toolResultMessages = a.executeToolCallsWithEvents(ctx, toolCalls, outputChan)

					return true, assistantMessage, toolResultMessages
				}

				select {
				case outputChan <- assistantMessage:
				case <-ctx.Done():
				}

				return false, assistantMessage, toolResultMessages
			}
		}
	}
}

// executeToolCallsWithEvents executes tool calls and emits events, returning tool result messages
func (a *OpenAICompatibleAgentImpl) executeToolCallsWithEvents(ctx context.Context, toolCalls []sdk.ChatCompletionMessageToolCall, outputChan chan<- *types.Message) []types.Message {
	toolResultMessages := make([]types.Message, 0)

	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "" || toolCall.Id == "" {
			continue
		}

		startEvent := &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("tool-start-%s", toolCall.Id),
			Role:      "assistant",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "data",
					"data": map[string]interface{}{
						"tool_name": toolCall.Function.Name,
						"status":    "started",
					},
				},
			},
		}

		select {
		case outputChan <- startEvent:
		case <-ctx.Done():
			return toolResultMessages
		}

		var args map[string]interface{}
		var result string
		var toolErr error

		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			a.logger.Error("failed to parse tool arguments", zap.String("tool", toolCall.Function.Name), zap.Error(err))
			result = fmt.Sprintf("Error parsing tool arguments: %s", err.Error())
			toolErr = err

			failedEvent := &types.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("tool-failed-%s", toolCall.Id),
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "data",
						"data": map[string]interface{}{
							"tool_name": toolCall.Function.Name,
							"status":    "failed",
						},
					},
				},
			}

			select {
			case outputChan <- failedEvent:
			case <-ctx.Done():
			}
		} else {
			result, toolErr = a.toolBox.ExecuteTool(ctx, toolCall.Function.Name, args)
			if toolErr != nil {
				a.logger.Error("failed to execute tool", zap.String("tool", toolCall.Function.Name), zap.Error(toolErr))
				result = fmt.Sprintf("Tool execution failed: %s", toolErr.Error())

				failedEvent := &types.Message{
					Kind:      "message",
					MessageID: fmt.Sprintf("tool-failed-%s", toolCall.Id),
					Role:      "assistant",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "data",
							"data": map[string]interface{}{
								"tool_name": toolCall.Function.Name,
								"status":    "failed",
							},
						},
					},
				}

				select {
				case outputChan <- failedEvent:
				case <-ctx.Done():
				}
			} else {
				completedEvent := &types.Message{
					Kind:      "message",
					MessageID: fmt.Sprintf("tool-completed-%s", toolCall.Id),
					Role:      "assistant",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "data",
							"data": map[string]interface{}{
								"tool_name": toolCall.Function.Name,
								"status":    "completed",
							},
						},
					},
				}

				select {
				case outputChan <- completedEvent:
				case <-ctx.Done():
					return toolResultMessages
				}
			}
		}

		// Always add a tool result message, regardless of success or failure
		toolResultMessage := types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("tool-result-%s", toolCall.Id),
			Role:      "tool",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "data",
					"data": map[string]interface{}{
						"tool_call_id": toolCall.Id,
						"result":       result,
						"error":        toolErr != nil,
					},
				},
			},
		}

		select {
		case outputChan <- &toolResultMessage:
		case <-ctx.Done():
			return toolResultMessages
		}

		toolResultMessages = append(toolResultMessages, toolResultMessage)
	}

	return toolResultMessages
}
