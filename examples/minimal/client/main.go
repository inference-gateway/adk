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
	defer func() {
		_ = logger.Sync()
	}()

	logger.Info("client starting", zap.String("server_url", cfg.ServerURL))

	// Create client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	// Create a simple task
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create the message
	message := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Hello, this is a test message. Please respond with a greeting.",
			},
		},
	}

	logger.Info("sending message to server")

	// Send the message using SendTask
	params := types.MessageSendParams{
		Message: message,
	}

	response, err := a2aClient.SendTask(ctx, params)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Extract task from response
	if response.Result == nil {
		log.Fatal("No result in response")
	}

	// Parse the task from the result
	taskBytes, err := json.Marshal(response.Result)
	if err != nil {
		log.Fatalf("Failed to marshal task: %v", err)
	}

	var task types.Task
	if err := json.Unmarshal(taskBytes, &task); err != nil {
		log.Fatalf("Failed to unmarshal task: %v", err)
	}

	logger.Info("task created", zap.String("task_id", task.ID), zap.String("state", string(task.Status.State)))

	// Poll for task completion
	logger.Debug("polling for task completion")
	for range 10 {
		time.Sleep(500 * time.Millisecond)

		getParams := types.TaskQueryParams{
			ID: task.ID,
		}

		getResponse, err := a2aClient.GetTask(ctx, getParams)
		if err != nil {
			log.Printf("Failed to get task status: %v", err)
			continue
		}

		if getResponse.Result == nil {
			continue
		}

		// Parse the updated task
		updatedTaskBytes, err := json.Marshal(getResponse.Result)
		if err != nil {
			continue
		}

		var updatedTask types.Task
		if err := json.Unmarshal(updatedTaskBytes, &updatedTask); err != nil {
			continue
		}

		logger.Debug("task state", zap.String("state", string(updatedTask.Status.State)))

		if updatedTask.Status.State == types.TaskStateCompleted {
			// Pretty print the completed task
			responseJSON, err := json.MarshalIndent(updatedTask, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal response: %v", err)
			}

			logger.Info("task completed")
			fmt.Println(string(responseJSON))
			return
		}

		if updatedTask.Status.State == types.TaskStateFailed {
			logger.Error("task failed")
			return
		}
	}

	logger.Warn("task did not complete within timeout")
}
