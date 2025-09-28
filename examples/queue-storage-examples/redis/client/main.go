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

	logger.Info("Starting A2A client for Redis queue storage demo", 
		zap.String("server_url", serverURL))

	// Create A2A client
	a2aClient := client.NewA2AClient(serverURL, logger)

	// Test server health with retry for Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var health *types.HealthResponse
	for i := 0; i < 6; i++ {
		health, err = a2aClient.GetHealth(ctx)
		if err == nil {
			break
		}
		logger.Info("Waiting for server and Redis to be ready...", 
			zap.Int("attempt", i+1),
			zap.Error(err))
		time.Sleep(5 * time.Second)
	}

	if err != nil {
		logger.Fatal("Failed to get server health after retries", zap.Error(err))
	}

	logger.Info("Server health check passed", zap.String("status", health.Status))

	// Submit multiple tasks to demonstrate Redis queue persistence
	tasks := []string{
		"Generate financial report with Redis persistence",
		"Process large dataset with queue durability",
		"Send batch email notifications via reliable queue",
		"Backup critical data using persistent task storage",
		"Analyze customer behavior with scalable processing",
		"Generate machine learning model with fault tolerance",
		"Process payment transactions with reliable queuing",
		"Update inventory across multiple systems",
	}

	var submittedTasks []string

	logger.Info("Submitting tasks to Redis queue for persistent processing")

	for i, taskContent := range tasks {
		contextID := fmt.Sprintf("redis-context-%d", i+1)
		
		logger.Info("Submitting task to Redis queue",
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

		logger.Info("Task submitted to Redis queue successfully",
			zap.String("task_id", taskID),
			zap.String("context_id", contextID))

		// Small delay between submissions to see queue behavior
		time.Sleep(1 * time.Second)
	}

	// Wait for processing to complete
	logger.Info("Waiting for Redis queue processing to complete...")
	time.Sleep(8 * time.Second)

	// Check status of submitted tasks
	logger.Info("Checking task status in Redis storage...")
	
	for _, taskID := range submittedTasks {
		task, err := a2aClient.GetTask(ctx, taskID)
		if err != nil {
			logger.Error("Failed to get task status from Redis",
				zap.Error(err),
				zap.String("task_id", taskID))
			continue
		}

		logger.Info("Task status from Redis storage",
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID),
			zap.String("state", string(task.Status.State)),
			zap.Int("history_length", len(task.History)))

		// Print the response if available
		if len(task.History) > 0 {
			lastMessage := task.History[len(task.History)-1]
			logger.Info("Task response from Redis queue",
				zap.String("task_id", task.ID),
				zap.String("role", lastMessage.Role),
				zap.String("preview", lastMessage.Content[:min(120, len(lastMessage.Content))]))
		}
	}

	// Demonstrate Redis queue benefits
	logger.Info("Redis queue demo completed successfully",
		zap.Int("tasks_submitted", len(submittedTasks)))

	logger.Info("Redis Queue Storage Benefits Demonstrated:")
	logger.Info("✓ Persistent task storage - tasks survive server restarts")
	logger.Info("✓ Scalable processing - multiple servers can share the queue")
	logger.Info("✓ Reliable delivery - Redis ensures task durability")
	logger.Info("✓ Production ready - suitable for high-volume workloads")
	logger.Info("✓ Monitoring support - Redis provides comprehensive metrics")
	logger.Info("✓ Clustering support - can scale to Redis clusters")

	logger.Info("Next Steps:")
	logger.Info("• Restart the server to see task persistence in action")
	logger.Info("• Scale to multiple server instances sharing the same Redis")
	logger.Info("• Monitor Redis performance with redis-cli or RedisInsight")
	logger.Info("• Configure Redis clustering for production deployments")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}