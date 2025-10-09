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

	config "github.com/inference-gateway/adk/examples/queue-storage/redis/server/config"
	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

// RedisTaskHandler implements a task handler demonstrating Redis queue storage
type RedisTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// NewRedisTaskHandler creates a new Redis task handler
func NewRedisTaskHandler(logger *zap.Logger) *RedisTaskHandler {
	return &RedisTaskHandler{
		logger: logger,
	}
}

// SetAgent sets the OpenAI-compatible agent for the task handler
func (h *RedisTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *RedisTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// HandleTask processes tasks using Redis queue storage backend
func (h *RedisTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
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

	h.logger.Info("Processing task with Redis queue storage",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("input", inputContent))

	// Simulate processing work that benefits from Redis storage:
	// - Task persistence across server restarts
	// - Shared queue for multiple server instances
	// - Reliable task processing with durability
	// - Task history and auditing capabilities

	// Add a response message demonstrating Redis benefits
	response := fmt.Sprintf("Task processed successfully using Redis queue storage. Benefits: persistent storage, horizontal scaling, and reliable processing. Original input: %s", inputContent)

	responseMessage := types.Message{
		Kind:      "response",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		ContextID: &task.ContextID,
		TaskID:    &task.ID,
		Role:      "assistant",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: response,
			},
		},
	}

	task.History = append(task.History, responseMessage)

	h.logger.Info("Task processing completed with Redis storage",
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

	logger.Info("Starting A2A server with Redis queue storage",
		zap.String("environment", cfg.Environment),
		zap.String("queue_provider", cfg.A2A.QueueConfig.Provider),
		zap.String("queue_url", cfg.A2A.QueueConfig.URL),
		zap.Any("queue_options", cfg.A2A.QueueConfig.Options),
		zap.Any("queue_credentials", maskCredentials(cfg.A2A.QueueConfig.Credentials)))

	// Create task handler
	taskHandler := NewRedisTaskHandler(logger)

	// Build A2A server with Redis storage
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

	logger.Info("A2A server with Redis queue storage started successfully")

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutting down A2A server...")
}

// maskCredentials masks sensitive credential values for logging
func maskCredentials(credentials map[string]string) map[string]string {
	if credentials == nil {
		return nil
	}

	masked := make(map[string]string)
	for k, v := range credentials {
		if v == "" {
			masked[k] = ""
		} else {
			masked[k] = "***masked***"
		}
	}
	return masked
}
