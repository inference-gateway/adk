package server

import (
	"context"

	config "github.com/inference-gateway/adk/server/config"
	zap "go.uber.org/zap"
)

// AgentBuilder provides a fluent interface for building OpenAI-compatible agents with custom configurations.
// This interface allows for flexible agent construction with optional components and settings.
// Use NewAgentBuilder to create an instance, then chain method calls to configure the agent.
//
// Example:
//
//	agent := NewAgentBuilder(logger).
//	  WithConfig(agentConfig).
//	  WithLLMClient(client).
//	  WithCallbacks(&CallbackConfig{
//	    BeforeAgent: []BeforeAgentCallback{myBeforeAgentCallback},
//	    AfterAgent:  []AfterAgentCallback{myAfterAgentCallback},
//	  }).
//	  Build()
type AgentBuilder interface {
	// WithConfig sets the agent configuration
	WithConfig(config *config.AgentConfig) AgentBuilder
	// WithLLMClient sets a pre-configured LLM client
	WithLLMClient(client LLMClient) AgentBuilder
	// WithToolBox sets a custom toolbox
	WithToolBox(toolBox ToolBox) AgentBuilder
	// WithDefaultToolBox sets the default toolbox
	WithDefaultToolBox() AgentBuilder
	// WithSystemPrompt sets the system prompt (overrides config)
	WithSystemPrompt(prompt string) AgentBuilder
	// WithMaxChatCompletion sets the maximum chat completion iterations for the agent
	WithMaxChatCompletion(max int) AgentBuilder
	// WithMaxConversationHistory sets the maximum conversation history for the agent
	WithMaxConversationHistory(max int) AgentBuilder
	// WithCallbacks sets the callback configuration for the agent
	// Callbacks allow you to hook into various points of the agent's execution lifecycle
	// including before/after agent execution, model calls, and tool execution
	WithCallbacks(config *CallbackConfig) AgentBuilder
	// GetConfig returns the current agent configuration (for testing purposes)
	GetConfig() *config.AgentConfig
	// Build creates and returns the configured agent
	Build() (*OpenAICompatibleAgentImpl, error)
}

var _ AgentBuilder = (*AgentBuilderImpl)(nil)

// AgentBuilderImpl is the concrete implementation of the AgentBuilder interface.
// It provides a fluent interface for building OpenAI-compatible agents with custom configurations.
type AgentBuilderImpl struct {
	logger          *zap.Logger
	config          *config.AgentConfig
	llmClient       LLMClient
	toolBox         ToolBox
	systemPrompt    *string // Use pointer to distinguish between not set and empty string
	callbackConfig  *CallbackConfig
}

// NewAgentBuilder creates a new agent builder with required dependencies.
//
// Parameters:
//   - logger: Logger instance to use for the agent
//
// Returns:
//
//	AgentBuilder interface that can be used to configure the agent before building.
//
// Example:
//
//	logger, _ := zap.NewDevelopment()
//	agent, err := NewAgentBuilder(logger).
//	  WithConfig(agentConfig).
//	  Build()
func NewAgentBuilder(logger *zap.Logger) AgentBuilder {
	defaultCfg, err := config.NewWithDefaults(context.Background(), nil)
	var agentConfig *config.AgentConfig
	if err == nil && defaultCfg != nil {
		agentConfig = &defaultCfg.AgentConfig

		if agentConfig.Provider == "" {
			agentConfig.Provider = "openai"
		}
		if agentConfig.Model == "" {
			agentConfig.Model = "gpt-3.5-turbo"
		}
	} else {
		agentConfig = &config.AgentConfig{
			Provider:                    "openai",
			Model:                       "gpt-3.5-turbo",
			MaxChatCompletionIterations: 10,
			MaxConversationHistory:      20,
			SystemPrompt:                "You are a helpful AI assistant.",
		}
	}

	return &AgentBuilderImpl{
		logger: logger,
		config: agentConfig,
	}
}

// WithConfig sets the agent configuration
func (b *AgentBuilderImpl) WithConfig(userConfig *config.AgentConfig) AgentBuilder {
	if userConfig != nil {
		b.config = userConfig
	}
	return b
}

// WithLLMClient sets a pre-configured LLM client
func (b *AgentBuilderImpl) WithLLMClient(client LLMClient) AgentBuilder {
	b.llmClient = client
	return b
}

// WithToolBox sets a custom toolbox
func (b *AgentBuilderImpl) WithToolBox(toolBox ToolBox) AgentBuilder {
	b.toolBox = toolBox
	return b
}

// WithDefaultToolBox sets the default toolbox
func (b *AgentBuilderImpl) WithDefaultToolBox() AgentBuilder {
	b.toolBox = NewDefaultToolBox(&b.config.ToolBoxConfig)
	return b
}

// WithSystemPrompt sets the system prompt (overrides config)
func (b *AgentBuilderImpl) WithSystemPrompt(prompt string) AgentBuilder {
	b.systemPrompt = &prompt
	return b
}

// WithMaxChatCompletion sets the maximum chat completion iterations for the agent
func (b *AgentBuilderImpl) WithMaxChatCompletion(max int) AgentBuilder {
	b.config.MaxChatCompletionIterations = max
	return b
}

// WithMaxConversationHistory sets the maximum conversation history for the agent
func (b *AgentBuilderImpl) WithMaxConversationHistory(max int) AgentBuilder {
	b.config.MaxConversationHistory = max
	return b
}

// WithCallbacks sets the callback configuration for the agent
// This allows hooking into the agent lifecycle, model calls, and tool execution
func (b *AgentBuilderImpl) WithCallbacks(config *CallbackConfig) AgentBuilder {
	b.callbackConfig = config
	return b
}

// GetConfig returns the current agent configuration (for testing purposes)
func (b *AgentBuilderImpl) GetConfig() *config.AgentConfig {
	return b.config
}

// Build creates and returns the configured agent
func (b *AgentBuilderImpl) Build() (*OpenAICompatibleAgentImpl, error) {
	var agent *OpenAICompatibleAgentImpl

	if b.config != nil {
		agent = NewOpenAICompatibleAgentWithConfig(b.logger, b.config)
	} else {
		agent = NewOpenAICompatibleAgent(b.logger)
	}

	// Override system prompt if explicitly set
	if b.systemPrompt != nil {
		if agent.config == nil {
			agent.config = &config.AgentConfig{}
		}
		agent.config.SystemPrompt = *b.systemPrompt
	}

	if b.llmClient != nil {
		agent.SetLLMClient(b.llmClient)
	}

	if b.toolBox != nil {
		agent.SetToolBox(b.toolBox)
	}

	// Set up callback executor if callbacks are configured
	if b.callbackConfig != nil {
		agent.SetCallbackExecutor(NewCallbackExecutor(b.callbackConfig, b.logger))
	}

	return agent, nil
}

// SimpleAgent creates a basic agent with default configuration
func SimpleAgent(logger *zap.Logger) (*OpenAICompatibleAgentImpl, error) {
	return NewAgentBuilder(logger).Build()
}

// AgentWithConfig creates an agent with the provided configuration
func AgentWithConfig(logger *zap.Logger, config *config.AgentConfig) (*OpenAICompatibleAgentImpl, error) {
	return NewAgentBuilder(logger).WithConfig(config).Build()
}

// AgentWithLLM creates an agent with a pre-configured LLM client
func AgentWithLLM(logger *zap.Logger, llmClient LLMClient) (*OpenAICompatibleAgentImpl, error) {
	return NewAgentBuilder(logger).WithLLMClient(llmClient).Build()
}

// FullyConfiguredAgent creates an agent with all components configured
func FullyConfiguredAgent(logger *zap.Logger, config *config.AgentConfig, llmClient LLMClient, toolBox ToolBox) (*OpenAICompatibleAgentImpl, error) {
	return NewAgentBuilder(logger).
		WithConfig(config).
		WithLLMClient(llmClient).
		WithToolBox(toolBox).
		Build()
}
