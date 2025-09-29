package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/inference-gateway/adk/examples/queue-storage-examples/in-memory/server/config"
	"github.com/inference-gateway/adk/server"
	"github.com/inference-gateway/adk/types"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// DemoTaskHandler implements a simple task handler for demonstration
type DemoTaskHandler struct {
	logger *zap.Logger
}

// NewDemoTaskHandler creates a new demo task handler
func NewDemoTaskHandler(logger *zap.Logger) *DemoTaskHandler {
	return &DemoTaskHandler{
		logger: logger,
	}
}

// HandleTask processes tasks by simply adding a response message
func (h *DemoTaskHandler) HandleTask(ctx context.Context, task *types.Task) error {
	h.logger.Info("Processing task with in-memory queue",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("input", task.Input.Content))

	// Simulate some processing work
	// In a real scenario, this could be any background processing:
	// - Data processing
	// - File operations
	// - External API calls
	// - Batch operations

	// Add a response message
	response := fmt.Sprintf("Task processed successfully using in-memory queue storage. Original input: %s", task.Input.Content)

	responseMessage := &types.Message{
		Role:    "assistant",
		Content: response,
	}

	task.History = append(task.History, responseMessage)

	h.logger.Info("Task processing completed",
		zap.String("task_id", task.ID),
		zap.String("response", response))

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

	logger.Info("Starting A2A server with in-memory queue storage",
		zap.String("environment", cfg.Environment),
		zap.String("queue_provider", cfg.A2A.QueueConfig.Provider),
		zap.Int("queue_max_size", cfg.A2A.QueueConfig.MaxSize),
		zap.Duration("cleanup_interval", cfg.A2A.QueueConfig.CleanupInterval))

	// Create task handler
	taskHandler := NewDemoTaskHandler(logger)

	// Build A2A server with in-memory storage
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

	logger.Info("A2A server with in-memory queue storage started successfully")

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutting down A2A server...")
}
