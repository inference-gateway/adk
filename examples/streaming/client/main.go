package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/inference-gateway/adk/client"
	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

func main() {
	// Get server URL from environment or use default
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create client
	a2aClient := client.NewClientWithLogger(serverURL, logger)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test streaming capabilities
	fmt.Println("🚀 Testing streaming capabilities...")

	// Create message with proper structure for streaming
	message := types.Message{
		Role: "user",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: "Please write a detailed explanation about machine learning. Stream your response as you generate it.",
			},
		},
	}

	// Send the streaming message
	params := types.MessageSendParams{
		Message: message,
		Configuration: &types.MessageSendConfiguration{
			Blocking:            boolPtr(false),
			AcceptedOutputModes: []string{"text/plain"},
		},
	}

	fmt.Printf("Sending streaming request: %s\n", message.Parts[0].(types.TextPart).Text)

	// Test streaming
	eventChan, err := a2aClient.SendTaskStreaming(ctx, params)
	if err != nil {
		log.Printf("Failed to send streaming message: %v", err)
		return
	}

	fmt.Println("📡 Streaming response:")
	var eventCount int
	var finalResponse string

	// Process streaming events (expect 2: working → completed)
	for event := range eventChan {
		eventCount++

		// Parse status update
		if event.Result == nil {
			continue
		}

		resultBytes, _ := json.Marshal(event.Result)
		var statusUpdate types.TaskStatusUpdateEvent
		if err := json.Unmarshal(resultBytes, &statusUpdate); err != nil {
			logger.Debug("failed to parse event", zap.Int("event", eventCount), zap.Error(err))
			continue
		}

		// Handle different task states
		switch statusUpdate.Status.State {
		case types.TaskStateWorking:
			logger.Info("task started", zap.Int("event", eventCount))

		case types.TaskStateCompleted:
			logger.Info("task completed", zap.Int("event", eventCount))
			// Extract final message
			if statusUpdate.Status.Message != nil && len(statusUpdate.Status.Message.Parts) > 0 {
				if textPart, ok := statusUpdate.Status.Message.Parts[0].(types.TextPart); ok {
					finalResponse = textPart.Text
				}
			}

		case types.TaskStateFailed:
			logger.Error("task failed", zap.Int("event", eventCount))

		case types.TaskStateCanceled:
			logger.Info("task canceled", zap.Int("event", eventCount))

		default:
			logger.Debug("unknown state",
				zap.Int("event", eventCount),
				zap.String("state", string(statusUpdate.Status.State)))
		}
	}

	fmt.Printf("\n✅ Streaming completed. Total events: %d\n", eventCount)
	if finalResponse != "" {
		fmt.Printf("📝 Response:\n%s\n", finalResponse)
	}

	// Also test regular (non-streaming) message for comparison
	fmt.Println("\n--- Testing regular message ---")

	regularMessage := types.Message{
		Role: "user",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: "What is the capital of France?",
			},
		},
	}

	regularParams := types.MessageSendParams{
		Message: regularMessage,
	}

	response, err := a2aClient.SendTask(ctx, regularParams)
	if err != nil {
		log.Printf("Failed to send regular message: %v", err)
		return
	}

	// Display the response
	if response.Result != nil {
		responseJSON, _ := json.MarshalIndent(response.Result, "", "  ")
		fmt.Printf("Regular response:\n%s\n", string(responseJSON))
	}
}

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}
