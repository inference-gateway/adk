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

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Prepare different types of messages to test AI capabilities with tools
	prompts := []string{
		"What's the weather in London?",
		"What time is it?",
		"Can you check the weather in Paris and tell me the current time?",
	}

	for i, prompt := range prompts {
		fmt.Printf("\n--- Request %d ---\n", i+1)
		fmt.Printf("Sending: %s\n", prompt)

		// Create message with proper structure
		message := types.Message{
			Role: "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": prompt,
				},
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

		// Extract task ID from response
		var taskResult struct {
			ID string `json:"id"`
		}
		resultBytes, ok := response.Result.(json.RawMessage)
		if !ok {
			logger.Error("failed to parse result as json.RawMessage")
			continue
		}
		if err := json.Unmarshal(resultBytes, &taskResult); err != nil {
			logger.Error("failed to parse task ID", zap.Error(err))
			continue
		}

		fmt.Printf("Task ID: %s\n", taskResult.ID)
		fmt.Print("Polling for result")

		// Poll for task completion
		for {
			time.Sleep(500 * time.Millisecond)
			fmt.Print(".")

			taskResponse, err := a2aClient.GetTask(ctx, types.TaskQueryParams{
				ID: taskResult.ID,
			})
			if err != nil {
				logger.Error("failed to get task status", zap.Error(err))
				fmt.Println()
				break
			}

			var task types.Task
			taskResultBytes, ok := taskResponse.Result.(json.RawMessage)
			if !ok {
				logger.Error("failed to parse task result as json.RawMessage")
				fmt.Println()
				break
			}
			if err := json.Unmarshal(taskResultBytes, &task); err != nil {
				logger.Error("failed to parse task", zap.Error(err))
				fmt.Println()
				break
			}

			// Check if task is completed
			if task.Status.State == types.TaskStateCompleted {
				fmt.Println("\n✓ Task completed!")

				// Display the response
				if task.Status.Message != nil {
					for _, part := range task.Status.Message.Parts {
						if partMap, ok := part.(map[string]any); ok {
							if text, ok := partMap["text"].(string); ok {
								fmt.Printf("\nResponse: %s\n", text)
							}
						}
					}
				}
				break
			} else if task.Status.State == types.TaskStateFailed {
				fmt.Println("\n✗ Task failed")
				if task.Status.Message != nil {
					responseJSON, _ := json.MarshalIndent(task.Status.Message, "", "  ")
					fmt.Printf("Error: %s\n", string(responseJSON))
				}
				break
			}
		}
	}
}
