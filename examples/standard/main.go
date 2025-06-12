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
	"go.uber.org/zap"
)

func main() {
	fmt.Println("🚀 Running Standard A2A Server Example")

	// Create a basic logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Create basic configuration
	cfg := server.Config{
		AgentName:        "standard-example-agent",
		AgentDescription: "A simple example A2A agent demonstrating basic functionality",
		AgentURL:         "http://localhost:8080",
		AgentVersion:     "1.0.0",
		Port:             "8080",
		Debug:            true,
		// Optional: Configure LLM provider client settings
		// LLMProviderClientConfig: &server.LLMProviderClientConfig{
		//     Provider:                    "openai",
		//     Model:                       "gpt-4",
		//     BaseURL:                     "https://api.openai.com/v1",
		//     APIKey:                      "your-api-key",
		//     Timeout:                     30 * time.Second,
		//     MaxRetries:                  3,
		//     MaxChatCompletionIterations: 10,
		//     MaxTokens:                   4096,
		//     Temperature:                 0.7,
		// },
		CapabilitiesConfig: &server.CapabilitiesConfig{
			Streaming:              true,
			PushNotifications:      false,
			StateTransitionHistory: false,
		},
		TLSConfig: &server.TLSConfig{
			Enable: false,
		},
		AuthConfig: &server.AuthConfig{
			Enable: false,
		},
		QueueConfig: &server.QueueConfig{
			MaxSize:         100,
			CleanupInterval: 30 * time.Second,
		},
		ServerConfig: &server.ServerConfig{
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	// Create the A2A server with default handlers
	a2aServer := server.NewDefaultA2AServer(cfg, logger)

	// Start the server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := a2aServer.Stop(shutdownCtx); err != nil {
			logger.Error("error during shutdown", zap.Error(err))
		}
		cancel()
	}()

	logger.Info("starting standard A2A server",
		zap.String("port", cfg.Port),
		zap.String("agent_name", cfg.AgentName))

	fmt.Printf("🌐 Server starting on http://localhost:%s\n", cfg.Port)
	fmt.Println("📋 Available endpoints:")
	fmt.Println("  • GET  /health - Health check")
	fmt.Println("  • GET  /.well-known/agent.json - Agent capabilities")
	fmt.Println("  • POST /a2a - A2A protocol endpoint")
	fmt.Println("👋 Press Ctrl+C to stop the server")

	if err := a2aServer.Start(ctx); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}
