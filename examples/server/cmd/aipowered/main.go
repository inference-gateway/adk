package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inference-gateway/a2a/adk/server"
	"github.com/inference-gateway/a2a/adk/server/config"
	"go.uber.org/zap"
)

// This is an alternative minimal example showing the simplest possible setup
// To run this instead of main.go: go run minimal_example.go
func main() {
	fmt.Println("🤖 Starting Minimal A2A Server Example...")

	// Step 1: Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Step 2: Create basic configuration
	cfg := config.Config{
		AgentName:        "Simple AI Assistant",
		AgentDescription: "A basic AI assistant with weather and time tools",
		Port:             "8080", // Default port
	}

	// Step 3: Create a simple toolbox
	toolBox := server.NewDefaultToolBox()

	// Add a simple weather tool
	weatherTool := server.NewBasicTool(
		"get_weather",
		"Get weather information for a location",
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
			return fmt.Sprintf(`{"location": "%s", "temperature": "20°C", "condition": "sunny"}`, location), nil
		},
	)
	toolBox.AddTool(weatherTool)

	// Step 4: Check for LLM configuration
	var a2aServer server.A2AServer
	if apiKey := os.Getenv("LLM_API_KEY"); apiKey != "" {
		// With LLM - use convenience function
		llmConfig := &config.LLMProviderClientConfig{
			Provider: "openai",
			Model:    "gpt-4",
			APIKey:   apiKey,
		}
		a2aServer = server.SimpleA2AServerWithAgent(cfg, logger, llmConfig, toolBox)
		logger.Info("✅ Server created with AI capabilities")
	} else {
		// Without LLM - manual setup for mock mode
		agent := server.NewDefaultOpenAICompatibleAgent(logger)
		agent.SetSystemPrompt("You are a helpful assistant with access to tools.")
		agent.SetToolBox(toolBox)

		a2aServer = server.NewA2AServerBuilder(cfg, logger).
			WithAIPoweredAgent(agent).
			Build()
		logger.Info("✅ Server created in mock mode (set LLM_API_KEY for AI features)")
	}

	// Step 5: Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	logger.Info("🌐 Server running", zap.String("port", cfg.Port))
	fmt.Printf("\n🎯 Try: POST http://localhost:%s/a2a\n", cfg.Port)
	fmt.Println("📝 Example request body:")
	fmt.Println(`{
  "jsonrpc": "2.0",
  "method": "message/send",
  "params": {
    "message": {
      "role": "user",
      "content": "What's the weather in Paris?"
    }
  },
  "id": 1
}`)

	// Step 6: Wait for shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	} else {
		logger.Info("✅ Goodbye!")
	}
}
