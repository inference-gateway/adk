package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"

	"github.com/inference-gateway/adk/server"
	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/types"
)

// SkillsTaskHandler implements a task handler with LLM integration and built-in skills
type SkillsTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// NewSkillsTaskHandler creates a new skills task handler
func NewSkillsTaskHandler(logger *zap.Logger) *SkillsTaskHandler {
	return &SkillsTaskHandler{logger: logger}
}

// HandleTask processes tasks using the configured AI agent with skills
func (h *SkillsTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	if h.agent == nil {
		return nil, fmt.Errorf("no AI agent configured")
	}

	taskCtx := context.WithValue(ctx, server.TaskContextKey, task)

	streamChan, err := h.agent.RunWithStream(taskCtx, []types.Message{*message})
	if err != nil {
		return nil, fmt.Errorf("failed to get AI response: %w", err)
	}

	var fullResponse string
	var artifacts []types.Artifact

	// Process all streaming events to completion
	for cloudEvent := range streamChan {
		switch cloudEvent.Type() {
		case "adk.agent.delta":
			// Extract delta from cloud event
			var deltaMsg types.Message
			if err := cloudEvent.DataAs(&deltaMsg); err == nil {
				for _, part := range deltaMsg.Parts {
					if textPart, ok := part.(types.TextPart); ok {
						fullResponse += textPart.Text
					}
				}
			}
		case "adk.agent.iteration.completed":
			// Task completion event from agent
			h.logger.Debug("agent completed iteration")
		case "adk.agent.artifact.created":
			// Extract artifact from cloud event
			var artifact types.Artifact
			if err := cloudEvent.DataAs(&artifact); err == nil {
				artifacts = append(artifacts, artifact)
				h.logger.Info("artifact created", zap.String("artifact_id", artifact.ArtifactID))
			}
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

	// Add artifacts if any were created
	if len(artifacts) > 0 {
		task.Artifacts = artifacts
	}

	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &responseMessage
	task.History = append(task.History, responseMessage)

	return task, nil
}

// SetAgent sets the OpenAI-compatible agent
func (h *SkillsTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *SkillsTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// Skills-Enabled A2A Server Example
//
// This example demonstrates an A2A server with built-in skills for file operations
// and web access. The server can process natural language requests and use skills
// like reading files, writing content, editing files, searching the web, and fetching
// web content.
//
// Configuration is provided via environment variables. See README.md for complete
// configuration options.
//
// Essential environment variables:
//   - SKILLS_ENABLED: Enable built-in skills globally (default: false)
//   - AGENT_CLIENT_PROVIDER: LLM provider (required)
//   - AGENT_CLIENT_MODEL: LLM model (required)
//   - AGENT_CLIENT_API_KEY: LLM API key (required)
//
// To run: go run main.go
func main() {
	// Create configuration with defaults
	cfg := &config.Config{
		AgentName:        server.BuildAgentName,
		AgentDescription: server.BuildAgentDescription,
		AgentVersion:     server.BuildAgentVersion,
		Debug:            false,
		CapabilitiesConfig: config.CapabilitiesConfig{
			Streaming:              true,
			PushNotifications:      false,
			StateTransitionHistory: false,
		},
		QueueConfig: config.QueueConfig{
			CleanupInterval: 5 * time.Minute,
		},
		ServerConfig: config.ServerConfig{
			Port: "8080",
		},
		// Initialize with default skills config (disabled by default)
		SkillsConfig: *config.GetDefaultSkillsConfig(),
	}

	// Load configuration from environment variables
	ctx := context.Background()
	if err := envconfig.Process(ctx, cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize logger based on debug setting
	var logger *zap.Logger
	var err error
	if cfg.Debug {
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
		zap.String("agent_name", cfg.AgentName),
		zap.String("port", cfg.ServerConfig.Port),
		zap.Bool("debug", cfg.Debug),
		zap.String("provider", cfg.AgentConfig.Provider),
		zap.String("model", cfg.AgentConfig.Model),
		zap.Bool("skills_enabled", cfg.SkillsConfig.Enabled),
	)

	// Create agent with skills
	agent, _, agentSkills, err := server.CreateAgentWithSkills(logger, &cfg.AgentConfig, &cfg.SkillsConfig)
	if err != nil {
		logger.Fatal("failed to create agent with skills", zap.Error(err))
	}

	logger.Info("agent created with skills",
		zap.Int("skill_count", len(agentSkills)),
		zap.Bool("skills_enabled", cfg.SkillsConfig.Enabled))

	// Log enabled skills
	if cfg.SkillsConfig.Enabled && len(agentSkills) > 0 {
		skillNames := make([]string, len(agentSkills))
		for i, skill := range agentSkills {
			skillNames[i] = skill.Name
		}
		logger.Info("enabled skills", zap.Strings("skills", skillNames))
	}

	// Create task handler with AI agent
	taskHandler := NewSkillsTaskHandler(logger)
	taskHandler.SetAgent(agent)

	// Determine server URL
	agentURL := cfg.AgentURL
	if agentURL == "" {
		agentURL = fmt.Sprintf("http://localhost:%s", cfg.ServerConfig.Port)
	}

	// Create agent card with skills
	agentCard := types.AgentCard{
		Name:            cfg.AgentName,
		Description:     cfg.AgentDescription,
		Version:         cfg.AgentVersion,
		URL:             agentURL,
		ProtocolVersion: "0.3.0",
		Capabilities: types.AgentCapabilities{
			Streaming:              &cfg.CapabilitiesConfig.Streaming,
			PushNotifications:      &cfg.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: &cfg.CapabilitiesConfig.StateTransitionHistory,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills:             agentSkills,
	}

	// Build and start server
	a2aServer, err := server.NewA2AServerBuilder(*cfg, logger).
		WithBackgroundTaskHandler(taskHandler).
		WithDefaultStreamingTaskHandler().
		WithAgent(agent).
		WithAgentCard(agentCard).
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

	logger.Info("server running",
		zap.String("port", cfg.ServerConfig.Port),
		zap.String("agent_url", agentURL),
		zap.Int("skills_count", len(agentSkills)))

	if cfg.SkillsConfig.Enabled {
		logger.Info("skills are enabled - agent can perform file operations and web access")
		
		if cfg.SkillsConfig.Safety.EnableSandbox {
			if len(cfg.SkillsConfig.Safety.SandboxPaths) > 0 {
				logger.Info("file operations sandboxed to configured paths",
					zap.Strings("sandbox_paths", cfg.SkillsConfig.Safety.SandboxPaths))
			} else {
				logger.Warn("sandbox enabled but no sandbox paths configured - all file operations will be restricted")
			}
		} else {
			logger.Warn("sandbox disabled - file operations are unrestricted")
		}
	} else {
		logger.Info("skills are disabled - use SKILLS_ENABLED=true to enable built-in skills")
	}

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