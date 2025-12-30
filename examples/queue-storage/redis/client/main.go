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
	logger.Info("redis queue storage demo")

	// Create A2A client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	// Test server health with retry for Redis connection
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer healthCancel()

	var health *client.HealthResponse
	for i := 0; i < 6; i++ {
		health, err = a2aClient.GetHealth(healthCtx)
		if err == nil {
			break
		}
		logger.Info("waiting for server and redis to be ready",
			zap.Int("attempt", i+1),
			zap.Error(err))
		time.Sleep(5 * time.Second)
	}

	if err != nil {
		logger.Fatal("failed to get server health after retries", zap.Error(err))
	}

	logger.Info("server health check passed", zap.String("status", health.Status))

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

	logger.Info("submitting tasks to redis queue for persistent processing")

	taskCtx, taskCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer taskCancel()

	for i, taskContent := range tasks {
		contextID := fmt.Sprintf("redis-context-%d", i+1)

		logger.Info("submitting task to redis queue",
			zap.String("context_id", contextID),
			zap.String("content", taskContent))

		message := types.Message{
			ContextID: &contextID,
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

		logger.Info("task submitted to redis queue successfully",
			zap.String("task_id", task.ID),
			zap.String("context_id", contextID))

		// Small delay between submissions to see queue behavior
		time.Sleep(1 * time.Second)
	}

	// Wait for processing to complete
	logger.Info("waiting for redis queue processing to complete")
	time.Sleep(8 * time.Second)

	// Check status of submitted tasks
	logger.Info("checking task status in redis storage")

	for _, taskID := range submittedTasks {
		// Create fresh context for each GetTask call
		getTaskCtx, getTaskCancel := context.WithTimeout(context.Background(), 5*time.Second)
		params := types.TaskQueryParams{
			ID: taskID,
		}
		resp, err := a2aClient.GetTask(getTaskCtx, params)
		getTaskCancel()
		if err != nil {
			logger.Error("failed to get task status from redis",
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

		logger.Info("task status from redis storage",
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
			logger.Info("task response from redis queue",
				zap.String("task_id", task.ID),
				zap.String("role", lastMessage.Role),
				zap.String("content", contentPreview))
		}
	}

	// Demonstrate Redis queue benefits
	logger.Info("redis queue demo completed successfully",
		zap.Int("tasks_submitted", len(submittedTasks)))

	logger.Info("redis queue storage benefits demonstrated",
		zap.Strings("benefits", []string{
			"persistent task storage - tasks survive server restarts",
			"scalable processing - multiple servers can share the queue",
			"reliable delivery - redis ensures task durability",
			"production ready - suitable for high-volume workloads",
			"monitoring support - redis provides comprehensive metrics",
			"clustering support - can scale to redis clusters",
		}))

	logger.Info("next steps",
		zap.Strings("steps", []string{
			"restart the server to see task persistence in action",
			"scale to multiple server instances sharing the same redis",
			"monitor redis performance with redis-cli or redisinsight",
			"configure redis clustering for production deployments",
		}))
}
