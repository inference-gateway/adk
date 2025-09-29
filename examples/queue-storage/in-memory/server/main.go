package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	config "github.com/inference-gateway/adk/examples/queue-storage/in-memory/server/config"
	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

// DemoTaskHandler implements a simple task handler for demonstration
type DemoTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// NewDemoTaskHandler creates a new demo task handler
func NewDemoTaskHandler(logger *zap.Logger) *DemoTaskHandler {
	return &DemoTaskHandler{
		logger: logger,
	}
}

// SetAgent sets the OpenAI-compatible agent for the task handler
func (h *DemoTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *DemoTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// HandleTask processes tasks by simply adding a response message
func (h *DemoTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	var inputContent string
	// Extract input from the current message being processed
	if message != nil {
		for _, part := range message.Parts {
			if textPart, ok := part.(types.TextPart); ok {
				inputContent = textPart.Text
				break
			}
		}
	}

	h.logger.Info("Processing task with in-memory queue",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("input", inputContent))

	// Simulate some processing work
	// In a real scenario, this could be any background processing:
	// - Data processing
	// - File operations
	// - External API calls
	// - Batch operations
	// - AI agent for natural language processing

	// Add a response message
	response := fmt.Sprintf("Task processed successfully using in-memory queue storage. Original input: %s", inputContent)

	responseMessage := types.Message{
		Kind:      "response",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: response,
			},
		},
	}

	task.History = append(task.History, responseMessage)

	h.logger.Info("Task processing completed",
		zap.String("task_id", task.ID),
		zap.String("response", response))

	return task, nil
}

func main() {
	// Load configuration
	var cfg config.Config
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		log.Fatal("Failed to process environment config:", err)
	}

	// Setup logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	logger.Info("Starting A2A server with in-memory queue storage",
		zap.String("environment", cfg.Environment),
		zap.String("queue_provider", cfg.A2A.QueueConfig.Provider),
		zap.Int("queue_max_size", cfg.A2A.QueueConfig.MaxSize),
		zap.Duration("cleanup_interval", cfg.A2A.QueueConfig.CleanupInterval))

	// Create task handler
	taskHandler := NewDemoTaskHandler(logger)

	// Build A2A server with in-memory storage
	a2aServer, err := server.NewA2AServerBuilder(serverConfig.Config(cfg.A2A), logger).
		WithBackgroundTaskHandler(taskHandler).
		WithDefaultStreamingTaskHandler().
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
		logger.Fatal("Failed to build A2A server", zap.Error(err))
	}

	// Setup graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start server
	if err := a2aServer.Start(ctx); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}

	logger.Info("A2A server with in-memory queue storage started successfully")

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutting down A2A server...")
}
