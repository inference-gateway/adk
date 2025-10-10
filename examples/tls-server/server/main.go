package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/adk/examples/tls-example/server/config"
)

// SimpleTaskHandler implements a basic task handler for TLS demonstration
type SimpleTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// NewSimpleTaskHandler creates a new simple task handler
func NewSimpleTaskHandler(logger *zap.Logger) *SimpleTaskHandler {
	return &SimpleTaskHandler{logger: logger}
}

// HandleTask processes tasks with a simple echo response
func (h *SimpleTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	userInput := ""
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

	if userInput == "" {
		userInput = "Hello from TLS server!"
	}

	responseText := fmt.Sprintf("🔒 Secure TLS Response: %s (via HTTPS)", userInput)

	// Create response message
	responseMessage := types.Message{
		Role: "assistant",
		Parts: []types.Part{
			map[string]any{
				"type": "text",
				"text": responseText,
			},
		},
	}

	// Update task with response
	task.History = append(task.History, responseMessage)
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &responseMessage

	h.logger.Info("processed task over TLS",
		zap.String("input", userInput),
		zap.String("task_id", task.ID),
	)

	return task, nil
}

// SetAgent sets the OpenAI-compatible agent
func (h *SimpleTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *SimpleTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// TLS-Enabled A2A Server Example
//
// This example demonstrates an A2A server running with TLS encryption.
// The server accepts HTTPS connections and demonstrates secure communication
// between client and server.
//
// Configuration can be provided via environment variables:
//   - ENVIRONMENT: Runtime environment (default: development)
//   - A2A_AGENT_NAME: Agent name (default: tls-agent)
//   - A2A_SERVER_PORT: Server port (default: 8443)
//   - A2A_SERVER_TLS_ENABLED: Enable TLS (default: true)
//   - A2A_SERVER_TLS_CERT_FILE: Path to TLS certificate
//   - A2A_SERVER_TLS_KEY_FILE: Path to TLS private key
//   - A2A_DEBUG: Enable debug logging (default: false)
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
				Port: "8443",
				TLSConfig: serverConfig.TLSConfig{
					Enable:   true,
					CertPath: "/certs/server.crt",
					KeyPath:  "/certs/server.key",
				},
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
		zap.Bool("tls_enabled", cfg.A2A.ServerConfig.TLSConfig.Enable),
		zap.String("cert_file", cfg.A2A.ServerConfig.TLSConfig.CertPath),
		zap.String("key_file", cfg.A2A.ServerConfig.TLSConfig.KeyPath),
		zap.Bool("debug", cfg.A2A.Debug),
	)

	// Validate TLS configuration
	if cfg.A2A.ServerConfig.TLSConfig.Enable {
		if cfg.A2A.ServerConfig.TLSConfig.CertPath == "" || cfg.A2A.ServerConfig.TLSConfig.KeyPath == "" {
			logger.Fatal("TLS enabled but certificate or key file not specified")
		}

		// Check if certificate files exist
		if _, err := os.Stat(cfg.A2A.ServerConfig.TLSConfig.CertPath); os.IsNotExist(err) {
			logger.Fatal("TLS certificate file does not exist", zap.String("path", cfg.A2A.ServerConfig.TLSConfig.CertPath))
		}
		if _, err := os.Stat(cfg.A2A.ServerConfig.TLSConfig.KeyPath); os.IsNotExist(err) {
			logger.Fatal("TLS key file does not exist", zap.String("path", cfg.A2A.ServerConfig.TLSConfig.KeyPath))
		}
	}

	// Create task handler
	taskHandler := NewSimpleTaskHandler(logger)

	// Build and start server
	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithBackgroundTaskHandler(taskHandler).
		WithDefaultStreamingTaskHandler().
		WithAgentCard(types.AgentCard{
			Name:            cfg.A2A.AgentName,
			Description:     cfg.A2A.AgentDescription,
			Version:         cfg.A2A.AgentVersion,
			URL:             fmt.Sprintf("https://localhost:%s", cfg.A2A.ServerConfig.Port),
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
