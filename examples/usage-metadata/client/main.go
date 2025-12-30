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
	ServerURL   string `env:"A2A_SERVER_URL,default=http://localhost:8080"`
}

func main() {
	// Load configuration
	ctx := context.Background()
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize logger
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

	logger.Info("usage metadata client starting", zap.String("server_url", cfg.ServerURL))

	// Create A2A client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test different types of prompts to demonstrate usage tracking
	testCases := []struct {
		name   string
		prompt string
	}{
		{
			name:   "Simple Question",
			prompt: "What is 42 + 58?",
		},
		{
			name:   "Tool Usage",
			prompt: "Please calculate 156 divided by 12 using the calculate tool",
		},
		{
			name:   "Multiple Operations",
			prompt: "Calculate: (25 * 4) and then (100 / 5)",
		},
	}

	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║          Usage Metadata Example Client                ║")
	fmt.Println("║  Demonstrating Token Usage & Execution Metrics        ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")

	for i, tc := range testCases {
		fmt.Printf("\n┌─────────────────────────────────────────────────────┐\n")
		fmt.Printf("│ Test Case %d: %-39s │\n", i+1, tc.name)
		fmt.Printf("└─────────────────────────────────────────────────────┘\n")
		fmt.Printf("Prompt: %s\n\n", tc.prompt)

		// Create message
		message := types.Message{
			Role:  "user",
			Parts: []types.Part{types.NewTextPart(tc.prompt)},
		}

		// Send the task
		params := types.MessageSendParams{
			Message: message,
		}

		response, err := a2aClient.SendTask(ctx, params)
		if err != nil {
			logger.Error("failed to send task", zap.Error(err))
			continue
		}

		// Extract task ID
		var taskResult struct {
			ID string `json:"id"`
		}
		resultBytes, ok := response.Result.(json.RawMessage)
		if !ok {
			logger.Error("failed to parse result")
			continue
		}
		if err := json.Unmarshal(resultBytes, &taskResult); err != nil {
			logger.Error("failed to parse task ID", zap.Error(err))
			continue
		}

		fmt.Printf("Task ID: %s\n", taskResult.ID)
		fmt.Print("Waiting for completion")

		// Poll for completion
		var task types.Task
		for {
			time.Sleep(500 * time.Millisecond)
			fmt.Print(".")

			taskResponse, err := a2aClient.GetTask(ctx, types.TaskQueryParams{
				ID: taskResult.ID,
			})
			if err != nil {
				logger.Error("failed to get task", zap.Error(err))
				fmt.Println()
				break
			}

			taskResultBytes, ok := taskResponse.Result.(json.RawMessage)
			if !ok {
				logger.Error("failed to parse task result")
				fmt.Println()
				break
			}
			if err := json.Unmarshal(taskResultBytes, &task); err != nil {
				logger.Error("failed to unmarshal task", zap.Error(err))
				fmt.Println()
				break
			}

			if task.Status.State == types.TaskStateCompleted {
				fmt.Println(" ✓")

				// Display response
				if task.Status.Message != nil {
					fmt.Println("Response:")
					for _, part := range task.Status.Message.Parts {
						if part.Text != nil {
							fmt.Printf("  %s\n", *part.Text)
						}
					}
				}

				// Display usage metadata
				var metadata map[string]any
				if task.Metadata != nil {
					metadata = *task.Metadata
				}
				displayUsageMetadata(metadata)
				break
			} else if task.Status.State == types.TaskStateFailed {
				fmt.Println(" ✗")
				fmt.Println("Task failed")
				break
			}
		}
	}

	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("All test cases completed!")
	fmt.Println("═══════════════════════════════════════════════════════")
}

// displayUsageMetadata formats and displays the usage metadata from a task
func displayUsageMetadata(metadata map[string]any) {
	if metadata == nil {
		fmt.Println("\n⚠ No metadata available")
		return
	}

	fmt.Println("\n┌── Usage Metadata ──────────────────────────────────┐")

	// Display token usage
	if usage, ok := metadata["usage"].(map[string]any); ok {
		fmt.Println("│ Token Usage:")
		if promptTokens, ok := usage["prompt_tokens"].(float64); ok {
			fmt.Printf("│   • Prompt Tokens:     %8.0f\n", promptTokens)
		}
		if completionTokens, ok := usage["completion_tokens"].(float64); ok {
			fmt.Printf("│   • Completion Tokens: %8.0f\n", completionTokens)
		}
		if totalTokens, ok := usage["total_tokens"].(float64); ok {
			fmt.Printf("│   • Total Tokens:      %8.0f\n", totalTokens)
		}
		fmt.Println("│")
	} else {
		fmt.Println("│ Token Usage: No LLM calls detected")
		fmt.Println("│")
	}

	// Display execution statistics
	if stats, ok := metadata["execution_stats"].(map[string]any); ok {
		fmt.Println("│ Execution Statistics:")
		if iterations, ok := stats["iterations"].(float64); ok {
			fmt.Printf("│   • Iterations:        %8.0f\n", iterations)
		}
		if messages, ok := stats["messages"].(float64); ok {
			fmt.Printf("│   • Messages:          %8.0f\n", messages)
		}
		if toolCalls, ok := stats["tool_calls"].(float64); ok {
			fmt.Printf("│   • Tool Calls:        %8.0f\n", toolCalls)
		}
		if failedTools, ok := stats["failed_tools"].(float64); ok {
			fmt.Printf("│   • Failed Tools:      %8.0f\n", failedTools)
		}
	}

	fmt.Println("└────────────────────────────────────────────────────┘")
}
