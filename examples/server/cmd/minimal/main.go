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
	zap "go.uber.org/zap"
)

// SimpleTaskHandler implements a basic task handler without LLM
type SimpleTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// NewSimpleTaskHandler creates a new simple task handler
func NewSimpleTaskHandler(logger *zap.Logger) *SimpleTaskHandler {
	return &SimpleTaskHandler{logger: logger}
}

// HandleTask processes tasks with simple echo responses
func (h *SimpleTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	userInput := ""
	if message != nil {
		for _, part := range message.Parts {
			if partMap, ok := part.(map[string]any); ok {
				if text, ok := partMap["text"].(string); ok {
					userInput = text
					break
				}
			}
		}
	}

	responseText := fmt.Sprintf("Echo: %s", userInput)
	if userInput == "" {
		responseText = "Hello! Send me a message and I'll echo it back."
	}

	task.History = append(task.History, types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": responseText,
			},
		},
	})

	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &task.History[len(task.History)-1]

	return task, nil
}

// SetAgent sets the OpenAI-compatible agent
func (h *SimpleTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *SimpleTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// Minimal A2A Server Example
//
// This example demonstrates a simple A2A server that echoes user messages
// without requiring any AI/LLM integration.
//
// To run: go run main.go
func main() {
	fmt.Println("🤖 Starting Minimal A2A Server...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Configuration
	cfg := config.Config{
		AgentName:        "minimal-agent",
		AgentDescription: "A minimal A2A server that echoes messages",
		AgentVersion:     "1.0.0",
		Debug:            true,
		QueueConfig: config.QueueConfig{
			CleanupInterval: 5 * time.Minute,
		},
		ServerConfig: config.ServerConfig{
			Port: port,
		},
	}

	// Create task handler
	taskHandler := NewSimpleTaskHandler(logger)

	// Build and start server
	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithBackgroundTaskHandler(taskHandler).
		WithAgentCard(types.AgentCard{
			Name:            cfg.AgentName,
			Description:     cfg.AgentDescription,
			Version:         cfg.AgentVersion,
			URL:             fmt.Sprintf("http://localhost:%s", port),
			ProtocolVersion: "1.0.0",
			Capabilities: types.AgentCapabilities{
				Streaming:              &[]bool{false}[0],
				PushNotifications:      &[]bool{false}[0],
				StateTransitionHistory: &[]bool{false}[0],
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills:             []types.AgentSkill{},
		}).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	logger.Info("✅ server created")

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("🌐 server running on port " + port)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	} else {
		logger.Info("✅ goodbye!")
	}
}
