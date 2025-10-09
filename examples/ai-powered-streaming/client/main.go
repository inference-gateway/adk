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

// extractTextFromParts extracts text from message parts
func extractTextFromParts(parts []types.Part) string {
	for _, part := range parts {
		if textPart, ok := part.(types.TextPart); ok {
			return textPart.Text
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
				types.TextPart{
					Kind: "text",
					Text: task.text,
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

		eventChan, err := a2aClient.SendTaskStreaming(ctx, params)
		if err != nil {
			log.Printf("❌ Failed: %v", err)
			continue
		}

		fmt.Printf("%s Response: ", task.icon)

		eventCount := 0

		for event := range eventChan {
			eventCount++
			if event.Result == nil {
				continue
			}

			// According to A2A spec, all streaming responses are TaskStatusUpdateEvent
			resultBytes, _ := json.Marshal(event.Result)
			var statusUpdate types.TaskStatusUpdateEvent
			if err := json.Unmarshal(resultBytes, &statusUpdate); err != nil {
				logger.Debug("failed to parse status update", zap.Error(err))
				continue
			}

			// Handle status update event - only sent when status actually changes
			logger.Info("task status changed",
				zap.Int("event", eventCount),
				zap.String("new_state", string(statusUpdate.Status.State)),
				zap.String("task_id", statusUpdate.TaskID),
				zap.Bool("final", statusUpdate.Final))

			// If status includes a message (e.g., completion with final text), display it
			if statusUpdate.Status.Message != nil {
				text := extractTextFromParts(statusUpdate.Status.Message.Parts)
				if text != "" {
					logger.Info("received final message",
						zap.Int("text_length", len(text)))
					fmt.Print(text)
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
