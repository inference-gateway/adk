package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	config "github.com/inference-gateway/adk/server/config"
	otel "github.com/inference-gateway/adk/server/otel"
	types "github.com/inference-gateway/adk/types"
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
	// WithBackgroundTaskHandler sets a custom task handler for polling/queue-based scenarios.
	// This handler will be used for message/send requests and background queue processing.
	WithBackgroundTaskHandler(handler TaskHandler) A2AServerBuilder

	// WithStreamingTaskHandler sets a custom task handler for streaming scenarios.
	// This handler will be used for message/stream requests.
	WithStreamingTaskHandler(handler TaskHandler) A2AServerBuilder

	// WithDefaultBackgroundTaskHandler sets a default background task handler optimized for background scenarios.
	// This handler automatically handles input-required pausing without requiring custom implementation.
	WithDefaultBackgroundTaskHandler() A2AServerBuilder // WithDefaultStreamingTaskHandler sets a default streaming task handler optimized for streaming scenarios.

	// This handler automatically handles input-required pausing with streaming-aware behavior.
	WithDefaultStreamingTaskHandler() A2AServerBuilder

	// WithDefaultTaskHandlers sets both default polling and streaming task handlers.
	// This is a convenience method that sets up optimized handlers for both scenarios.
	WithDefaultTaskHandlers() A2AServerBuilder

	// WithTaskResultProcessor sets a custom task result processor for handling tool call results.
	// This allows custom business logic for determining when tasks should be completed.
	WithTaskResultProcessor(processor TaskResultProcessor) A2AServerBuilder

	// WithAgent sets a pre-configured OpenAI-compatible agent for processing tasks.
	// This is useful when you have already configured an agent with specific settings.
	WithAgent(agent OpenAICompatibleAgent) A2AServerBuilder

	// WithAgentCard sets a custom agent card that overrides the default card generation.
	// This gives full control over the agent's advertised capabilities and metadata.
	WithAgentCard(agentCard types.AgentCard) A2AServerBuilder

	// WithAgentCardFromFile loads and sets an agent card from a JSON file.
	// This provides a convenient way to load agent configuration from a static file.
	// The optional overrides map allows dynamic replacement of JSON attribute values.
	WithAgentCardFromFile(filePath string, overrides map[string]interface{}) A2AServerBuilder

	// WithSecurityConfiguredAgentCard sets an agent card and automatically configures security
	// based on the server's authentication configuration.
	WithSecurityConfiguredAgentCard(agentCard types.AgentCard) A2AServerBuilder

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
	cfg                  config.Config         // Base configuration for the server
	logger               *zap.Logger           // Logger instance for the server
	pollingTaskHandler   TaskHandler           // Optional custom task handler for polling scenarios
	streamingTaskHandler TaskHandler           // Optional custom task handler for streaming scenarios
	taskResultProcessor  TaskResultProcessor   // Optional custom task result processor
	agent                OpenAICompatibleAgent // Optional pre-configured agent
	agentCard            *types.AgentCard      // Optional custom agent card
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

// WithBackgroundTaskHandler sets a custom task handler for polling/queue-based scenarios
func (b *A2AServerBuilderImpl) WithBackgroundTaskHandler(handler TaskHandler) A2AServerBuilder {
	b.pollingTaskHandler = handler
	return b
}

// WithStreamingTaskHandler sets a custom task handler for streaming scenarios
func (b *A2AServerBuilderImpl) WithStreamingTaskHandler(handler TaskHandler) A2AServerBuilder {
	b.streamingTaskHandler = handler
	return b
}

// WithDefaultBackgroundTaskHandler sets a default background task handler optimized for background scenarios
func (b *A2AServerBuilderImpl) WithDefaultBackgroundTaskHandler() A2AServerBuilder {
	b.pollingTaskHandler = NewDefaultBackgroundTaskHandler(b.logger, b.agent)
	return b
}

// WithDefaultStreamingTaskHandler sets a default streaming task handler optimized for streaming scenarios
func (b *A2AServerBuilderImpl) WithDefaultStreamingTaskHandler() A2AServerBuilder {
	b.streamingTaskHandler = NewDefaultStreamingTaskHandler(b.logger, b.agent)
	return b
}

// WithDefaultTaskHandlers sets both default background and streaming task handlers
func (b *A2AServerBuilderImpl) WithDefaultTaskHandlers() A2AServerBuilder {
	b.pollingTaskHandler = NewDefaultBackgroundTaskHandler(b.logger, b.agent)
	b.streamingTaskHandler = NewDefaultStreamingTaskHandler(b.logger, b.agent)
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
func (b *A2AServerBuilderImpl) WithAgentCard(agentCard types.AgentCard) A2AServerBuilder {
	b.agentCard = &agentCard
	return b
}

// WithSecurityConfiguredAgentCard sets an agent card and automatically configures security
func (b *A2AServerBuilderImpl) WithSecurityConfiguredAgentCard(agentCard types.AgentCard) A2AServerBuilder {
	// Create security configuration from auth config
	securityConfig := CreateSecurityConfigFromAuthConfig(b.cfg.AuthConfig)

	// Configure security in the agent card
	ConfigureAgentCardSecurity(&agentCard, securityConfig)

	b.agentCard = &agentCard
	return b
}

// WithAgentCardFromFile loads and sets an agent card from a JSON file
// The optional overrides map allows dynamic replacement of JSON attribute values
func (b *A2AServerBuilderImpl) WithAgentCardFromFile(filePath string, overrides map[string]interface{}) A2AServerBuilder {
	if filePath == "" {
		return b
	}

	b.logger.Info("loading agent card from file", zap.String("file_path", filePath))

	data, err := os.ReadFile(filePath)
	if err != nil {
		b.logger.Error("failed to read agent card file", zap.String("file_path", filePath), zap.Error(err))
		return b
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		b.logger.Error("failed to parse agent card JSON", zap.String("file_path", filePath), zap.Error(err))
		return b
	}

	for key, value := range overrides {
		b.logger.Debug("overriding agent card attribute",
			zap.String("key", key),
			zap.Any("value", value))
		rawData[key] = value
	}

	modifiedData, err := json.Marshal(rawData)
	if err != nil {
		b.logger.Error("failed to marshal modified agent card data", zap.String("file_path", filePath), zap.Error(err))
		return b
	}

	var agentCard types.AgentCard
	if err := json.Unmarshal(modifiedData, &agentCard); err != nil {
		b.logger.Error("failed to parse modified agent card JSON", zap.String("file_path", filePath), zap.Error(err))
		return b
	}

	b.logger.Info("successfully loaded agent card from file",
		zap.String("name", agentCard.Name),
		zap.String("version", agentCard.Version),
		zap.Int("overrides_count", len(overrides)))
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
	if b.agentCard == nil {
		return nil, fmt.Errorf("agent card must be configured before building the server - use WithAgentCard() or WithAgentCardFromFile()")
	}

	if err := b.validateTaskHandlerConfiguration(); err != nil {
		return nil, err
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
		b.logger.Info("configured openai-compatible agent for optional use by task handler")
	}

	if b.pollingTaskHandler != nil {
		server.SetBackgroundTaskHandler(b.pollingTaskHandler)
	}

	if b.streamingTaskHandler != nil {
		server.SetStreamingTaskHandler(b.streamingTaskHandler)
	}

	if b.taskResultProcessor != nil {
		server.SetTaskResultProcessor(b.taskResultProcessor)
	}

	if b.agentCard != nil {
		server.SetAgentCard(*b.agentCard)
	}

	return server, nil
}

// validateTaskHandlerConfiguration ensures task handlers are configured based on agent card capabilities
func (b *A2AServerBuilderImpl) validateTaskHandlerConfiguration() error {
	streamingEnabled := false
	if b.agentCard.Capabilities.Streaming != nil {
		streamingEnabled = *b.agentCard.Capabilities.Streaming
	}

	if b.pollingTaskHandler == nil {
		return fmt.Errorf("background task handler must be configured - use WithBackgroundTaskHandler() for custom handler or WithDefaultBackgroundTaskHandler() for a ready-to-use default handler")
	}

	if streamingEnabled && b.streamingTaskHandler == nil {
		return fmt.Errorf("streaming task handler must be configured when streaming is enabled in agent capabilities - use WithStreamingTaskHandler() for custom handler or WithDefaultStreamingTaskHandler() for a ready-to-use default handler")
	}

	return nil
}

// SimpleA2AServerWithAgent creates a basic A2A server with an OpenAI-compatible agent
// This is a convenience function for agent-based use cases
func SimpleA2AServerWithAgent(cfg config.Config, logger *zap.Logger, agent OpenAICompatibleAgent, agentCard types.AgentCard) (A2AServer, error) {
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
	pollingTaskHandler TaskHandler,
	streamingTaskHandler TaskHandler,
	taskResultProcessor TaskResultProcessor,
	agentCard types.AgentCard,
) (A2AServer, error) {
	return NewA2AServerBuilder(cfg, logger).
		WithBackgroundTaskHandler(pollingTaskHandler).
		WithStreamingTaskHandler(streamingTaskHandler).
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
	agentCard types.AgentCard,
) (A2AServer, error) {
	if toolBox != nil {
		if agentImpl, ok := agent.(*OpenAICompatibleAgentImpl); ok {
			agentImpl.SetToolBox(toolBox)
		}
	}

	return NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithTaskResultProcessor(taskResultProcessor).
		WithAgentCard(agentCard).
		Build()
}
