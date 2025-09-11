package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/inference-gateway/adk/client"
	adk "github.com/inference-gateway/adk/types"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
)

// Config represents the application configuration
type Config struct {
	ServerURL        string        `env:"A2A_SERVER_URL,default=http://localhost:8080"`
	StreamingTimeout time.Duration `env:"STREAMING_TIMEOUT,default=60s"`
}

// processStreamingEvents handles the streaming event loop and returns the total event count
func processStreamingEvents(ctx context.Context, eventChan <-chan any, logger *zap.Logger) int {
	logger.Info("=== Starting Stream Processing ===")

	var eventCount int

	// Process streaming events from the returned channel
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				logger.Info("=== Stream Channel Closed ===")
				return eventCount
			}

			eventCount++

			// Handle different types of streaming events
			switch v := event.(type) {
			case string:
				// Simple string events
				logger.Info("received streaming text",
					zap.Int("event_number", eventCount),
					zap.String("content", v))
				fmt.Printf("[Event %d] Text: %s\n", eventCount, v)

			case map[string]any:
				// Complex event objects (e.g., task updates, messages)
				if eventType, exists := v["kind"]; exists {
					switch eventType {
					case "message":
						if parts, ok := v["parts"].([]any); ok {
							for _, part := range parts {
								if partMap, ok := part.(map[string]any); ok {
									if text, exists := partMap["text"]; exists {
										logger.Info("received streaming message part",
											zap.Int("event_number", eventCount),
											zap.String("text", fmt.Sprintf("%v", text)))
										fmt.Printf("[Event %d] Message: %v\n", eventCount, text)
									}
								}
							}
						}

					case "status-update":
						if taskId, exists := v["taskId"]; exists {
							if status, exists := v["status"]; exists {
								logger.Info("received task status update",
									zap.Int("event_number", eventCount),
									zap.String("task_id", fmt.Sprintf("%v", taskId)),
									zap.Any("status", status))
								fmt.Printf("[Event %d] Status Update - Task: %v, Status: %v\n", eventCount, taskId, status)
							}
						}

					default:
						logger.Info("received unknown event type",
							zap.Int("event_number", eventCount),
							zap.String("type", fmt.Sprintf("%v", eventType)),
							zap.Any("event", v))
						fmt.Printf("[Event %d] Unknown Event Type: %v\n", eventCount, v)
					}
				} else {
					logger.Info("received untyped object event",
						zap.Int("event_number", eventCount),
						zap.Any("event", v))
					fmt.Printf("[Event %d] Object: %v\n", eventCount, v)
				}

			default:
				// Handle any other type of event
				logger.Info("received generic event",
					zap.Int("event_number", eventCount),
					zap.Any("event", v))
				fmt.Printf("[Event %d] Generic: %v\n", eventCount, v)
			}

		case <-ctx.Done():
			logger.Info("stream processing cancelled due to context timeout")
			return eventCount
		}
	}
}

func main() {
	// Load configuration from environment variables
	ctx := context.Background()
	var config Config
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatal("failed to process configuration", zap.Error(err))
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("starting a2a streaming example",
		zap.String("server_url", config.ServerURL),
		zap.Duration("streaming_timeout", config.StreamingTimeout))

	// Create A2A client
	a2aClient := client.NewClientWithLogger(config.ServerURL, logger)

	// Check agent capabilities first
	logger.Info("checking agent capabilities")
	agentCard, err := a2aClient.GetAgentCard(ctx)
	if err != nil {
		logger.Fatal("failed to get agent card", zap.Error(err))
	}

	logger.Info("agent card retrieved",
		zap.String("agent_name", agentCard.Name),
		zap.String("agent_version", agentCard.Version),
		zap.String("agent_description", agentCard.Description))

	// Verify streaming capability
	if agentCard.Capabilities.Streaming == nil || !*agentCard.Capabilities.Streaming {
		logger.Fatal("agent does not support streaming capabilities",
			zap.String("agent_name", agentCard.Name),
			zap.Bool("streaming_supported", agentCard.Capabilities.Streaming != nil && *agentCard.Capabilities.Streaming))
	}

	logger.Info("agent streaming capability verified",
		zap.Bool("streaming_supported", *agentCard.Capabilities.Streaming))

	// Create context with timeout for streaming
	ctx, cancel := context.WithTimeout(context.Background(), config.StreamingTimeout)
	defer cancel()

	// Prepare message parameters for streaming
	msgParams := adk.MessageSendParams{
		Message: adk.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("streaming-msg-%d", time.Now().Unix()),
			Role:      "user",
			Parts: []adk.Part{
				map[string]any{
					"kind": "text",
					"text": "Please write a detailed explanation about machine learning in artificial intelligence. Stream your response as you generate it.",
				},
			},
		},
		Configuration: &adk.MessageSendConfiguration{
			Blocking:            boolPtr(false),
			AcceptedOutputModes: []string{"text"},
		},
	}

	logger.Info("starting streaming task",
		zap.String("message_id", msgParams.Message.MessageID))

	// Track streaming progress
	var eventCount int
	var streamError error

	// Start streaming task
	logger.Info("initiating streaming request")
	eventChan, err := a2aClient.SendTaskStreaming(ctx, msgParams)
	if err != nil {
		streamError = err
		logger.Error("streaming task failed", zap.Error(err))
	} else {
		logger.Info("streaming task started successfully")
		eventCount = processStreamingEvents(ctx, eventChan, logger)
		logger.Info("streaming task completed successfully")
	}

	// Display final results
	logger.Info("=== Streaming Summary ===")

	if streamError != nil {
		logger.Fatal("streaming failed",
			zap.Error(streamError),
			zap.Int("events_received", eventCount))
	}

	logger.Info("streaming completed successfully",
		zap.Int("total_events", eventCount),
		zap.Duration("total_time", time.Since(time.Now().Add(-config.StreamingTimeout))))

	fmt.Printf("\n=== Final Summary ===\n")
	fmt.Printf("Total events received: %d\n", eventCount)

	if eventCount == 0 {
		fmt.Println("No streaming events received. This could indicate:")
		fmt.Println("- The server doesn't support streaming")
		fmt.Println("- The server is not configured for streaming responses")
		fmt.Println("- Network or connection issues")
		logger.Warn("no streaming events received - check server streaming capabilities")
	} else {
		fmt.Printf("Streaming session completed successfully!\n")
	}
}

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}
