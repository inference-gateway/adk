package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	adk "github.com/inference-gateway/a2a/adk"
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

	// WithAgentCard sets a custom agent card that overrides the default card generation.
	// This gives full control over the agent's advertised capabilities and metadata.
	WithAgentCard(agentCard adk.AgentCard) A2AServerBuilder

	// WithAgentCardFromFile loads and sets an agent card from a JSON file.
	// This provides a convenient way to load agent configuration from a static file.
	WithAgentCardFromFile(filePath string) A2AServerBuilder

	// WithLogger sets a custom logger for the builder and resulting server.
	// This allows using a logger configured with appropriate level based on the Debug config.
	WithLogger(logger *zap.Logger) A2AServerBuilder

	// Build creates and returns the configured A2A server.
	// This method applies configuration defaults and initializes all components.
	Build() (A2AServer, error)
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
	agentCard           *adk.AgentCard        // Optional custom agent card
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
	needsDefaults := isCapabilitiesConfigEmpty(cfg.CapabilitiesConfig) || isAgentConfigEmpty(cfg.AgentConfig)

	if needsDefaults {
		defaultCfg, err := config.NewWithDefaults(context.Background(), nil)
		if err == nil {
			if isCapabilitiesConfigEmpty(cfg.CapabilitiesConfig) {
				cfg.CapabilitiesConfig = defaultCfg.CapabilitiesConfig
			}
			if isAgentConfigEmpty(cfg.AgentConfig) {
				cfg.AgentConfig = defaultCfg.AgentConfig
			}
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

// isAgentConfigEmpty checks if the agent config has all zero values (needs defaults)
func isAgentConfigEmpty(agentConfig config.AgentConfig) bool {
	return agentConfig.Provider == "" &&
		agentConfig.Model == "" &&
		agentConfig.MaxConversationHistory == 0 &&
		agentConfig.MaxChatCompletionIterations == 0 &&
		agentConfig.Timeout == 0 &&
		agentConfig.MaxRetries == 0 &&
		agentConfig.MaxTokens == 0 &&
		agentConfig.Temperature == 0 &&
		agentConfig.TopP == 0 &&
		agentConfig.SystemPrompt == ""
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

// WithAgentCard sets a custom agent card that overrides the default card generation
func (b *A2AServerBuilderImpl) WithAgentCard(agentCard adk.AgentCard) A2AServerBuilder {
	b.agentCard = &agentCard
	return b
}

// WithAgentCardFromFile loads and sets an agent card from a JSON file
func (b *A2AServerBuilderImpl) WithAgentCardFromFile(filePath string) A2AServerBuilder {
	if filePath == "" {
		return b
	}

	b.logger.Info("loading agent card from file", zap.String("file_path", filePath))

	data, err := os.ReadFile(filePath)
	if err != nil {
		b.logger.Error("failed to read agent card file", zap.String("file_path", filePath), zap.Error(err))
		return b
	}

	var agentCard adk.AgentCard
	if err := json.Unmarshal(data, &agentCard); err != nil {
		b.logger.Error("failed to parse agent card JSON", zap.String("file_path", filePath), zap.Error(err))
		return b
	}

	b.logger.Info("successfully loaded agent card from file", zap.String("name", agentCard.Name), zap.String("version", agentCard.Version))
	b.agentCard = &agentCard
	return b
}

// WithLogger sets a custom logger for the builder
func (b *A2AServerBuilderImpl) WithLogger(logger *zap.Logger) A2AServerBuilder {
	b.logger = logger
	return b
}

// Build creates and returns the configured A2A server.
func (b *A2AServerBuilderImpl) Build() (A2AServer, error) {
	// Validate that an agent card is configured
	if b.agentCard == nil {
		return nil, fmt.Errorf("agent card must be configured before building the server - use WithAgentCard() or WithAgentCardFromFile()")
	}

	var telemetryInstance otel.OpenTelemetry
	if b.cfg.TelemetryConfig.Enable {
		var err error
		telemetryInstance, err = otel.NewOpenTelemetry(&b.cfg, b.logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize telemetry: %w", err)
		}
		metricsAddr := b.cfg.TelemetryConfig.MetricsConfig.Host + ":" + b.cfg.TelemetryConfig.MetricsConfig.Port
		b.logger.Info("telemetry enabled - metrics will be available", zap.String("metrics_url", metricsAddr+"/metrics"))
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

	if b.agentCard != nil {
		server.SetAgentCard(*b.agentCard)
	}

	return server, nil
}

// SimpleA2AServerWithAgent creates a basic A2A server with an OpenAI-compatible agent
// This is a convenience function for agent-based use cases
func SimpleA2AServerWithAgent(cfg config.Config, logger *zap.Logger, agent OpenAICompatibleAgent, agentCard adk.AgentCard) (A2AServer, error) {
	return NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithAgentCard(agentCard).
		Build()
}

// CustomA2AServer creates an A2A server with custom components
// This provides more control over the server configuration
func CustomA2AServer(
	cfg config.Config,
	logger *zap.Logger,
	taskHandler TaskHandler,
	taskResultProcessor TaskResultProcessor,
	agentCard adk.AgentCard,
) (A2AServer, error) {
	return NewA2AServerBuilder(cfg, logger).
		WithTaskHandler(taskHandler).
		WithTaskResultProcessor(taskResultProcessor).
		WithAgentCard(agentCard).
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
	agentCard adk.AgentCard,
) (A2AServer, error) {
	if toolBox != nil {
		if defaultAgent, ok := agent.(*DefaultOpenAICompatibleAgent); ok {
			defaultAgent.toolBox = toolBox
		}
	}

	return NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithTaskResultProcessor(taskResultProcessor).
		WithAgentCard(agentCard).
		Build()
}
