package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
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

			// Return a special error that our task handler will catch
			return "", &InputRequiredError{Message: message}
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

	// Wrap the toolbox to intercept INPUT_REQUIRED responses - actually not needed anymore
	// since we're throwing the error directly from the tool function
	// pausableToolBox := NewPausableToolBox(toolBox, logger)

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

CRITICAL: You have access to function calling. When you need more information, you MUST call the input_required function using proper function calling, NOT just mention it in text. Actually invoke the function with the required parameters.

When you determine that you need more information to provide a quality response, immediately call the input_required function with a clear message explaining what details you need.`

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
	p.logger.Info("processing task with pausable task handler",
		zap.String("task_id", task.ID),
		zap.String("message_role", message.Role))

	// Use the injected agent (or fall back to the one passed in)
	activeAgent := p.agent
	if activeAgent == nil {
		activeAgent = agent
	}

	if activeAgent == nil {
		return p.handleWithoutAgent(ctx, task, message)
	}

	// Prepare conversation messages for the agent
	messages := make([]types.Message, 0)

	// Add the current message first
	if message != nil {
		messages = append(messages, *message)
	}

	// Add existing history
	if task.History != nil {
		messages = append(messages, task.History...)
	}

	// Get available tools from the agent's toolbox (if configured)
	var tools []sdk.ChatCompletionTool
	if activeAgent.(*server.OpenAICompatibleAgentImpl) != nil {
		// For now, we'll recreate the tools from our toolbox since we can't access them directly
		// This should match the tools that were configured when building the agent
		tempToolBox := server.NewDefaultToolBox()

		// Add the same input_required tool
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
				return "", &InputRequiredError{Message: message}
			},
		)
		tempToolBox.AddTool(inputRequiredTool)

		// Add other tools that were configured
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
		tempToolBox.AddTool(weatherTool)

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
		tempToolBox.AddTool(timeTool)

		tools = tempToolBox.GetTools()
	}

	// Create a context with timeout for the LLM call (longer timeout for Deepseek)
	llmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Process with the agent
	response, err := activeAgent.Run(llmCtx, messages, tools)
	if err != nil {
		// Check if this is an InputRequiredError (from our input_required tool)
		// It might be wrapped, so we need to unwrap it
		var inputErr *InputRequiredError
		if errors.As(err, &inputErr) {
			p.logger.Info("LLM called input_required tool, pausing task",
				zap.String("task_id", task.ID),
				zap.String("user_message", inputErr.Message))

			return p.pauseTaskForInput(task, inputErr.Message), nil
		}

		// Also check for the specific error message pattern in case it's wrapped differently
		if strings.Contains(err.Error(), "INPUT_REQUIRED:") {
			// Extract the message from the error string
			errorMsg := err.Error()
			if idx := strings.Index(errorMsg, "INPUT_REQUIRED:"); idx != -1 {
				inputMessage := strings.TrimSpace(errorMsg[idx+len("INPUT_REQUIRED:"):])
				p.logger.Info("LLM called input_required tool (wrapped error), pausing task",
					zap.String("task_id", task.ID),
					zap.String("user_message", inputMessage))

				return p.pauseTaskForInput(task, inputMessage), nil
			}
		}

		// Check if it's a timeout or connection error - provide fallback behavior
		if llmCtx.Err() == context.DeadlineExceeded || strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "connection") {
			p.logger.Warn("LLM request timed out or failed, providing simple response",
				zap.Error(err),
				zap.String("task_id", task.ID))

			// Create a simple fallback response
			response := &types.Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("fallback-response-%s", task.ID),
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "I'm sorry, I'm experiencing connection issues with the AI service. Please try your request again later.",
					},
				},
			}

			// Update task history and complete
			if task.History == nil {
				task.History = []types.Message{}
			}
			task.History = append(task.History, *response)
			task.Status.State = types.TaskStateCompleted
			task.Status.Message = response

			return task, nil
		}

		// Regular error handling
		p.logger.Error("agent processing failed", zap.Error(err))
		return task, err
	}

	// Update task history with complete conversation
	conversationHistory := activeAgent.GetConversationHistory()
	if len(conversationHistory) > 0 {
		task.History = conversationHistory
	} else if response != nil {
		// Fallback: add just the response to history
		if task.History == nil {
			task.History = []types.Message{}
		}
		task.History = append(task.History, *response)
	}

	// Task completed successfully
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = response

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

	// Update task state to input-required
	task.Status.State = types.TaskStateInputRequired
	task.Status.Message = message

	p.logger.Info("Task paused for user input",
		zap.String("task_id", task.ID),
		zap.String("state", string(task.Status.State)))

	return task
}

// InputRequiredError signals that the agent should pause for user input
type InputRequiredError struct {
	Message string
}

func (e *InputRequiredError) Error() string {
	return fmt.Sprintf("INPUT_REQUIRED: %s", e.Message)
}
