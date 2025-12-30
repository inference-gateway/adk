package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/adk/examples/usage-metadata/server/config"
)

// Usage Metadata Example A2A Server
//
// This example demonstrates automatic token usage and execution metrics tracking
// in A2A task responses. The server tracks LLM token consumption and execution
// statistics, then populates the Task.Metadata field with this information.
//
// Configuration via environment variables:
//   - ENVIRONMENT: Runtime environment (default: development)
//   - A2A_AGENT_NAME: Agent name (default: usage-metadata-agent)
//   - A2A_SERVER_PORT: Server port (default: 8080)
//   - A2A_DEBUG: Enable debug logging (default: false)
//   - A2A_AGENT_CLIENT_PROVIDER: LLM provider (required)
//   - A2A_AGENT_CLIENT_MODEL: LLM model (required)
//   - A2A_AGENT_CLIENT_ENABLE_USAGE_METADATA: Enable usage tracking (default: true)
//
// To run: go run main.go
func main() {
	// Create configuration with defaults
	cfg := &config.Config{
		Environment: "development",
		A2A: serverConfig.Config{
			AgentName:        server.BuildAgentName,
			AgentDescription: server.BuildAgentDescription,
			AgentVersion:     server.BuildAgentVersion,
			Debug:            false,
			CapabilitiesConfig: serverConfig.CapabilitiesConfig{
				Streaming:              true,
				PushNotifications:      false,
				StateTransitionHistory: false,
			},
			QueueConfig: serverConfig.QueueConfig{
				CleanupInterval: 5 * time.Minute,
			},
			ServerConfig: serverConfig.ServerConfig{
				Port: "8080",
			},
			AgentConfig: serverConfig.AgentConfig{
				// Usage metadata enabled by default
				EnableUsageMetadata: true,
			},
		},
	}

	// Load configuration from environment variables
	ctx := context.Background()
	if err := envconfig.Process(ctx, cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize logger based on environment
	var logger *zap.Logger
	var err error
	if cfg.Environment == "development" || cfg.Environment == "dev" || cfg.A2A.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	logger.Info("usage metadata example server starting",
		zap.String("environment", cfg.Environment),
		zap.String("agent_name", cfg.A2A.AgentName),
		zap.String("port", cfg.A2A.ServerConfig.Port),
		zap.Bool("debug", cfg.A2A.Debug),
		zap.String("provider", cfg.A2A.AgentConfig.Provider),
		zap.String("model", cfg.A2A.AgentConfig.Model),
		zap.Bool("usage_metadata_enabled", cfg.A2A.AgentConfig.EnableUsageMetadata),
	)

	// Validate required configuration
	if cfg.A2A.AgentConfig.Provider == "" {
		logger.Fatal("A2A_AGENT_CLIENT_PROVIDER is required")
	}
	if cfg.A2A.AgentConfig.Model == "" {
		logger.Fatal("A2A_AGENT_CLIENT_MODEL is required")
	}

	// Create toolbox with a simple calculation tool to demonstrate tool tracking
	toolBox := server.NewDefaultToolBox(&cfg.A2A.AgentConfig.ToolBoxConfig)

	// Add calculator tool to demonstrate tool call tracking
	calculatorTool := server.NewBasicTool(
		"calculate",
		"Perform basic arithmetic calculations",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"operation": map[string]any{
					"type":        "string",
					"description": "The operation to perform (add, subtract, multiply, divide)",
					"enum":        []string{"add", "subtract", "multiply", "divide"},
				},
				"a": map[string]any{
					"type":        "number",
					"description": "First number",
				},
				"b": map[string]any{
					"type":        "number",
					"description": "Second number",
				},
			},
			"required": []string{"operation", "a", "b"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			operation := args["operation"].(string)
			a := args["a"].(float64)
			b := args["b"].(float64)

			var result float64
			switch operation {
			case "add":
				result = a + b
			case "subtract":
				result = a - b
			case "multiply":
				result = a * b
			case "divide":
				if b == 0 {
					return "", fmt.Errorf("division by zero")
				}
				result = a / b
			default:
				return "", fmt.Errorf("unknown operation: %s", operation)
			}

			return fmt.Sprintf(`{"result": %.2f}`, result), nil
		},
	)
	toolBox.AddTool(calculatorTool)

	// Create AI agent with LLM client
	llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.A2A.AgentConfig, logger)
	if err != nil {
		logger.Fatal("failed to create LLM client", zap.Error(err))
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(&cfg.A2A.AgentConfig).
		WithLLMClient(llmClient).
		WithSystemPrompt("You are a helpful AI assistant that can perform calculations. When asked to do math, use the calculate tool. Be concise and clear in your responses.").
		WithMaxChatCompletion(10).
		WithToolBox(toolBox).
		Build()
	if err != nil {
		logger.Fatal("failed to create AI agent", zap.Error(err))
	}

	// Build and start server with default handlers
	// The default handlers automatically track and populate usage metadata
	agentURL := fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port)
	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithDefaultBackgroundTaskHandler().
		WithDefaultStreamingTaskHandler().
		WithAgent(agent).
		WithAgentCard(types.AgentCard{
			Name:            cfg.A2A.AgentName,
			Description:     cfg.A2A.AgentDescription,
			Version:         cfg.A2A.AgentVersion,
			URL:             &agentURL,
			ProtocolVersion: "0.3.0",
			Capabilities: types.AgentCapabilities{
				Streaming:              &cfg.A2A.CapabilitiesConfig.Streaming,
				PushNotifications:      &cfg.A2A.CapabilitiesConfig.PushNotifications,
				StateTransitionHistory: &cfg.A2A.CapabilitiesConfig.StateTransitionHistory,
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills: []types.AgentSkill{
				{
					Name:        "calculate",
					Description: "Perform basic arithmetic calculations",
				},
			},
		}).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("server running - usage metadata will be included in task responses",
		zap.String("port", cfg.A2A.ServerConfig.Port),
		zap.Bool("usage_metadata_enabled", cfg.A2A.AgentConfig.EnableUsageMetadata),
	)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
