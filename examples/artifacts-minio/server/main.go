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

// Config holds the configuration for the artifacts MinIO example server
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
Enhanced analysis report that can process client-uploaded data using MinIO cloud storage.

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
- Artifact stored securely in MinIO cloud storage

`, fileName, fileContent)
	}

	baseReport += fmt.Sprintf(`## Technical Details
- Generated at: %s
- Task ID: %s
- Client file uploaded: %v
- Content type: Markdown document
- Storage backend: MinIO cloud storage
- Bucket: artifacts
- Object versioning: Enabled

## Conclusions
This demonstrates both artifact creation and client file upload processing
through the MinIO cloud storage provider, providing scalable and distributed
artifact storage capabilities.
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

	// Store the artifact using the artifacts server with MinIO storage
	artifactID := task.ID
	filename := "analysis_report.md"

	storage := h.artifactsServer.GetStorage()
	url, err := storage.Store(ctx, artifactID, filename, strings.NewReader(artifactContent))
	if err != nil {
		h.logger.Error("failed to store artifact in MinIO", zap.Error(err))
		return nil, fmt.Errorf("failed to store artifact in MinIO: %w", err)
	}

	h.logger.Info("artifact stored successfully in MinIO",
		zap.String("artifact_id", artifactID),
		zap.String("filename", filename),
		zap.String("url", url))

	// Create artifact object for the response
	artifact := types.Artifact{
		ArtifactID:  artifactID,
		Name:        stringPtr("Analysis Report"),
		Description: stringPtr("A detailed analysis report based on your request, stored in MinIO cloud storage"),
		Parts: []types.Part{
			types.FilePart{
				Kind: "file",
				File: types.FileWithUri{
					URI:      url,
					MIMEType: stringPtr("text/markdown"),
					Name:     stringPtr(filename),
				},
			},
		},
	}

	// Create response message with artifact
	responseText := fmt.Sprintf("I've created an analysis report for your request: \"%s\". The report has been saved as an artifact in MinIO cloud storage and is available for download.", userText)

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

// Artifacts MinIO Example
//
// This example demonstrates an A2A server that creates downloadable artifacts
// using MinIO cloud storage. The server creates analysis reports as markdown files
// and makes them available for download via HTTP endpoints using MinIO as the
// storage backend.
//
// Features:
// - Creates markdown artifacts for user requests
// - Stores artifacts using MinIO cloud storage provider
// - Serves artifacts via HTTP download endpoints with MinIO backend
// - Includes proper artifact metadata in responses
// - Scalable cloud storage with object versioning
//
// Configuration via environment variables:
//   - ENVIRONMENT: Runtime environment (default: development)
//   - A2A_AGENT_NAME: Agent name (default: artifacts-minio-agent)
//   - A2A_SERVER_PORT: A2A server port (default: 8080)
//   - A2A_ARTIFACTS_ENABLE: Enable artifacts support (default: true)
//   - A2A_ARTIFACTS_SERVER_HOST: Artifacts server host (default: localhost)
//   - A2A_ARTIFACTS_SERVER_PORT: Artifacts server port (default: 8081)
//   - A2A_ARTIFACTS_STORAGE_PROVIDER: Storage provider (default: minio)
//   - A2A_ARTIFACTS_STORAGE_MINIO_ENDPOINT: MinIO endpoint (default: localhost:9000)
//   - A2A_ARTIFACTS_STORAGE_MINIO_ACCESS_KEY: MinIO access key (default: minioadmin)
//   - A2A_ARTIFACTS_STORAGE_MINIO_SECRET_KEY: MinIO secret key (default: minioadmin)
//   - A2A_ARTIFACTS_STORAGE_MINIO_BUCKET: MinIO bucket name (default: artifacts)
//   - A2A_ARTIFACTS_STORAGE_MINIO_USE_SSL: Use SSL for MinIO (default: false)
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
					Provider: "minio",
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
		zap.String("minio_endpoint", cfg.A2A.ArtifactsConfig.StorageConfig.Endpoint),
		zap.String("minio_bucket", cfg.A2A.ArtifactsConfig.StorageConfig.BucketName),
		zap.Bool("minio_use_ssl", cfg.A2A.ArtifactsConfig.StorageConfig.UseSSL),
		zap.Bool("debug", cfg.A2A.Debug),
	)

	// Create artifacts server with MinIO storage
	artifactsServer, err := server.
		NewArtifactsServerBuilder(&cfg.A2A.ArtifactsConfig, logger).
		Build()
	if err != nil {
		logger.Fatal("failed to create artifacts server with MinIO storage", zap.Error(err))
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
