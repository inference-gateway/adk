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
	m.logger.Info("mock agent starting streaming")
	eventChan := make(chan cloudevents.Event, 100)

	go func() {
		defer close(eventChan)
		defer m.logger.Info("mock agent finished streaming")

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
				m.logger.Info("mock agent streaming cancelled")
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
					Role: "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": delta,
						},
					},
				}

				event := cloudevents.NewEvent()
				event.SetType("adk.agent.delta")
				event.SetData(cloudevents.ApplicationJSON, deltaMessage)

				m.logger.Info("sending delta", zap.String("delta", delta))
				eventChan <- event
				time.Sleep(200 * time.Millisecond)
			}
		}

		// Send task completion event with final message
		m.logger.Info("sending task completion event")
		finalMessage := types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("mock-completion-%d", time.Now().UnixNano()),
			Role:      "assistant",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": fullText,
				},
			},
		}

		completeEvent := types.NewIterationCompletedEvent(1, "mock-streaming-task", &finalMessage)
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
	h.logger.Info("processing mock background task", zap.String("task_id", task.ID))

	// Extract user input
	userInput := "Hello!"
	if message != nil {
		for _, part := range message.Parts {
			if partMap, ok := part.(map[string]any); ok {
				if text, ok := partMap["text"].(string); ok {
					userInput = text
					break
				}
			}
		}
	}

	// Create mock response
	response := fmt.Sprintf("Mock response: I received your message '%s'. This is a mock response since no AI provider is configured.", userInput)

	responseMessage := types.Message{
		Role: "assistant",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": response,
			},
		},
	}

	task.History = append(task.History, responseMessage)
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &responseMessage

	return task, nil
}

// HandleStreamingTask processes streaming tasks with mock responses
func (h *MockTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan server.StreamEvent, error) {
	h.logger.Info("processing mock streaming task", zap.String("task_id", task.ID))

	eventChan := make(chan server.StreamEvent, 100)

	go func() {
		defer close(eventChan)

		// Send status update
		eventChan <- &server.StatusStreamEvent{
			Status: map[string]any{
				"task_id": task.ID,
				"state":   "working",
			},
		}

		// Stream a mock response character by character
		response := "This is a mock streaming response. Each character appears with a delay to simulate real-time streaming. No AI provider is configured, so this is a demonstration of the streaming capabilities."

		for _, char := range response {
			select {
			case <-ctx.Done():
				eventChan <- &server.ErrorStreamEvent{
					ErrorMessage: "Task cancelled",
				}
				return
			default:
				eventChan <- &server.DeltaStreamEvent{
					Data: string(char),
				}
				time.Sleep(20 * time.Millisecond)
			}
		}

		// Send completion
		task.Status.State = types.TaskStateCompleted
		task.Status.Message = &types.Message{
			Role: "assistant",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": response,
				},
			},
		}

		eventChan <- &server.TaskCompleteStreamEvent{
			Task: task,
		}
	}()

	return eventChan, nil
}

// Streaming A2A Server Example
//
// This example demonstrates an A2A server with streaming capabilities.
// The server can stream responses in real-time.
//
// To run: go run main.go
func main() {
	fmt.Println("🚀 Starting Streaming A2A Server...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

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
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	// Log configuration info
	logger.Info("configuration loaded",
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
			URL:             fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port),
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

	logger.Info("✅ server created")

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("🌐 server running on port " + cfg.A2A.ServerConfig.Port)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	} else {
		logger.Info("✅ goodbye!")
	}
}
