package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"
)

func main() {
	fmt.Println("🤖 Starting AI-Powered A2A Server...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Load configuration from environment
	// Agent metadata is injected at build time via LD flags
	// Use: go build -ldflags="-X github.com/inference-gateway/adk/server.BuildAgentName=my-agent ..."
	cfg := config.Config{
		AgentName:        server.BuildAgentName,
		AgentDescription: server.BuildAgentDescription,
		AgentVersion:     server.BuildAgentVersion,
		CapabilitiesConfig: config.CapabilitiesConfig{
			Streaming:              true,
			PushNotifications:      false,
			StateTransitionHistory: false,
		},
		QueueConfig: config.QueueConfig{
			CleanupInterval: 5 * time.Minute,
		},
		ServerConfig: config.ServerConfig{
			Port: "8080",
		},
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, &cfg); err != nil {
		logger.Fatal("failed to process environment config", zap.Error(err))
	}

	// Create toolbox with sample tools
	toolBox := server.NewDefaultToolBox()

	// Add weather tool
	weatherTool := server.NewBasicTool(
		"get_weather",
		"Get current weather information for a location",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city name",
				},
			},
			"required": []string{"location"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			location := args["location"].(string)
			return fmt.Sprintf(`{"location": "%s", "temperature": "22°C", "condition": "sunny", "humidity": "65%%"}`, location), nil
		},
	)
	toolBox.AddTool(weatherTool)

	// Add time tool
	timeTool := server.NewBasicTool(
		"get_current_time",
		"Get the current date and time",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			now := time.Now()
			return fmt.Sprintf(`{"current_time": "%s", "timezone": "%s"}`,
				now.Format("2006-01-02 15:04:05"), now.Location()), nil
		},
	)
	toolBox.AddTool(timeTool)

	// Create AI agent with LLM client
	llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.AgentConfig, logger)
	if err != nil {
		logger.Fatal("failed to create LLM client", zap.Error(err))
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(&cfg.AgentConfig).
		WithLLMClient(llmClient).
		WithSystemPrompt("You are a helpful AI assistant. Be concise and friendly in your responses.").
		WithMaxChatCompletion(10).
		WithToolBox(toolBox).
		Build()
	if err != nil {
		logger.Fatal("failed to create AI agent", zap.Error(err))
	}

	// Create and start server with default background task handler
	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithDefaultTaskHandlers().
		WithAgentCard(types.AgentCard{
			Name:        cfg.AgentName,
			Description: cfg.AgentDescription,
			URL:         cfg.AgentURL,
			Version:     cfg.AgentVersion,
			Capabilities: types.AgentCapabilities{
				Streaming:              &cfg.CapabilitiesConfig.Streaming,
				PushNotifications:      &cfg.CapabilitiesConfig.PushNotifications,
				StateTransitionHistory: &cfg.CapabilitiesConfig.StateTransitionHistory,
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
		}).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	logger.Info("✅ AI-powered A2A server created",
		zap.String("provider", cfg.AgentConfig.Provider),
		zap.String("model", cfg.AgentConfig.Model),
		zap.String("tools", "weather, time"))

	// Display agent metadata (from build-time LD flags)
	logger.Info("🤖 agent metadata",
		zap.String("name", server.BuildAgentName),
		zap.String("description", server.BuildAgentDescription),
		zap.String("version", server.BuildAgentVersion))

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	// Wait for shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	} else {
		logger.Info("✅ goodbye!")
	}
}
