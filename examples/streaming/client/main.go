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
	ServerURL   string `env:"SERVER_URL,default=http://localhost:8080"`
}

func main() {
	// Load configuration
	ctx := context.Background()
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize logger based on environment
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

	logger.Info("client starting", zap.String("server_url", cfg.ServerURL))

	// Create client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test streaming capabilities
	logger.Info("testing streaming capabilities")

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

	logger.Info("sending streaming request", zap.String("prompt", message.Parts[0].(types.TextPart).Text))

	// Test streaming
	eventChan, err := a2aClient.SendTaskStreaming(ctx, params)
	if err != nil {
		logger.Error("failed to send streaming message", zap.Error(err))
		return
	}

	logger.Info("streaming response started")
	var eventCount int
	var finalResponse string

	// Process streaming events (delta updates + status changes)
	for event := range eventChan {
		eventCount++

		if event.Result == nil {
			continue
		}

		resultBytes, _ := json.Marshal(event.Result)

		// Try to parse as Task (for delta events)
		var task types.Task
		if err := json.Unmarshal(resultBytes, &task); err == nil && task.Kind == "task" {
			// Handle delta message
			if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
				for _, part := range task.Status.Message.Parts {
					if textPart, ok := part.(types.TextPart); ok {
						fmt.Print(textPart.Text)
						finalResponse += textPart.Text
					}
				}
			}
			continue
		}

		// Try to parse as TaskStatusUpdateEvent (for status changes)
		var statusUpdate types.TaskStatusUpdateEvent
		if err := json.Unmarshal(resultBytes, &statusUpdate); err == nil && statusUpdate.Kind == "status-update" {
			// Handle different task states
			switch statusUpdate.Status.State {
			case types.TaskStateWorking:
				logger.Info("task started", zap.Int("event", eventCount))

			case types.TaskStateCompleted:
				logger.Info("task completed", zap.Int("event", eventCount))

			case types.TaskStateFailed:
				logger.Error("task failed", zap.Int("event", eventCount))

			case types.TaskStateCancelled:
				logger.Info("task canceled", zap.Int("event", eventCount))
			}
			continue
		}

		logger.Debug("unknown event type", zap.Int("event", eventCount))
	}

	logger.Info("streaming completed", zap.Int("total_events", eventCount))
	fmt.Printf("\n\nFinal streamed response:\n%s\n", finalResponse)
}

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}
