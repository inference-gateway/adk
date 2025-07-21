package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	server "github.com/inference-gateway/a2a/adk/server"
	config "github.com/inference-gateway/a2a/adk/server/config"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"
)

// A2A Server with JSON AgentCard Example
//
// This example demonstrates how to create an A2A server that loads
// its agent metadata from a JSON file instead of code-based configuration.
//
// REQUIRED Configuration:
//
//	AGENT_CARD_FILE_PATH - Path to the JSON AgentCard file (default: "./.well-known/agent.json")
//
// Optional Configuration:
//
//	PORT                  - Server port (default: "8080")
//	AGENT_CLIENT_API_KEY  - Your LLM provider API key for AI capabilities
//	AGENT_CLIENT_PROVIDER - LLM provider: "openai", "anthropic", etc.
//
// Examples:
//
//	# Use default agent card file
//	go run main.go
//
//	# Use custom agent card file
//	AGENT_CARD_FILE_PATH="/path/to/my-card.json" go run main.go
//
//	# With AI capabilities
//	export AGENT_CLIENT_API_KEY="sk-..." AGENT_CARD_FILE_PATH="./.well-known/agent.json" && go run main.go
//
// To run: go run main.go
func main() {
	fmt.Println("🤖 Starting A2A Server with JSON AgentCard...")

	// Step 1: Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Step 2: Load configuration from environment
	cfg := config.Config{
		QueueConfig: config.QueueConfig{
			CleanupInterval: 5 * time.Minute,
		},
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, &cfg); err != nil {
		logger.Fatal("failed to process environment config", zap.Error(err))
	}

	// Step 3: Create basic toolbox (you can customize this)
	toolBox := server.NewDefaultToolBox()

	// Add weather tool for demo
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
			return fmt.Sprintf(`{"location": "%s", "temperature": "22°C", "condition": "sunny"}`, location), nil
		},
	)
	toolBox.AddTool(weatherTool)

	// Step 4: Create agent (with or without AI depending on API key)
	var agent server.OpenAICompatibleAgent
	apiKey := os.Getenv("AGENT_CLIENT_API_KEY")

	if apiKey != "" {
		// Create AI-powered agent
		llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.AgentConfig, logger)
		if err != nil {
			logger.Fatal("failed to create LLM client", zap.Error(err))
		}

		agent, err = server.NewAgentBuilder(logger).
			WithConfig(&cfg.AgentConfig).
			WithLLMClient(llmClient).
			WithSystemPrompt("You are a helpful weather assistant. Provide accurate and friendly weather information.").
			WithToolBox(toolBox).
			Build()
		if err != nil {
			logger.Fatal("failed to create AI agent", zap.Error(err))
		}
		logger.Info("✅ AI-powered agent created")
	} else {
		// Create mock agent
		agent, err = server.NewAgentBuilder(logger).
			WithToolBox(toolBox).
			Build()
		if err != nil {
			logger.Fatal("failed to create agent", zap.Error(err))
		}
		logger.Info("✅ mock agent created (set AGENT_CLIENT_API_KEY for AI features)")
	}

	// Step 5: Create server with AgentCard from JSON file
	a2aServer := server.NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithAgentCardFromFile(os.Getenv("AGENT_CARD_FILE_PATH")). // Defaults to empty, auto-loads if set
		Build()

	// Alternative: You can also load the agent card explicitly
	// if cardPath := os.Getenv("AGENT_CARD_FILE_PATH"); cardPath != "" {
	//     if err := a2aServer.LoadAgentCardFromFile(cardPath); err != nil {
	//         logger.Warn("failed to load agent card from file", zap.String("path", cardPath), zap.Error(err))
	//     } else {
	//         logger.Info("✅ loaded agent card from file", zap.String("path", cardPath))
	//     }
	// }

	logger.Info("✅ A2A server created with JSON AgentCard support")

	// Display the loaded agent card
	agentCard := a2aServer.GetAgentCard()
	if agentCard != nil {
		logger.Info("🤖 agent card loaded",
			zap.String("name", agentCard.Name),
			zap.String("description", agentCard.Description),
			zap.String("version", agentCard.Version),
			zap.Int("skills", len(agentCard.Skills)))
	} else {
		logger.Warn("⚠️ no agent card loaded - server will return 503 for agent info requests")
	}

	// Step 6: Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("🌐 server running", zap.String("port", cfg.ServerConfig.Port))

	// Show example usage
	fmt.Printf("\n🎯 Test the agent card endpoint:\n")
	fmt.Printf("curl http://localhost:%s/.well-known/agent.json | jq .\n", cfg.ServerConfig.Port)

	fmt.Printf("\n🎯 Test message sending:\n")
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
        "parts": [{"kind": "text", "text": "What'\''s the weather in Tokyo?"}]
      }
    },
    "id": 1
  }'`, cfg.ServerConfig.Port)
	fmt.Println()

	// Show environment variables
	cardPath := os.Getenv("AGENT_CARD_FILE_PATH")
	if cardPath == "" {
		fmt.Printf("\n💡 Set AGENT_CARD_FILE_PATH to load a custom agent card:\n")
		fmt.Printf("  AGENT_CARD_FILE_PATH='./.well-known/agent.json' go run main.go\n")
	} else {
		fmt.Printf("\n📄 Using agent card file: %s\n", cardPath)
	}

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
