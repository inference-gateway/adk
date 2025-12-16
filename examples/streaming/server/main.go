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

	config "github.com/inference-gateway/adk/examples/streaming/server/config"
)

// MockAgent provides a mock OpenAI-compatible agent
type MockAgent struct {
	logger *zap.Logger
}

func (m *MockAgent) RunWithStream(ctx context.Context, messages []types.Message) (<-chan cloudevents.Event, error) {
	m.logger.Debug("mock agent starting streaming")
	eventChan := make(chan cloudevents.Event, 100)

	// Extract task from context
	var taskID string
	var contextID string
	if task, ok := ctx.Value(server.TaskContextKey).(*types.Task); ok && task != nil {
		taskID = task.ID
		contextID = task.ContextID
	}

	go func() {
		defer close(eventChan)
		defer m.logger.Debug("mock agent finished streaming")

		// Send initial status change event - task is now working
		statusEvent := cloudevents.NewEvent()
		statusEvent.SetType(types.EventTaskStatusChanged)
		statusEvent.SetData(cloudevents.ApplicationJSON, types.TaskStatus{
			State: types.TaskStateWorking,
		})
		eventChan <- statusEvent

		// Stream a mock response word by word
		words := []string{
			"This", "is", "a", "mock", "streaming", "response.", "Each", "word", "appears",
			"with", "a", "delay", "to", "simulate", "real-time", "streaming.", "No", "AI",
			"provider", "is", "configured,", "so", "this", "is", "a", "demonstration",
			"of", "the", "streaming", "capabilities.", "The", "mock", "agent", "can",
			"stream", "responses", "just", "like", "a", "real", "AI", "would.",
		}
		var fullText string

		for i, word := range words {
			select {
			case <-ctx.Done():
				m.logger.Debug("mock agent streaming cancelled")
				return
			default:
				// Build the delta (just the new token)
				var delta string
				if i > 0 {
					delta = " " + word
					fullText += " " + word
				} else {
					delta = word
					fullText += word
				}

				// Create a delta message with just the new token
				deltaMessage := types.Message{
					Role: types.RoleAgent,
					Parts: []types.Part{
						types.CreateTextPart(delta),
					},
				}

				event := cloudevents.NewEvent()
				event.SetType(types.EventDelta)
				event.SetData(cloudevents.ApplicationJSON, deltaMessage)

				m.logger.Debug("sending delta", zap.String("delta", delta))
				eventChan <- event
				time.Sleep(200 * time.Millisecond)
			}
		}

		// Send task completion event with final message
		m.logger.Debug("sending task completion event")
		finalMessage := types.Message{
			MessageID: fmt.Sprintf("mock-completion-%d", time.Now().UnixNano()),
			Role:      types.RoleAgent,
			TaskID:    &taskID,
			ContextID: &contextID,
			Parts: []types.Part{
				types.CreateTextPart(fullText),
			},
		}

		completeEvent := types.NewIterationCompletedEvent(1, taskID, &finalMessage)
		eventChan <- completeEvent
	}()

	return eventChan, nil
}

// MockTaskHandler provides mock responses for both streaming and background tasks when no AI is configured
type MockTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// GetAgent returns the mock agent
func (h *MockTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// SetAgent sets the agent
func (h *MockTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// HandleTask processes background tasks with mock responses
func (h *MockTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	h.logger.Debug("processing mock background task", zap.String("task_id", task.ID))

	// Extract user input
	userInput := "Hello!"
	if message != nil {
		for _, part := range message.Parts {
			if part.Text != nil {
				userInput = *part.Text
				break
			}
		}
	}

	// Create mock response
	response := fmt.Sprintf("Mock response: I received your message '%s'. This is a mock response since no AI provider is configured.", userInput)

	responseMessage := types.Message{
		MessageID: fmt.Sprintf("mock-response-%s", task.ID),
		Role:      types.RoleAgent,
		TaskID:    &task.ID,
		ContextID: &task.ContextID,
		Parts: []types.Part{
			types.CreateTextPart(response),
		},
	}

	task.History = append(task.History, responseMessage)
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &responseMessage

	return task, nil
}

// HandleStreamingTask processes streaming tasks by forwarding CloudEvents from the agent
func (h *MockTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error) {
	h.logger.Debug("processing mock streaming task", zap.String("task_id", task.ID))

	if h.agent == nil {
		return nil, fmt.Errorf("no agent configured for streaming")
	}

	// Forward CloudEvents directly from agent (no conversion)
	// Flow: agent → handler → protocol handler → client
	return h.agent.RunWithStream(ctx, []types.Message{*message})
}

// Streaming A2A Server Example
//
// This example demonstrates an A2A server with streaming capabilities.
// The server can stream responses in real-time.
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

	logger.Info("server starting",
		zap.String("environment", cfg.Environment),
		zap.String("agent_name", cfg.A2A.AgentName),
		zap.String("port", cfg.A2A.ServerConfig.Port),
		zap.Bool("debug", cfg.A2A.Debug),
		zap.String("provider", cfg.A2A.AgentConfig.Provider),
		zap.String("model", cfg.A2A.AgentConfig.Model),
	)

	// Create server builder
	serverBuilder := server.NewA2AServerBuilder(cfg.A2A, logger)

	// Create mock agent
	mockAgent := &MockAgent{logger: logger}

	// Create mock task handler with the mock agent
	mockHandler := &MockTaskHandler{logger: logger, agent: mockAgent}

	serverBuilder = serverBuilder.
		WithAgent(mockAgent).
		WithBackgroundTaskHandler(mockHandler).
		WithDefaultStreamingTaskHandler()

	// Build and start server
	a2aServer, err := serverBuilder.
		WithAgentCard(types.AgentCard{
			Name:            server.BuildAgentName,
			Description:     server.BuildAgentDescription,
			Version:         server.BuildAgentVersion,
			URL:             stringPtr(fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port)),
			ProtocolVersion: "3.0.0",
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

// stringPtr returns a pointer to a string value
func stringPtr(s string) *string {
	return &s
}
