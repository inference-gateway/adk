package server

import (
	config "github.com/inference-gateway/a2a/adk/server/config"
	zap "go.uber.org/zap"
)

// A2AServerBuilder provides a fluent interface for building A2A servers
type A2AServerBuilder struct {
	cfg                 config.Config
	logger              *zap.Logger
	taskHandler         TaskHandler
	taskResultProcessor TaskResultProcessor
	agentInfoProvider   AgentInfoProvider
	agent               OpenAICompatibleAgent
}

// NewA2AServerBuilder creates a new server builder with required dependencies
func NewA2AServerBuilder(cfg config.Config, logger *zap.Logger) *A2AServerBuilder {
	return &A2AServerBuilder{
		cfg:    cfg,
		logger: logger,
	}
}

// WithTaskHandler sets a custom task handler
func (b *A2AServerBuilder) WithTaskHandler(handler TaskHandler) *A2AServerBuilder {
	b.taskHandler = handler
	return b
}

// WithTaskResultProcessor sets a custom task result processor
func (b *A2AServerBuilder) WithTaskResultProcessor(processor TaskResultProcessor) *A2AServerBuilder {
	b.taskResultProcessor = processor
	return b
}

// WithAgentInfoProvider sets a custom agent info provider
func (b *A2AServerBuilder) WithAgentInfoProvider(provider AgentInfoProvider) *A2AServerBuilder {
	b.agentInfoProvider = provider
	return b
}

// WithAgent sets a custom OpenAI-compatible agent
func (b *A2AServerBuilder) WithAgent(agent OpenAICompatibleAgent) *A2AServerBuilder {
	b.agent = agent
	return b
}

// WithAgentAndTools creates an agent with LLM configuration and tools
func (b *A2AServerBuilder) WithAgentAndTools(llmConfig *config.LLMProviderClientConfig, toolBox ToolBox) *A2AServerBuilder {
	agent, err := NewOpenAICompatibleAgentWithConfig(b.logger, llmConfig)
	if err != nil {
		b.logger.Error("failed to create openai-compatible agent", zap.Error(err))
	} else {
		agent.SetToolBox(toolBox)
		b.agent = agent
	}

	return b
}

// Build creates and returns the configured A2A server
func (b *A2AServerBuilder) Build() A2AServer {
	server := NewDefaultA2AServer()

	if b.agent != nil {
		server.SetAgent(b.agent)

		if b.taskHandler == nil {
			server.SetTaskHandler(NewAgentTaskHandler(b.logger, b.agent))
			b.logger.Info("configured agent task handler with openai-compatible agent")
		}
	}

	if b.taskHandler != nil {
		server.SetTaskHandler(b.taskHandler)
	}

	if b.taskResultProcessor != nil {
		server.SetTaskResultProcessor(b.taskResultProcessor)
	}

	if b.agentInfoProvider != nil {
		server.SetAgentInfoProvider(b.agentInfoProvider)
	}

	return server
}

// SimpleA2AServerWithAgent creates a basic A2A server with an OpenAI-compatible agent
// This is a convenience function for agent-based use cases
func SimpleA2AServerWithAgent(cfg config.Config, logger *zap.Logger, llmConfig *config.LLMProviderClientConfig, toolBox ToolBox) A2AServer {
	return NewA2AServerBuilder(cfg, logger).
		WithAgentAndTools(llmConfig, toolBox).
		Build()
}

// CustomA2AServer creates an A2A server with custom components
// This provides more control over the server configuration
func CustomA2AServer(
	cfg config.Config,
	logger *zap.Logger,
	taskHandler TaskHandler,
	taskResultProcessor TaskResultProcessor,
	agentInfoProvider AgentInfoProvider,
) A2AServer {
	return NewA2AServerBuilder(cfg, logger).
		WithTaskHandler(taskHandler).
		WithTaskResultProcessor(taskResultProcessor).
		WithAgentInfoProvider(agentInfoProvider).
		Build()
}

// CustomA2AServerWithAgent creates an A2A server with custom components and an agent
// This provides maximum control over the server configuration
func CustomA2AServerWithAgent(
	cfg config.Config,
	logger *zap.Logger,
	agent OpenAICompatibleAgent,
	toolBox ToolBox,
	taskResultProcessor TaskResultProcessor,
	agentInfoProvider AgentInfoProvider,
) A2AServer {
	if toolBox != nil {
		agent.SetToolBox(toolBox)
	}

	return NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithTaskResultProcessor(taskResultProcessor).
		WithAgentInfoProvider(agentInfoProvider).
		Build()
}
