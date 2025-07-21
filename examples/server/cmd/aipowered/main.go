package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inference-gateway/a2a/adk"
	server "github.com/inference-gateway/a2a/adk/server"
	config "github.com/inference-gateway/a2a/adk/server/config"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"
)

func main() {
	fmt.Println("🤖 Starting AI-Powered A2A Server...")

	// Step 1: Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Step 2: Check for required API key
	apiKey := os.Getenv("AGENT_CLIENT_API_KEY")
	if apiKey == "" {
		fmt.Println("\n❌ ERROR: AI provider configuration required!")
		fmt.Println("\n🔑 Please set AGENT_CLIENT_API_KEY environment variable:")
		fmt.Println("\n📋 Examples:")
		fmt.Println("  # OpenAI")
		fmt.Println("  export AGENT_CLIENT_API_KEY=\"sk-...\"")
		fmt.Println("\n  # Anthropic")
		fmt.Println("  export AGENT_CLIENT_API_KEY=\"sk-ant-...\"")
		fmt.Println("  export AGENT_CLIENT_PROVIDER=\"anthropic\"")
		fmt.Println("\n  # Via Inference Gateway")
		fmt.Println("  export AGENT_CLIENT_API_KEY=\"your-key\"")
		fmt.Println("  export AGENT_CLIENT_BASE_URL=\"http://localhost:3000/v1\"")
		fmt.Println("\n💡 For a server without AI, use the minimal example instead:")
		fmt.Println("  go run ../minimal/main.go")
		os.Exit(1)
	}

	// Step 3: Load configuration from environment
	// Agent metadata is injected at build time via LD flags
	// Use: go build -ldflags="-X github.com/inference-gateway/a2a/adk/server.BuildAgentName=my-agent ..."
	cfg := config.Config{
		AgentName:        server.BuildAgentName,
		AgentDescription: server.BuildAgentDescription,
		AgentVersion:     server.BuildAgentVersion,
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

	// Step 4: Create toolbox with sample tools
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

	// Step 5: Create AI agent with LLM client
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

	// Step 6: Create and start server
	a2aServer, err := server.SimpleA2AServerWithAgent(cfg, logger, agent, adk.AgentCard{
		Name:        cfg.AgentName,
		Description: cfg.AgentDescription,
		URL:         cfg.AgentURL,
		Version:     cfg.AgentVersion,
		Capabilities: adk.AgentCapabilities{
			Streaming:              &cfg.CapabilitiesConfig.Streaming,
			PushNotifications:      &cfg.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: &cfg.CapabilitiesConfig.StateTransitionHistory,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
	})

	// Alternative: Use NewA2AServerBuilder with JSON AgentCard file
	// You can also load an agent card from a JSON file with optional overrides:
	//
	// Example 1: Load without overrides
	// a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
	//	WithAgent(agent).
	//	WithAgentCardFromFile(os.Getenv("AGENT_CARD_FILE_PATH"), nil).
	//	Build()
	//
	// Example 2: Load with runtime overrides (useful for deployment environments)
	// overrides := map[string]interface{}{
	//	"name": cfg.AgentName,           // Override with environment-specific name
	//	"version": cfg.AgentVersion,     // Override with build-time version
	//	"url": cfg.AgentURL,             // Override with deployment URL
	//	"capabilities": map[string]interface{}{
	//		"streaming": cfg.CapabilitiesConfig.Streaming,
	//		"pushNotifications": cfg.CapabilitiesConfig.PushNotifications,
	//	},
	// }
	// a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
	//	WithAgent(agent).
	//	WithAgentCardFromFile("./.well-known/agent.json", overrides).
	//	Build()
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

	logger.Info("🌐 server running", zap.String("port", cfg.ServerConfig.Port))
	fmt.Printf("\n🎯 Test with curl:\n")
	fmt.Printf(`curl -X POST http://localhost:%s/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "params": {
      "message": {
        "kind": "message",
        "messageId": "msg-123",
        "role": "user",
        "parts": [
          {
            "kind": "text",
            "text": "What'\''s the weather in Tokyo?"
          }
        ]
      }
    },
    "id": 1
  }'`, cfg.ServerConfig.Port)
	fmt.Println()

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
