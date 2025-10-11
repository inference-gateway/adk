package server

import (
	"context"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	config "github.com/inference-gateway/adk/server/config"
	utils "github.com/inference-gateway/adk/server/utils"
	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// OpenAICompatibleAgent represents an agent that can interact with OpenAI-compatible LLM APIs and execute tools
// The agent is stateless and does not maintain conversation history
// Tools are configured during agent creation via the toolbox
// All agent execution is event-driven via RunWithStream
type OpenAICompatibleAgent interface {
	// RunWithStream processes a conversation and returns a streaming response
	// Uses the agent's configured toolbox for tool execution
	// Events are emitted for deltas, tool execution, completions, and errors
	RunWithStream(ctx context.Context, messages []types.Message) (<-chan cloudevents.Event, error)
}

// OpenAICompatibleAgentImpl is the implementation of OpenAICompatibleAgent
// This implementation is stateless and does not maintain conversation history
type OpenAICompatibleAgentImpl struct {
	logger           *zap.Logger
	llmClient        LLMClient
	toolBox          ToolBox
	callbackExecutor CallbackExecutor
	converter        utils.MessageConverter
	config           *config.AgentConfig
}

// NewOpenAICompatibleAgent creates a new OpenAICompatibleAgentImpl
func NewOpenAICompatibleAgent(logger *zap.Logger) *OpenAICompatibleAgentImpl {
	defaultConfig := &config.AgentConfig{
		MaxChatCompletionIterations: 10,
		SystemPrompt:                "You are a helpful AI assistant.",
	}
	return &OpenAICompatibleAgentImpl{
		logger:    logger,
		converter: utils.NewMessageConverter(logger),
		config:    defaultConfig,
	}
}

// NewOpenAICompatibleAgentWithConfig creates a new OpenAICompatibleAgentImpl with configuration
func NewOpenAICompatibleAgentWithConfig(logger *zap.Logger, cfg *config.AgentConfig) *OpenAICompatibleAgentImpl {
	return &OpenAICompatibleAgentImpl{
		logger:    logger,
		converter: utils.NewMessageConverter(logger),
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

// SetCallbackExecutor sets the callback executor for the agent
func (a *OpenAICompatibleAgentImpl) SetCallbackExecutor(executor CallbackExecutor) {
	a.callbackExecutor = executor
}

// GetCallbackExecutor returns the callback executor for the agent if available or a provided default
func (a *OpenAICompatibleAgentImpl) GetCallbackExecutor() CallbackExecutor {
	if a.callbackExecutor == nil {
		return NewCallbackExecutor(nil, a.logger)
	}
	return a.callbackExecutor
}
