package server

import (
	"context"
	"encoding/json"
	"fmt"

	adk "github.com/inference-gateway/a2a/adk"
	config "github.com/inference-gateway/a2a/adk/server/config"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// OpenAICompatibleAgent represents an agent that can interact with OpenAI-compatible LLM APIs and execute tools
type OpenAICompatibleAgent interface {
	// ProcessTask processes a task with optional tool calling capabilities
	ProcessTask(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error)

	// SetSystemPrompt sets the system prompt for the agent
	SetSystemPrompt(prompt string)

	// GetSystemPrompt returns the current system prompt
	GetSystemPrompt() string

	// SetToolBox sets the toolbox for the agent
	SetToolBox(toolBox ToolBox)

	// GetToolBox returns the current toolbox
	GetToolBox() ToolBox

	// SetLLMClient sets the LLM client for the agent
	SetLLMClient(client LLMClient)

	// GetLLMClient returns the current LLM client
	GetLLMClient() LLMClient
}

// DefaultOpenAICompatibleAgent is the default implementation of OpenAICompatibleAgent
type DefaultOpenAICompatibleAgent struct {
	logger       *zap.Logger
	llmClient    LLMClient
	toolBox      ToolBox
	systemPrompt string
}

// NewDefaultOpenAICompatibleAgent creates a new DefaultOpenAICompatibleAgent
func NewDefaultOpenAICompatibleAgent(logger *zap.Logger) *DefaultOpenAICompatibleAgent {
	return &DefaultOpenAICompatibleAgent{
		logger:       logger,
		systemPrompt: "You are a helpful AI assistant.",
	}
}

// NewOpenAICompatibleAgentWithLLM creates a new agent with an LLM client
func NewOpenAICompatibleAgentWithLLM(logger *zap.Logger, llmClient LLMClient) *DefaultOpenAICompatibleAgent {
	agent := NewDefaultOpenAICompatibleAgent(logger)
	agent.llmClient = llmClient
	return agent
}

// NewOpenAICompatibleAgentWithConfig creates a new agent with LLM configuration
func NewOpenAICompatibleAgentWithConfig(logger *zap.Logger, config *config.AgentConfig) (*DefaultOpenAICompatibleAgent, error) {
	client, err := NewOpenAICompatibleLLMClient(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create llm client: %w", err)
	}

	agent := NewDefaultOpenAICompatibleAgent(logger)
	agent.llmClient = client
	return agent, nil
}

// ProcessTask processes a task with optional tool calling capabilities
func (a *DefaultOpenAICompatibleAgent) ProcessTask(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error) {
	if a.llmClient == nil {
		return a.processWithoutLLM(task, message), nil
	}

	messages := make([]adk.Message, 0)

	if a.systemPrompt != "" {
		systemMessage := adk.Message{
			Kind:      "message",
			MessageID: "system-prompt",
			Role:      "system",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": a.systemPrompt,
				},
			},
		}
		messages = append(messages, systemMessage)
	}

	messages = append(messages, task.History...)

	messages = append(messages, *message)

	if a.toolBox != nil && len(a.toolBox.GetTools()) > 0 {
		return a.processWithToolCalling(ctx, task, messages)
	}

	return a.processWithoutToolCalling(ctx, task, messages)
}

// processWithoutToolCalling processes the task without tool calling
func (a *DefaultOpenAICompatibleAgent) processWithoutToolCalling(ctx context.Context, task *adk.Task, messages []adk.Message) (*adk.Task, error) {
	result, err := a.llmClient.CreateChatCompletion(ctx, messages)
	if err != nil {
		a.logger.Error("llm completion failed", zap.Error(err))
		task.Status.State = adk.TaskStateFailed
		errorMessage := &adk.Message{
			Kind:      "message",
			MessageID: "error-" + task.ID,
			Role:      "assistant",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": fmt.Sprintf("LLM request failed: %v", err),
				},
			},
		}
		task.Status.Message = errorMessage
		return task, nil
	}

	// Cast to A2A message (should be the case when no tools are provided)
	response, ok := result.(*adk.Message)
	if !ok {
		a.logger.Error("unexpected response type from llm client")
		task.Status.State = adk.TaskStateFailed
		return task, fmt.Errorf("unexpected response type from llm client")
	}

	task.History = append(task.History, *response)
	task.Status.State = adk.TaskStateCompleted
	task.Status.Message = response

	a.logger.Info("task completed with llm response")
	return task, nil
}

// processWithToolCalling processes the task with tool calling capability
func (a *DefaultOpenAICompatibleAgent) processWithToolCalling(ctx context.Context, task *adk.Task, messages []adk.Message) (*adk.Task, error) {
	tools := a.toolBox.GetTools()

	result, err := a.llmClient.CreateChatCompletion(ctx, messages, tools...)
	if err != nil {
		a.logger.Error("llm completion failed", zap.Error(err))
		task.Status.State = adk.TaskStateFailed
		errorMessage := &adk.Message{
			Kind:      "message",
			MessageID: "error-" + task.ID,
			Role:      "assistant",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": fmt.Sprintf("LLM request failed: %v", err),
				},
			},
		}
		task.Status.Message = errorMessage
		return task, nil
	}

	sdkResponse, ok := result.(*sdk.CreateChatCompletionResponse)
	if !ok {
		a.logger.Error("unexpected response type from llm client when using tools")
		task.Status.State = adk.TaskStateFailed
		return task, fmt.Errorf("unexpected response type from llm client")
	}

	if len(sdkResponse.Choices) == 0 {
		a.logger.Error("no choices in llm response")
		task.Status.State = adk.TaskStateFailed
		return task, fmt.Errorf("no choices in llm response")
	}

	choice := sdkResponse.Choices[0]

	if choice.Message.ToolCalls != nil && len(*choice.Message.ToolCalls) > 0 {
		return a.processToolCalls(ctx, task, messages, *choice.Message.ToolCalls)
	}

	response := &adk.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("msg-%d", len(task.History)+1),
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": choice.Message.Content,
			},
		},
	}

	task.History = append(task.History, *response)
	task.Status.State = adk.TaskStateCompleted
	task.Status.Message = response

	a.logger.Info("task completed with llm response (no tools used)")
	return task, nil
}

// processWithoutLLM processes the task without LLM when no client is available
func (a *DefaultOpenAICompatibleAgent) processWithoutLLM(task *adk.Task, message *adk.Message) *adk.Task {
	response := &adk.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("msg-%d", len(task.History)+1),
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "I'm an AI assistant, but I don't have access to an LLM client right now. Please configure an LLM client to enable my full capabilities.",
			},
		},
	}

	task.History = append(task.History, *response)
	task.Status.State = adk.TaskStateCompleted
	task.Status.Message = response

	a.logger.Info("task completed without llm")
	return task
}

// SetSystemPrompt sets the system prompt for the agent
func (a *DefaultOpenAICompatibleAgent) SetSystemPrompt(prompt string) {
	a.systemPrompt = prompt
}

// GetSystemPrompt returns the current system prompt
func (a *DefaultOpenAICompatibleAgent) GetSystemPrompt() string {
	return a.systemPrompt
}

// SetToolBox sets the toolbox for the agent
func (a *DefaultOpenAICompatibleAgent) SetToolBox(toolBox ToolBox) {
	a.toolBox = toolBox
}

// GetToolBox returns the current toolbox
func (a *DefaultOpenAICompatibleAgent) GetToolBox() ToolBox {
	return a.toolBox
}

// SetLLMClient sets the LLM client for the agent
func (a *DefaultOpenAICompatibleAgent) SetLLMClient(client LLMClient) {
	a.llmClient = client
}

// GetLLMClient returns the current LLM client
func (a *DefaultOpenAICompatibleAgent) GetLLMClient() LLMClient {
	return a.llmClient
}

// processToolCalls handles tool calling workflow
func (a *DefaultOpenAICompatibleAgent) processToolCalls(ctx context.Context, task *adk.Task, messages []adk.Message, toolCalls []sdk.ChatCompletionMessageToolCall) (*adk.Task, error) {
	a.logger.Info("processing tool calls", zap.Int("count", len(toolCalls)))

	// Execute each tool call
	toolResults := make([]adk.Message, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		if toolCall.Type != "function" {
			continue
		}

		function := toolCall.Function
		if function.Name == "" {
			continue
		}

		// Parse tool arguments
		var args map[string]interface{}
		if function.Arguments != "" {
			if err := json.Unmarshal([]byte(function.Arguments), &args); err != nil {
				a.logger.Error("failed to parse tool arguments",
					zap.String("tool", function.Name),
					zap.Error(err))
				continue
			}
		}

		// Execute tool
		result, err := a.toolBox.ExecuteTool(ctx, function.Name, args)
		if err != nil {
			result = fmt.Sprintf("Error executing tool: %v", err)
			a.logger.Error("tool execution failed",
				zap.String("tool", function.Name),
				zap.Error(err))
		} else {
			a.logger.Info("tool executed successfully",
				zap.String("tool", function.Name))
		}

		// Create tool result message
		toolResultMessage := adk.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("tool-result-%s", toolCall.Id),
			Role:      "tool",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind":         "tool_result",
					"tool_call_id": toolCall.Id,
					"tool_name":    function.Name,
					"result":       result,
				},
			},
		}

		toolResults = append(toolResults, toolResultMessage)
		task.History = append(task.History, toolResultMessage)
	}

	// Get final response from LLM with tool results
	finalMessages := append(messages, toolResults...)

	finalResult, err := a.llmClient.CreateChatCompletion(ctx, finalMessages)
	if err != nil {
		a.logger.Error("final llm completion after tool calls failed", zap.Error(err))
		task.Status.State = adk.TaskStateFailed
		errorMessage := &adk.Message{
			Kind:      "message",
			MessageID: "error-" + task.ID,
			Role:      "assistant",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": fmt.Sprintf("Final LLM request failed: %v", err),
				},
			},
		}
		task.Status.Message = errorMessage
		return task, nil
	}

	finalResponse, ok := finalResult.(*adk.Message)
	if !ok {
		a.logger.Error("unexpected response type from final llm completion")
		task.Status.State = adk.TaskStateFailed
		return task, fmt.Errorf("unexpected response type from final llm completion")
	}

	task.History = append(task.History, *finalResponse)
	task.Status.State = adk.TaskStateCompleted
	task.Status.Message = finalResponse

	a.logger.Info("task completed with tool calling workflow")
	return task, nil
}
