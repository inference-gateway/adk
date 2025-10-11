package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
)

// Config holds client configuration
type Config struct {
	ServerURL string `env:"SERVER_URL,default=http://localhost:8080"`
}

func main() {
	// Load configuration
	ctx := context.Background()
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("client starting", zap.String("server_url", cfg.ServerURL))

	// Create A2A client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	logger.Info("Input-Required Non-Streaming Demo Client")
	logger.Info("This client demonstrates the input-required flow where the server pauses tasks to request additional information from the user.")
	fmt.Println()
	fmt.Println("🔄 Input-Required Non-Streaming Demo")
	fmt.Println("=====================================")
	fmt.Println()
	fmt.Println("This demo shows how agents can pause tasks to request additional information:")
	fmt.Println("- Try: 'What's the weather?' (will ask for location)")
	fmt.Println("- Try: 'Calculate something' (will ask for numbers)")
	fmt.Println("- Try: 'Hello' (simple greeting, no input required)")
	fmt.Println("- Try: 'Help me' (will ask for clarification)")
	fmt.Println()
	fmt.Println("Type 'quit' to exit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("💬 Your message: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "quit" || input == "exit" {
			fmt.Println("👋 Goodbye!")
			break
		}

		// Demonstrate the input-required flow
		if err := demonstrateInputRequiredFlow(a2aClient, input, logger); err != nil {
			logger.Error("demo failed", zap.Error(err))
			fmt.Printf("❌ Error: %v\n\n", err)
		}
	}
}

// demonstrateInputRequiredFlow shows a complete input-required conversation
func demonstrateInputRequiredFlow(a2aClient client.A2AClient, initialMessage string, logger *zap.Logger) error {
	ctx := context.Background()

	// Create initial message
	message := types.Message{
		MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Role:      "user",
		Parts: []types.Part{
			types.NewTextPart(initialMessage),
		},
	}

	// Send initial message
	fmt.Printf("📤 Sending: %s\n", initialMessage)

	params := types.MessageSendParams{
		Message: message,
	}

	// Send message and get task
	response, err := a2aClient.SendTask(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Extract task ID from response
	var taskResult struct {
		ID        string `json:"id"`
		ContextID string `json:"contextId"`
	}
	resultBytes, ok := response.Result.(json.RawMessage)
	if !ok {
		return fmt.Errorf("failed to parse result as json.RawMessage")
	}
	if err := json.Unmarshal(resultBytes, &taskResult); err != nil {
		return fmt.Errorf("failed to parse task ID: %w", err)
	}

	fmt.Printf("🆔 Task ID: %s\n", taskResult.ID)
	fmt.Printf("🔗 Context ID: %s\n", taskResult.ContextID)

	// Monitor task until completion or input required
	taskID := taskResult.ID
	contextID := taskResult.ContextID
	for {
		// Wait a moment for task processing
		time.Sleep(500 * time.Millisecond)

		// Poll for task updates
		taskResponse, err := a2aClient.GetTask(ctx, types.TaskQueryParams{
			ID: taskID,
		})
		if err != nil {
			return fmt.Errorf("failed to poll task: %w", err)
		}

		var currentTask types.Task
		taskResultBytes, ok := taskResponse.Result.(json.RawMessage)
		if !ok {
			return fmt.Errorf("failed to parse task result as json.RawMessage")
		}
		if err := json.Unmarshal(taskResultBytes, &currentTask); err != nil {
			return fmt.Errorf("failed to parse task: %w", err)
		}

		fmt.Printf("📊 Task Status: %s\n", currentTask.Status.State)

		switch currentTask.Status.State {
		case types.TaskStateCompleted:
			// Task completed successfully
			if currentTask.Status.Message != nil {
				responseText := extractMessageText(currentTask.Status.Message)
				fmt.Printf("✅ Response: %s\n\n", responseText)
			}
			return nil

		case types.TaskStateInputRequired:
			// Server is requesting additional input
			if currentTask.Status.Message != nil {
				requestText := extractMessageText(currentTask.Status.Message)
				fmt.Printf("❓ Input Required: %s\n", requestText)
			}

			// Get user input
			fmt.Print("💬 Your response: ")
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return fmt.Errorf("failed to read user input")
			}

			userResponse := strings.TrimSpace(scanner.Text())
			if userResponse == "" {
				fmt.Println("⚠️  Empty response, trying again...")
				continue
			}

			// Create follow-up message with the same context ID
			followUpMessage := types.Message{
				MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
				ContextID: &contextID,
				Role:      "user",
				Parts: []types.Part{
					types.NewTextPart(userResponse),
				},
			}

			// Send follow-up message to continue the task
			fmt.Printf("📤 Sending follow-up: %s (context: %s)\n", userResponse, contextID)

			followUpParams := types.MessageSendParams{
				Message: followUpMessage,
			}

			continuedResponse, err := a2aClient.SendTask(ctx, followUpParams)
			if err != nil {
				return fmt.Errorf("failed to send follow-up message: %w", err)
			}

			// Extract new task ID
			var continuedTaskResult struct {
				ID string `json:"id"`
			}
			continuedResultBytes, ok := continuedResponse.Result.(json.RawMessage)
			if !ok {
				return fmt.Errorf("failed to parse continued result as json.RawMessage")
			}
			if err := json.Unmarshal(continuedResultBytes, &continuedTaskResult); err != nil {
				return fmt.Errorf("failed to parse continued task ID: %w", err)
			}

			taskID = continuedTaskResult.ID
			fmt.Printf("🔄 Continuing with Task ID: %s\n", taskID)

		case types.TaskStateFailed:
			// Task failed
			if currentTask.Status.Message != nil {
				errorText := extractMessageText(currentTask.Status.Message)
				fmt.Printf("❌ Task Failed: %s\n\n", errorText)
			} else {
				fmt.Printf("❌ Task Failed: Unknown error\n\n")
			}
			return nil

		case types.TaskStateCanceled:
			// Task was canceled
			fmt.Printf("🚫 Task Canceled\n\n")
			return nil

		case types.TaskStateWorking, types.TaskStateSubmitted:
			// Task is still processing, continue polling
			fmt.Printf("⏳ Task is still processing...\n")
			continue

		default:
			fmt.Printf("❓ Unknown task state: %s\n\n", currentTask.Status.State)
			return nil
		}
	}
}

// extractMessageText extracts text content from a message
func extractMessageText(message *types.Message) string {
	for _, part := range message.Parts {
		if partMap, ok := part.(map[string]any); ok {
			if kind, exists := partMap["kind"]; exists && kind == "text" {
				if text, exists := partMap["text"].(string); exists {
					return text
				}
			}
		} else if textPart, ok := part.(types.TextPart); ok {
			return textPart.Text
		}
	}
	return "(no text content)"
}
