package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	config "github.com/inference-gateway/adk/examples/input-required/non-streaming/server/config"
	server "github.com/inference-gateway/adk/server"
	types "github.com/inference-gateway/adk/types"
)

// InputRequiredTaskHandler demonstrates handling tasks that require user input
type InputRequiredTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// NewInputRequiredTaskHandler creates a new InputRequiredTaskHandler
func NewInputRequiredTaskHandler(logger *zap.Logger) *InputRequiredTaskHandler {
	return &InputRequiredTaskHandler{
		logger: logger,
	}
}

// SetAgent sets the agent for the task handler
func (h *InputRequiredTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured agent
func (h *InputRequiredTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// HandleTask processes tasks and demonstrates input-required flow
func (h *InputRequiredTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	h.logger.Info("processing task with input-required demonstration",
		zap.String("task_id", task.ID),
		zap.String("message_role", message.Role))

	// If we have an agent, use it to process the message
	if h.agent != nil {
		return h.processWithAgent(ctx, task, message)
	}

	// Without agent, demonstrate input-required behavior manually
	return h.processWithoutAgent(ctx, task, message)
}

// processWithAgent uses the AI agent to handle the task
func (h *InputRequiredTaskHandler) processWithAgent(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	h.logger.Info("processing with AI agent", zap.String("task_id", task.ID))

	// Prepare messages for agent
	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

	// Add current message to conversation
	messages = append(messages, *message)

	// Create context with task information for agent tools
	toolCtx := context.WithValue(ctx, server.TaskContextKey, task)

	// Process with agent - agent will automatically use input_required tool when needed
	eventChan, err := h.agent.RunWithStream(toolCtx, messages)
	if err != nil {
		h.logger.Error("agent processing failed", zap.Error(err))
		task.Status.State = types.TaskStateFailed
		task.Status.Message = &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("error-%s", task.ID),
			Role:      "assistant",
			Parts: []types.Part{
				types.TextPart{
					Kind: "text",
					Text: fmt.Sprintf("Failed to process task: %v", err),
				},
			},
		}
		return task, nil
	}

	// Collect all events from agent
	var finalMessage *types.Message
	var inputRequiredMessage *types.Message

	for event := range eventChan {
		h.logger.Debug("received event", zap.String("type", event.Type()))

		// Handle different event types
		switch event.Type() {
		case types.EventDelta:
			// Extract message from delta event
			var msg types.Message
			if err := event.DataAs(&msg); err == nil {
				finalMessage = &msg
			}

		case types.EventInputRequired:
			// Extract input required message
			var msg types.Message
			if err := event.DataAs(&msg); err == nil {
				inputRequiredMessage = &msg
				h.logger.Info("agent requested user input",
					zap.String("task_id", task.ID),
					zap.String("message", getMessageText(&msg)))
			}

		case types.EventIterationCompleted:
			// Extract final message from iteration completed
			var msg types.Message
			if err := event.DataAs(&msg); err == nil {
				finalMessage = &msg
			}
		}
	}

	// Update task based on results
	if inputRequiredMessage != nil {
		// Task requires user input
		task.Status.State = types.TaskStateInputRequired
		task.Status.Message = inputRequiredMessage
		task.History = append(task.History, *message, *inputRequiredMessage)
		h.logger.Info("task paused for user input", zap.String("task_id", task.ID))
	} else if finalMessage != nil {
		// Task completed successfully
		task.Status.State = types.TaskStateCompleted
		task.Status.Message = finalMessage
		task.History = append(task.History, *message, *finalMessage)
		h.logger.Info("task completed successfully", zap.String("task_id", task.ID))
	} else {
		// No clear result, mark as failed
		task.Status.State = types.TaskStateFailed
		task.Status.Message = &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("error-%s", task.ID),
			Role:      "assistant",
			Parts: []types.Part{
				types.TextPart{
					Kind: "text",
					Text: "No response received from agent",
				},
			},
		}
		task.History = append(task.History, *message)
	}

	return task, nil
}

// processWithoutAgent demonstrates input-required behavior without AI
func (h *InputRequiredTaskHandler) processWithoutAgent(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	h.logger.Info("processing without AI agent - demonstrating input-required flow",
		zap.String("task_id", task.ID))

	messageText := getMessageText(message)
	h.logger.Info("received message", zap.String("text", messageText))

	// Add the incoming message to history
	task.History = append(task.History, *message)

	// Check if this is a follow-up to a previous input-required state
	// by looking at the conversation history
	previousContext := getPreviousContext(task.History)
	h.logger.Info("previous context", zap.String("context", previousContext))

	// If this looks like a follow-up response (short message without keywords)
	if previousContext != "" && !contains(messageText, "weather") && !contains(messageText, "calculate") && !contains(messageText, "hello") && len(messageText) < 50 {
		switch previousContext {
		case "weather":
			// User provided location for weather query
			responseMessage := &types.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("response-%s", task.ID),
				Role:      "assistant",
				Parts: []types.Part{
					types.TextPart{
						Kind: "text",
						Text: fmt.Sprintf("The weather in %s is sunny and 72°F! (This is a demo response - no real weather data is fetched)", messageText),
					},
				},
			}
			task.Status.State = types.TaskStateCompleted
			task.Status.Message = responseMessage
			task.History = append(task.History, *responseMessage)
			return task, nil

		case "calculate":
			// User provided calculation
			responseMessage := &types.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("response-%s", task.ID),
				Role:      "assistant",
				Parts: []types.Part{
					types.TextPart{
						Kind: "text",
						Text: fmt.Sprintf("Based on your input '%s', here's the result! (This is a demo response)", messageText),
					},
				},
			}
			task.Status.State = types.TaskStateCompleted
			task.Status.Message = responseMessage
			task.History = append(task.History, *responseMessage)
			return task, nil
		}
	}

	// Simulate different scenarios based on message content
	switch {
	case contains(messageText, "weather"):
		// Request location if not provided
		if !contains(messageText, "in ") && !contains(messageText, "at ") {
			inputMessage := &types.Message{
				Kind:      "input_required",
				MessageID: fmt.Sprintf("input-required-%s", task.ID),
				Role:      "assistant",
				Parts: []types.Part{
					types.TextPart{
						Kind: "text",
						Text: "I'd be happy to help you with the weather! Could you please specify which location you'd like the weather for?",
					},
				},
			}

			task.Status.State = types.TaskStateInputRequired
			task.Status.Message = inputMessage
			task.History = append(task.History, *inputMessage)

			h.logger.Info("requesting location for weather query", zap.String("task_id", task.ID))
			return task, nil
		}

		// If location is provided, give weather response
		responseMessage := &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("response-%s", task.ID),
			Role:      "assistant",
			Parts: []types.Part{
				types.TextPart{
					Kind: "text",
					Text: "The weather is sunny and 72°F! (This is a demo response - no real weather data is fetched)",
				},
			},
		}

		task.Status.State = types.TaskStateCompleted
		task.Status.Message = responseMessage
		task.History = append(task.History, *responseMessage)

	case contains(messageText, "calculate") || contains(messageText, "math"):
		// Request specific numbers if not provided
		if !hasNumbers(messageText) {
			inputMessage := &types.Message{
				Kind:      "input_required",
				MessageID: fmt.Sprintf("input-required-%s", task.ID),
				Role:      "assistant",
				Parts: []types.Part{
					types.TextPart{
						Kind: "text",
						Text: "I can help you with calculations! Could you please provide the specific numbers or equation you'd like me to calculate?",
					},
				},
			}

			task.Status.State = types.TaskStateInputRequired
			task.Status.Message = inputMessage
			task.History = append(task.History, *inputMessage)

			h.logger.Info("requesting specific calculation details", zap.String("task_id", task.ID))
			return task, nil
		}

		// If numbers are provided, give calculation response
		responseMessage := &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("response-%s", task.ID),
			Role:      "assistant",
			Parts: []types.Part{
				types.TextPart{
					Kind: "text",
					Text: "Based on your calculation request, I can help you with that math problem! (This is a demo response)",
				},
			},
		}

		task.Status.State = types.TaskStateCompleted
		task.Status.Message = responseMessage
		task.History = append(task.History, *responseMessage)

	case contains(messageText, "hello") || contains(messageText, "hi"):
		// Simple greeting, no input required
		responseMessage := &types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("response-%s", task.ID),
			Role:      "assistant",
			Parts: []types.Part{
				types.TextPart{
					Kind: "text",
					Text: "Hello! I'm an assistant that demonstrates the input-required flow. Try asking me about the weather or a calculation to see how I request additional information when needed!",
				},
			},
		}

		task.Status.State = types.TaskStateCompleted
		task.Status.Message = responseMessage
		task.History = append(task.History, *responseMessage)

	default:
		// For unclear requests, ask for clarification
		inputMessage := &types.Message{
			Kind:      "input_required",
			MessageID: fmt.Sprintf("input-required-%s", task.ID),
			Role:      "assistant",
			Parts: []types.Part{
				types.TextPart{
					Kind: "text",
					Text: "I'd be happy to help! Could you please provide more details about what you'd like me to do? For example, you could ask about the weather or request a calculation.",
				},
			},
		}

		task.Status.State = types.TaskStateInputRequired
		task.Status.Message = inputMessage
		task.History = append(task.History, *inputMessage)

		h.logger.Info("requesting clarification for unclear request", zap.String("task_id", task.ID))
	}

	return task, nil
}

// Helper functions
func getMessageText(message *types.Message) string {
	for _, part := range message.Parts {
		if textPart, ok := part.(types.TextPart); ok {
			return textPart.Text
		}
	}
	return ""
}

func contains(text, substr string) bool {
	// Simple case-insensitive contains check
	return len(text) >= len(substr) &&
		findInLower(toLower(text), toLower(substr)) >= 0
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

	logger.Info("starting input-required non-streaming server",
		zap.String("port", cfg.A2A.ServerConfig.Port),
		zap.Bool("ai_enabled", cfg.A2A.AgentConfig.Provider != ""))

	// Create task handler
	taskHandler := NewInputRequiredTaskHandler(logger)

	// Add AI agent if configured
	if cfg.A2A.AgentConfig.Provider != "" {
		logger.Info("configuring AI agent",
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
			WithSystemPrompt(`You are a helpful assistant that demonstrates the input-required flow.

When users ask for information that requires additional details (like weather without location, calculations without numbers, or unclear requests), use the input_required tool to ask for the missing information.

Examples:
- If asked "What's the weather?" without a location, use input_required to ask for the location
- If asked "Calculate this" without numbers, use input_required to ask for the specific calculation
- If the request is unclear or ambiguous, use input_required to ask for clarification

Be specific about what information you need and why it's needed to provide a complete answer.`).
			WithToolBox(toolBox).
			Build()

		if err != nil {
			logger.Fatal("failed to create agent", zap.Error(err))
		}

		taskHandler.SetAgent(agent)
	} else {
		logger.Info("no AI provider configured - running in demo mode")
	}

	// Build and start server
	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithBackgroundTaskHandler(taskHandler).
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

	logger.Info("server started successfully",
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
