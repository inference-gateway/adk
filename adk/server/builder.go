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

// Build creates and returns the configured A2A server
func (b *A2AServerBuilder) Build() A2AServer {
	server := NewDefaultA2AServer(b.cfg, b.logger)

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
