package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	uuid "github.com/google/uuid"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

// Build-time variables set via ldflags
var (
	AgentName        = "artifacts-filesystem-agent"
	AgentDescription = "An agent that creates and serves artifacts using filesystem storage"
	AgentVersion     = "0.1.0"
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
	artifactsServer server.ArtifactsServer
}

// NewArtifactsTaskHandler creates a new artifacts task handler
func NewArtifactsTaskHandler(logger *zap.Logger, artifactsServer server.ArtifactsServer) *ArtifactsTaskHandler {
	return &ArtifactsTaskHandler{
		logger:          logger,
		artifactsServer: artifactsServer,
	}
}

// extractMessageContent extracts text and file content from message parts
func extractMessageContent(message *types.Message) (string, string, string) {
	var userText, fileName, fileContent string

	if message == nil {
		return userText, fileName, fileContent
	}

	for _, part := range message.Parts {
		partMap, ok := part.(map[string]any)
		if !ok {
			continue
		}

		// Extract text content
		if text, ok := partMap["text"].(string); ok {
			userText = text
		}

		// Extract file content
		if kind, ok := partMap["kind"].(string); ok && kind == "file" {
			if filename, ok := partMap["filename"].(string); ok {
				fileName = filename
			}

			if fileData, ok := partMap["file"].(map[string]any); ok {
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

	// Store the artifact using the artifacts server
	artifactID := task.ID
	filename := "analysis_report.md"

	storage := h.artifactsServer.GetStorage()
	url, err := storage.Store(ctx, artifactID, filename, strings.NewReader(artifactContent))
	if err != nil {
		h.logger.Error("failed to store artifact", zap.Error(err))
		return nil, fmt.Errorf("failed to store artifact: %w", err)
	}

	h.logger.Info("artifact stored successfully",
		zap.String("artifact_id", artifactID),
		zap.String("filename", filename),
		zap.String("url", url))

	// Create artifact object for the response
	artifact := types.Artifact{
		ArtifactID:  artifactID,
		Name:        stringPtr("Analysis Report"),
		Description: stringPtr("A detailed analysis report based on your request"),
		Parts: []types.Part{
			map[string]any{
				"kind":     "file",
				"filename": filename,
				"uri":      url,
				"mimeType": "text/markdown",
			},
		},
	}

	// Create response message with artifact
	responseText := fmt.Sprintf("I've created an analysis report for your request: \"%s\". The report has been saved as an artifact and is available for download.", userText)

	responseMessage := types.Message{
		Kind:      "message",
		MessageID: uuid.New().String(),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": responseText,
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

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
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
	fmt.Println("🤖 Starting Artifacts Filesystem A2A Server...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create configuration with defaults
	cfg := &Config{
		Environment: "development",
		A2A: serverConfig.Config{
			AgentName:        AgentName,
			AgentDescription: AgentDescription,
			AgentVersion:     AgentVersion,
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
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	// Log configuration info
	logger.Info("configuration loaded",
		zap.String("environment", cfg.Environment),
		zap.String("agent_name", cfg.A2A.AgentName),
		zap.String("a2a_port", cfg.A2A.ServerConfig.Port),
		zap.String("artifacts_port", cfg.A2A.ArtifactsConfig.ServerConfig.Port),
		zap.Bool("artifacts_enabled", cfg.A2A.ArtifactsConfig.Enable),
		zap.String("storage_provider", cfg.A2A.ArtifactsConfig.StorageConfig.Provider),
		zap.Bool("debug", cfg.A2A.Debug),
	)

	// Create artifacts server
	artifactsServer, err := server.
		NewArtifactsServerBuilder(&cfg.A2A.ArtifactsConfig, logger).
		Build()
	if err != nil {
		logger.Fatal("failed to create artifacts server", zap.Error(err))
	}

	// Create task handler with artifacts support
	taskHandler := NewArtifactsTaskHandler(logger, artifactsServer)

	// Build A2A server
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
			DefaultOutputModes: []string{"text/plain", "application/json"},
			Skills:             []types.AgentSkill{},
		}).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	logger.Info("✅ servers created")

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

	logger.Info("🌐 A2A server running on port " + cfg.A2A.ServerConfig.Port)
	logger.Info("📁 Artifacts server running on port " + cfg.A2A.ArtifactsConfig.ServerConfig.Port)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 shutting down...")

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

	logger.Info("✅ goodbye!")
}
