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
			Role:  types.RoleUser,
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

	// ────────────────────────────────────────────────────────────────
	// Streaming demo: the EnableUsageMetadata flag also applies to the
	// default streaming task handler. After the stream finishes, we
	// fetch the final task snapshot and read Task.Metadata.
	// ────────────────────────────────────────────────────────────────
	fmt.Printf("\n┌─────────────────────────────────────────────────────┐\n")
	fmt.Printf("│ Streaming Test: Usage Metadata via stream           │\n")
	fmt.Printf("└─────────────────────────────────────────────────────┘\n")
	runStreamingDemo(ctx, a2aClient, logger)

	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("All test cases completed!")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("Tip: re-run with A2A_AGENT_CLIENT_ENABLE_USAGE_METADATA=false")
	fmt.Println("on the server to confirm Task.Metadata is omitted.")
	fmt.Println("═══════════════════════════════════════════════════════")
}

// runStreamingDemo submits a streaming task, consumes the event stream until
// the task reaches a terminal state, then fetches the final task snapshot to
// display the usage metadata captured by the streaming task handler.
func runStreamingDemo(ctx context.Context, a2aClient client.A2AClient, logger *zap.Logger) {
	prompt := "Calculate 7 * 6 using the calculate tool, then briefly explain the result."
	fmt.Printf("Prompt: %s\n\n", prompt)

	message := types.Message{
		Role:  types.RoleUser,
		Parts: []types.Part{types.NewTextPart(prompt)},
	}

	blocking := false
	params := types.MessageSendParams{
		Message: message,
		Configuration: &types.MessageSendConfiguration{
			Blocking:            &blocking,
			AcceptedOutputModes: []string{"text/plain"},
		},
	}

	eventChan, err := a2aClient.SendTaskStreaming(ctx, params)
	if err != nil {
		logger.Error("failed to start streaming task", zap.Error(err))
		return
	}

	fmt.Print("Streaming")
	var (
		taskID       string
		finalState   types.TaskState
		eventCount   int
		streamedText string
	)

	for event := range eventChan {
		eventCount++
		fmt.Print(".")

		if event.Result == nil {
			continue
		}

		resultBytes, err := json.Marshal(event.Result)
		if err != nil {
			continue
		}

		// Streaming events alternate between full Task snapshots and
		// TaskStatusUpdateEvent entries. The wire payload carries a
		// "kind" discriminator that isn't on the generated types yet, so
		// peek at the raw JSON to route the decode.
		var disc struct {
			Kind string `json:"kind"`
		}
		_ = json.Unmarshal(resultBytes, &disc)

		switch disc.Kind {
		case "task":
			var task types.Task
			if err := json.Unmarshal(resultBytes, &task); err == nil {
				if task.ID != "" {
					taskID = task.ID
				}
				if task.Status.Message != nil {
					for _, part := range task.Status.Message.Parts {
						if part.Text != nil {
							streamedText += *part.Text
						}
					}
				}
				finalState = task.Status.State
			}
		case "status-update":
			var statusUpdate types.TaskStatusUpdateEvent
			if err := json.Unmarshal(resultBytes, &statusUpdate); err == nil {
				if statusUpdate.TaskID != "" {
					taskID = statusUpdate.TaskID
				}
				finalState = statusUpdate.Status.State
			}
		}
	}

	fmt.Printf(" done (%d events)\n", eventCount)

	if taskID == "" {
		fmt.Println("⚠ No task ID observed from stream - cannot fetch metadata")
		return
	}

	fmt.Printf("Task ID: %s\n", taskID)
	fmt.Printf("Terminal state: %s\n", finalState)
	if streamedText != "" {
		fmt.Printf("Streamed response: %s\n", streamedText)
	}

	// Re-fetch the task to get the post-completion snapshot, which is where
	// the default streaming handler writes the usage metadata.
	taskResponse, err := a2aClient.GetTask(ctx, types.TaskQueryParams{ID: taskID})
	if err != nil {
		logger.Error("failed to fetch final task", zap.Error(err))
		return
	}

	taskResultBytes, ok := taskResponse.Result.(json.RawMessage)
	if !ok {
		logger.Error("unexpected GetTask response shape")
		return
	}

	var finalTask types.Task
	if err := json.Unmarshal(taskResultBytes, &finalTask); err != nil {
		logger.Error("failed to unmarshal final task", zap.Error(err))
		return
	}

	var metadata map[string]any
	if finalTask.Metadata != nil {
		metadata = *finalTask.Metadata
	}
	displayUsageMetadata(metadata)
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
