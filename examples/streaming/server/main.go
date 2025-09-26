package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"
)

func main() {
	fmt.Println("🚀 Starting Streaming A2A Server...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Printf("Failed to sync logger: %v", err)
		}
	}()

	// Load configuration
	cfg := config.Config{
		AgentName:        "streaming-agent",
		AgentDescription: "An AI agent that streams responses in real-time",
		AgentVersion:     "1.0.0",
		CapabilitiesConfig: config.CapabilitiesConfig{
			Streaming:              true, // Enable streaming
			PushNotifications:      false,
			StateTransitionHistory: false,
		},
		ServerConfig: config.ServerConfig{
			Port: "8080",
		},
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, &cfg); err != nil {
		logger.Fatal("Failed to process environment config", zap.Error(err))
	}

	// Create server builder
	serverBuilder := server.NewA2AServerBuilder(cfg, logger)

	// Configure AI agent if API key is available
	if cfg.AgentConfig.APIKey != "" {
		logger.Info("Creating AI agent for streaming",
			zap.String("provider", cfg.AgentConfig.Provider),
			zap.String("model", cfg.AgentConfig.Model))

		// Create LLM client
		llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.AgentConfig, logger)
		if err != nil {
			logger.Fatal("Failed to create LLM client", zap.Error(err))
		}

		// Create AI agent
		agent, err := server.NewAgentBuilder(logger).
			WithConfig(&cfg.AgentConfig).
			WithLLMClient(llmClient).
			WithSystemPrompt("You are a helpful AI assistant that provides clear, detailed responses.").
			Build()
		if err != nil {
			logger.Fatal("Failed to create AI agent", zap.Error(err))
		}

		serverBuilder = serverBuilder.WithAgent(agent).
			WithDefaultStreamingTaskHandler()
	} else {
		logger.Info("No API key configured, using mock streaming handler")

		// Use a simple mock streaming handler
		mockHandler := &MockStreamingHandler{logger: logger}
		serverBuilder = serverBuilder.WithStreamingTaskHandler(mockHandler)
	}

	// Set agent card
	serverBuilder = serverBuilder.WithAgentCard(types.AgentCard{
		Name:        cfg.AgentName,
		Description: cfg.AgentDescription,
		URL:         cfg.AgentURL,
		Version:     cfg.AgentVersion,
		Capabilities: types.AgentCapabilities{
			Streaming:              &cfg.CapabilitiesConfig.Streaming,
			PushNotifications:      &cfg.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: &cfg.CapabilitiesConfig.StateTransitionHistory,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
	})

	// Build server
	a2aServer, err := serverBuilder.Build()
	if err != nil {
		logger.Fatal("Failed to create A2A server", zap.Error(err))
	}

	logger.Info("✅ Streaming A2A server created",
		zap.String("name", cfg.AgentName),
		zap.Bool("streaming", cfg.CapabilitiesConfig.Streaming))

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Wait for shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	} else {
		logger.Info("✅ Server stopped")
	}
}

// MockStreamingHandler provides mock streaming responses when no AI is configured
type MockStreamingHandler struct {
	logger *zap.Logger
}

// GetAgent returns nil since this is a mock handler
func (h *MockStreamingHandler) GetAgent() server.OpenAICompatibleAgent {
	return nil
}

// SetAgent is a no-op for mock handler
func (h *MockStreamingHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	// No-op for mock handler
}

func (h *MockStreamingHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan server.StreamEvent, error) {
	eventChan := make(chan server.StreamEvent, 100)

	go func() {
		defer close(eventChan)

		// Send status update
		eventChan <- &server.StatusStreamEvent{
			Status: map[string]interface{}{
				"task_id": task.ID,
				"state":   "in_progress",
			},
		}

		// Stream a mock response character by character
		response := "This is a mock streaming response. Each character appears with a delay to simulate real-time streaming."

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
