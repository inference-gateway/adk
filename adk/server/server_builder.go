package server

import (
	"context"

	config "github.com/inference-gateway/a2a/adk/server/config"
	otel "github.com/inference-gateway/a2a/adk/server/otel"
	zap "go.uber.org/zap"
)

// A2AServerBuilder provides a fluent interface for building A2A servers with custom configurations.
// This interface allows for flexible server construction with optional components and settings.
// Use NewA2AServerBuilder to create an instance, then chain method calls to configure the server.
//
// Example:
//
//	server := NewA2AServerBuilder(config, logger).
//	  WithAgent(agent).
//	  Build()
type A2AServerBuilder interface {
	// WithTaskHandler sets a custom task handler for processing A2A tasks.
	// If not set, a default task handler will be used.
	WithTaskHandler(handler TaskHandler) A2AServerBuilder

	// WithTaskResultProcessor sets a custom task result processor for handling tool call results.
	// This allows custom business logic for determining when tasks should be completed.
	WithTaskResultProcessor(processor TaskResultProcessor) A2AServerBuilder

	// WithAgent sets a pre-configured OpenAI-compatible agent for processing tasks.
	// This is useful when you have already configured an agent with specific settings.
	WithAgent(agent OpenAICompatibleAgent) A2AServerBuilder

	// WithLogger sets a custom logger for the builder and resulting server.
	// This allows using a logger configured with appropriate level based on the Debug config.
	WithLogger(logger *zap.Logger) A2AServerBuilder

	// Build creates and returns the configured A2A server.
	// This method applies configuration defaults and initializes all components.
	Build() A2AServer
}

var _ A2AServerBuilder = (*A2AServerBuilderImpl)(nil)

// A2AServerBuilderImpl is the concrete implementation of the A2AServerBuilder interface.
// It provides a fluent interface for building A2A servers with custom configurations.
// This struct holds the configuration and optional components that will be used to create the server.
type A2AServerBuilderImpl struct {
	cfg                 config.Config         // Base configuration for the server
	logger              *zap.Logger           // Logger instance for the server
	taskHandler         TaskHandler           // Optional custom task handler
	taskResultProcessor TaskResultProcessor   // Optional custom task result processor
	agent               OpenAICompatibleAgent // Optional pre-configured agent
}

// NewA2AServerBuilder creates a new server builder with required dependencies.
// The configuration passed here will be used to configure the server.
// Any nil nested configuration objects will be populated with sensible defaults when Build() is called.
//
// Parameters:
//   - cfg: The base configuration for the server (agent name, port, etc.)
//   - logger: Logger instance to use for the server (should match cfg.Debug level)
//
// Returns:
//
//	A2AServerBuilder interface that can be used to further configure the server before building.
//
// Example:
//
//	cfg := config.Config{
//	  AgentName: "my-agent",
//	  Port: "8080",
//	  Debug: true,
//	}
//	logger, _ := zap.NewDevelopment() // Use development logger for debug
//	server := NewA2AServerBuilder(cfg, logger).
//	  WithAgent(myAgent).
//	  Build()
func NewA2AServerBuilder(cfg config.Config, logger *zap.Logger) A2AServerBuilder {
	if isCapabilitiesConfigEmpty(cfg.CapabilitiesConfig) {
		defaultCfg, err := config.NewWithDefaults(context.Background(), nil)
		if err == nil {
			cfg.CapabilitiesConfig = defaultCfg.CapabilitiesConfig
		}
	}

	return &A2AServerBuilderImpl{
		cfg:    cfg,
		logger: logger,
	}
}

// isCapabilitiesConfigEmpty checks if the capabilities config has all zero values
func isCapabilitiesConfigEmpty(capabilities config.CapabilitiesConfig) bool {
	return !capabilities.Streaming && !capabilities.PushNotifications && !capabilities.StateTransitionHistory
}

// WithTaskHandler sets a custom task handler
func (b *A2AServerBuilderImpl) WithTaskHandler(handler TaskHandler) A2AServerBuilder {
	b.taskHandler = handler
	return b
}

// WithTaskResultProcessor sets a custom task result processor
func (b *A2AServerBuilderImpl) WithTaskResultProcessor(processor TaskResultProcessor) A2AServerBuilder {
	b.taskResultProcessor = processor
	return b
}

// WithAgent sets a custom OpenAI-compatible agent
func (b *A2AServerBuilderImpl) WithAgent(agent OpenAICompatibleAgent) A2AServerBuilder {
	b.agent = agent
	return b
}

// WithLogger sets a custom logger for the builder
func (b *A2AServerBuilderImpl) WithLogger(logger *zap.Logger) A2AServerBuilder {
	b.logger = logger
	return b
}

// Build creates and returns the configured A2A server.
func (b *A2AServerBuilderImpl) Build() A2AServer {
	var telemetryInstance otel.OpenTelemetry
	if b.cfg.TelemetryConfig.Enable {
		var err error
		telemetryInstance, err = otel.NewOpenTelemetry(&b.cfg, b.logger)
		if err != nil {
			b.logger.Error("failed to initialize telemetry", zap.Error(err))
		} else {
			b.logger.Info("telemetry enabled - metrics will be available on :9090/metrics")
		}
	}

	server := NewA2AServer(&b.cfg, b.logger, telemetryInstance)

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

	return server
}

// SimpleA2AServerWithAgent creates a basic A2A server with an OpenAI-compatible agent
// This is a convenience function for agent-based use cases
func SimpleA2AServerWithAgent(cfg config.Config, logger *zap.Logger, agent OpenAICompatibleAgent) A2AServer {
	return NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		Build()
}

// CustomA2AServer creates an A2A server with custom components
// This provides more control over the server configuration
func CustomA2AServer(
	cfg config.Config,
	logger *zap.Logger,
	taskHandler TaskHandler,
	taskResultProcessor TaskResultProcessor,
) A2AServer {
	return NewA2AServerBuilder(cfg, logger).
		WithTaskHandler(taskHandler).
		WithTaskResultProcessor(taskResultProcessor).
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
) A2AServer {
	if toolBox != nil {
		if defaultAgent, ok := agent.(*DefaultOpenAICompatibleAgent); ok {
			defaultAgent.toolBox = toolBox
		}
	}

	return NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithTaskResultProcessor(taskResultProcessor).
		Build()
}
