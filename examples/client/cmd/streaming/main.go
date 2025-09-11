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
			processEvent(event, eventCount, logger)

		case <-ctx.Done():
			logger.Info("stream processing cancelled due to context timeout")
			return eventCount
		}
	}
}

// processEvent handles individual streaming events with reduced nesting
func processEvent(event any, eventCount int, logger *zap.Logger) {
	switch v := event.(type) {
	case map[string]any:
		handleMapEvent(v, eventCount, logger)
	default:
		handleGenericEvent(v, eventCount, logger)
	}
}


// handleMapEvent processes complex event objects
func handleMapEvent(eventMap map[string]any, eventCount int, logger *zap.Logger) {
	eventType, exists := eventMap["kind"]
	if !exists {
		handleUntypedObjectEvent(eventMap, eventCount, logger)
		return
	}

	switch eventType {
	case "message":
		handleMessageEvent(eventMap, eventCount, logger)
	case "status-update":
		handleStatusUpdateEvent(eventMap, eventCount, logger)
	default:
		handleUnknownEventType(eventMap, eventType, eventCount, logger)
	}
}

// handleMessageEvent processes message events with early returns for validation
func handleMessageEvent(eventMap map[string]any, eventCount int, logger *zap.Logger) {
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

		logger.Info("received streaming message part",
			zap.Int("event_number", eventCount),
			zap.String("text", fmt.Sprintf("%v", text)))
		fmt.Printf("[Event %d] Message: %v\n", eventCount, text)
	}
}

// handleStatusUpdateEvent processes status update events with guard clauses
func handleStatusUpdateEvent(eventMap map[string]any, eventCount int, logger *zap.Logger) {
	taskId, hasTaskId := eventMap["taskId"]
	if !hasTaskId {
		return
	}

	status, hasStatus := eventMap["status"]
	if !hasStatus {
		return
	}

	logger.Info("received task status update",
		zap.Int("event_number", eventCount),
		zap.String("task_id", fmt.Sprintf("%v", taskId)),
		zap.Any("status", status))
	fmt.Printf("[Event %d] Status Update - Task: %v, Status: %v\n", eventCount, taskId, status)
}

// handleUnknownEventType processes unknown event types
func handleUnknownEventType(eventMap map[string]any, eventType any, eventCount int, logger *zap.Logger) {
	logger.Info("received unknown event type",
		zap.Int("event_number", eventCount),
		zap.String("type", fmt.Sprintf("%v", eventType)),
		zap.Any("event", eventMap))
	fmt.Printf("[Event %d] Unknown Event Type: %v\n", eventCount, eventMap)
}

// handleUntypedObjectEvent processes object events without a kind field
func handleUntypedObjectEvent(eventMap map[string]any, eventCount int, logger *zap.Logger) {
	logger.Info("received untyped object event",
		zap.Int("event_number", eventCount),
		zap.Any("event", eventMap))
	fmt.Printf("[Event %d] Object: %v\n", eventCount, eventMap)
}

// handleGenericEvent processes any other type of event
func handleGenericEvent(event any, eventCount int, logger *zap.Logger) {
	logger.Info("received generic event",
		zap.Int("event_number", eventCount),
		zap.Any("event", event))
	fmt.Printf("[Event %d] Generic: %v\n", eventCount, event)
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

	// Execution phase - run streaming task
	eventCount := executeStreamingTask(ctx, a2aClient, config, logger)

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
func validateAgentCapabilities(ctx context.Context, client *client.A2AClient, logger *zap.Logger) error {
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

// executeStreamingTask runs the streaming operation and returns event count
func executeStreamingTask(ctx context.Context, a2aClient *client.A2AClient, config Config, logger *zap.Logger) int {
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
	eventCount := processStreamingEvents(streamCtx, eventChan, logger)
	logger.Info("streaming task completed successfully")

	return eventCount
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
