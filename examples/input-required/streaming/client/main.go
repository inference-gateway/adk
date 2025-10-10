package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/inference-gateway/adk/client"
	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create A2A client
	a2aClient := client.NewA2AClient("http://localhost:8080", logger)

	logger.Info("Input-Required Streaming Demo Client")
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
func demonstrateStreamingInputRequiredFlow(a2aClient *client.A2AClient, initialMessage string, logger *zap.Logger) error {
	ctx := context.Background()

	// Create initial message
	message := types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Role:      "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": initialMessage,
			},
		},
	}

	// Send initial message with streaming
	fmt.Printf("📤 Sending: %s\n", initialMessage)
	fmt.Print("📥 Streaming response: ")
	
	params := types.MessageStreamParams{
		ContextID: fmt.Sprintf("demo-context-%d", time.Now().UnixNano()),
		Message:   message,
	}

	// Start streaming
	eventChan, err := a2aClient.SendMessageStreaming(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to start streaming: %w", err)
	}

	var currentTaskID string
	var currentContextID string
	var streamingText strings.Builder
	var inputRequiredMessage string
	var taskCompleted bool
	var taskInputRequired bool

	// Process streaming events
	for event := range eventChan {
		logger.Debug("received streaming event", zap.String("type", event.Type()))

		switch event.Type() {
		case types.EventDelta:
			// Real-time text streaming
			var msg types.Message
			if err := event.DataAs(&msg); err == nil {
				text := extractMessageText(&msg)
				fmt.Print(text)
				streamingText.WriteString(text)
			}

		case types.EventInputRequired:
			// Input required from user
			var msg types.Message
			if err := event.DataAs(&msg); err == nil {
				inputRequiredMessage = extractMessageText(&msg)
				taskInputRequired = true
				if msg.TaskID != nil {
					currentTaskID = *msg.TaskID
				}
				if msg.ContextID != nil {
					currentContextID = *msg.ContextID
				}
			}

		case types.EventTaskStatusChanged:
			// Task status update
			var status types.TaskStatus
			if err := event.DataAs(&status); err == nil {
				switch status.State {
				case types.TaskStateCompleted:
					taskCompleted = true
				case types.TaskStateInputRequired:
					taskInputRequired = true
				case types.TaskStateFailed:
					fmt.Print("\n❌ Task failed")
					return nil
				}
			}

		case types.EventIterationCompleted:
			// Iteration completed - could be end of stream or input required
			var msg types.Message
			if err := event.DataAs(&msg); err == nil {
				if msg.Kind == "input_required" {
					inputRequiredMessage = extractMessageText(&msg)
					taskInputRequired = true
				}
			}

		case types.EventStreamFailed:
			fmt.Print("\n❌ Stream failed")
			return nil
		}
	}

	// Handle final state
	if taskCompleted {
		fmt.Printf("\n✅ Response complete!\n\n")
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

		// Create follow-up message
		followUpMessage := types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Role:      "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": userResponse,
				},
			},
		}

		// Continue streaming with follow-up
		fmt.Printf("📤 Sending follow-up: %s\n", userResponse)
		fmt.Print("📥 Continued streaming: ")

		// Use same context ID to continue the conversation
		contextID := currentContextID
		if contextID == "" {
			contextID = params.ContextID
		}

		followUpParams := types.MessageStreamParams{
			ContextID: contextID,
			Message:   followUpMessage,
		}

		// Continue streaming
		continuedEventChan, err := a2aClient.SendMessageStreaming(ctx, followUpParams)
		if err != nil {
			return fmt.Errorf("failed to continue streaming: %w", err)
		}

		// Process continued streaming events
		for event := range continuedEventChan {
			switch event.Type() {
			case types.EventDelta:
				var msg types.Message
				if err := event.DataAs(&msg); err == nil {
					text := extractMessageText(&msg)
					fmt.Print(text)
				}

			case types.EventTaskStatusChanged:
				var status types.TaskStatus
				if err := event.DataAs(&status); err == nil {
					if status.State == types.TaskStateCompleted {
						fmt.Printf("\n✅ Conversation complete!\n\n")
						return nil
					} else if status.State == types.TaskStateFailed {
						fmt.Printf("\n❌ Task failed\n\n")
						return nil
					}
				}

			case types.EventStreamFailed:
				fmt.Printf("\n❌ Stream failed\n\n")
				return nil
			}
		}

		fmt.Printf("\n✅ Stream completed!\n\n")
	} else {
		fmt.Printf("\n⚠️  Stream ended without clear completion\n\n")
	}

	return nil
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
		}
	}
	return "(no text content)"
}