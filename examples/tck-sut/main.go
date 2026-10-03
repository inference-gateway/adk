package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	uuid "github.com/google/uuid"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

const (
	port             = "9999"
	streamingTimeout = 2 * time.Second
)

var streamedText = map[string]string{
	"tck-stream-001":           "Stream hello from TCK",
	"tck-stream-003":           "Stream task lifecycle",
	"tck-stream-ordering-001":  "Ordered output",
	"tck-stream-artifact-text": "Streamed text content",
}

// TCKHandler implements the a2aproject/a2a-tck scenarios (scenarios/*.feature), selecting
// the behaviour by the messageId prefix the TCK sends.
type TCKHandler struct {
	agent server.OpenAICompatibleAgent
}

// HandleTask implements the core_operations.feature scenarios.
func (h *TCKHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	id := message.MessageID
	switch {
	case strings.HasPrefix(id, "tck-reject-task"):
		return task, errors.New("rejected")
	case strings.HasPrefix(id, "tck-input-required"):
		task.Status.State = types.TaskStateInputRequired
		return task, nil
	case strings.HasPrefix(id, "tck-complete-task"):
		return complete(task, "Hello from TCK"), nil
	case strings.HasPrefix(id, "tck-artifact-text"):
		addArtifact(task, types.CreateTextPart("Generated text content"))
	case strings.HasPrefix(id, "tck-artifact-file-url"):
		url := "https://example.com/output.txt"
		addArtifact(task, types.CreateFilePart("output.txt", "text/plain", nil, &url))
	case strings.HasPrefix(id, "tck-artifact-file"):
		addArtifact(task, fileArtifactPart())
	case strings.HasPrefix(id, "tck-artifact-data"):
		addArtifact(task, types.CreateDataPart(map[string]any{"key": "value", "count": 42}))
	default:
		return complete(task, "Unhandled messageId prefix: "+id), nil
	}
	return complete(task, ""), nil
}

// RespondToMessage answers the tck-message-response scenario with a direct Message, leaving every
// other prefix to the task flow.
func (h *TCKHandler) RespondToMessage(ctx context.Context, message *types.Message) (*types.Message, error) {
	if !strings.HasPrefix(message.MessageID, "tck-message-response") {
		return nil, nil
	}
	return &types.Message{
		MessageID: uuid.New().String(),
		ContextID: message.ContextID,
		Role:      types.RoleAgent,
		Parts:     []types.Part{types.CreateTextPart("Direct message response")},
	}, nil
}

// HandleStreamingTask implements the streaming.feature scenarios and falls back to the
// core scenarios for any other prefix.
func (h *TCKHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error) {
	events := make(chan cloudevents.Event, 8)
	go func() {
		defer close(events)
		id := message.MessageID
		switch {
		case strings.HasPrefix(id, "tck-stream-002"):
		case strings.HasPrefix(id, "test-resubscribe-message-id"):
			events <- statusEvent(types.TaskStateWorking)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * streamingTimeout):
			}
		case strings.HasPrefix(id, "tck-stream-artifact-file"):
			events <- statusEvent(types.TaskStateWorking)
			events <- artifactEvent(newArtifact(fileArtifactPart()), false, true)
		case strings.HasPrefix(id, "tck-stream-artifact-chunked"):
			events <- statusEvent(types.TaskStateWorking)
			chunk := newArtifact(types.CreateTextPart("chunk-1 "))
			events <- artifactEvent(chunk, false, false)
			chunk.Parts = []types.Part{types.CreateTextPart("chunk-2")}
			events <- artifactEvent(chunk, true, true)
		case streamedTextFor(id) != "":
			events <- statusEvent(types.TaskStateWorking)
			events <- artifactEvent(newArtifact(types.CreateTextPart(streamedTextFor(id))), false, true)
		default:
			result, err := h.HandleTask(ctx, task, message)
			if err != nil {
				events <- statusEvent(types.TaskStateFailed)
				return
			}
			events <- statusEvent(result.Status.State)
			return
		}
		events <- statusEvent(types.TaskStateCompleted)
	}()
	return events, nil
}

// SetAgent is required by the TaskHandler interface but unused here.
func (h *TCKHandler) SetAgent(agent server.OpenAICompatibleAgent) { h.agent = agent }

// GetAgent is required by the TaskHandler interface but unused here.
func (h *TCKHandler) GetAgent() server.OpenAICompatibleAgent { return h.agent }

func streamedTextFor(messageID string) string {
	for prefix, text := range streamedText {
		if strings.HasPrefix(messageID, prefix) {
			return text
		}
	}
	return ""
}

func fileArtifactPart() types.Part {
	raw := base64.StdEncoding.EncodeToString([]byte("tck"))
	return types.CreateFilePart("output.txt", "text/plain", &raw, nil)
}

func newArtifact(parts ...types.Part) types.Artifact {
	return types.Artifact{ArtifactID: uuid.New().String(), Parts: parts}
}

func addArtifact(task *types.Task, parts ...types.Part) {
	task.Artifacts = append(task.Artifacts, newArtifact(parts...))
}

// complete marks the task completed, replying with text unless it is empty.
func complete(task *types.Task, text string) *types.Task {
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = nil
	if text == "" {
		return task
	}
	reply := types.Message{
		MessageID: uuid.New().String(),
		ContextID: task.ContextID,
		TaskID:    &task.ID,
		Role:      types.RoleAgent,
		Parts:     []types.Part{types.CreateTextPart(text)},
	}
	task.History = append(task.History, reply)
	task.Status.Message = &reply
	return task
}

func artifactEvent(artifact types.Artifact, appendParts, lastChunk bool) cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetType(types.EventTaskArtifactUpdated)
	_ = event.SetData(cloudevents.ApplicationJSON, types.TaskArtifactUpdateEvent{
		Artifact:  artifact,
		Append:    &appendParts,
		LastChunk: &lastChunk,
	})
	return event
}

func statusEvent(state types.TaskState) cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetType(types.EventTaskStatusChanged)
	_ = event.SetData(cloudevents.ApplicationJSON, types.TaskStatus{State: state})
	return event
}

// TCK System Under Test
//
// An ADK server that implements the a2aproject/a2a-tck scenarios so the TCK can run
// against the Go ADK. See README.md for running the TCK against it.
func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()

	streaming := true
	pushNotifications := true
	handler := &TCKHandler{}
	card := types.AgentCard{
		Name:        "tck-sut",
		Description: "System under test for the A2A TCK",
		Version:     "1.0.0",
		SupportedInterfaces: []types.AgentInterface{
			{URL: "http://localhost:" + port + "/a2a", ProtocolBinding: "JSONRPC", ProtocolVersion: "1.0"},
		},
		Capabilities:       types.AgentCapabilities{Streaming: &streaming, PushNotifications: &pushNotifications},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Skills: []types.AgentSkill{
			{ID: "tck", Name: "TCK Conformance", Description: "Handles TCK conformance test messages", Tags: []string{"tck"}},
		},
	}

	a2aServer, err := server.NewA2AServerBuilder(serverConfig.Config{
		AgentName:        "tck-sut",
		AgentDescription: "System under test for the A2A TCK",
		AgentVersion:     "1.0.0",
		ServerConfig:     serverConfig.ServerConfig{Port: port},
	}, logger).
		WithBackgroundTaskHandler(handler).
		WithStreamingTaskHandler(handler).
		WithAgentCard(card).
		WithExtendedAgentCard(card).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
