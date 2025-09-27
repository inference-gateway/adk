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
	defer logger.Sync() //nolint:errcheck

	fmt.Println("🤖⚡ AI-Powered Streaming Client Demo")
	fmt.Printf("Connecting to: %s\n\n", serverURL)

	// Create client
	a2aClient := client.NewClientWithLogger(serverURL, logger)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Test 1: AI-powered streaming with weather query
	fmt.Println("=== Test 1: AI Streaming with Weather Query ===")
	testAIStreamingWeather(ctx, a2aClient, logger)

	// Test 2: AI-powered streaming with general conversation
	fmt.Println("\n=== Test 2: AI Streaming Conversation ===")
	testAIStreamingConversation(ctx, a2aClient, logger)

	// Test 3: Regular background task with AI
	fmt.Println("\n=== Test 3: Regular AI Task (Non-Streaming) ===")
	testRegularAITask(ctx, a2aClient, logger)

	fmt.Println("\n✅ All tests completed!")
}

func testAIStreamingWeather(ctx context.Context, a2aClient client.A2AClient, logger *zap.Logger) {
	// Create message requesting weather information
	message := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Can you get the weather for New York and then explain what activities would be good for that weather? Please stream your response as you think through this.",
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

	fmt.Printf("Request: %s\n", message.Parts[0].(map[string]any)["text"])
	fmt.Println("📡 AI Streaming Response:")

	// Test streaming
	eventChan, err := a2aClient.SendTaskStreaming(ctx, params)
	if err != nil {
		log.Printf("Failed to send streaming message: %v", err)
		return
	}

	var eventCount int
	var accumulatedText string

	// Process streaming events
	for event := range eventChan {
		eventCount++

		if event.Result == nil {
			continue
		}

		if deltaText := extractDeltaText(event.Result); deltaText != "" {
			accumulatedText += deltaText
			fmt.Print(deltaText) // Print delta in real-time
			logger.Debug("received delta",
				zap.Int("event", eventCount),
				zap.String("delta", deltaText))
		} else {
			logger.Debug("received non-delta event", zap.Int("event", eventCount))
		}
	}

	fmt.Printf("\n✅ Weather streaming completed. Total events: %d\n", eventCount)
}

func testAIStreamingConversation(ctx context.Context, a2aClient client.A2AClient, logger *zap.Logger) {
	// Create message for general AI conversation
	message := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Tell me an interesting story about artificial intelligence in the future. Make it engaging and stream your thoughts as you create the story.",
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

	fmt.Printf("Request: %s\n", message.Parts[0].(map[string]any)["text"])
	fmt.Println("📡 AI Streaming Response:")

	// Test streaming
	eventChan, err := a2aClient.SendTaskStreaming(ctx, params)
	if err != nil {
		log.Printf("Failed to send streaming message: %v", err)
		return
	}

	var eventCount int
	var accumulatedText string

	// Process streaming events
	for event := range eventChan {
		eventCount++

		if event.Result == nil {
			continue
		}

		if deltaText := extractDeltaText(event.Result); deltaText != "" {
			accumulatedText += deltaText
			fmt.Print(deltaText) // Print delta in real-time
			logger.Debug("received delta",
				zap.Int("event", eventCount),
				zap.String("delta", deltaText))
		} else {
			logger.Debug("received non-delta event", zap.Int("event", eventCount))
		}
	}

	fmt.Printf("\n✅ Conversation streaming completed. Total events: %d\n", eventCount)
}

func testRegularAITask(ctx context.Context, a2aClient client.A2AClient, logger *zap.Logger) {
	// Create message for regular AI task
	regularMessage := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "What's the current time and how can I improve my productivity?",
			},
		},
	}

	regularParams := types.MessageSendParams{
		Message: regularMessage,
	}

	fmt.Printf("Request: %s\n", regularMessage.Parts[0].(map[string]any)["text"])

	response, err := a2aClient.SendTask(ctx, regularParams)
	if err != nil {
		log.Printf("Failed to send regular message: %v", err)
		return
	}

	// Display the response
	if response.Result != nil {
		if task, ok := response.Result.(*types.Task); ok && task.Status.Message != nil {
			responseText := extractTextFromParts(task.Status.Message.Parts)
			if responseText != "" {
				fmt.Printf("AI Response: %s\n", responseText)
			} else {
				responseJSON, _ := json.MarshalIndent(response.Result, "", "  ")
				fmt.Printf("Response:\n%s\n", string(responseJSON))
			}
		} else {
			responseJSON, _ := json.MarshalIndent(response.Result, "", "  ")
			fmt.Printf("Response:\n%s\n", string(responseJSON))
		}
	}

	fmt.Println("✅ Regular AI task completed")
}

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}
