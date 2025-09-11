package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/inference-gateway/adk/server"
	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

// ArtifactDemoTaskHandler demonstrates artifact creation in task processing
type ArtifactDemoTaskHandler struct {
	logger         *zap.Logger
	artifactHelper *server.ArtifactHelper
	agent          server.OpenAICompatibleAgent
}

// NewArtifactDemoTaskHandler creates a new artifact demo task handler
func NewArtifactDemoTaskHandler(logger *zap.Logger) *ArtifactDemoTaskHandler {
	return &ArtifactDemoTaskHandler{
		logger:         logger,
		artifactHelper: server.NewArtifactHelper(),
	}
}

// SetAgent sets the OpenAI-compatible agent for the task handler
func (h *ArtifactDemoTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *ArtifactDemoTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// HandleTask processes a task and demonstrates creating various types of artifacts
func (h *ArtifactDemoTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	h.logger.Info("processing artifact demo task", zap.String("task_id", task.ID))

	// Extract user request from the message
	userRequest := h.extractUserRequest(message)
	h.logger.Info("user request", zap.String("request", userRequest))

	// Create various artifacts based on the request
	if err := h.createArtifactsForTask(task, userRequest); err != nil {
		return nil, fmt.Errorf("failed to create artifacts: %w", err)
	}

	// Create a response message
	response := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: h.buildResponseText(task),
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

	h.logger.Info("artifact demo task completed",
		zap.String("task_id", task.ID),
		zap.Int("artifacts_created", len(task.Artifacts)))

	return task, nil
}

// extractUserRequest extracts the user's request from the message
func (h *ArtifactDemoTaskHandler) extractUserRequest(message *types.Message) string {
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

// createArtifactsForTask creates different types of artifacts based on the user request
func (h *ArtifactDemoTaskHandler) createArtifactsForTask(task *types.Task, userRequest string) error {
	// 1. Create a text artifact with analysis
	textArtifact := h.artifactHelper.CreateTextArtifact(
		"Request Analysis",
		"Analysis of the user's request",
		fmt.Sprintf("User Request: %s\n\nLength: %d characters\nWords: %d\nTimestamp: %s",
			userRequest,
			len(userRequest),
			len(strings.Fields(userRequest)),
			time.Now().Format(time.RFC3339),
		),
	)
	h.artifactHelper.AddArtifactToTask(task, textArtifact)

	// 2. Create a JSON data artifact with structured information
	analysisData := map[string]any{
		"request":         userRequest,
		"character_count": len(userRequest),
		"word_count":      len(strings.Fields(userRequest)),
		"timestamp":       time.Now().Format(time.RFC3339),
		"processing_stats": map[string]any{
			"task_id":    task.ID,
			"context_id": task.ContextID,
			"artifacts":  []string{},
		},
	}

	dataArtifact := h.artifactHelper.CreateDataArtifact(
		"Request Data",
		"Structured data analysis of the request",
		analysisData,
	)
	h.artifactHelper.AddArtifactToTask(task, dataArtifact)

	// 3. Create a CSV file artifact with sample data
	csvContent := "timestamp,request_length,word_count\n" +
		fmt.Sprintf("%s,%d,%d\n",
			time.Now().Format(time.RFC3339),
			len(userRequest),
			len(strings.Fields(userRequest)),
		)

	mimeType := h.artifactHelper.GetMimeTypeFromExtension("analysis.csv")
	csvArtifact := h.artifactHelper.CreateFileArtifactFromBytes(
		"Request Analysis CSV",
		"CSV file containing request analysis data",
		"analysis.csv",
		[]byte(csvContent),
		mimeType,
	)
	h.artifactHelper.AddArtifactToTask(task, csvArtifact)

	// 4. Create a multi-part artifact combining text and data
	multiParts := []types.Part{
		types.TextPart{
			Kind: "text",
			Text: "Multi-part artifact combining analysis and metadata",
		},
		types.DataPart{
			Kind: "data",
			Data: map[string]any{
				"summary":    "This artifact demonstrates multiple content types",
				"components": []string{"text", "data"},
				"created_at": time.Now().Unix(),
			},
		},
	}

	multiPartArtifact := h.artifactHelper.CreateMultiPartArtifact(
		"Multi-Part Analysis",
		"Artifact containing both text and structured data",
		multiParts,
	)
	h.artifactHelper.AddArtifactToTask(task, multiPartArtifact)

	// 5. Create a reference artifact pointing to external resources
	externalArtifact := h.artifactHelper.CreateFileArtifactFromURI(
		"A2A Protocol Documentation",
		"Link to the official A2A protocol documentation",
		"a2a-protocol.html",
		"https://a2aprotocol.ai/docs/",
		stringPtr("text/html"),
	)
	h.artifactHelper.AddArtifactToTask(task, externalArtifact)

	return nil
}

// buildResponseText creates a response text that mentions the created artifacts
func (h *ArtifactDemoTaskHandler) buildResponseText(task *types.Task) string {
	artifactCount := len(task.Artifacts)

	response := fmt.Sprintf("I've processed your request and created %d artifacts:\n\n", artifactCount)

	for i, artifact := range task.Artifacts {
		name := "Unnamed Artifact"
		if artifact.Name != nil {
			name = *artifact.Name
		}

		description := "No description"
		if artifact.Description != nil {
			description = *artifact.Description
		}

		partTypes := make([]string, 0)
		for _, part := range artifact.Parts {
			switch p := part.(type) {
			case types.TextPart:
				partTypes = append(partTypes, p.Kind)
			case types.FilePart:
				partTypes = append(partTypes, p.Kind)
			case types.DataPart:
				partTypes = append(partTypes, p.Kind)
			}
		}

		response += fmt.Sprintf("%d. %s\n   - %s\n   - Contains: %v\n   - ID: %s\n\n",
			i+1, name, description, partTypes, artifact.ArtifactID)
	}

	response += "All artifacts are attached to this task and can be accessed via the A2A protocol using the task ID."

	return response
}

// stringPtr is a helper function to create string pointers
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

	// Create configuration
	cfg := &config.Config{
		AgentName:        "Artifact Demo Agent",
		AgentDescription: "An A2A agent that demonstrates artifact creation and usage",
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
	}

	// Create the A2A server
	server := server.NewA2AServer(cfg, logger, nil)

	// Set our custom task handler that creates artifacts
	taskHandler := NewArtifactDemoTaskHandler(logger)
	server.SetBackgroundTaskHandler(taskHandler)

	// Create and set a sample agent card
	agentCard := types.AgentCard{
		Name:            "Artifact Demo Agent",
		Version:         "1.0.0",
		Description:     "Demonstrates artifact creation and management in A2A protocol",
		ProtocolVersion: "1.0.0",
		URL:             "http://localhost:8080",
		Capabilities: types.AgentCapabilities{
			Streaming: boolPtr(false), // Focus on background task processing for this demo
		},
		Skills: []types.AgentSkill{
			{
				ID:          "artifact-creation",
				Name:        "Artifact Creation",
				Description: "Creates various types of artifacts from user requests",
				Tags:        []string{"artifacts", "demo", "analysis"},
				Examples: []string{
					"Analyze this text and create artifacts",
					"Process my request and generate analysis files",
					"Create structured data from my input",
				},
			},
		},
	}

	server.SetAgentCard(agentCard)

	// Start the server
	ctx := context.Background()
	logger.Info("Starting artifact demo server",
		zap.String("port", cfg.ServerConfig.Port),
		zap.String("agent_name", cfg.AgentName))

	fmt.Printf(`
Artifact Demo Server Started!

The server is running on http://localhost:%s

This demo shows how to:
- Create different types of artifacts (text, file, data, multi-part)
- Add artifacts to tasks during processing
- Structure artifacts according to A2A protocol specification

You can send requests to test artifact creation:

Example using curl:
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
            "text": "Please analyze this sample text and create some artifacts for me"
          }
        ]
      }
    }
  }'

Then retrieve the task with artifacts:
curl -X POST http://localhost:%s/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tasks/get",
    "id": "get-1",
    "params": {
      "id": "TASK_ID_FROM_PREVIOUS_RESPONSE"
    }
  }'

Press Ctrl+C to stop the server.
`, cfg.ServerConfig.Port, cfg.ServerConfig.Port, cfg.ServerConfig.Port)

	if err := server.Start(ctx); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}

// boolPtr creates a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}
