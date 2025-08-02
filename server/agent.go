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

// OpenAICompatibleAgent represents an agent that can interact with OpenAI-compatible LLM APIs and execute tools
type OpenAICompatibleAgent interface {
	// Run processes a conversation and returns the assistant's response
	// Takes conversation messages and available tools, returns the response message
	Run(ctx context.Context, messages []types.Message, tools []sdk.ChatCompletionTool) (*types.Message, error)

	// RunWithStream processes a conversation and returns a streaming response
	// Takes conversation messages and available tools, returns a stream of response chunks
	RunWithStream(ctx context.Context, messages []types.Message, tools []sdk.ChatCompletionTool) (<-chan *types.Message, error)

	// GetConversationHistory returns the full conversation history including tool calls and results
	// from the last Run() execution
	GetConversationHistory() []types.Message
}

// OpenAICompatibleAgentImpl is the implementation of OpenAICompatibleAgent
type OpenAICompatibleAgentImpl struct {
	logger              *zap.Logger
	llmClient           LLMClient
	toolBox             ToolBox
	converter           utils.MessageConverter
	config              *config.AgentConfig
	conversationHistory []types.Message
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

// Run processes a conversation and returns the assistant's response
func (a *OpenAICompatibleAgentImpl) Run(ctx context.Context, messages []types.Message, tools []sdk.ChatCompletionTool) (*types.Message, error) {
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

	a.conversationHistory = make([]types.Message, len(messages))
	copy(a.conversationHistory, messages)

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
		a.conversationHistory = append(a.conversationHistory, *assistantA2A)

		if assistantMessage.ToolCalls == nil || len(*assistantMessage.ToolCalls) == 0 || a.toolBox == nil {
			return assistantA2A, nil
		}

		for _, toolCall := range *assistantMessage.ToolCalls {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments for %s: %w", toolCall.Function.Name, err)
			}

			result, err := a.toolBox.ExecuteTool(ctx, toolCall.Function.Name, args)
			if err != nil {
				return nil, fmt.Errorf("failed to execute tool %s: %w", toolCall.Function.Name, err)
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
						},
					},
				},
			}
			a.conversationHistory = append(a.conversationHistory, *toolA2A)
		}
	}

	return nil, fmt.Errorf("maximum iterations (%d) reached without final response", maxIterations)
}

// GetConversationHistory returns the full conversation history including tool calls and results
func (a *OpenAICompatibleAgentImpl) GetConversationHistory() []types.Message {
	history := make([]types.Message, len(a.conversationHistory))
	copy(history, a.conversationHistory)
	return history
}

// RunWithStream processes a conversation and returns a streaming response
func (a *OpenAICompatibleAgentImpl) RunWithStream(ctx context.Context, messages []types.Message, tools []sdk.ChatCompletionTool) (<-chan *types.Message, error) {
	if a.llmClient == nil {
		return nil, fmt.Errorf("no LLM client configured for agent")
	}

	sdkMessages, err := a.converter.ConvertToSDK(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages to SDK format: %w", err)
	}

	if a.config != nil && a.config.SystemPrompt != "" {
		systemMessage := sdk.Message{
			Role:    sdk.System,
			Content: a.config.SystemPrompt,
		}
		sdkMessages = append([]sdk.Message{systemMessage}, sdkMessages...)
	}

	streamResponseChan, streamErrorChan := a.llmClient.CreateStreamingChatCompletion(ctx, sdkMessages, tools...)

	outputChan := make(chan *types.Message, 10)

	go func() {
		defer close(outputChan)

		var fullContent string
		var toolCalls []sdk.ChatCompletionMessageToolCall
		var assistantMessage *types.Message

		for {
			select {
			case <-ctx.Done():
				return
			case streamErr := <-streamErrorChan:
				if streamErr != nil {
					a.logger.Error("streaming failed", zap.Error(streamErr))
				}
				return
			case streamResp, ok := <-streamResponseChan:
				if !ok {
					if assistantMessage != nil && fullContent != "" && len(toolCalls) == 0 {
						select {
						case outputChan <- assistantMessage:
						case <-ctx.Done():
						}
					}
					return
				}

				if streamResp == nil || len(streamResp.Choices) == 0 {
					continue
				}

				choice := streamResp.Choices[0]

				if choice.FinishReason == "tool_calls" && len(toolCalls) > 0 && a.toolBox != nil {
					a.handleStreamingToolExecutionWithEvents(ctx, messages, assistantMessage, tools, toolCalls, outputChan)
				}

				if assistantMessage == nil {
					assistantMessage = &types.Message{
						Kind:      "message",
						MessageID: fmt.Sprintf("assistant-stream-%d", len(messages)),
						Role:      "assistant",
						Parts:     make([]types.Part, 0),
					}
				}

				if choice.Delta.Content != "" {
					fullContent += choice.Delta.Content

					chunkMessage := &types.Message{
						Kind:      "message",
						MessageID: fmt.Sprintf("chunk-%d", len(fullContent)),
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
						return
					}
				}

				for _, toolCallChunk := range choice.Delta.ToolCalls {
					if toolCallChunk.Index >= len(toolCalls) {
						for len(toolCalls) <= toolCallChunk.Index {
							toolCalls = append(toolCalls, sdk.ChatCompletionMessageToolCall{
								Type:     "function",
								Function: sdk.ChatCompletionMessageToolCallFunction{},
							})
						}
					}

					toolCall := &toolCalls[toolCallChunk.Index]
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

				if fullContent != "" {
					assistantMessage.Parts = []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": fullContent,
						},
					}
				}
			}
		}
	}()

	return outputChan, nil
}

// handleStreamingToolExecutionWithEvents executes tool calls and emits events during streaming
func (a *OpenAICompatibleAgentImpl) handleStreamingToolExecutionWithEvents(ctx context.Context, originalMessages []types.Message, assistantMessage *types.Message, tools []sdk.ChatCompletionTool, toolCalls []sdk.ChatCompletionMessageToolCall, outputChan chan<- *types.Message) {
	if a.toolBox == nil {
		a.logger.Error("no toolbox configured for tool execution")
		return
	}

	for _, toolCall := range toolCalls {
		startEvent := &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("tool-start-%s-%d", toolCall.Function.Name, time.Now().UnixNano()),
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
			return
		}
	}

	for _, toolCall := range toolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			a.logger.Error("failed to parse tool arguments", zap.String("tool", toolCall.Function.Name), zap.Error(err))
			continue
		}

		result, err := a.toolBox.ExecuteTool(ctx, toolCall.Function.Name, args)
		if err != nil {
			a.logger.Error("failed to execute tool", zap.String("tool", toolCall.Function.Name), zap.Error(err))
			continue
		}

		completedEvent := &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("tool-completed-%s-%d", toolCall.Function.Name, time.Now().UnixNano()),
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
			return
		}

		toolResultMessage := &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("tool-result-%s-%d", toolCall.Function.Name, time.Now().UnixNano()),
			Role:      "tool",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "data",
					"data": map[string]interface{}{
						"tool_call_id": toolCall.Id,
						"tool_name":    toolCall.Function.Name,
						"result":       result,
					},
				},
			},
		}

		select {
		case outputChan <- toolResultMessage:
		case <-ctx.Done():
			return
		}

		a.conversationHistory = append(a.conversationHistory, *toolResultMessage)
	}
}
