package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/inference-gateway/adk/server"
	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

// MinIOArtifactsTaskHandler demonstrates using MinIO storage with artifacts server
type MinIOArtifactsTaskHandler struct {
	logger         *zap.Logger
	artifactHelper *server.ArtifactHelper
	agent          server.OpenAICompatibleAgent
	storage        server.ArtifactStorageProvider
}

// NewMinIOArtifactsTaskHandler creates a new MinIO-based task handler
func NewMinIOArtifactsTaskHandler(logger *zap.Logger, storage server.ArtifactStorageProvider) *MinIOArtifactsTaskHandler {
	return &MinIOArtifactsTaskHandler{
		logger:         logger,
		artifactHelper: server.NewArtifactHelper(),
		storage:        storage,
	}
}

// SetAgent sets the OpenAI-compatible agent for the task handler
func (h *MinIOArtifactsTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *MinIOArtifactsTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// HandleTask processes a task and creates artifacts stored in MinIO
func (h *MinIOArtifactsTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	h.logger.Info("processing MinIO artifacts task", zap.String("task_id", task.ID))

	// Extract user request
	userRequest := h.extractUserRequest(message)

	// Create multiple artifacts to demonstrate MinIO capabilities
	artifacts := []struct {
		filename    string
		content     string
		contentType string
		description string
	}{
		{
			filename:    "analysis-report.txt",
			content:     fmt.Sprintf("MinIO Analysis Report\n\nRequest: %s\nLength: %d characters\nTimestamp: %s\nStorage: MinIO S3-compatible object storage", userRequest, len(userRequest), time.Now().Format(time.RFC3339)),
			contentType: "text/plain",
			description: "Text analysis report stored in MinIO",
		},
		{
			filename:    "metadata.json",
			content:     fmt.Sprintf(`{"request_id": "%s", "timestamp": "%s", "storage_provider": "minio", "request_length": %d, "processed_by": "MinIO Artifacts Agent"}`, task.ID, time.Now().Format(time.RFC3339), len(userRequest)),
			contentType: "application/json",
			description: "Request metadata in JSON format",
		},
		{
			filename:    "summary.md",
			content:     fmt.Sprintf("# Processing Summary\n\n## Request Details\n- **ID**: %s\n- **Timestamp**: %s\n- **Content Length**: %d characters\n- **Storage**: MinIO\n\n## Content\n```\n%s\n```\n\n## Storage Benefits\n- **Scalability**: MinIO provides S3-compatible object storage\n- **Reliability**: Built-in data protection and versioning\n- **Performance**: High-performance distributed storage\n- **Compatibility**: Works with existing S3 tools and libraries", task.ID, time.Now().Format(time.RFC3339), len(userRequest), userRequest),
			contentType: "text/markdown",
			description: "Markdown summary with MinIO storage benefits",
		},
	}

	// Store all artifacts in MinIO and create artifact references
	taskArtifacts := []types.Artifact{}
	artifactURLs := []string{}

	for _, artifact := range artifacts {
		// Store the artifact content in MinIO
		url, err := h.storage.Store(ctx, task.ID, artifact.filename, strings.NewReader(artifact.content))
		if err != nil {
			h.logger.Error("failed to store artifact in MinIO", 
				zap.Error(err), 
				zap.String("filename", artifact.filename))
			return nil, fmt.Errorf("failed to store artifact %s in MinIO: %w", artifact.filename, err)
		}

		// Create artifact with URI pointing to the artifacts server
		taskArtifact := h.artifactHelper.CreateFileArtifactFromURI(
			strings.TrimSuffix(artifact.filename, fmt.Sprintf(".%s", strings.Split(artifact.filename, ".")[1])),
			artifact.description,
			artifact.filename,
			url,
			stringPtr(artifact.contentType),
		)

		taskArtifacts = append(taskArtifacts, taskArtifact)
		artifactURLs = append(artifactURLs, url)

		h.logger.Info("stored artifact in MinIO", 
			zap.String("filename", artifact.filename), 
			zap.String("url", url))
	}

	// Add all artifacts to task
	for _, artifact := range taskArtifacts {
		h.artifactHelper.AddArtifactToTask(task, artifact)
	}

	// Create response message
	responseText := fmt.Sprintf(`I've processed your request using MinIO storage and created %d downloadable artifacts:

%s

**MinIO Storage Benefits:**
- **Cloud-Native**: S3-compatible object storage that scales infinitely
- **High Performance**: Optimized for high-throughput workloads
- **Data Protection**: Built-in erasure coding and versioning
- **Multi-Cloud**: Works across different cloud providers
- **Enterprise Ready**: Production-grade features like encryption and access control

All artifacts are securely stored in MinIO and accessible via the artifacts server endpoints.`, 
		len(artifacts),
		func() string {
			var lines []string
			for i, url := range artifactURLs {
				lines = append(lines, fmt.Sprintf("- **%s**: %s", artifacts[i].filename, url))
			}
			return strings.Join(lines, "\n")
		}())

	response := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: responseText,
			},
		},
	}

	// Add response to history
	if task.History == nil {
		task.History = []types.Message{}
	}
	task.History = append(task.History, *response)

	// Mark task as completed
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = response

	h.logger.Info("MinIO artifacts task completed",
		zap.String("task_id", task.ID),
		zap.Int("artifacts_count", len(taskArtifacts)))

	return task, nil
}

func (h *MinIOArtifactsTaskHandler) extractUserRequest(message *types.Message) string {
	for _, part := range message.Parts {
		if textPart, ok := part.(map[string]any); ok {
			if kind, exists := textPart["kind"].(string); exists && kind == "text" {
				if text, textExists := textPart["text"].(string); textExists {
					return text
				}
			}
		}
		if textPart, ok := part.(types.TextPart); ok && textPart.Kind == "text" {
			return textPart.Text
		}
	}
	return ""
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func main() {
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Configure MinIO connection from environment variables with sensible defaults
	minioEndpoint := getEnvOrDefault("MINIO_ENDPOINT", "localhost:9000")
	minioAccessKey := getEnvOrDefault("MINIO_ACCESS_KEY", "admin")
	minioSecretKey := getEnvOrDefault("MINIO_SECRET_KEY", "password123")
	minioBucket := getEnvOrDefault("MINIO_BUCKET", "artifacts")
	minioUseSSL := getEnvOrDefault("MINIO_USE_SSL", "false") == "true"

	// Create main A2A server configuration
	cfg := &config.Config{
		AgentName:        "MinIO Artifacts Agent",
		AgentDescription: "An A2A agent that demonstrates MinIO-based artifact storage with downloadable content",
		AgentVersion:     "1.0.0",
		ServerConfig: config.ServerConfig{
			Port:         "8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		QueueConfig: config.QueueConfig{
			MaxSize:         100,
			CleanupInterval: 5 * time.Minute,
		},
		ArtifactsConfig: config.ArtifactsConfig{
			Enable: true,
			ServerConfig: config.ArtifactsServerConfig{
				Port:         "8081",
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  60 * time.Second,
			},
			StorageConfig: config.ArtifactsStorageConfig{
				Provider: "minio",
				MinIOConfig: config.MinIOConfig{
					Endpoint:  minioEndpoint,
					AccessKey: minioAccessKey,
					SecretKey: minioSecretKey,
					Bucket:    minioBucket,
					UseSSL:    minioUseSSL,
				},
			},
		},
	}

	// Create the artifacts server with MinIO storage
	artifactsServer, err := server.NewArtifactsServerBuilder(&cfg.ArtifactsConfig, logger).
		WithMinIOStorage(
			minioEndpoint,
			minioAccessKey,
			minioSecretKey,
			minioBucket,
			minioUseSSL,
			"http://localhost:8081", // Base URL for artifact downloads
		).
		Build()
	if err != nil {
		logger.Fatal("Failed to build artifacts server with MinIO storage", zap.Error(err))
	}

	// Create the main A2A server
	a2aServer := server.NewA2AServer(cfg, logger, nil)

	// Set up our MinIO task handler with access to MinIO storage
	taskHandler := NewMinIOArtifactsTaskHandler(logger, artifactsServer.GetStorage())
	a2aServer.SetBackgroundTaskHandler(taskHandler)

	// Create and set agent card
	agentCard := types.AgentCard{
		Name:            "MinIO Artifacts Agent",
		Version:         "1.0.0",
		Description:     "Demonstrates MinIO-based artifact storage with the A2A protocol for scalable, cloud-native file management",
		ProtocolVersion: "1.0.0",
		URL:             "http://localhost:8080",
		Capabilities: types.AgentCapabilities{
			Streaming: boolPtr(false),
		},
		Skills: []types.AgentSkill{
			{
				ID:          "minio-artifact-creation",
				Name:        "MinIO-Based Artifact Creation",
				Description: "Creates multiple downloadable artifacts stored in MinIO S3-compatible object storage",
				Tags:        []string{"artifacts", "minio", "s3", "cloud-storage", "scalable"},
				Examples: []string{
					"Process this data and store results in MinIO",
					"Create downloadable reports using cloud storage",
					"Generate artifacts with S3-compatible storage backend",
					"Analyze content and provide MinIO-hosted downloads",
				},
			},
		},
	}

	a2aServer.SetAgentCard(agentCard)

	// Create context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start artifacts server in background
	go func() {
		logger.Info("starting artifacts server with MinIO storage", 
			zap.String("port", cfg.ArtifactsConfig.ServerConfig.Port),
			zap.String("minio_endpoint", minioEndpoint),
			zap.String("bucket", minioBucket))
		if err := artifactsServer.Start(ctx); err != nil {
			logger.Error("artifacts server failed", zap.Error(err))
		}
	}()

	// Give artifacts server time to start
	time.Sleep(100 * time.Millisecond)

	// Start main A2A server
	logger.Info("starting main A2A server",
		zap.String("port", cfg.ServerConfig.Port),
		zap.String("agent_name", cfg.AgentName))

	fmt.Printf(`
MinIO Artifacts Demo Started!

Main A2A Server: http://localhost:%s
Artifacts Server: http://localhost:%s
MinIO Console:    http://localhost:9001 (if running via docker-compose)

Configuration:
- MinIO Endpoint: %s
- MinIO Bucket:   %s
- MinIO SSL:      %t

This demo shows how to:
- Use MinIO S3-compatible storage for artifacts
- Scale artifact storage horizontally
- Leverage cloud-native storage features
- Provide enterprise-grade data protection
- Create multiple artifact types (text, JSON, markdown)

Environment Variables (optional):
- MINIO_ENDPOINT=%s
- MINIO_ACCESS_KEY=%s  
- MINIO_SECRET_KEY=%s
- MINIO_BUCKET=%s
- MINIO_USE_SSL=%t

Example request:
curl -X POST http://localhost:%s/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "id": "test-1",
    "params": {
      "message": {
        "kind": "message",
        "messageId": "msg-1",
        "role": "user",
        "parts": [
          {
            "kind": "text",
            "text": "Please analyze this data and create downloadable artifacts using MinIO storage"
          }
        ]
      }
    }
  }'

The response will contain multiple artifacts stored in MinIO with downloadable URLs.

Press Ctrl+C to stop both servers.
`, cfg.ServerConfig.Port, cfg.ArtifactsConfig.ServerConfig.Port, minioEndpoint, minioBucket, minioUseSSL, 
minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, minioUseSSL, cfg.ServerConfig.Port)

	if err := a2aServer.Start(ctx); err != nil {
		logger.Fatal("Failed to start A2A server", zap.Error(err))
	}
}

func getEnvOrDefault(envVar, defaultValue string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return defaultValue
}