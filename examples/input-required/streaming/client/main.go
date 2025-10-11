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

	// Create A2A client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	logger.Info("Input-Required Streaming Demo Client", zap.String("server_url", cfg.ServerURL))
	logger.Info("This client demonstrates the input-required flow with real-time streaming where the server pauses tasks to request additional information from the user.")
	fmt.Println()
	fmt.Println("🔄 Input-Required Streaming Demo")
	fmt.Println("=================================")
	fmt.Println()
	fmt.Println("This demo shows how agents can pause streaming tasks to request additional information:")
	fmt.Println("- Try: 'What's the weather?' (will ask for location)")
	fmt.Println("- Try: 'Calculate something' (will ask for numbers)")
	fmt.Println("- Try: 'Hello' (simple greeting, no input required)")
	fmt.Println("- Try: 'Help me' (will ask for clarification)")
	fmt.Println()
	fmt.Println("You'll see responses stream in real-time!")
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

		// Demonstrate the streaming input-required flow
		if err := demonstrateStreamingInputRequiredFlow(a2aClient, input, logger); err != nil {
			logger.Error("demo failed", zap.Error(err))
			fmt.Printf("❌ Error: %v\n\n", err)
		}
	}
}

// demonstrateStreamingInputRequiredFlow shows a complete streaming input-required conversation
func demonstrateStreamingInputRequiredFlow(a2aClient client.A2AClient, initialMessage string, logger *zap.Logger) error {
	ctx := context.Background()

	// Create initial message
	message := types.Message{
		MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Role:      "user",
		Parts: []types.Part{
			types.NewTextPart(initialMessage),
		},
	}

	// Send initial message with streaming
	fmt.Printf("📤 Sending: %s\n", initialMessage)
	fmt.Print("📥 Streaming response: ")

	params := types.MessageSendParams{
		Message: message,
		Configuration: &types.MessageSendConfiguration{
			Blocking:            boolPtr(false),
			AcceptedOutputModes: []string{"text/plain"},
		},
	}

	// Start streaming
	eventChan, err := a2aClient.SendTaskStreaming(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to start streaming: %w", err)
	}

	var streamingText strings.Builder
	var inputRequiredMessage string
	var taskCompleted bool
	var taskInputRequired bool
	var currentTaskID string
	var currentContextID string

	// Process streaming events
	for event := range eventChan {
		logger.Debug("received streaming event")

		// Parse the event result
		if event.Result == nil {
			continue
		}

		resultBytes, _ := json.Marshal(event.Result)

		// Try to parse as Task (for delta events)
		var task types.Task
		if err := json.Unmarshal(resultBytes, &task); err == nil && task.Kind == "task" {
			// Handle delta message - display text in real-time
			if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
				text := extractMessageText(task.Status.Message)
				fmt.Print(text)
				streamingText.WriteString(text)
			}
			continue
		}

		// Try to parse as TaskStatusUpdateEvent (for status changes)
		var statusUpdate types.TaskStatusUpdateEvent
		if err := json.Unmarshal(resultBytes, &statusUpdate); err != nil {
			logger.Debug("failed to parse event", zap.Error(err))
			continue
		}

		if statusUpdate.Kind != "status-update" {
			continue
		}

		// Handle different task states
		switch statusUpdate.Status.State {
		case types.TaskStateWorking:
			logger.Info("task started")

		case types.TaskStateCompleted:
			logger.Info("task completed")
			taskCompleted = true

		case types.TaskStateInputRequired:
			logger.Info("input required")
			taskInputRequired = true
			currentTaskID = statusUpdate.TaskID
			currentContextID = statusUpdate.ContextID
			if statusUpdate.Status.Message != nil {
				inputRequiredMessage = extractMessageText(statusUpdate.Status.Message)
			}

		case types.TaskStateFailed:
			logger.Error("task failed")
			fmt.Print("\n❌ Task failed")
			return nil

		case types.TaskStateCanceled:
			logger.Info("task canceled")
			fmt.Print("\n🚫 Task canceled")
			return nil

		default:
			logger.Debug("unknown state", zap.String("state", string(statusUpdate.Status.State)))
		}
	}

	// Handle final state
	if taskCompleted {
		fmt.Printf("\n\n✅ Response complete!\n")
		if streamingText.Len() > 0 {
			fmt.Printf("\n📝 Full response:\n%s\n", streamingText.String())
		}
		return nil
	}

	if taskInputRequired && inputRequiredMessage != "" {
		fmt.Printf("\n❓ Input Required: %s\n", inputRequiredMessage)

		// Get user input for continuation
		fmt.Print("💬 Your response: ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return fmt.Errorf("failed to read user input")
		}

		userResponse := strings.TrimSpace(scanner.Text())
		if userResponse == "" {
			fmt.Println("⚠️  Empty response, ending conversation...")
			return nil
		}

		// Create follow-up message with task context
		followUpMessage := types.Message{
			MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Role:      "user",
			TaskID:    &currentTaskID,
			ContextID: &currentContextID,
			Parts: []types.Part{
				types.NewTextPart(userResponse),
			},
		}

		// Continue streaming with follow-up
		fmt.Printf("📤 Sending follow-up: %s\n", userResponse)
		fmt.Print("📥 Continued streaming: ")

		followUpParams := types.MessageSendParams{
			Message: followUpMessage,
			Configuration: &types.MessageSendConfiguration{
				Blocking:            boolPtr(false),
				AcceptedOutputModes: []string{"text/plain"},
			},
		}

		// Continue streaming
		continuedEventChan, err := a2aClient.SendTaskStreaming(ctx, followUpParams)
		if err != nil {
			return fmt.Errorf("failed to continue streaming: %w", err)
		}

		// Process continued streaming events
		var continuedText strings.Builder
		for event := range continuedEventChan {
			// Parse the event result
			if event.Result == nil {
				continue
			}

			resultBytes, _ := json.Marshal(event.Result)

			// Try to parse as Task (for delta events)
			var task types.Task
			if err := json.Unmarshal(resultBytes, &task); err == nil && task.Kind == "task" {
				// Handle delta message - display text in real-time
				if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
					text := extractMessageText(task.Status.Message)
					fmt.Print(text)
					continuedText.WriteString(text)
				}
				continue
			}

			// Try to parse as TaskStatusUpdateEvent (for status changes)
			var statusUpdate types.TaskStatusUpdateEvent
			if err := json.Unmarshal(resultBytes, &statusUpdate); err != nil {
				logger.Debug("failed to parse continued event", zap.Error(err))
				continue
			}

			if statusUpdate.Kind != "status-update" {
				continue
			}

			// Handle different task states
			switch statusUpdate.Status.State {
			case types.TaskStateCompleted:
				logger.Info("continued task completed")
				fmt.Printf("\n\n✅ Conversation complete!\n")
				if continuedText.Len() > 0 {
					fmt.Printf("\n📝 Final response:\n%s\n", continuedText.String())
				}
				return nil
			case types.TaskStateFailed:
				logger.Error("continued task failed")
				fmt.Printf("\n❌ Task failed\n\n")
				return nil
			}
		}

		fmt.Printf("\n\n✅ Stream completed!\n")
		if continuedText.Len() > 0 {
			fmt.Printf("\n📝 Full response:\n%s\n", continuedText.String())
		}
	} else {
		fmt.Printf("\n⚠️  Stream ended without clear completion\n\n")
	}

	return nil
}

// extractMessageText extracts text content from a message
func extractMessageText(message *types.Message) string {
	for _, part := range message.Parts {
		if textPart, ok := part.(types.TextPart); ok {
			return textPart.Text
		}
	}
	return ""
}

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}
