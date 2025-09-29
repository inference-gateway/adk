package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/inference-gateway/adk/client"
	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Setup logger
	logConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development: true,
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

	// Get server URL from environment or use default
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	logger.Info("Starting A2A client for in-memory queue storage demo",
		zap.String("server_url", serverURL))

	// Create A2A client
	a2aClient := client.NewA2AClient(serverURL, logger)

	// Test server health
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := a2aClient.GetHealth(ctx)
	if err != nil {
		logger.Fatal("Failed to get server health", zap.Error(err))
	}

	logger.Info("Server health check passed", zap.String("status", health.Status))

	// Submit multiple tasks to demonstrate queue processing
	tasks := []string{
		"Process order #12345",
		"Generate monthly report for March 2024",
		"Send notification to user@example.com",
		"Backup database to cloud storage",
		"Clean up temporary files older than 30 days",
	}

	var submittedTasks []string

	for i, taskContent := range tasks {
		contextID := fmt.Sprintf("demo-context-%d", i+1)

		logger.Info("Submitting task to in-memory queue",
			zap.String("context_id", contextID),
			zap.String("content", taskContent))

		taskID, err := a2aClient.SubmitTask(ctx, contextID, taskContent)
		if err != nil {
			logger.Error("Failed to submit task",
				zap.Error(err),
				zap.String("content", taskContent))
			continue
		}

		submittedTasks = append(submittedTasks, taskID)

		logger.Info("Task submitted successfully",
			zap.String("task_id", taskID),
			zap.String("context_id", contextID))

		// Small delay between submissions to see queue behavior
		time.Sleep(1 * time.Second)
	}

	// Wait a bit for processing to complete
	logger.Info("Waiting for tasks to be processed by in-memory queue...")
	time.Sleep(5 * time.Second)

	// Check status of submitted tasks
	for _, taskID := range submittedTasks {
		task, err := a2aClient.GetTask(ctx, taskID)
		if err != nil {
			logger.Error("Failed to get task status",
				zap.Error(err),
				zap.String("task_id", taskID))
			continue
		}

		logger.Info("Task status",
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID),
			zap.String("state", string(task.Status.State)),
			zap.Int("history_length", len(task.History)))

		// Print the last message if available
		if len(task.History) > 0 {
			lastMessage := task.History[len(task.History)-1]
			logger.Info("Task response",
				zap.String("task_id", task.ID),
				zap.String("role", lastMessage.Role),
				zap.String("content", lastMessage.Content[:min(100, len(lastMessage.Content))]))
		}
	}

	// Demonstrate queue stats
	logger.Info("In-memory queue demo completed successfully",
		zap.Int("tasks_submitted", len(submittedTasks)))

	logger.Info("Demo highlights:")
	logger.Info("✓ Tasks queued and processed using in-memory storage")
	logger.Info("✓ No external dependencies required")
	logger.Info("✓ Fast processing with direct memory access")
	logger.Info("✓ Perfect for development and testing scenarios")
	logger.Info("⚠ Tasks will be lost if server restarts (memory-only)")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
