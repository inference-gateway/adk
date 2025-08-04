package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"
)

func main() {
	fmt.Println("🤖 Starting AI-Powered A2A Server with Input-Required Pausing...")

	// Step 1: Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Step 2: Load configuration from environment
	cfg := config.Config{
		AgentName:        "pausable-task-agent",
		AgentDescription: "An AI-powered agent that can pause task execution to request additional user input when needed",
		AgentVersion:     "1.0.0",
		QueueConfig: config.QueueConfig{
			CleanupInterval: 5 * time.Minute,
		},
		ServerConfig: config.ServerConfig{
			Port: "8080",
		},
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, &cfg); err != nil {
		logger.Fatal("failed to process environment config", zap.Error(err))
	}

	// Step 3: Create toolbox with input_required tool
	toolBox := server.NewDefaultToolBox()

	// Add input_required tool that the LLM can call to pause task execution
	inputRequiredTool := server.NewBasicTool(
		"input_required",
		"Request additional input from the user when current information is insufficient to complete the task",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "The message to display to the user explaining what information is needed",
				},
			},
			"required": []string{"message"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			message := args["message"].(string)

			logger.Info("LLM called input_required tool - this will pause the task",
				zap.String("message", message))

			// Return success response indicating input is required
			return fmt.Sprintf("Input requested from user: %s", message), nil
		},
	)
	toolBox.AddTool(inputRequiredTool)

	// Add helper tools that might be useful
	weatherTool := server.NewBasicTool(
		"get_weather",
		"Get current weather information for a location",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city name",
				},
			},
			"required": []string{"location"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			location := args["location"].(string)
			return fmt.Sprintf(`{"location": "%s", "temperature": "22°C", "condition": "sunny", "humidity": "65%%"}`, location), nil
		},
	)
	toolBox.AddTool(weatherTool)

	timeTool := server.NewBasicTool(
		"get_current_time",
		"Get the current date and time",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			now := time.Now()
			return fmt.Sprintf(`{"current_time": "%s", "timezone": "%s"}`,
				now.Format("2006-01-02 15:04:05"), now.Location()), nil
		},
	)
	toolBox.AddTool(timeTool)

	// Step 4: Create AI agent with LLM client
	llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.AgentConfig, logger)
	if err != nil {
		logger.Fatal("failed to create LLM client", zap.Error(err))
	}

	// Create agent with system prompt that encourages intelligent input pausing
	systemPrompt := `You are a helpful AI assistant that creates detailed content and provides information.

When users request something that could benefit from more specific information, you should:
1. First analyze if you have enough context to provide a useful response
2. If the request is vague or could be significantly improved with more details, CALL the "input_required" function to request clarification
3. Be specific about what information would help you provide a better response

For example:
- If someone asks for a "presentation outline" without specifying the audience, topic depth, or duration, call the input_required function
- If someone asks for weather without a location, call the input_required function  
- If someone asks for advice without context, call the input_required function

IMPORTANT: Always use JSON for tool calls.`

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(&cfg.AgentConfig).
		WithLLMClient(llmClient).
		WithSystemPrompt(systemPrompt).
		WithMaxChatCompletion(10).
		WithToolBox(toolBox).
		Build()
	if err != nil {
		logger.Fatal("failed to create AI agent", zap.Error(err))
	}

	// Step 5: Create a custom task handler that can pause tasks for input
	pausableTaskHandler := NewPausableTaskHandler(agent, logger)

	// Step 6: Create and start server
	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithTaskHandler(pausableTaskHandler).
		WithAgentCard(types.AgentCard{
			Name:        cfg.AgentName,
			Description: cfg.AgentDescription,
			URL:         cfg.AgentURL,
			Version:     cfg.AgentVersion,
			Capabilities: types.AgentCapabilities{
				Streaming:              &cfg.CapabilitiesConfig.Streaming,
				PushNotifications:      &cfg.CapabilitiesConfig.PushNotifications,
				StateTransitionHistory: &cfg.CapabilitiesConfig.StateTransitionHistory,
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
		}).
		Build()

	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	// The agent is configured with the input_required tool
	logger.Info("Agent configured with input_required tool for pausing tasks")

	logger.Info("✅ AI-powered A2A server created with input-required pausing",
		zap.String("provider", cfg.AgentConfig.Provider),
		zap.String("model", cfg.AgentConfig.Model),
		zap.String("tools", "input_required, weather, time"))

	// Display agent metadata
	logger.Info("🤖 agent metadata",
		zap.String("name", cfg.AgentName),
		zap.String("description", cfg.AgentDescription),
		zap.String("version", cfg.AgentVersion))

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("🌐 server running", zap.String("port", cfg.ServerConfig.Port))
	fmt.Printf("\n🎯 Test with the pausedtask client:\n")
	fmt.Printf("  cd ../../../client/cmd/pausedtask\n")
	fmt.Printf("  go run main.go\n")
	fmt.Printf("\n📋 Example requests that will trigger input-required pausing:\n")
	fmt.Printf("  - \"Create a presentation outline about climate change\"\n")
	fmt.Printf("  - \"Help me plan a workshop\"\n")
	fmt.Printf("  - \"What's the weather like?\"\n")
	fmt.Printf("\n🧠 The LLM will analyze requests and use the input_required tool when it needs more context.\n")
	fmt.Println()

	// Wait for shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	} else {
		logger.Info("✅ goodbye!")
	}
}

// PausableTaskHandler is a TaskHandler that can pause tasks when tools request user input
type PausableTaskHandler struct {
	agent  server.OpenAICompatibleAgent
	logger *zap.Logger
}

func NewPausableTaskHandler(agent server.OpenAICompatibleAgent, logger *zap.Logger) *PausableTaskHandler {
	return &PausableTaskHandler{
		agent:  agent,
		logger: logger,
	}
}

func (p *PausableTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message, agent server.OpenAICompatibleAgent) (*types.Task, error) {
	activeAgent := p.agent
	if activeAgent == nil {
		activeAgent = agent
	}

	if activeAgent == nil {
		return p.handleWithoutAgent(ctx, task, message)
	}

	messages := make([]types.Message, len(task.History))
	copy(messages, task.History)

	agentResponse, err := activeAgent.Run(ctx, messages)
	if err != nil {
		return task, err
	}

	if agentResponse.Response != nil {
		lastMessage := agentResponse.Response
		if lastMessage.Kind == "input_required" {
			inputMessage := "Please provide more information to continue."
			if len(lastMessage.Parts) > 0 {
				if textPart, ok := lastMessage.Parts[0].(map[string]interface{}); ok {
					if text, exists := textPart["text"].(string); exists && text != "" {
						inputMessage = text
					}
				}
			}
			task.History = append(task.History, *agentResponse.Response)
			return p.pauseTaskForInput(task, inputMessage), nil
		}
	}

	if len(agentResponse.AdditionalMessages) > 0 {
		task.History = append(task.History, agentResponse.AdditionalMessages...)
	}
	if agentResponse.Response != nil {
		task.History = append(task.History, *agentResponse.Response)
	}

	task.Status.State = types.TaskStateCompleted
	task.Status.Message = agentResponse.Response

	return task, nil
}

// handleWithoutAgent processes a task without agent capabilities
func (p *PausableTaskHandler) handleWithoutAgent(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	response := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "I received your message. I'm a pausable task handler but no AI agent is configured. To enable AI responses with pausable capabilities, configure an OpenAI-compatible agent.",
			},
		},
	}

	if task.History == nil {
		task.History = []types.Message{}
	}
	task.History = append(task.History, *response)
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = response

	return task, nil
}

// pauseTaskForInput updates a task to input-required state with the given message
func (p *PausableTaskHandler) pauseTaskForInput(task *types.Task, inputMessage string) *types.Task {
	p.logger.Info("Pausing task for user input",
		zap.String("task_id", task.ID),
		zap.String("input_message", inputMessage))

	// Create the input request message for the user
	message := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("input-request-%d", time.Now().Unix()),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": inputMessage,
			},
		},
	}

	// Add the assistant's input request message to conversation history
	if task.History == nil {
		task.History = []types.Message{}
	}
	task.History = append(task.History, *message)

	// Update task state to input-required
	task.Status.State = types.TaskStateInputRequired
	task.Status.Message = message

	p.logger.Info("Task paused for user input",
		zap.String("task_id", task.ID),
		zap.String("state", string(task.Status.State)),
		zap.Int("conversation_history_count", len(task.History)))

	return task
}
