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

// extractDeltaText attempts to extract delta text from various event structures
func extractDeltaText(result any) string {
	resultBytes, _ := json.Marshal(result)

	// Try TaskStatusUpdateEvent
	var taskEvent types.TaskStatusUpdateEvent
	if err := json.Unmarshal(resultBytes, &taskEvent); err == nil && taskEvent.Status.Message != nil {
		return extractTextFromParts(taskEvent.Status.Message.Parts)
	}

	// Try Task directly
	var task types.Task
	if err := json.Unmarshal(resultBytes, &task); err == nil && task.Status.Message != nil {
		return extractTextFromParts(task.Status.Message.Parts)
	}

	return ""
}

// extractTextFromParts extracts text from message parts
func extractTextFromParts(parts []types.Part) string {
	for _, part := range parts {
		if partMap, ok := part.(map[string]any); ok {
			if text, ok := partMap["text"].(string); ok {
				return text
			}
		}
	}
	return ""
}

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
			map[string]any{
				"kind": "text",
				"text": "Please write a detailed explanation about machine learning. Stream your response as you generate it.",
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

	fmt.Printf("Sending streaming request: %s\n", message.Parts[0].(map[string]any)["text"])

	// Test streaming
	eventChan, err := a2aClient.SendTaskStreaming(ctx, params)
	if err != nil {
		log.Printf("Failed to send streaming message: %v", err)
		return
	}

	fmt.Println("📡 Streaming response:")
	var eventCount int

	// Process streaming events
	var accumulatedText string
	for event := range eventChan {
		eventCount++

		if event.Result == nil {
			continue
		}

		if deltaText := extractDeltaText(event.Result); deltaText != "" {
			accumulatedText += deltaText
			logger.Info("received delta",
				zap.Int("event", eventCount),
				zap.String("delta", deltaText))
		} else {
			logger.Debug("received non-delta event", zap.Int("event", eventCount))
		}
	}

	fmt.Printf("✅ Streaming completed. Total events: %d\n", eventCount)

	// Also test regular (non-streaming) message for comparison
	fmt.Println("\n--- Testing regular message ---")

	regularMessage := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "What is the capital of France?",
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
