package server

import (
	zap "go.uber.org/zap"
)

// A2AServerBuilder provides a fluent interface for building A2A servers
type A2AServerBuilder struct {
	cfg                 Config
	logger              *zap.Logger
	taskHandler         TaskHandler
	taskResultProcessor TaskResultProcessor
	agentInfoProvider   AgentInfoProvider
	llmClient           LLMClient
	llmConfig           *LLMProviderClientConfig
}

// NewA2AServerBuilder creates a new server builder with required dependencies
func NewA2AServerBuilder(cfg Config, logger *zap.Logger) *A2AServerBuilder {
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

// WithLLMClient sets a custom LLM client
func (b *A2AServerBuilder) WithLLMClient(client LLMClient) *A2AServerBuilder {
	b.llmClient = client
	return b
}

// WithOpenAICompatibleLLMClient configures an OpenAI-compatible LLM client using the provided config
func (b *A2AServerBuilder) WithOpenAICompatibleLLMClient(config *LLMProviderClientConfig) *A2AServerBuilder {
	if config != nil {
		if client, err := NewOpenAICompatibleLLMClient(config, b.logger); err == nil {
			b.llmClient = client
			b.llmConfig = config
		} else {
			b.logger.Error("failed to create openai-compatible llm client", zap.Error(err))
		}
	}
	return b
}

// WithLLMTaskHandler configures an LLM-powered task handler using the configured LLM client
func (b *A2AServerBuilder) WithLLMTaskHandler() *A2AServerBuilder {
	if b.llmClient != nil {
		if b.llmConfig != nil {
			b.taskHandler = NewLLMTaskHandlerWithConfig(b.logger, b.llmClient, b.llmConfig)
		} else {
			b.taskHandler = NewLLMTaskHandler(b.logger, b.llmClient)
		}
	} else {
		b.logger.Warn("llm client not configured, cannot create llm task handler")
	}
	return b
}

// WithSystemPrompt sets a custom system prompt for the LLM task handler
func (b *A2AServerBuilder) WithSystemPrompt(prompt string) *A2AServerBuilder {
	if b.llmConfig == nil {
		b.llmConfig = &LLMProviderClientConfig{}
	}
	b.llmConfig.SystemPrompt = prompt
	return b
}

// WithOpenAICompatibleLLMAndTaskHandler is a convenience method that sets up both LLM client and task handler
func (b *A2AServerBuilder) WithOpenAICompatibleLLMAndTaskHandler(config *LLMProviderClientConfig) *A2AServerBuilder {
	return b.WithOpenAICompatibleLLMClient(config).WithLLMTaskHandler()
}

// Build creates and returns the configured A2A server
func (b *A2AServerBuilder) Build() A2AServer {
	server := NewDefaultA2AServer(b.cfg, b.logger)

	if b.taskHandler != nil {
		server.SetTaskHandler(b.taskHandler)
	} else if b.llmClient != nil {
		// If no custom task handler is set but LLM client is available,
		// create a default task handler with LLM support
		var defaultHandler *DefaultTaskHandler
		if b.llmConfig != nil {
			defaultHandler = NewDefaultTaskHandlerWithLLM(b.logger, b.llmClient)
		} else {
			defaultHandler = NewDefaultTaskHandlerWithLLM(b.logger, b.llmClient)
		}
		server.SetTaskHandler(defaultHandler)
		b.logger.Info("configured default task handler with llm support")
	}

	if b.taskResultProcessor != nil {
		server.SetTaskResultProcessor(b.taskResultProcessor)
	}

	if b.agentInfoProvider != nil {
		server.SetAgentInfoProvider(b.agentInfoProvider)
	}

	if b.llmClient != nil {
		server.SetLLMClient(b.llmClient)

		// If we have a default task handler, also set the LLM client on it
		if defaultHandler, ok := server.GetTaskHandler().(*DefaultTaskHandler); ok {
			defaultHandler.SetLLMClient(b.llmClient)
		}
	}

	return server
}

// SimpleA2AServer creates a basic A2A server with minimal configuration
// This is a convenience function for simple use cases
func SimpleA2AServer(cfg Config, logger *zap.Logger) A2AServer {
	return NewA2AServerBuilder(cfg, logger).Build()
}

// CustomA2AServer creates an A2A server with custom components
// This provides more control over the server configuration
func CustomA2AServer(
	cfg Config,
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
