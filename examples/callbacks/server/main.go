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
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/adk/examples/callbacks/server/config"
)

// CallbacksTaskHandler implements task handlers with callback support
type CallbacksTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

// NewCallbacksTaskHandler creates a new task handler
func NewCallbacksTaskHandler(logger *zap.Logger) *CallbacksTaskHandler {
	return &CallbacksTaskHandler{logger: logger}
}

// HandleTask processes background tasks
func (h *CallbacksTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	if h.agent == nil {
		return nil, fmt.Errorf("no AI agent configured")
	}

	taskCtx := context.WithValue(ctx, server.TaskContextKey, task)

	streamChan, err := h.agent.RunWithStream(taskCtx, []types.Message{*message})
	if err != nil {
		return nil, fmt.Errorf("failed to get AI response: %w", err)
	}

	var fullResponse string
	for cloudEvent := range streamChan {
		switch cloudEvent.Type() {
		case types.EventDelta:
			var deltaMsg types.Message
			if err := cloudEvent.DataAs(&deltaMsg); err == nil {
				for _, part := range deltaMsg.Parts {
					if textPart, ok := part.(types.TextPart); ok {
						fullResponse += textPart.Text
					}
				}
			}
		case types.EventIterationCompleted:
			h.logger.Info("AI agent completed iteration")
		}
	}

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

	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &responseMessage
	task.History = append(task.History, responseMessage)

	return task, nil
}

// HandleStreamingTask processes streaming tasks
func (h *CallbacksTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error) {
	h.logger.Info("processing streaming task with callbacks", zap.String("task_id", task.ID))

	if h.agent == nil {
		return nil, fmt.Errorf("no AI agent configured for streaming")
	}

	return h.agent.RunWithStream(ctx, []types.Message{*message})
}

// SetAgent sets the OpenAI-compatible agent
func (h *CallbacksTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the OpenAI-compatible agent
func (h *CallbacksTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// Callbacks Example A2A Server
//
// This example demonstrates the callback feature in the ADK, showing how to:
// - Use BeforeAgent/AfterAgent callbacks to intercept agent execution
// - Use BeforeModel/AfterModel callbacks to intercept LLM calls
// - Use BeforeTool/AfterTool callbacks to intercept tool execution
//
// Configuration via environment variables:
//   - ENVIRONMENT: Runtime environment (default: development)
//   - A2A_AGENT_NAME: Agent name (default: callbacks-agent)
//   - A2A_SERVER_PORT: Server port (default: 8080)
//   - A2A_AGENT_CLIENT_PROVIDER: LLM provider (required: openai, anthropic)
//   - A2A_AGENT_CLIENT_MODEL: LLM model (required)
//
// To run: go run main.go
func main() {
	cfg := &config.Config{
		Environment: "development",
		A2A: serverConfig.Config{
			AgentName:        "callbacks-example-agent",
			AgentDescription: "An example agent demonstrating callback hooks",
			AgentVersion:     "1.0.0",
			Debug:            true,
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
		},
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	var logger *zap.Logger
	var err error
	if cfg.Environment == "development" || cfg.A2A.Debug {
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

	logger.Info("callbacks example starting",
		zap.String("environment", cfg.Environment),
		zap.String("agent_name", cfg.A2A.AgentName),
		zap.String("port", cfg.A2A.ServerConfig.Port),
	)

	if cfg.A2A.AgentConfig.Provider == "" {
		logger.Fatal("A2A_AGENT_CLIENT_PROVIDER is required")
	}
	if cfg.A2A.AgentConfig.Model == "" {
		logger.Fatal("A2A_AGENT_CLIENT_MODEL is required")
	}

	toolBox := server.NewDefaultToolBox(&cfg.A2A.AgentConfig.ToolBoxConfig)

	echoTool := server.NewBasicTool(
		"echo",
		"Echo the provided message back",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "The message to echo",
				},
			},
			"required": []string{"message"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			msg := args["message"].(string)
			return fmt.Sprintf("Echo: %s", msg), nil
		},
	)
	toolBox.AddTool(echoTool)

	callbackConfig := &server.CallbackConfig{
		BeforeAgent: []server.BeforeAgentCallback{
			func(ctx context.Context, callbackCtx *server.CallbackContext) *types.Message {
				logger.Info("BeforeAgent callback triggered",
					zap.String("agent_name", callbackCtx.AgentName),
					zap.String("task_id", callbackCtx.TaskID),
				)

				// Example: Check for blocked words and return early
				// Uncomment to enable guardrail:
				// if strings.Contains(callbackCtx.TaskID, "blocked") {
				// 	return &types.Message{
				// 		Kind:      "message",
				// 		Role:      "assistant",
				// 		Parts:     []types.Part{types.NewTextPart("Request blocked by guardrail")},
				// 	}
				// }

				// Return nil to proceed with normal execution
				return nil
			},
		},

		AfterAgent: []server.AfterAgentCallback{
			func(ctx context.Context, callbackCtx *server.CallbackContext, agentOutput *types.Message) *types.Message {
				logger.Info("AfterAgent callback triggered",
					zap.String("agent_name", callbackCtx.AgentName),
					zap.String("task_id", callbackCtx.TaskID),
				)

				// Example: Log the output (without modifying it)
				// You could modify the output here if needed
				return nil
			},
		},

		BeforeModel: []server.BeforeModelCallback{
			func(ctx context.Context, callbackCtx *server.CallbackContext, llmRequest *server.LLMRequest) *server.LLMResponse {
				logger.Info("BeforeModel callback triggered",
					zap.String("task_id", callbackCtx.TaskID),
					zap.Int("message_count", len(llmRequest.Contents)),
				)

				// Example: Implement response caching
				// Uncomment to return a cached response:
				// return &server.LLMResponse{
				// 	Content: &types.Message{
				// 		Kind:  "message",
				// 		Role:  "assistant",
				// 		Parts: []types.Part{types.NewTextPart("Cached response")},
				// 	},
				// }

				return nil
			},
		},

		AfterModel: []server.AfterModelCallback{
			func(ctx context.Context, callbackCtx *server.CallbackContext, llmResponse *server.LLMResponse) *server.LLMResponse {
				logger.Info("AfterModel callback triggered",
					zap.String("task_id", callbackCtx.TaskID),
				)

				// Example: Post-process or sanitize the response
				return nil
			},
		},

		BeforeTool: []server.BeforeToolCallback{
			func(ctx context.Context, tool server.Tool, args map[string]interface{}, toolCtx *server.ToolContext) map[string]interface{} {
				toolName := ""
				if tool != nil {
					toolName = tool.GetName()
				}
				logger.Info("BeforeTool callback triggered",
					zap.String("tool_name", toolName),
					zap.String("task_id", toolCtx.TaskID),
				)

				// Example: Implement tool-level authorization
				// if toolName == "sensitive_tool" && !isAuthorized(ctx) {
				// 	return map[string]interface{}{"result": "Unauthorized", "error": "Access denied"}
				// }

				return nil
			},
		},

		AfterTool: []server.AfterToolCallback{
			func(ctx context.Context, tool server.Tool, args map[string]interface{}, toolCtx *server.ToolContext, toolResult map[string]interface{}) map[string]interface{} {
				toolName := ""
				if tool != nil {
					toolName = tool.GetName()
				}
				logger.Info("AfterTool callback triggered",
					zap.String("tool_name", toolName),
					zap.String("task_id", toolCtx.TaskID),
					zap.Any("result", toolResult),
				)

				// Example: Sanitize sensitive data from tool results
				return nil
			},
		},
	}

	llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.A2A.AgentConfig, logger)
	if err != nil {
		logger.Fatal("failed to create LLM client", zap.Error(err))
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(&cfg.A2A.AgentConfig).
		WithLLMClient(llmClient).
		WithSystemPrompt("You are a helpful AI assistant demonstrating callback functionality. You have access to an echo tool.").
		WithMaxChatCompletion(10).
		WithToolBox(toolBox).
		WithCallbacks(callbackConfig).
		Build()
	if err != nil {
		logger.Fatal("failed to create AI agent", zap.Error(err))
	}

	taskHandler := NewCallbacksTaskHandler(logger)
	taskHandler.SetAgent(agent)

	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithBackgroundTaskHandler(taskHandler).
		WithStreamingTaskHandler(taskHandler).
		WithAgent(agent).
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
			Skills: []types.AgentSkill{
				{
					Name:        "echo",
					Description: "Echo messages back with callback logging",
				},
				{
					Name:        "callbacks_demo",
					Description: "Demonstrates all callback hooks in action",
				},
			},
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

	logger.Info("callbacks example server running",
		zap.String("port", cfg.A2A.ServerConfig.Port),
		zap.String("callbacks", "BeforeAgent, AfterAgent, BeforeModel, AfterModel, BeforeTool, AfterTool"),
	)

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
