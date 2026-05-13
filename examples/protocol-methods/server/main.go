package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	uuid "github.com/google/uuid"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/adk/examples/protocol-methods/server/config"
)

// SlowEchoTaskHandler is a background task handler that intentionally takes a
// few seconds to complete each task. The artificial delay gives the client
// enough time to demonstrate `tasks/cancel`, `tasks/list`, and the
// `tasks/pushNotificationConfig/*` family before the task settles into a
// terminal state.
type SlowEchoTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
	delay  time.Duration
}

// NewSlowEchoTaskHandler constructs a SlowEchoTaskHandler that sleeps for
// `delay` before completing each task.
func NewSlowEchoTaskHandler(logger *zap.Logger, delay time.Duration) *SlowEchoTaskHandler {
	return &SlowEchoTaskHandler{logger: logger, delay: delay}
}

// HandleTask processes background tasks. It sets the task to WORKING, sleeps for
// the configured delay (respecting cancellation), and then echoes the user
// message back. Honoring `ctx.Done()` is what makes `tasks/cancel` observable
// to the client - the moment the task manager cancels the context, this
// handler returns and the task transitions to CANCELLED.
func (h *SlowEchoTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	h.logger.Info("starting slow task",
		zap.String("task_id", task.ID),
		zap.Duration("delay", h.delay))

	userInput := ""
	if message != nil {
		for _, part := range message.Parts {
			if part.Text != nil {
				userInput = *part.Text
				break
			}
		}
	}

	select {
	case <-ctx.Done():
		h.logger.Info("task cancelled before completion", zap.String("task_id", task.ID))
		return task, ctx.Err()
	case <-time.After(h.delay):
	}

	responseText := fmt.Sprintf("Echo: %s", userInput)
	if userInput == "" {
		responseText = "Hello! Send me a message and I'll echo it back."
	}

	responseMessage := types.Message{
		MessageID: uuid.New().String(),
		ContextID: &task.ContextID,
		TaskID:    &task.ID,
		Role:      types.RoleAgent,
		Parts: []types.Part{
			types.CreateTextPart(responseText),
		},
	}

	task.History = append(task.History, responseMessage)
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &responseMessage

	h.logger.Info("slow task completed", zap.String("task_id", task.ID))
	return task, nil
}

// HandleStreamingTask satisfies the StreamableTaskHandler interface so the
// server can demonstrate `tasks/resubscribe`. It emits a sequence of delta
// events with a small delay between them so a client can resubscribe in the
// middle of the stream.
func (h *SlowEchoTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error) {
	h.logger.Info("starting streaming task",
		zap.String("task_id", task.ID),
		zap.Duration("delay", h.delay))

	eventChan := make(chan cloudevents.Event, 16)
	go func() {
		defer close(eventChan)

		statusEvent := cloudevents.NewEvent()
		statusEvent.SetType(types.EventTaskStatusChanged)
		statusEvent.SetData(cloudevents.ApplicationJSON, types.TaskStatus{State: types.TaskStateWorking})
		eventChan <- statusEvent

		words := []string{"resubscribed", "stream", "in", "progress", "from", "the", "server"}
		var fullText string
		for i, word := range words {
			select {
			case <-ctx.Done():
				h.logger.Info("streaming task cancelled", zap.String("task_id", task.ID))
				return
			case <-time.After(h.delay / time.Duration(len(words)+1)):
			}

			delta := word
			if i > 0 {
				delta = " " + word
			}
			fullText += delta

			deltaMessage := types.Message{
				Role: types.RoleAgent,
				Parts: []types.Part{
					types.CreateTextPart(delta),
				},
			}
			event := cloudevents.NewEvent()
			event.SetType(types.EventDelta)
			event.SetData(cloudevents.ApplicationJSON, deltaMessage)
			eventChan <- event
		}

		finalMessage := types.Message{
			MessageID: uuid.New().String(),
			Role:      types.RoleAgent,
			TaskID:    &task.ID,
			ContextID: &task.ContextID,
			Parts: []types.Part{
				types.CreateTextPart(fullText),
			},
		}
		eventChan <- types.NewIterationCompletedEvent(1, task.ID, &finalMessage)
	}()

	return eventChan, nil
}

// SetAgent is required by the TaskHandler interface but unused here.
func (h *SlowEchoTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) { h.agent = agent }

// GetAgent is required by the TaskHandler interface but unused here.
func (h *SlowEchoTaskHandler) GetAgent() server.OpenAICompatibleAgent { return h.agent }

// Protocol Methods A2A Server Example
//
// This example runs an A2A server that intentionally exercises every JSON-RPC
// method beyond the common `message/send`, `message/stream`, and `tasks/get`
// trio. The bundled client (../client) walks through each of:
//
//   - `tasks/cancel`
//   - `tasks/list` (with pagination)
//   - `tasks/pushNotificationConfig/set`
//   - `tasks/pushNotificationConfig/get`
//   - `tasks/pushNotificationConfig/list`
//   - `tasks/pushNotificationConfig/delete`
//   - `tasks/resubscribe`
//   - `agent/getAuthenticatedExtendedCard`
//
// The slow task handler is deliberately sluggish so the client has time to
// list and cancel tasks before they reach a terminal state.
//
// To run: go run main.go
func main() {
	cfg := &config.Config{
		Environment: "development",
		A2A: serverConfig.Config{
			AgentName:        "protocol-methods-agent",
			AgentDescription: "Demonstrates the full A2A JSON-RPC surface beyond message/send and tasks/get",
			AgentVersion:     "0.1.0",
			Debug:            false,
			CapabilitiesConfig: serverConfig.CapabilitiesConfig{
				Streaming:              true,
				PushNotifications:      true,
				StateTransitionHistory: false,
			},
			QueueConfig: serverConfig.QueueConfig{
				CleanupInterval: 5 * time.Minute,
			},
			ServerConfig: serverConfig.ServerConfig{
				Port: "8080",
			},
		},
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

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
	defer func() { _ = logger.Sync() }()

	logger.Info("server starting",
		zap.String("environment", cfg.Environment),
		zap.String("agent_name", cfg.A2A.AgentName),
		zap.String("port", cfg.A2A.ServerConfig.Port),
		zap.Bool("debug", cfg.A2A.Debug),
	)

	taskHandler := NewSlowEchoTaskHandler(logger, 6*time.Second)

	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithBackgroundTaskHandler(taskHandler).
		WithStreamingTaskHandler(taskHandler).
		WithAgentCard(types.AgentCard{
			Name:            cfg.A2A.AgentName,
			Description:     cfg.A2A.AgentDescription,
			Version:         cfg.A2A.AgentVersion,
			URL:             new(fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port)),
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("server running", zap.String("port", cfg.A2A.ServerConfig.Port))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
