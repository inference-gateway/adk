package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/inference-gateway/adk/client"
	"github.com/inference-gateway/adk/types"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
)

// Config holds the client configuration
type Config struct {
	ServerURL     string        `env:"SERVER_URL,default=http://localhost:8080"`
	ClientTimeout time.Duration `env:"CLIENT_TIMEOUT,default=90s"`
}

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
	// Load configuration from environment
	var config Config
	if err := envconfig.Process(context.Background(), &config); err != nil {
		log.Fatalf("Failed to process config: %v", err)
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	fmt.Println("🤖 AI-Powered Streaming Demo")
	fmt.Printf("Server: %s\n\n", config.ServerURL)

	a2aClient := client.NewClientWithLogger(config.ServerURL, logger)
	ctx, cancel := context.WithTimeout(context.Background(), config.ClientTimeout)
	defer cancel()

	tasks := []struct {
		name string
		text string
		icon string
	}{
		{"Ask about weather", "What's the weather in New York? Suggest activities for that weather.", "🌤️"},
		// {"Request a story", "Tell me a short story about AI in the future.", "📖"},
	}

	for i, task := range tasks {
		fmt.Printf("%d️⃣ STREAMING: %s\n", i+1, task.name)

		message := types.Message{
			Role: "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": task.text,
				},
			},
		}

		params := types.MessageSendParams{
			Message: message,
			Configuration: &types.MessageSendConfiguration{
				Blocking:            boolPtr(false),
				AcceptedOutputModes: []string{"text/plain"},
			},
		}

		fmt.Printf("%s Response: ", task.icon)

		eventChan, err := a2aClient.SendTaskStreaming(ctx, params)
		if err != nil {
			log.Printf("❌ Failed: %v", err)
			continue
		}

		eventCount := 0
		for event := range eventChan {
			eventCount++
			if event.Result != nil {
				if deltaText := extractDeltaText(event.Result); deltaText != "" {
					fmt.Print(deltaText)
				}
			}
		}

		fmt.Printf("\n✅ Streamed %d events\n\n", eventCount)
	}

	fmt.Println("✅ Demo completed!")
}

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}
