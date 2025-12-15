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
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/adk/examples/minimal/server/config"
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
			switch p := part.(type) {
			case types.TextPart:
				userInput = p.Text
			}
		}
	}

	responseText := fmt.Sprintf("Echo: %s", userInput)
	if userInput == "" {
		responseText = "Hello! Send me a message and I'll echo it back."
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
// Configuration can be provided via environment variables:
//   - ENVIRONMENT: Runtime environment (default: development)
//   - A2A_AGENT_NAME: Agent name (default: minimal-agent)
//   - A2A_SERVER_PORT: Server port (default: 8080)
//   - A2A_DEBUG: Enable debug logging (default: false)
//
// To run: go run main.go
func main() {
	// Create configuration with defaults
	cfg := &config.Config{
		Environment: "development",
		A2A: serverConfig.Config{
			AgentName:        "minimal-agent",
			AgentDescription: "A minimal A2A server that echoes messages",
			AgentVersion:     "0.3.0",
			Debug:            false,
			CapabilitiesConfig: serverConfig.CapabilitiesConfig{
				Streaming:              false,
				PushNotifications:      false,
				StateTransitionHistory: false,
			},
			QueueConfig: serverConfig.QueueConfig{
				CleanupInterval: 5 * time.Minute,
			},
			ServerConfig: serverConfig.ServerConfig{
				Port: "8080",
			},
		},
	}

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

	logger.Info("server starting",
		zap.String("environment", cfg.Environment),
		zap.String("agent_name", cfg.A2A.AgentName),
		zap.String("port", cfg.A2A.ServerConfig.Port),
		zap.Bool("debug", cfg.A2A.Debug),
	)

	// Create task handler
	taskHandler := NewSimpleTaskHandler(logger)

	// Build and start server
	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithBackgroundTaskHandler(taskHandler).
		WithAgentCard(types.AgentCard{
			Name:            cfg.A2A.AgentName,
			Description:     cfg.A2A.AgentDescription,
			Version:         cfg.A2A.AgentVersion,
			URL:             fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port),
			ProtocolVersion: "0.3.0",
			Capabilities: types.AgentCapabilities{
				Streaming:              &cfg.A2A.CapabilitiesConfig.Streaming,
				PushNotifications:      &cfg.A2A.CapabilitiesConfig.PushNotifications,
				StateTransitionHistory: &cfg.A2A.CapabilitiesConfig.StateTransitionHistory,
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills:             []types.AgentSkill{},
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
