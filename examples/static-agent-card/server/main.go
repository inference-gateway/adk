package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	uuid "github.com/google/uuid"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/adk/examples/static-agent-card/server/config"
)

// StaticCardTaskHandler implements a basic task handler that demonstrates
// working with a static agent card loaded from a JSON file
type StaticCardTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// NewStaticCardTaskHandler creates a new task handler
func NewStaticCardTaskHandler(logger *zap.Logger) *StaticCardTaskHandler {
	return &StaticCardTaskHandler{logger: logger}
}

// HandleTask processes tasks with context-aware responses
func (h *StaticCardTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	userInput := ""
	if message != nil {
		for _, part := range message.Parts {
			if textPart, ok := part.(types.TextPart); ok {
				userInput = textPart.Text
				break
			}
		}
	}

	var responseText string
	switch userInput {
	case "":
		responseText = "Hello! I'm the static card agent. My configuration is loaded from a JSON file. Send me a message and I'll echo it back with some helpful context!"
	case "help":
		responseText = "I'm a demonstration agent that shows how to use WithAgentCardFromFile(). My capabilities and metadata are defined in agent-card.json. Try sending me any message!"
	default:
		responseText = fmt.Sprintf("🔄 Echo from static-card-agent: %s\n\n💡 This response comes from an agent whose configuration (name, description, capabilities, skills) was loaded from agent-card.json using WithAgentCardFromFile().", userInput)
	}

	responseMessage := types.Message{
		MessageID: uuid.New().String(),
		ContextID: &task.ContextID,
		TaskID:    &task.ID,
		Role:      "assistant",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: responseText,
			},
		},
	}

	task.History = append(task.History, responseMessage)
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &responseMessage

	return task, nil
}

// SetAgent sets the OpenAI-compatible agent
func (h *StaticCardTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *StaticCardTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// Static Agent Card A2A Server Example
//
// This example demonstrates loading agent card configuration from a JSON file
// using WithAgentCardFromFile() instead of hardcoding it in Go code.
//
// Configuration can be provided via environment variables:
//   - ENVIRONMENT: Runtime environment (default: development)
//   - A2A_AGENT_CARD_FILE: Path to agent card JSON file (default: agent-card.json)
//   - A2A_SERVER_PORT: Server port (default: 8080)
//   - A2A_DEBUG: Enable debug logging (default: false)
//
// To run: go run main.go
func main() {
	// Create configuration with defaults
	cfg := &config.Config{
		Environment: "development",
		A2A: config.A2AConfig{
			AgentCardFile: "agent-card.json",
		},
	}

	// Set default A2A configuration
	cfg.A2A.Debug = false
	cfg.A2A.ServerConfig.Port = "8080"
	cfg.A2A.QueueConfig.CleanupInterval = 5 * time.Minute

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

	// Log configuration info
	logger.Info("server starting",
		zap.String("environment", cfg.Environment),
		zap.String("agent_card_file", cfg.A2A.AgentCardFile),
		zap.String("port", cfg.A2A.ServerConfig.Port),
		zap.Bool("debug", cfg.A2A.Debug),
	)

	// Create task handler
	taskHandler := NewStaticCardTaskHandler(logger)

	// Build and start server with agent card loaded from file
	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A.Config, logger).
		WithBackgroundTaskHandler(taskHandler).
		WithAgentCardFromFile(cfg.A2A.AgentCardFile, map[string]any{
			"url": fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port),
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

	logger.Info("server running", zap.String("port", cfg.A2A.ServerConfig.Port))

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
