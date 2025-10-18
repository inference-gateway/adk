package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	uuid "github.com/google/uuid"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

// Config holds the configuration for the artifacts filesystem example server
type Config struct {
	// Environment determines runtime environment (development, production, etc.)
	Environment string `env:"ENVIRONMENT,default=development"`

	// A2A contains all A2A server configuration
	// This is prefixed with A2A_ in environment variables
	A2A serverConfig.Config `env:",prefix=A2A_"`
}

// ArtifactsTaskHandler implements a task handler that creates downloadable artifacts
type ArtifactsTaskHandler struct {
	logger          *zap.Logger
	agent           server.OpenAICompatibleAgent
	artifactService server.ArtifactService
}

// NewArtifactsTaskHandler creates a new artifacts task handler
func NewArtifactsTaskHandler(logger *zap.Logger, artifactService server.ArtifactService) *ArtifactsTaskHandler {
	return &ArtifactsTaskHandler{
		logger:          logger,
		artifactService: artifactService,
	}
}

// extractMessageContent extracts text and file content from message parts
func extractMessageContent(message *types.Message) (string, string, string) {
	var userText, fileName, fileContent string

	if message == nil {
		return userText, fileName, fileContent
	}

	for _, part := range message.Parts {
		// Extract text content
		if textPart, ok := part.(types.TextPart); ok {
			userText = textPart.Text
		}

		// Extract file content
		if filePart, ok := part.(types.FilePart); ok {
			if fileData, ok := filePart.File.(map[string]any); ok {
				if name, ok := fileData["name"].(string); ok {
					fileName = name
				}
				if bytes, ok := fileData["bytes"].(string); ok {
					if decoded, err := base64.StdEncoding.DecodeString(bytes); err == nil {
						fileContent = string(decoded)
					}
				}
			}
		}
	}

	return userText, fileName, fileContent
}

// createReportContent generates the analysis report content
func createReportContent(userText, fileName, fileContent, taskID string) string {
	baseReport := fmt.Sprintf(`# Analysis Report

User Request: %s

## Summary
Enhanced analysis report that can process client-uploaded data.

`, userText)

	// Add file analysis section if file was uploaded
	if fileContent != "" {
		baseReport += fmt.Sprintf(`## Uploaded File Analysis

**File:** %s

**Content:**
%s

**Insights:**
- File successfully processed from client upload
- Data has been analyzed and incorporated into this report
- This demonstrates bidirectional artifact exchange in A2A protocol

`, fileName, fileContent)
	}

	baseReport += fmt.Sprintf(`## Technical Details
- Generated at: %s
- Task ID: %s
- Client file uploaded: %v
- Content type: Markdown document

## Conclusions
This demonstrates both artifact creation and client file upload processing
through the filesystem storage provider.
`, time.Now().Format(time.RFC3339), taskID, fileContent != "")

	return baseReport
}

// HandleTask processes tasks and creates artifacts
func (h *ArtifactsTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	// Extract content from message
	userText, fileName, fileContent := extractMessageContent(message)

	h.logger.Info("processing task",
		zap.String("user_text", userText),
		zap.String("uploaded_file", fileName),
		zap.Bool("has_file_content", fileContent != ""))

	// Generate report content
	artifactContent := createReportContent(userText, fileName, fileContent, task.ID)

	// Create artifact using artifact service
	filename := "analysis_report.md"
	mimeType := "text/markdown"

	artifact, err := h.artifactService.CreateFileArtifact(
		"Analysis Report",
		"A detailed analysis report based on your request",
		filename,
		[]byte(artifactContent),
		&mimeType,
	)
	if err != nil {
		h.logger.Error("failed to create artifact", zap.Error(err))
		return nil, fmt.Errorf("failed to create artifact: %w", err)
	}

	h.logger.Info("artifact created successfully",
		zap.String("artifact_id", artifact.ArtifactID),
		zap.String("filename", filename))

	// Create response message with artifact
	responseText := fmt.Sprintf("I've created an analysis report for your request: \"%s\". The report has been saved as an artifact and is available for download.", userText)

	responseMessage := types.Message{
		Kind:      "message",
		MessageID: uuid.New().String(),
		ContextID: &task.ContextID,
		TaskID:    &task.ID,
		Role:      "assistant",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: responseText,
			},
		},
	}

	task.History = append(task.History, responseMessage)
	task.Artifacts = append(task.Artifacts, artifact)
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &responseMessage

	return task, nil
}

// SetAgent sets the OpenAI-compatible agent
func (h *ArtifactsTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *ArtifactsTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// Artifacts Filesystem Example
//
// This example demonstrates an A2A server that creates downloadable artifacts
// using filesystem storage. The server creates analysis reports as markdown files
// and makes them available for download via HTTP endpoints.
//
// Features:
// - Creates markdown artifacts for user requests
// - Stores artifacts using filesystem storage provider
// - Serves artifacts via HTTP download endpoints
// - Includes proper artifact metadata in responses
//
// Configuration via environment variables:
//   - ENVIRONMENT: Runtime environment (default: development)
//   - A2A_AGENT_NAME: Agent name (default: artifacts-filesystem-agent)
//   - A2A_SERVER_PORT: A2A server port (default: 8080)
//   - A2A_ARTIFACTS_ENABLE: Enable artifacts support (default: true)
//   - A2A_ARTIFACTS_SERVER_HOST: Artifacts server host (default: localhost)
//   - A2A_ARTIFACTS_SERVER_PORT: Artifacts server port (default: 8081)
//   - A2A_ARTIFACTS_STORAGE_PROVIDER: Storage provider (default: filesystem)
//   - A2A_ARTIFACTS_STORAGE_BASE_PATH: Base path for artifacts (default: ./artifacts)
//
// To run: go run main.go
func main() {
	// Create configuration with defaults
	cfg := &Config{
		Environment: "development",
		A2A: serverConfig.Config{
			AgentName:        server.BuildAgentName,
			AgentDescription: server.BuildAgentDescription,
			AgentVersion:     server.BuildAgentVersion,
			Debug:            false,
			CapabilitiesConfig: serverConfig.CapabilitiesConfig{
				Streaming:              false,
				PushNotifications:      false,
				StateTransitionHistory: false,
			},
			QueueConfig: serverConfig.QueueConfig{
				CleanupInterval: 5 * time.Minute,
			},
			ServerConfig: serverConfig.ServerConfig{
				Port: "8080",
			},
			ArtifactsConfig: serverConfig.ArtifactsConfig{
				Enable: true,
				ServerConfig: serverConfig.ArtifactsServerConfig{
					Port: "8081",
				},
				StorageConfig: serverConfig.ArtifactsStorageConfig{
					Provider: "filesystem",
					BasePath: "./artifacts",
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
		zap.String("a2a_port", cfg.A2A.ServerConfig.Port),
		zap.String("artifacts_port", cfg.A2A.ArtifactsConfig.ServerConfig.Port),
		zap.Bool("artifacts_enabled", cfg.A2A.ArtifactsConfig.Enable),
		zap.String("storage_provider", cfg.A2A.ArtifactsConfig.StorageConfig.Provider),
		zap.Bool("debug", cfg.A2A.Debug),
	)

	// Step 1: Create artifact service (encapsulates storage)
	artifactService, err := server.NewArtifactService(&cfg.A2A.ArtifactsConfig, logger)
	if err != nil {
		logger.Fatal("failed to create artifact service", zap.Error(err))
	}

	// Step 2: Create artifacts server with shared artifact service (avoids double initialization)
	artifactsServer, err := server.
		NewArtifactsServerBuilder(&cfg.A2A.ArtifactsConfig, logger).
		WithArtifactService(artifactService).
		Build()
	if err != nil {
		logger.Fatal("failed to create artifacts server", zap.Error(err))
	}

	// Create task handler with artifact service support
	taskHandler := NewArtifactsTaskHandler(logger, artifactService)

	// Step 3: Build A2A server with artifact service injected
	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithArtifactService(artifactService).
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
			DefaultOutputModes: []string{"text/plain", "application/json"},
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

	logger.Info("server running", zap.String("port", cfg.A2A.ServerConfig.Port))

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
