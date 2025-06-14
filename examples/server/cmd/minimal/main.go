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

// Minimal A2A Server Example
//
// This example demonstrates the simplest possible A2A server setup using NewDefaultA2AServer().
// This creates a basic server that handles A2A protocol messages without any AI capabilities.
//
// What this server provides:
// ✅ A2A protocol message handling (message/send, message/stream, tasks/get, tasks/cancel)
// ✅ Agent metadata endpoint (/.well-known/agent.json)
// ✅ Health check endpoint (/health)
// ✅ Automatic configuration from environment variables
// ❌ No AI/LLM integration
// ❌ No custom tools
//
// To run: go run main.go
func main() {
	fmt.Println("🤖 Starting Minimal A2A Server (Non-AI)...")

	// Step 1: Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Step 2: Create the simplest possible A2A server
	// This uses all default configurations and creates a basic server
	// that handles A2A protocol messages without any AI capabilities
	a2aServer := server.NewDefaultA2AServer(nil)

	logger.Info("✅ minimal A2A server created (no AI capabilities)")

	// Step 3: Start the server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	// Get the port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("🌐 server running", zap.String("port", port))
	fmt.Printf("\n🎯 Test the server:\n")
	fmt.Printf("📋 Agent info: http://localhost:%s/.well-known/agent.json\n", port)
	fmt.Printf("💚 Health check: http://localhost:%s/health\n", port)
	fmt.Printf("📡 A2A endpoint: http://localhost:%s/a2a\n", port)

	fmt.Println("\n📝 Example A2A request:")
	fmt.Printf(`curl -X POST http://localhost:%s/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "content": "Hello!"
      }
    },
    "id": 1
  }'`, port)
	fmt.Println()

	// Step 4: Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 shutting down server...")

	// Step 5: Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	} else {
		logger.Info("✅ goodbye!")
	}
}
