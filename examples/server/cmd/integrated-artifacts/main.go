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

// IntegratedArtifactsTaskHandler demonstrates using artifacts server with A2A protocol
type IntegratedArtifactsTaskHandler struct {
	logger         *zap.Logger
	artifactHelper *server.ArtifactHelper
	agent          server.OpenAICompatibleAgent
	storage        server.ArtifactStorageProvider
}

// NewIntegratedArtifactsTaskHandler creates a new integrated task handler
func NewIntegratedArtifactsTaskHandler(logger *zap.Logger, storage server.ArtifactStorageProvider) *IntegratedArtifactsTaskHandler {
	return &IntegratedArtifactsTaskHandler{
		logger:         logger,
		artifactHelper: server.NewArtifactHelper(),
		storage:        storage,
	}
}

// SetAgent sets the OpenAI-compatible agent for the task handler
func (h *IntegratedArtifactsTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *IntegratedArtifactsTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// HandleTask processes a task and creates artifacts with downloadable URLs
func (h *IntegratedArtifactsTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	h.logger.Info("processing integrated artifacts task", zap.String("task_id", task.ID))

	// Extract user request
	userRequest := h.extractUserRequest(message)

	// Create a text artifact using the artifacts server storage
	textContent := fmt.Sprintf("Analysis Report\n\nRequest: %s\nLength: %d characters\nTimestamp: %s",
		userRequest,
		len(userRequest),
		time.Now().Format(time.RFC3339),
	)

	// Store the artifact content in the artifacts server storage
	artifactID := task.ID
	filename := "analysis-report.txt"

	url, err := h.storage.Store(ctx, artifactID, filename, strings.NewReader(textContent))
	if err != nil {
		h.logger.Error("failed to store artifact", zap.Error(err))
		return nil, fmt.Errorf("failed to store artifact: %w", err)
	}

	// Create artifact with URI pointing to the artifacts server
	artifact := h.artifactHelper.CreateFileArtifactFromURI(
		"Analysis Report",
		"Downloadable analysis report",
		filename,
		url,
		stringPtr("text/plain"),
	)

	// Add artifact to task
	h.artifactHelper.AddArtifactToTask(task, artifact)

	// Create response message
	response := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: fmt.Sprintf("I've processed your request and created an analysis report. The artifact is available for download at: %s", url),
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

	h.logger.Info("integrated artifacts task completed",
		zap.String("task_id", task.ID),
		zap.String("artifact_url", url))

	return task, nil
}

func (h *IntegratedArtifactsTaskHandler) extractUserRequest(message *types.Message) string {
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

func main() {
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create main A2A server configuration
	cfg := &config.Config{
		AgentName:        "Integrated Artifacts Agent",
		AgentDescription: "An A2A agent that creates downloadable artifacts via separate artifacts server",
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
				Provider: "filesystem",
				BasePath: "./artifacts",
			},
		},
	}

	// Create the artifacts server
	artifactsServer, err := server.NewArtifactsServerBuilder(&cfg.ArtifactsConfig, logger).
		WithFilesystemStorage("./artifacts", "http://localhost:8081").
		Build()
	if err != nil {
		logger.Fatal("Failed to build artifacts server", zap.Error(err))
	}

	// Create the main A2A server
	a2aServer := server.NewA2AServer(cfg, logger, nil)

	// Set up our integrated task handler with access to artifacts storage
	taskHandler := NewIntegratedArtifactsTaskHandler(logger, artifactsServer.GetStorage())
	a2aServer.SetBackgroundTaskHandler(taskHandler)

	// Create and set agent card
	agentCard := types.AgentCard{
		Name:            "Integrated Artifacts Agent",
		Version:         "1.0.0",
		Description:     "Demonstrates integration between A2A protocol and artifacts server for downloadable content",
		ProtocolVersion: "1.0.0",
		URL:             "http://localhost:8080",
		Capabilities: types.AgentCapabilities{
			Streaming: boolPtr(false),
		},
		Skills: []types.AgentSkill{
			{
				ID:          "artifact-creation-with-download",
				Name:        "Downloadable Artifact Creation",
				Description: "Creates artifacts that can be downloaded via separate artifacts server endpoints",
				Tags:        []string{"artifacts", "download", "integration"},
				Examples: []string{
					"Analyze this text and create a downloadable report",
					"Process my data and generate downloadable analysis",
					"Create a downloadable summary of my request",
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
		logger.Info("starting artifacts server", zap.String("port", cfg.ArtifactsConfig.ServerConfig.Port))
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
Integrated Artifacts Demo Started!

Main A2A Server: http://localhost:%s
Artifacts Server: http://localhost:%s

This demo shows how to:
- Integrate A2A protocol with separate artifacts server
- Create artifacts with downloadable URLs
- Use artifacts server storage from task handlers
- Provide downloadable content to A2A clients

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
            "text": "Please analyze this sample text and create a downloadable report"
          }
        ]
      }
    }
  }'

The response will contain artifacts with downloadable URLs pointing to the artifacts server.

Press Ctrl+C to stop both servers.
`, cfg.ServerConfig.Port, cfg.ArtifactsConfig.ServerConfig.Port, cfg.ServerConfig.Port)

	if err := a2aServer.Start(ctx); err != nil {
		logger.Fatal("Failed to start A2A server", zap.Error(err))
	}
}

func boolPtr(b bool) *bool {
	return &b
}
