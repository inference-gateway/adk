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
	envconfig "github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
)

// AI-Powered A2A Server Example
//
// This example demonstrates how to create an A2A server with AI capabilities.
// It supports multiple LLM providers through environment variables.
//
// Configuration Environment Variables:
//
//	AGENT_CLIENT_API_KEY     - Required for AI features (your LLM provider API key)
//	AGENT_CLIENT_PROVIDER    - LLM provider: "openai", "anthropic", "deepseek", "ollama" (default: "openai")
//	AGENT_CLIENT_MODEL       - Model name (defaults based on provider):
//	                           - OpenAI: "gpt-4"
//	                           - Anthropic: "claude-3-5-sonnet-20241022"
//	                           - DeepSeek: "deepseek-chat"
//	                           - Ollama: "llama3.2"
//	AGENT_CLIENT_BASE_URL    - API endpoint URL (custom deployments, inference gateway, etc.)
//	PORT                     - Server port (default: "8080")
//
// Examples:
//
//	# Using inference gateway (recommended)
//	export AGENT_CLIENT_API_KEY="your-key" AGENT_CLIENT_BASE_URL="http://localhost:3000/v1" && go run cmd/aipowered/main.go
//
//	# Direct OpenAI connection
//	export AGENT_CLIENT_API_KEY="sk-..." && go run cmd/aipowered/main.go
//
//	# Direct Anthropic connection
//	export AGENT_CLIENT_API_KEY="sk-ant-..." AGENT_CLIENT_PROVIDER="anthropic" && go run cmd/aipowered/main.go
//
//	# Local Ollama
//	export AGENT_CLIENT_PROVIDER="ollama" AGENT_CLIENT_MODEL="llama3.2" AGENT_CLIENT_BASE_URL="http://localhost:11434/v1" && go run cmd/aipowered/main.go
//
//	# Mock mode (no AI)
//	go run cmd/aipowered/main.go
func main() {
	fmt.Println("🤖 Starting AI-Powered A2A Server Example...")

	// Step 1: Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Step 2: Load configuration from environment
	cfg := config.Config{
		AgentName:        "AI-Powered Assistant",
		AgentDescription: "An AI assistant with weather and time tools using configurable LLM providers",
		Port:             "8080",
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, &cfg); err != nil {
		logger.Fatal("Failed to process environment config", zap.Error(err))
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
	if cfg.AgentConfig != nil && cfg.AgentConfig.APIKey != "" {
		// With LLM - use the AgentConfig from environment
		llmClient, err := server.NewOpenAICompatibleLLMClient(cfg.AgentConfig, logger)
		if err != nil {
			logger.Fatal("Failed to create LLM client", zap.Error(err))
		}
		agent := server.NewOpenAICompatibleAgentWithLLM(logger, llmClient)
		a2aServer = server.SimpleA2AServerWithAgent(cfg, logger, agent)
		logger.Info("✅ Server created with AI capabilities",
			zap.String("provider", cfg.AgentConfig.Provider),
			zap.String("model", cfg.AgentConfig.Model),
			zap.String("base_url", cfg.AgentConfig.BaseURL))
	} else {
		// Without LLM - manual setup for mock mode
		agent := server.NewDefaultOpenAICompatibleAgent(logger)
		agent.SetSystemPrompt("You are a helpful assistant with access to tools.")
		agent.SetToolBox(toolBox)

		a2aServer = server.NewA2AServerBuilder(cfg, logger).
			WithAgent(agent).
			Build()
		logger.Info("✅ Server created in mock mode (set AGENT_CLIENT_API_KEY for AI features)")
		logger.Info("💡 To enable AI: export AGENT_CLIENT_API_KEY='your-key' [AGENT_CLIENT_PROVIDER='openai|anthropic|deepseek'] [AGENT_CLIENT_MODEL='model-name']")
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
	fmt.Printf("\n🎯 Try this curl command:\n")
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
            "content": "What'\''s the weather in Paris?"
          }
        ]
      }
    },
    "id": 1
  }'
`, cfg.Port)

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
