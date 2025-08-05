package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// SimpleTaskHandler implements a basic task handler without LLM
type SimpleTaskHandler struct {
	logger *zap.Logger
}

// NewSimpleTaskHandler creates a new simple task handler
func NewSimpleTaskHandler(logger *zap.Logger) *SimpleTaskHandler {
	return &SimpleTaskHandler{
		logger: logger,
	}
}

// HandleTask processes tasks with simple predefined responses
func (h *SimpleTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message, agent server.OpenAICompatibleAgent) (*types.Task, error) {
	h.logger.Info("processing task with simple handler",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID))

	// Extract user input from message
	var userInput string
	if message != nil {
		for _, part := range message.Parts {
			if partMap, ok := part.(map[string]interface{}); ok {
				if text, exists := partMap["text"]; exists {
					if textStr, ok := text.(string); ok {
						userInput = textStr
						break
					}
				}
			}
		}
	}

	// Simple response logic based on user input
	var responseText string
	lowerInput := strings.ToLower(userInput)
	switch {
	case strings.Contains(lowerInput, "hello") || strings.Contains(lowerInput, "hi"):
		responseText = "Hello! I'm a simple A2A server without AI capabilities. I can respond to basic greetings and status checks."
	case strings.Contains(lowerInput, "status") || strings.Contains(lowerInput, "health"):
		responseText = "✅ Server is running normally. All systems operational."
	case strings.Contains(lowerInput, "help"):
		responseText = "Available commands: hello, status, help, time, or just say anything and I'll echo it back!"
	case strings.Contains(lowerInput, "time"):
		responseText = fmt.Sprintf("⏰ Current server time: %s", time.Now().Format("2006-01-02 15:04:05 UTC"))
	case userInput == "":
		responseText = "I received an empty message. Please send some text!"
	default:
		responseText = fmt.Sprintf("📝 You said: \"%s\"\n\nI'm a simple non-AI server, so I'm just echoing your message back to you. Try saying 'hello', 'status', 'help', or 'time' for special responses!", userInput)
	}

	// Create response message
	response := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": responseText,
			},
		},
	}

	// Update task with response
	task.History = append(task.History, *response)
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = response

	h.logger.Info("task completed with simple handler",
		zap.String("task_id", task.ID),
		zap.String("response_preview", responseText[:min(50, len(responseText))]))

	return task, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Minimal A2A Server Example
//
// This example demonstrates a simple A2A server that can handle basic conversations
// without requiring any AI/LLM integration. It uses a custom task handler to provide
// simple responses to user messages.
//
// What this server provides:
// ✅ A2A protocol message handling (message/send, message/stream, tasks/get, tasks/cancel)
// ✅ Agent metadata endpoint (/.well-known/agent.json)
// ✅ Health check endpoint (/health)
// ✅ Simple conversational responses without AI
// ✅ Echo functionality and basic commands
// ❌ No AI/LLM integration
// ❌ No advanced tools or function calling
//
// To run: go run main.go
func main() {
	fmt.Println("🤖 Starting Minimal A2A Server (Non-AI)...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create a simple A2A server with custom task handler

	// Get the port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Load configuration with agent metadata injected at build time
	// Agent metadata is set via LD flags during build:
	// go build -ldflags="-X github.com/inference-gateway/adk/server.BuildAgentName=my-agent ..."
	cfg := config.Config{
		AgentName:        server.BuildAgentName,
		AgentDescription: server.BuildAgentDescription,
		AgentVersion:     server.BuildAgentVersion,
		Debug:            true,
		QueueConfig: config.QueueConfig{
			CleanupInterval: 5 * time.Minute,
		},
		ServerConfig: config.ServerConfig{
			Port: port,
		},
	}

	// Create custom task handler that provides simple responses
	taskHandler := NewSimpleTaskHandler(logger)

	// Build server with custom task handler and agent card from file
	// Demonstrate the override functionality for dynamic agent card customization
	// This is useful when you want to use a template agent card but override certain values at runtime
	overrides := map[string]interface{}{
		"name":        cfg.AgentName,        // Override name from config
		"description": cfg.AgentDescription, // Override description from config
		"version":     cfg.AgentVersion,     // Override version from config
		"capabilities": map[string]interface{}{
			"streaming":              true,  // Enable streaming for this instance
			"pushNotifications":      false, // Disable push notifications
			"stateTransitionHistory": false, // Disable state transition history
		},
	}

	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithTaskHandler(taskHandler).
		WithAgentCardFromFile("./.well-known/agent.json", overrides).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	logger.Info("✅ minimal A2A server created with simple task handler")

	// Display agent metadata (from build-time LD flags)
	logger.Info("🤖 agent metadata",
		zap.String("name", server.BuildAgentName),
		zap.String("description", server.BuildAgentDescription),
		zap.String("version", server.BuildAgentVersion))

	// Start the server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

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
        "kind": "message",
        "messageId": "msg-001",
        "role": "user",
        "parts": [
          {
            "kind": "text",
            "text": "Hello! Can you help me?"
          }
        ]
      }
    },
    "id": 1
  }'`, port)
	fmt.Println()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	} else {
		logger.Info("✅ goodbye!")
	}
}
