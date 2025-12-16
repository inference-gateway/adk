package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
)

// Config holds client configuration
type Config struct {
	Environment string `env:"ENVIRONMENT,default=development"`
	ServerURL   string `env:"SERVER_URL,default=http://localhost:8080"`
}

func main() {
	// Load configuration
	ctx := context.Background()
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize logger based on environment
	var logger *zap.Logger
	var err error
	if cfg.Environment == "development" || cfg.Environment == "dev" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("client starting", zap.String("server_url", cfg.ServerURL))
	logger.Info("in-memory queue storage demo")

	// Create A2A client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	// Test server health
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer healthCancel()

	health, err := a2aClient.GetHealth(healthCtx)
	if err != nil {
		logger.Fatal("failed to get server health", zap.Error(err))
	}

	logger.Info("server health check passed", zap.String("status", health.Status))

	// Submit multiple tasks to demonstrate queue processing
	tasks := []string{
		"Process order #12345",
		"Generate monthly report for March 2024",
		"Send notification to user@example.com",
		"Backup database to cloud storage",
		"Clean up temporary files older than 30 days",
	}

	var submittedTasks []string

	taskCtx, taskCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer taskCancel()

	for i, taskContent := range tasks {
		contextID := fmt.Sprintf("demo-context-%d", i+1)

		logger.Info("submitting task to in-memory queue",
			zap.String("context_id", contextID),
			zap.String("content", taskContent))

		message := types.Message{
			ContextID: &contextID,
			Kind:      "request",
			MessageID: fmt.Sprintf("msg-%d", i+1),
			Role:      types.RoleUser,
			Parts: []types.Part{
				types.CreateTextPart(taskContent),
			},
		}
		params := types.MessageSendParams{
			Message: message,
		}
		resp, err := a2aClient.SendTask(taskCtx, params)
		if err != nil {
			logger.Error("failed to submit task",
				zap.Error(err),
				zap.String("content", taskContent))
			continue
		}

		// Parse the response to extract task
		var task types.Task
		resultBytes, ok := resp.Result.(json.RawMessage)
		if !ok {
			logger.Error("failed to cast response to json.RawMessage",
				zap.String("content", taskContent))
			continue
		}
		if err := json.Unmarshal(resultBytes, &task); err != nil {
			logger.Error("failed to parse task response",
				zap.Error(err),
				zap.String("content", taskContent))
			continue
		}

		submittedTasks = append(submittedTasks, task.ID)

		logger.Info("task submitted successfully",
			zap.String("task_id", task.ID),
			zap.String("context_id", contextID))

		// Small delay between submissions to see queue behavior
		time.Sleep(1 * time.Second)
	}

	// Wait a bit for processing to complete
	logger.Info("waiting for tasks to be processed by in-memory queue")
	time.Sleep(5 * time.Second)

	// Check status of submitted tasks
	for _, taskID := range submittedTasks {
		// Create fresh context for each GetTask call
		getTaskCtx, getTaskCancel := context.WithTimeout(context.Background(), 5*time.Second)
		params := types.TaskQueryParams{
			ID: taskID,
		}
		resp, err := a2aClient.GetTask(getTaskCtx, params)
		getTaskCancel()
		if err != nil {
			logger.Error("failed to get task status",
				zap.Error(err),
				zap.String("task_id", taskID))
			continue
		}

		// Parse the response directly as Task struct
		var task types.Task
		resultBytes, ok := resp.Result.(json.RawMessage)
		if !ok {
			logger.Error("failed to cast response to json.RawMessage",
				zap.String("task_id", taskID))
			continue
		}
		if err := json.Unmarshal(resultBytes, &task); err != nil {
			logger.Error("failed to parse task response",
				zap.Error(err),
				zap.String("task_id", taskID))
			continue
		}
		status := task.Status
		history := task.History

		logger.Info("task status",
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID),
			zap.String("state", string(status.State)),
			zap.Int("history_length", len(history)))

		// Print the last message if available
		if len(history) > 0 {
			lastMessage := history[len(history)-1]
			// Extract text from the last message parts
			var content string
			for _, part := range lastMessage.Parts {
				if part.Text != nil {
					content = *part.Text
					break
				}
			}
			contentPreview := content
			if len(content) > 100 {
				contentPreview = content[:100] + "..."
			}
			logger.Info("task response",
				zap.String("task_id", task.ID),
				zap.String("role", lastMessage.Role),
				zap.String("content", contentPreview))
		}
	}

	// Demonstrate queue stats
	logger.Info("in-memory queue demo completed successfully",
		zap.Int("tasks_submitted", len(submittedTasks)))

	logger.Info("demo highlights",
		zap.Strings("features", []string{
			"tasks queued and processed using in-memory storage",
			"no external dependencies required",
			"fast processing with direct memory access",
			"perfect for development and testing scenarios",
			"tasks will be lost if server restarts (memory-only)",
		}))
}
