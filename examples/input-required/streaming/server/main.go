package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	config "github.com/inference-gateway/adk/examples/input-required/streaming/server/config"
	server "github.com/inference-gateway/adk/server"
	types "github.com/inference-gateway/adk/types"
)

// StreamingInputRequiredTaskHandler demonstrates streaming with input-required flow
type StreamingInputRequiredTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// NewStreamingInputRequiredTaskHandler creates a new StreamingInputRequiredTaskHandler
func NewStreamingInputRequiredTaskHandler(logger *zap.Logger) *StreamingInputRequiredTaskHandler {
	return &StreamingInputRequiredTaskHandler{
		logger: logger,
	}
}

// SetAgent sets the agent for the task handler
func (h *StreamingInputRequiredTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured agent
func (h *StreamingInputRequiredTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// HandleStreamingTask processes tasks with real-time streaming and input-required flow
func (h *StreamingInputRequiredTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error) {
	h.logger.Info("processing streaming task with input-required demonstration",
		zap.String("task_id", task.ID),
		zap.String("message_role", message.Role))

	outputChan := make(chan cloudevents.Event, 100)

	go func() {
		defer close(outputChan)

		// Add incoming message to task history
		task.History = append(task.History, *message)

		// If we have an agent, use it for processing
		if h.agent != nil {
			h.processWithAgentStreaming(ctx, task, message, outputChan)
		} else {
			h.processWithoutAgentStreaming(ctx, task, message, outputChan)
		}
	}()

	return outputChan, nil
}

// processWithAgentStreaming uses the AI agent for streaming processing
func (h *StreamingInputRequiredTaskHandler) processWithAgentStreaming(ctx context.Context, task *types.Task, message *types.Message, outputChan chan<- cloudevents.Event) {
	h.logger.Info("processing with AI agent streaming", zap.String("task_id", task.ID))

	// Prepare messages for agent
	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

	// Create context with task information
	toolCtx := context.WithValue(ctx, server.TaskContextKey, task)

	// Process with agent streaming
	agentEventChan, err := h.agent.RunWithStream(toolCtx, messages)
	if err != nil {
		h.logger.Error("agent streaming failed", zap.Error(err))
		h.sendErrorEvent(outputChan, task.ID, fmt.Sprintf("Failed to start agent streaming: %v", err))
		return
	}

	// Forward all events from agent to output channel
	var finalMessage *types.Message
	var inputRequiredMessage *types.Message

	for event := range agentEventChan {
		// Forward the event to client
		select {
		case outputChan <- event:
		case <-ctx.Done():
			return
		}

		// Track important events
		switch event.Type() {
		case types.EventDelta:
			var msg types.Message
			if err := event.DataAs(&msg); err == nil {
				finalMessage = &msg
			}

		case types.EventInputRequired:
			var msg types.Message
			if err := event.DataAs(&msg); err == nil {
				inputRequiredMessage = &msg
				h.logger.Info("agent requested user input via streaming",
					zap.String("task_id", task.ID),
					zap.String("message", getMessageText(&msg)))
			}

		case types.EventIterationCompleted:
			var msg types.Message
			if err := event.DataAs(&msg); err == nil {
				finalMessage = &msg
			}
		}
	}

	// Send final task status
	if inputRequiredMessage != nil {
		h.sendTaskStatusEvent(outputChan, task.ID, types.TaskStateInputRequired)
	} else if finalMessage != nil {
		h.sendTaskStatusEvent(outputChan, task.ID, types.TaskStateCompleted)
	} else {
		h.sendTaskStatusEvent(outputChan, task.ID, types.TaskStateFailed)
	}
}

// processWithoutAgentStreaming demonstrates streaming without AI
func (h *StreamingInputRequiredTaskHandler) processWithoutAgentStreaming(ctx context.Context, task *types.Task, message *types.Message, outputChan chan<- cloudevents.Event) {
	h.logger.Info("processing without AI agent - demonstrating streaming input-required flow",
		zap.String("task_id", task.ID))

	messageText := getMessageText(message)
	h.logger.Info("received streaming message", zap.String("text", messageText))

	// Simulate thinking with streaming status
	h.sendStreamingStatus(outputChan, task.ID, "Analyzing your request...")
	time.Sleep(500 * time.Millisecond)

	// Check if this is a follow-up to a previous input-required state
	previousContext := getPreviousContext(task.History)
	h.logger.Info("previous context", zap.String("context", previousContext))

	// If this looks like a follow-up response (short message without keywords)
	if previousContext != "" && !contains(messageText, "weather") && !contains(messageText, "calculate") && !contains(messageText, "hello") && len(messageText) < 50 {
		switch previousContext {
		case "weather":
			// User provided location for weather query
			h.sendStreamingText(outputChan, task.ID, fmt.Sprintf("Great! Let me check the weather for %s... ", messageText))
			time.Sleep(800 * time.Millisecond)
			h.sendStreamingText(outputChan, task.ID, "The weather is sunny and 72°F! ")
			time.Sleep(300 * time.Millisecond)
			h.sendStreamingText(outputChan, task.ID, "Perfect day for outdoor activities!")
			h.sendTaskStatusEvent(outputChan, task.ID, types.TaskStateCompleted)
			return

		case "calculate":
			// User provided numbers for calculation
			h.sendStreamingText(outputChan, task.ID, "Let me calculate that for you... ")
			time.Sleep(500 * time.Millisecond)
			h.sendStreamingText(outputChan, task.ID, "The result is 42! ")
			time.Sleep(300 * time.Millisecond)
			h.sendStreamingText(outputChan, task.ID, "(Mock calculation)")
			h.sendTaskStatusEvent(outputChan, task.ID, types.TaskStateCompleted)
			return
		}
	}

	// Determine response based on message content
	switch {
	case contains(messageText, "weather"):
		if !contains(messageText, "in ") && !contains(messageText, "at ") {
			// Need location - request input
			h.sendStreamingText(outputChan, task.ID, "I'd be happy to help you with the weather! ")
			time.Sleep(300 * time.Millisecond)
			h.sendInputRequiredEvent(outputChan, task.ID, "Could you please specify which location you'd like the weather for?")
		} else {
			// Have location - provide weather
			h.sendStreamingText(outputChan, task.ID, "Let me check the weather for you... ")
			time.Sleep(800 * time.Millisecond)
			h.sendStreamingText(outputChan, task.ID, "The weather is sunny and 72°F! ")
			time.Sleep(300 * time.Millisecond)
			h.sendStreamingText(outputChan, task.ID, "(This is a demo response)")
			h.sendTaskStatusEvent(outputChan, task.ID, types.TaskStateCompleted)
		}

	case contains(messageText, "calculate") || contains(messageText, "math"):
		if !hasNumbers(messageText) {
			// Need numbers - request input
			h.sendStreamingText(outputChan, task.ID, "I can help you with calculations! ")
			time.Sleep(300 * time.Millisecond)
			h.sendInputRequiredEvent(outputChan, task.ID, "Could you please provide the specific numbers or equation you'd like me to calculate?")
		} else {
			// Have numbers - provide calculation
			h.sendStreamingText(outputChan, task.ID, "Let me work on that calculation... ")
			time.Sleep(600 * time.Millisecond)
			h.sendStreamingText(outputChan, task.ID, "Based on your calculation request, I can help you with that math problem! ")
			time.Sleep(300 * time.Millisecond)
			h.sendStreamingText(outputChan, task.ID, "(This is a demo response)")
			h.sendTaskStatusEvent(outputChan, task.ID, types.TaskStateCompleted)
		}

	case contains(messageText, "hello") || contains(messageText, "hi"):
		// Simple greeting
		h.sendStreamingText(outputChan, task.ID, "Hello! ")
		time.Sleep(400 * time.Millisecond)
		h.sendStreamingText(outputChan, task.ID, "I'm an assistant that demonstrates the input-required flow with streaming. ")
		time.Sleep(400 * time.Millisecond)
		h.sendStreamingText(outputChan, task.ID, "Try asking me about the weather or a calculation to see how I request additional information!")
		h.sendTaskStatusEvent(outputChan, task.ID, types.TaskStateCompleted)

	default:
		// Unclear request - ask for clarification
		h.sendStreamingText(outputChan, task.ID, "I'd be happy to help! ")
		time.Sleep(300 * time.Millisecond)
		h.sendInputRequiredEvent(outputChan, task.ID, "Could you please provide more details about what you'd like me to do? For example, you could ask about the weather or request a calculation.")
	}
}

// Helper methods for sending events
func (h *StreamingInputRequiredTaskHandler) sendStreamingText(outputChan chan<- cloudevents.Event, taskID, text string) {
	// Split text into chunks for realistic streaming
	words := strings.Fields(text)
	for i, word := range words {
		chunk := word
		if i < len(words)-1 {
			chunk += " "
		}

		deltaMessage := &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("delta-%s-%d", taskID, time.Now().UnixNano()),
			Role:      "assistant",
			Parts: []types.Part{
				types.TextPart{
					Kind: "text",
					Text: chunk,
				},
			},
		}

		event := types.NewDeltaEvent(deltaMessage)
		select {
		case outputChan <- event:
		default:
		}

		time.Sleep(50 * time.Millisecond) // Simulate typing delay
	}
}

func (h *StreamingInputRequiredTaskHandler) sendStreamingStatus(outputChan chan<- cloudevents.Event, taskID, status string) {
	statusMessage := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("status-%s-%d", taskID, time.Now().UnixNano()),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]any{
				"kind": "data",
				"data": map[string]any{
					"status": status,
				},
			},
		},
	}

	event := types.NewMessageEvent(types.EventTaskStatusChanged, statusMessage.MessageID, statusMessage)
	select {
	case outputChan <- event:
	default:
	}
}

func (h *StreamingInputRequiredTaskHandler) sendInputRequiredEvent(outputChan chan<- cloudevents.Event, taskID, message string) {
	inputMessage := &types.Message{
		Kind:      "input_required",
		MessageID: fmt.Sprintf("input-required-%s-%d", taskID, time.Now().UnixNano()),
		Role:      "assistant",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: message,
			},
		},
	}

	event := types.NewMessageEvent(types.EventInputRequired, inputMessage.MessageID, inputMessage)
	select {
	case outputChan <- event:
	default:
	}

	h.sendTaskStatusEvent(outputChan, taskID, types.TaskStateInputRequired)
}

func (h *StreamingInputRequiredTaskHandler) sendTaskStatusEvent(outputChan chan<- cloudevents.Event, taskID string, state types.TaskState) {
	statusEvent := cloudevents.NewEvent()
	statusEvent.SetID(fmt.Sprintf("status-%s-%d", taskID, time.Now().UnixNano()))
	statusEvent.SetType(types.EventTaskStatusChanged)
	statusEvent.SetSource("adk/task-handler")
	statusEvent.SetTime(time.Now())
	_ = statusEvent.SetData(cloudevents.ApplicationJSON, types.TaskStatus{
		State: state,
	})

	select {
	case outputChan <- statusEvent:
	default:
	}
}

func (h *StreamingInputRequiredTaskHandler) sendErrorEvent(outputChan chan<- cloudevents.Event, taskID, errorMessage string) {
	errorEvent := cloudevents.NewEvent()
	errorEvent.SetID(fmt.Sprintf("error-%s-%d", taskID, time.Now().UnixNano()))
	errorEvent.SetType(types.EventStreamFailed)
	errorEvent.SetSource("adk/task-handler")
	errorEvent.SetTime(time.Now())
	_ = errorEvent.SetData(cloudevents.ApplicationJSON, map[string]any{
		"error": errorMessage,
	})

	select {
	case outputChan <- errorEvent:
	default:
	}

	h.sendTaskStatusEvent(outputChan, taskID, types.TaskStateFailed)
}

// Helper functions (same as non-streaming version)
func getMessageText(message *types.Message) string {
	for _, part := range message.Parts {
		// Handle typed TextPart (when created locally)
		if textPart, ok := part.(types.TextPart); ok {
			return textPart.Text
		}
		// Handle map-based parts (when deserialized from JSON)
		if partMap, ok := part.(map[string]any); ok {
			if kind, exists := partMap["kind"]; exists && kind == "text" {
				if text, exists := partMap["text"].(string); exists {
					return text
				}
			}
		}
	}
	return ""
}

func contains(text, substr string) bool {
	return len(text) >= len(substr) &&
		findInLower(toLower(text), toLower(substr)) >= 0
}

// getPreviousContext analyzes the conversation history to determine
// what kind of input was previously requested
func getPreviousContext(history []types.Message) string {
	// Look backwards through history for input_required messages
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Kind == "input_required" {
			text := getMessageText(&msg)
			textLower := toLower(text)

			// Determine what was being asked about
			if contains(textLower, "weather") || contains(textLower, "location") {
				return "weather"
			}
			if contains(textLower, "calculat") || contains(textLower, "number") {
				return "calculate"
			}
		}
	}
	return ""
}

func hasNumbers(text string) bool {
	for _, r := range text {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]rune, len([]rune(s)))
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			result[i] = r - 'A' + 'a'
		} else {
			result[i] = r
		}
	}
	return string(result)
}

func findInLower(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Load configuration from environment
	cfg := &config.Config{}
	ctx := context.Background()
	if err := envconfig.Process(ctx, cfg); err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	logger.Info("starting input-required streaming server",
		zap.String("port", cfg.A2A.ServerConfig.Port),
		zap.Bool("ai_enabled", cfg.A2A.AgentConfig.Provider != ""))

	// Create task handler
	taskHandler := NewStreamingInputRequiredTaskHandler(logger)

	// Add AI agent if configured
	if cfg.A2A.AgentConfig.Provider != "" {
		logger.Info("configuring AI agent for streaming",
			zap.String("provider", cfg.A2A.AgentConfig.Provider),
			zap.String("model", cfg.A2A.AgentConfig.Model))

		// Create LLM client
		llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.A2A.AgentConfig, logger)
		if err != nil {
			logger.Fatal("failed to create LLM client", zap.Error(err))
		}

		// Create default toolbox (includes input_required tool)
		toolBox := server.NewDefaultToolBox()

		// Create agent with default toolbox
		agent, err := server.NewAgentBuilder(logger).
			WithConfig(&cfg.A2A.AgentConfig).
			WithLLMClient(llmClient).
			WithSystemPrompt(`You are a helpful assistant that demonstrates the input-required flow with real-time streaming.

When users ask for information that requires additional details (like weather without location, calculations without numbers, or unclear requests), use the input_required tool to ask for the missing information.

Examples:
- If asked "What's the weather?" without a location, use input_required to ask for the location
- If asked "Calculate this" without numbers, use input_required to ask for the specific calculation
- If the request is unclear or ambiguous, use input_required to ask for clarification

Be specific about what information you need and why it's needed to provide a complete answer. Your responses will be streamed in real-time to provide a better user experience.`).
			WithToolBox(toolBox).
			Build()

		if err != nil {
			logger.Fatal("failed to create agent", zap.Error(err))
		}

		taskHandler.SetAgent(agent)
	} else {
		logger.Info("no AI provider configured - running in demo mode with streaming")
	}

	// Build and start server
	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithStreamingTaskHandler(taskHandler).
		WithAgentCard(types.AgentCard{
			Name:            cfg.A2A.AgentName,
			Description:     cfg.A2A.AgentDescription,
			Version:         cfg.A2A.AgentVersion,
			URL:             fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port),
			ProtocolVersion: "0.3.0",
			Capabilities: types.AgentCapabilities{
				Streaming:              &cfg.A2A.CapabilitiesConfig.Streaming,
				PushNotifications:      &cfg.A2A.CapabilitiesConfig.PushNotifications,
				StateTransitionHistory: &cfg.A2A.CapabilitiesConfig.StateTransitionHistory,
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills:             []types.AgentSkill{},
		}).
		Build()
	if err != nil {
		logger.Fatal("failed to build server", zap.Error(err))
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in goroutine
	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Error("server error", zap.Error(err))
			cancel()
		}
	}()

	logger.Info("streaming server started successfully",
		zap.String("address", fmt.Sprintf(":%s", cfg.A2A.ServerConfig.Port)))

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		logger.Info("received interrupt signal, shutting down...")
	case <-ctx.Done():
		logger.Info("context canceled, shutting down...")
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("error during server shutdown", zap.Error(err))
	}

	logger.Info("server shutdown complete")
}
