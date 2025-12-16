package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	"github.com/inference-gateway/adk/client"
	"github.com/inference-gateway/adk/types"
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
	defer func() {
		_ = logger.Sync()
	}()

	logger.Info("client starting", zap.String("server_url", cfg.ServerURL))

	// Create client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Prepare different types of messages to test default handlers
	prompts := []string{
		"Hello, how are you?",
		"What's the weather like?",
		"Can you help me with something?",
	}

	for i, prompt := range prompts {
		fmt.Printf("\n--- Request %d ---\n", i+1)
		fmt.Printf("Sending: %s\n", prompt)

		// Create message with proper structure
		message := types.Message{
			Role: types.RoleUser,
			Parts: []types.Part{
				types.CreateTextPart(prompt),
			},
		}

		// Send the message
		params := types.MessageSendParams{
			Message: message,
		}

		response, err := a2aClient.SendTask(ctx, params)
		if err != nil {
			logger.Error("failed to send message", zap.Int("message_number", i+1), zap.Error(err))
			continue
		}

		// Extract task ID from response for polling
		taskResultBytes, err := json.Marshal(response.Result)
		if err != nil {
			logger.Error("failed to marshal task response", zap.Error(err))
			continue
		}

		var taskData map[string]any
		if err := json.Unmarshal(taskResultBytes, &taskData); err != nil {
			logger.Error("failed to parse task response", zap.Error(err))
			continue
		}

		taskID, ok := taskData["id"].(string)
		if !ok {
			logger.Error("task ID not found in response")
			continue
		}

		logger.Info("task created", zap.String("task_id", taskID))
		fmt.Printf("Polling for completion...\n")

		// Poll for task completion
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		completed := false
		for !completed {
			select {
			case <-ctx.Done():
				logger.Warn("context cancelled while polling task", zap.String("task_id", taskID))
				return
			case <-ticker.C:
				taskResp, err := a2aClient.GetTask(ctx, types.TaskQueryParams{ID: taskID})
				if err != nil {
					logger.Error("failed to get task status", zap.Error(err))
					completed = true
					break
				}

				taskRespBytes, err := json.Marshal(taskResp.Result)
				if err != nil {
					logger.Error("failed to marshal task response", zap.Error(err))
					completed = true
					break
				}

				var task types.Task
				if err := json.Unmarshal(taskRespBytes, &task); err != nil {
					logger.Error("failed to parse task", zap.Error(err))
					completed = true
					break
				}

				logger.Debug("task status", zap.String("task_id", taskID), zap.String("state", string(task.Status.State)))

				// Check if task is in a final state
				switch task.Status.State {
				case types.TaskStateCompleted:
					logger.Info("task completed successfully")
					if task.Status.Message != nil {
						messageJSON, _ := json.MarshalIndent(task.Status.Message, "", "  ")
						fmt.Printf("Final Response:\n%s\n", string(messageJSON))
					}
					completed = true
				case types.TaskStateInputRequired:
					logger.Info("task requires input - ending polling")
					if task.Status.Message != nil {
						messageJSON, _ := json.MarshalIndent(task.Status.Message, "", "  ")
						fmt.Printf("Partial Response:\n%s\n", string(messageJSON))
					}
					completed = true
				case types.TaskStateFailed, types.TaskStateCancelled, types.TaskStateRejected:
					logger.Warn("task ended", zap.String("state", string(task.Status.State)))
					if task.Status.Message != nil {
						messageJSON, _ := json.MarshalIndent(task.Status.Message, "", "  ")
						fmt.Printf("Response:\n%s\n", string(messageJSON))
					}
					completed = true
				}
			}
		}
	}
}
