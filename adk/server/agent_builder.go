package server

import (
	config "github.com/inference-gateway/a2a/adk/server/config"
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
//	  Build()
type AgentBuilder interface {
	// WithConfig sets the agent configuration
	WithConfig(config *config.AgentConfig) AgentBuilder
	// WithLLMClient sets a pre-configured LLM client
	WithLLMClient(client LLMClient) AgentBuilder
	// WithToolBox sets a custom toolbox
	WithToolBox(toolBox ToolBox) AgentBuilder
	// WithSystemPrompt sets the system prompt (overrides config)
	WithSystemPrompt(prompt string) AgentBuilder
	// WithMaxChatCompletion sets the maximum chat completion iterations for the agent
	WithMaxChatCompletion(max int) AgentBuilder
	// Build creates and returns the configured agent
	Build() (*DefaultOpenAICompatibleAgent, error)
}

// AgentBuilderImpl is the concrete implementation of the AgentBuilder interface.
// It provides a fluent interface for building OpenAI-compatible agents with custom configurations.
type AgentBuilderImpl struct {
	logger       *zap.Logger
	config       *config.AgentConfig
	llmClient    LLMClient
	toolBox      ToolBox
	systemPrompt *string // Use pointer to distinguish between not set and empty string
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
	return &AgentBuilderImpl{
		logger: logger,
	}
}

// WithConfig sets the agent configuration
func (b *AgentBuilderImpl) WithConfig(config *config.AgentConfig) AgentBuilder {
	b.config = config
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

// WithSystemPrompt sets the system prompt (overrides config)
func (b *AgentBuilderImpl) WithSystemPrompt(prompt string) AgentBuilder {
	b.systemPrompt = &prompt
	return b
}

// WithMaxChatCompletion sets the maximum chat completion iterations for the agent
func (b *AgentBuilderImpl) WithMaxChatCompletion(max int) AgentBuilder {
	if b.config == nil {
		b.config = &config.AgentConfig{}
	}
	b.config.MaxChatCompletionIterations = max
	return b
}

// Build creates and returns the configured agent
func (b *AgentBuilderImpl) Build() (*DefaultOpenAICompatibleAgent, error) {
	agentConfig := b.config
	if agentConfig == nil {
		agentConfig = &config.AgentConfig{
			MaxChatCompletionIterations: 10,
			SystemPrompt:                "You are a helpful AI assistant.",
		}
	}

	if b.systemPrompt != nil {
		agentConfig.SystemPrompt = *b.systemPrompt
	}

	agent := NewDefaultOpenAICompatibleAgentWithConfig(b.logger, agentConfig)

	if b.llmClient != nil {
		agent.llmClient = b.llmClient
	} else if b.config != nil {
		client, err := NewOpenAICompatibleLLMClient(b.config, b.logger)
		if err != nil {
			return nil, err
		}
		agent.llmClient = client
	}

	if b.toolBox != nil {
		agent.toolBox = b.toolBox
	}

	return agent, nil
}

// SimpleAgent creates a basic agent with default configuration
func SimpleAgent(logger *zap.Logger) (*DefaultOpenAICompatibleAgent, error) {
	return NewAgentBuilder(logger).Build()
}

// AgentWithConfig creates an agent with the provided configuration
func AgentWithConfig(logger *zap.Logger, config *config.AgentConfig) (*DefaultOpenAICompatibleAgent, error) {
	return NewAgentBuilder(logger).WithConfig(config).Build()
}

// AgentWithLLM creates an agent with a pre-configured LLM client
func AgentWithLLM(logger *zap.Logger, llmClient LLMClient) (*DefaultOpenAICompatibleAgent, error) {
	return NewAgentBuilder(logger).WithLLMClient(llmClient).Build()
}

// FullyConfiguredAgent creates an agent with all components configured
func FullyConfiguredAgent(logger *zap.Logger, config *config.AgentConfig, llmClient LLMClient, toolBox ToolBox) (*DefaultOpenAICompatibleAgent, error) {
	return NewAgentBuilder(logger).
		WithConfig(config).
		WithLLMClient(llmClient).
		WithToolBox(toolBox).
		Build()
}
