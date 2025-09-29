package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/inference-gateway/adk/examples/queue-storage-examples/redis/server/config"
	"github.com/inference-gateway/adk/server"
	"github.com/inference-gateway/adk/types"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// RedisTaskHandler implements a task handler demonstrating Redis queue storage
type RedisTaskHandler struct {
	logger *zap.Logger
}

// NewRedisTaskHandler creates a new Redis task handler
func NewRedisTaskHandler(logger *zap.Logger) *RedisTaskHandler {
	return &RedisTaskHandler{
		logger: logger,
	}
}

// HandleTask processes tasks using Redis queue storage backend
func (h *RedisTaskHandler) HandleTask(ctx context.Context, task *types.Task) error {
	h.logger.Info("Processing task with Redis queue storage",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("input", task.Input.Content))

	// Simulate processing work that benefits from Redis storage:
	// - Task persistence across server restarts
	// - Shared queue for multiple server instances
	// - Reliable task processing with durability
	// - Task history and auditing capabilities

	// Add a response message demonstrating Redis benefits
	response := fmt.Sprintf("Task processed successfully using Redis queue storage. Benefits: persistent storage, horizontal scaling, and reliable processing. Original input: %s", task.Input.Content)

	responseMessage := &types.Message{
		Role:    "assistant",
		Content: response,
	}

	task.History = append(task.History, responseMessage)

	h.logger.Info("Task processing completed with Redis storage",
		zap.String("task_id", task.ID),
		zap.String("response_preview", response[:min(100, len(response))]))

	return nil
}

func main() {
	// Load configuration
	var cfg config.Config
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		log.Fatal("Failed to process environment config:", err)
	}

	// Setup logger
	logLevel := zapcore.InfoLevel
	if cfg.A2A.Debug {
		logLevel = zapcore.DebugLevel
	}

	logConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(logLevel),
		Development: cfg.Environment == "development",
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := logConfig.Build()
	if err != nil {
		log.Fatal("Failed to build logger:", err)
	}
	defer logger.Sync()

	logger.Info("Starting A2A server with Redis queue storage",
		zap.String("environment", cfg.Environment),
		zap.String("queue_provider", cfg.A2A.QueueConfig.Provider),
		zap.String("queue_url", cfg.A2A.QueueConfig.URL),
		zap.Any("queue_options", cfg.A2A.QueueConfig.Options),
		zap.Any("queue_credentials", maskCredentials(cfg.A2A.QueueConfig.Credentials)))

	// Create task handler
	taskHandler := NewRedisTaskHandler(logger)

	// Build A2A server with Redis storage
	a2aServer, err := server.NewA2AServerBuilder(logger).
		WithConfig(&cfg.A2A).
		WithTaskHandler(taskHandler).
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
