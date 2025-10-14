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

	config "github.com/inference-gateway/adk/examples/artifacts-autonomous-tool/server/config"
	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

// Artifacts Autonomous Tool Example
//
// This example demonstrates an A2A server where an LLM can autonomously create
// artifacts using the built-in create_artifact tool. Unlike custom task handlers
// that explicitly create artifacts, this approach lets the AI decide when and what
// artifacts to create based on user requests.
//
// Key Features:
// - AI-powered agent with create_artifact tool enabled
// - LLM autonomously decides when to create artifacts
// - Filesystem-based artifact storage
// - Streaming task handler for real-time responses
// - Full artifact lifecycle management
//
// Configuration via environment variables:
//   - ENVIRONMENT: Runtime environment (default: development)
//   - A2A_AGENT_NAME: Agent name (default: artifacts-autonomous-agent)
//   - A2A_SERVER_PORT: A2A server port (default: 8080)
//   - A2A_DEBUG: Enable debug logging (default: false)
//   - A2A_AGENT_CLIENT_PROVIDER: LLM provider (required)
//   - A2A_AGENT_CLIENT_MODEL: LLM model (required)
//   - A2A_AGENT_CLIENT_TOOLS_CREATE_ARTIFACT: Enable create_artifact tool (default: true)
//   - A2A_ARTIFACTS_ENABLE: Enable artifacts support (default: true)
//   - A2A_ARTIFACTS_SERVER_PORT: Artifacts server port (default: 8081)
//   - A2A_ARTIFACTS_STORAGE_PROVIDER: Storage provider (default: filesystem)
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
			AgentConfig: serverConfig.AgentConfig{
				ToolBoxConfig: serverConfig.ToolBoxConfig{
					EnableCreateArtifact: true,
				},
			},
			ArtifactsConfig: serverConfig.ArtifactsConfig{
				Enable: true,
				ServerConfig: serverConfig.ArtifactsServerConfig{
					Port: "8081",
				},
				StorageConfig: serverConfig.ArtifactsStorageConfig{
					Provider: "filesystem",
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

	logger.Info("server starting",
		zap.String("environment", cfg.Environment),
		zap.String("agent_name", cfg.A2A.AgentName),
		zap.String("a2a_port", cfg.A2A.ServerConfig.Port),
		zap.String("artifacts_port", cfg.A2A.ArtifactsConfig.ServerConfig.Port),
		zap.String("artifacts_storage_path", cfg.A2A.ArtifactsConfig.StorageConfig.BasePath),
		zap.String("artifacts_storage_provider", cfg.A2A.ArtifactsConfig.StorageConfig.Provider),
		zap.Bool("artifacts_enabled", cfg.A2A.ArtifactsConfig.Enable),
		zap.Bool("create_artifact_tool_enabled", cfg.A2A.AgentConfig.ToolBoxConfig.EnableCreateArtifact),
		zap.String("provider", cfg.A2A.AgentConfig.Provider),
		zap.String("model", cfg.A2A.AgentConfig.Model),
	)

	// Step 1: Create artifact service (encapsulates storage)
	artifactService, err := server.NewArtifactService(&cfg.A2A.ArtifactsConfig, logger)
	if err != nil {
		logger.Fatal("failed to create artifact service", zap.Error(err))
	}

	// Step 2: Create artifacts server with injected service
	artifactsServer, err := server.
		NewArtifactsServerBuilder(&cfg.A2A.ArtifactsConfig, logger).
		Build()
	if err != nil {
		logger.Fatal("failed to create artifacts server", zap.Error(err))
	}

	// Create AI agent with LLM client and create_artifact tool
	llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.A2A.AgentConfig, logger)
	if err != nil {
		logger.Fatal("failed to create LLM client", zap.Error(err))
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(&cfg.A2A.AgentConfig).
		WithLLMClient(llmClient).
		WithSystemPrompt(`You are a helpful AI assistant that can create artifacts for users.

When users ask you to:
- Generate reports, analyses, or documentation
- Create code files, scripts, or configurations
- Export data in various formats (JSON, CSV, etc.)
- Save any content they might want to download

You should IMMEDIATELY use the create_artifact tool to save the content and make it available as a downloadable file.

Important guidelines:
1. Create the artifact in your FIRST response - don't ask for clarification unless absolutely necessary
2. Generate complete, working content based on the request
3. Choose appropriate filenames with correct extensions based on the content type
4. After creating the artifact, provide a brief summary of what was created

Be efficient and proactive in creating artifacts.`).
		WithMaxChatCompletion(10).
		WithDefaultToolBox().
		Build()
	if err != nil {
		logger.Fatal("failed to create AI agent", zap.Error(err))
	}

	// Step 3: Build A2A server with artifact service injected into task handlers
	// The service is passed to handlers so tools can create artifacts with proper storage
	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithAgent(agent).
		WithArtifactService(artifactService).
		WithDefaultTaskHandlers().
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
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	// Start servers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start artifacts server
	go func() {
		if err := artifactsServer.Start(ctx); err != nil {
			logger.Fatal("artifacts server failed to start", zap.Error(err))
		}
	}()

	// Start A2A server
	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("A2A server failed to start", zap.Error(err))
		}
	}()

	logger.Info("server running",
		zap.String("a2a_port", cfg.A2A.ServerConfig.Port),
		zap.String("artifacts_port", cfg.A2A.ArtifactsConfig.ServerConfig.Port))

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Stop A2A server
	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("A2A server shutdown error", zap.Error(err))
	}

	// Stop artifacts server
	if err := artifactsServer.Stop(shutdownCtx); err != nil {
		logger.Error("artifacts server shutdown error", zap.Error(err))
	}
}
