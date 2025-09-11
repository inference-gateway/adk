package main

import (
	"context"
	"fmt"
	"log"
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

// handleMessageEvent processes message events with early returns for validation
func handleMessageEvent(eventMap map[string]any) {
	parts, ok := eventMap["parts"].([]any)
	if !ok {
		return
	}

	for _, part := range parts {
		partMap, ok := part.(map[string]any)
		if !ok {
			continue
		}

		text, exists := partMap["text"]
		if !exists {
			continue
		}

		fmt.Printf("💬 %v", text)
	}
}

// handleStatusUpdateEvent processes status update events with guard clauses
func handleStatusUpdateEvent(eventMap map[string]any) {
	taskId, hasTaskId := eventMap["taskId"]
	if !hasTaskId {
		return
	}

	status, hasStatus := eventMap["status"]
	if !hasStatus {
		return
	}

	fmt.Printf("📊 Status Update - Task: %v, Status: %v\n", taskId, status)
}

func main() {
	ctx := context.Background()

	// Setup phase with early returns for failures
	config, logger := setupApplication(ctx)
	defer logger.Sync()

	a2aClient := client.NewClientWithLogger(config.ServerURL, logger)

	// Validation phase - verify agent capabilities
	if err := validateAgentCapabilities(ctx, a2aClient, logger); err != nil {
		logger.Fatal("agent validation failed", zap.Error(err))
	}

	// Create timeout context for streaming
	streamCtx, cancel := context.WithTimeout(ctx, config.StreamingTimeout)
	defer cancel()

	// Prepare message parameters
	msgParams := createMessageParams()
	logger.Info("starting streaming task", zap.String("message_id", msgParams.Message.MessageID))

	// Execute streaming request with early return on error
	logger.Info("initiating streaming request")
	eventChan, err := a2aClient.SendTaskStreaming(streamCtx, msgParams)
	if err != nil {
		logger.Fatal("streaming task failed", zap.Error(err))
	}

	logger.Info("streaming task started successfully")
	logger.Info("=== Starting Stream Processing ===")

	var eventCount int
	streamingComplete := false

	// Process streaming events from the returned channel
	for !streamingComplete {
		select {
		case event, ok := <-eventChan:
			if !ok {
				logger.Info("=== Stream Channel Closed ===")
				streamingComplete = true
				break
			}

			eventCount++

			// The event.Result contains the actual streaming data
			if event.Result == nil {
				continue
			}

			// Process based on the result type
			eventData, ok := event.Result.(map[string]any)
			if !ok {
				logger.Info("received non-map event",
					zap.Int("event_number", eventCount),
					zap.Any("result", event.Result))
				fmt.Printf("[Event %d] Data: %v\n", eventCount, event.Result)
				continue
			}

			// Handle the event based on its kind
			eventType, exists := eventData["kind"]
			if !exists {
				logger.Info("received object event without kind",
					zap.Int("event_number", eventCount),
					zap.Any("event", eventData))
				fmt.Printf("[Event %d] Object: %v\n", eventCount, eventData)
				continue
			}

			switch eventType {
			case "message":
				handleMessageEvent(eventData)
			case "status-update":
				handleStatusUpdateEvent(eventData)
			default:
				logger.Info("received unknown event type",
					zap.Int("event_number", eventCount),
					zap.String("type", fmt.Sprintf("%v", eventType)),
					zap.Any("event", eventData))
				fmt.Printf("[Event %d] Unknown Event Type: %v\n", eventCount, eventData)
			}

		case <-streamCtx.Done():
			logger.Info("stream processing cancelled due to context timeout")
			streamingComplete = true
		}
	}

	logger.Info("streaming task completed successfully")

	// Results phase - display summary
	displayResults(eventCount, logger)
}

// setupApplication initializes configuration and logging with protective checks
func setupApplication(ctx context.Context) (Config, *zap.Logger) {
	var config Config
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatal("failed to process configuration", zap.Error(err))
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}

	logger.Info("starting a2a streaming example",
		zap.String("server_url", config.ServerURL),
		zap.Duration("streaming_timeout", config.StreamingTimeout))

	return config, logger
}

// validateAgentCapabilities checks agent streaming support with early returns
func validateAgentCapabilities(ctx context.Context, client client.A2AClient, logger *zap.Logger) error {
	logger.Info("checking agent capabilities")

	agentCard, err := client.GetAgentCard(ctx)
	if err != nil {
		return fmt.Errorf("failed to get agent card: %w", err)
	}

	logger.Info("agent card retrieved",
		zap.String("agent_name", agentCard.Name),
		zap.String("agent_version", agentCard.Version),
		zap.String("agent_description", agentCard.Description))

	// Guard clause: check streaming capability
	if agentCard.Capabilities.Streaming == nil || !*agentCard.Capabilities.Streaming {
		return fmt.Errorf("agent does not support streaming capabilities")
	}

	logger.Info("agent streaming capability verified",
		zap.Bool("streaming_supported", *agentCard.Capabilities.Streaming))

	return nil
}

// createMessageParams builds the message parameters for streaming
func createMessageParams() adk.MessageSendParams {
	return adk.MessageSendParams{
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
}

// displayResults shows the final streaming summary
func displayResults(eventCount int, logger *zap.Logger) {
	logger.Info("=== Streaming Summary ===",
		zap.Int("total_events", eventCount))

	fmt.Printf("\n=== Final Summary ===\n")
	fmt.Printf("Total events received: %d\n", eventCount)

	if eventCount == 0 {
		displayNoEventsWarning(logger)
		return
	}

	fmt.Printf("Streaming session completed successfully!\n")
}

// displayNoEventsWarning shows helpful information when no events are received
func displayNoEventsWarning(logger *zap.Logger) {
	fmt.Println("No streaming events received. This could indicate:")
	fmt.Println("- The server doesn't support streaming")
	fmt.Println("- The server is not configured for streaming responses")
	fmt.Println("- Network or connection issues")
	logger.Warn("no streaming events received - check server streaming capabilities")
}

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}
