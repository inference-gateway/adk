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
			if part.Text != nil {
				inputContent = *part.Text
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
		MessageID: fmt.Sprintf("response-%s", task.ID),
		ContextID: &task.ContextID,
		TaskID:    &task.ID,
		Role:      types.RoleAgent,
		Parts: []types.Part{
			types.CreateTextPart(response),
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
			URL:             stringPtr(fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port)),
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
	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("server running", zap.String("port", cfg.A2A.ServerConfig.Port))

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutting down server")
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

// stringPtr returns a pointer to a string value
func stringPtr(s string) *string {
	return &s
}
