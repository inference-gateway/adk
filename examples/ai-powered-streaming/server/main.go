package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/adk/examples/ai-powered-streaming/server/config"
)

// AIStreamingTaskHandler implements both background and streaming task handlers with AI integration
type AIStreamingTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// NewAIStreamingTaskHandler creates a new AI streaming task handler
func NewAIStreamingTaskHandler(logger *zap.Logger) *AIStreamingTaskHandler {
	return &AIStreamingTaskHandler{logger: logger}
}

// HandleTask processes background tasks using the configured AI agent
func (h *AIStreamingTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	if h.agent == nil {
		return nil, fmt.Errorf("no AI agent configured")
	}

	taskCtx := context.WithValue(ctx, server.TaskContextKey, task)

	streamChan, err := h.agent.RunWithStream(taskCtx, []types.Message{*message})
	if err != nil {
		return nil, fmt.Errorf("failed to get AI response: %w", err)
	}

	var fullResponse string

	// Process all streaming events to completion
	for cloudEvent := range streamChan {
		switch cloudEvent.Type() {
		case types.EventDelta:
			// Extract delta from cloud event
			var deltaMsg types.Message
			if err := cloudEvent.DataAs(&deltaMsg); err == nil {
				for _, part := range deltaMsg.Parts {
					if textPart, ok := part.(types.TextPart); ok {
						fullResponse += textPart.Text
					}
				}
			}
		case types.EventIterationCompleted:
			// Task completion event from agent
			h.logger.Info("AI agent completed iteration")
		}
	}

	// Create final response message
	responseMessage := types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("msg-%s", task.ID),
		ContextID: &task.ContextID,
		TaskID:    &task.ID,
		Role:      "assistant",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: fullResponse,
			},
		},
	}

	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &responseMessage
	task.History = append(task.History, responseMessage)

	return task, nil
}

// HandleStreamingTask processes streaming tasks using the configured AI agent with real-time streaming
// It forwards CloudEvents from the agent directly without conversion
func (h *AIStreamingTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error) {
	h.logger.Info("processing AI streaming task", zap.String("task_id", task.ID))

	if h.agent == nil {
		return nil, fmt.Errorf("no AI agent configured for streaming")
	}

	// Forward CloudEvents directly from agent
	return h.agent.RunWithStream(ctx, []types.Message{*message})
}

// SetAgent sets the OpenAI-compatible agent
func (h *AIStreamingTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *AIStreamingTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// AI-Powered Streaming A2A Server Example
//
// This example demonstrates an A2A server with both AI/LLM integration and streaming capabilities.
// The server can process natural language requests using AI models and stream responses in real-time.
//
// Configuration can be provided via environment variables:
//   - ENVIRONMENT: Runtime environment (default: development)
//   - A2A_AGENT_NAME: Agent name (default: ai-streaming-agent)
//   - A2A_SERVER_PORT: Server port (default: 8080)
//   - A2A_DEBUG: Enable debug logging (default: false)
//   - A2A_AGENT_CLIENT_PROVIDER: LLM provider (required: openai, anthropic)
//   - A2A_AGENT_CLIENT_MODEL: LLM model (required)
//   - A2A_CAPABILITIES_STREAMING: Enable streaming (default: true)
//
// To run: go run main.go
func main() {
	// Create configuration with defaults
	cfg := &config.Config{
		Environment: "development",
		A2A: serverConfig.Config{
			AgentName:        server.BuildAgentName,
			AgentDescription: server.BuildAgentDescription,
			AgentVersion:     server.BuildAgentVersion,
			Debug:            false,
			CapabilitiesConfig: serverConfig.CapabilitiesConfig{
				Streaming:              true,
				PushNotifications:      false,
				StateTransitionHistory: false,
			},
			QueueConfig: serverConfig.QueueConfig{
				CleanupInterval: 5 * time.Minute,
			},
			ServerConfig: serverConfig.ServerConfig{
				Port: "8080",
			},
		},
	}

	// Load configuration from environment variables
	ctx := context.Background()
	if err := envconfig.Process(ctx, cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize logger based on environment
	var logger *zap.Logger
	var err error
	if cfg.Environment == "development" || cfg.Environment == "dev" || cfg.A2A.Debug {
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

	// Log configuration info
	logger.Info("server starting",
		zap.String("environment", cfg.Environment),
		zap.String("agent_name", cfg.A2A.AgentName),
		zap.String("port", cfg.A2A.ServerConfig.Port),
		zap.Bool("debug", cfg.A2A.Debug),
		zap.Bool("streaming_enabled", cfg.A2A.CapabilitiesConfig.Streaming),
		zap.String("provider", cfg.A2A.AgentConfig.Provider),
		zap.String("model", cfg.A2A.AgentConfig.Model),
	)

	// Validate required configuration for AI agent
	if cfg.A2A.AgentConfig.Provider == "" {
		logger.Fatal("A2A_AGENT_CLIENT_PROVIDER is required for AI agent functionality. Set to 'openai', 'anthropic', etc.")
	}
	if cfg.A2A.AgentConfig.Model == "" {
		logger.Fatal("A2A_AGENT_CLIENT_MODEL is required for AI agent functionality. Set to a valid model name.")
	}

	// Create toolbox with sample tools for AI capabilities
	toolBox := server.NewDefaultToolBox()

	// Add weather tool
	weatherTool := server.NewBasicTool(
		"get_weather",
		"Get current weather information for a location",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{
					"type":        "string",
					"description": "The city name",
				},
			},
			"required": []string{"location"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			location := args["location"].(string)
			return fmt.Sprintf(`{"location": "%s", "temperature": "22°C", "condition": "sunny", "humidity": "65%%"}`, location), nil
		},
	)
	toolBox.AddTool(weatherTool)

	// Add time tool
	timeTool := server.NewBasicTool(
		"get_current_time",
		"Get the current date and time",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			now := time.Now()
			return fmt.Sprintf(`{"current_time": "%s", "timezone": "%s"}`,
				now.Format("2006-01-02 15:04:05"), now.Location()), nil
		},
	)
	toolBox.AddTool(timeTool)

	// Create AI agent with LLM client
	llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.A2A.AgentConfig, logger)
	if err != nil {
		logger.Fatal("failed to create LLM client", zap.Error(err))
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(&cfg.A2A.AgentConfig).
		WithLLMClient(llmClient).
		WithSystemPrompt("You are a helpful AI assistant with access to weather and time tools. Provide informative and engaging responses. When streaming, think out loud and build your response naturally.").
		WithMaxChatCompletion(10).
		WithToolBox(toolBox).
		Build()
	if err != nil {
		logger.Fatal("failed to create AI agent", zap.Error(err))
	}

	// Create task handler with AI streaming capabilities
	taskHandler := NewAIStreamingTaskHandler(logger)
	taskHandler.SetAgent(agent)

	// Build and start server with both background and streaming handlers
	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithBackgroundTaskHandler(taskHandler).
		WithStreamingTaskHandler(taskHandler).
		WithAgent(agent).
		WithAgentCard(types.AgentCard{
			Name:            cfg.A2A.AgentName,
			Description:     cfg.A2A.AgentDescription + " with real-time AI streaming",
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
			Skills: []types.AgentSkill{
				{
					Name:        "weather_query",
					Description: "Get current weather information for any location",
				},
				{
					Name:        "time_query",
					Description: "Get current date and time information",
				},
				{
					Name:        "ai_conversation",
					Description: "Engage in natural language conversation with AI streaming",
				},
			},
		}).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("server running", zap.String("port", cfg.A2A.ServerConfig.Port))

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
