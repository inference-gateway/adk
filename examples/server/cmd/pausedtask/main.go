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
	// This tool will return an error that signals the task should be paused
	inputRequiredTool := server.NewBasicTool(
		"request_user_input",
		"Request additional input from the user when current information is insufficient to complete the task",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "The message to display to the user explaining what information is needed",
				},
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "Internal reason for why additional input is required",
				},
			},
			"required": []string{"message", "reason"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			message := args["message"].(string)
			reason := args["reason"].(string)

			logger.Info("LLM requested user input - returning marker for detection",
				zap.String("message", message),
				zap.String("reason", reason))

			// Return the marker that the PausableAgent will detect
			return fmt.Sprintf("INPUT_REQUIRED:%s", message), nil
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

	// Wrap the toolbox to intercept INPUT_REQUIRED responses
	pausableToolBox := NewPausableToolBox(toolBox, logger)

	// Step 4: Create AI agent with LLM client
	llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.AgentConfig, logger)
	if err != nil {
		logger.Fatal("failed to create LLM client", zap.Error(err))
	}

	// Create agent with system prompt that encourages intelligent input pausing
	systemPrompt := `You are a helpful AI assistant that creates detailed presentation outlines and provides information.

When users request something that could benefit from more specific information, you should:
1. First analyze if you have enough context to provide a useful response
2. If the request is vague or could be significantly improved with more details, use the request_user_input tool to ask for clarification
3. Be specific about what information would help you provide a better response

For example:
- If someone asks for a "presentation outline" without specifying the audience, topic depth, or duration, ask for these details
- If someone asks for weather without a location, ask for the location
- If someone asks for advice without context, ask for relevant background

IMPORTANT: When you call the request_user_input tool and it returns a result starting with "INPUT_REQUIRED:", 
you must IMMEDIATELY stop processing and wait for user input. Do not continue with your response or provide any additional content.
The tool result starting with "INPUT_REQUIRED:" indicates that the user needs to provide more information before you can continue.

Always be helpful and provide what you can, but don't hesitate to ask for more information when it would significantly improve your response.`

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(&cfg.AgentConfig).
		WithLLMClient(llmClient).
		WithSystemPrompt(systemPrompt).
		WithMaxChatCompletion(10).
		WithToolBox(pausableToolBox).
		Build()
	if err != nil {
		logger.Fatal("failed to create AI agent", zap.Error(err))
	}

	// Step 5: Create a custom task handler that can pause tasks for input
	pausableTaskHandler := NewPausableTaskHandler(logger)

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

	// The pausable agent will work with the server's built-in task management
	// No additional setup needed - the agent processes the INPUT_REQUIRED responses
	logger.Info("Pausable agent configured to detect INPUT_REQUIRED responses")

	logger.Info("✅ AI-powered A2A server created with input-required pausing",
		zap.String("provider", cfg.AgentConfig.Provider),
		zap.String("model", cfg.AgentConfig.Model),
		zap.String("tools", "request_user_input, weather, time"))

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
	fmt.Printf("\n🧠 The LLM will analyze requests and use the request_user_input tool when it needs more context.\n")
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
	logger *zap.Logger
}

func NewPausableTaskHandler(logger *zap.Logger) *PausableTaskHandler {
	return &PausableTaskHandler{
		logger: logger,
	}
}

func (p *PausableTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message, agent server.OpenAICompatibleAgent) (*types.Task, error) {
	p.logger.Info("processing task with pausable task handler",
		zap.String("task_id", task.ID),
		zap.String("message_role", message.Role))

	// Check if task is already in input-required state (resume scenario)
	if task.Status.State == types.TaskStateInputRequired {
		p.logger.Info("task already in input-required state, continuing processing",
			zap.String("task_id", task.ID))
	}

	// If no agent is provided, fall back to simple response
	if agent == nil {
		return p.handleWithoutAgent(ctx, task, message)
	}

	// Process the task using agent capabilities
	result, err := p.processWithAgent(ctx, task, message, agent)
	if err != nil {
		return result, err
	}

	// Check if any tool result in the history contains an INPUT_REQUIRED marker
	if result != nil && len(result.History) > 0 {
		for i := len(result.History) - 1; i >= 0; i-- {
			historyMessage := result.History[i]

			// Check tool result messages for INPUT_REQUIRED marker
			if historyMessage.Role == "tool" {
				for _, part := range historyMessage.Parts {
					if partMap, ok := part.(map[string]interface{}); ok {
						if dataContent, exists := partMap["data"]; exists {
							if dataMap, ok := dataContent.(map[string]interface{}); ok {
								if toolResult, exists := dataMap["result"]; exists {
									if resultStr, ok := toolResult.(string); ok {
										// Check for INPUT_REQUIRED marker
										if len(resultStr) > 15 && resultStr[:15] == "INPUT_REQUIRED:" {
											userMessage := resultStr[15:] // Extract the message after the marker

											p.logger.Info("Tool requested input pause, pausing task",
												zap.String("task_id", task.ID),
												zap.String("user_message", userMessage))

											// Create the input request message
											inputMessage := &types.Message{
												Kind:      "message",
												MessageID: fmt.Sprintf("input-request-%d", time.Now().Unix()),
												Role:      "assistant",
												Parts: []types.Part{
													map[string]interface{}{
														"kind": "text",
														"text": userMessage,
													},
												},
											}

											// Update the task state to input-required
											result.Status.State = types.TaskStateInputRequired
											result.Status.Message = inputMessage

											// Clean up the history to remove any messages after the tool call that requested input
											// Find where this tool result is and truncate history after that
											historyIndex := -1
											for j, msg := range result.History {
												if msg.MessageID == historyMessage.MessageID {
													historyIndex = j
													break
												}
											}
											if historyIndex >= 0 {
												// Keep history up to and including the tool result, but remove any assistant responses after
												result.History = result.History[:historyIndex+1]
											}

											p.logger.Info("Task paused for user input",
												zap.String("task_id", task.ID),
												zap.String("state", string(result.Status.State)))

											return result, nil
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return result, nil
}

// processWithAgent processes a task using the provided agent's capabilities
func (p *PausableTaskHandler) processWithAgent(ctx context.Context, task *types.Task, message *types.Message, agent server.OpenAICompatibleAgent) (*types.Task, error) {
	// TODO: Implement the agent-based processing logic here
	// This should use the agent's LLM client, toolbox, and system prompt to process the task
	// For now, return a simple response indicating the functionality is not yet implemented

	response := &types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "I received your message and have access to an AI agent with pausable capabilities, but the full agent processing logic is not yet implemented in this task handler.",
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

// PausableToolBox wraps a toolbox to intercept INPUT_REQUIRED responses
type PausableToolBox struct {
	underlying server.ToolBox
	logger     *zap.Logger
}

// InputRequiredError signals that the agent should pause for user input
type InputRequiredError struct {
	Message string
}

func (e *InputRequiredError) Error() string {
	return fmt.Sprintf("INPUT_REQUIRED: %s", e.Message)
}

// NewPausableToolBox creates a new pausable toolbox wrapper
func NewPausableToolBox(underlying server.ToolBox, logger *zap.Logger) *PausableToolBox {
	return &PausableToolBox{
		underlying: underlying,
		logger:     logger,
	}
}

func (p *PausableToolBox) GetTools() []sdk.ChatCompletionTool {
	return p.underlying.GetTools()
}

func (p *PausableToolBox) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	result, err := p.underlying.ExecuteTool(ctx, name, args)
	if err != nil {
		return result, err
	}

	// Check if the tool result contains INPUT_REQUIRED marker
	if len(result) > 15 && result[:15] == "INPUT_REQUIRED:" {
		userMessage := result[15:] // Extract the message after the marker
		p.logger.Info("Tool returned INPUT_REQUIRED marker, throwing pause error",
			zap.String("tool", name),
			zap.String("user_message", userMessage))
		return "", &InputRequiredError{Message: userMessage}
	}

	return result, nil
}

func (p *PausableToolBox) GetToolNames() []string {
	return p.underlying.GetToolNames()
}

func (p *PausableToolBox) HasTool(toolName string) bool {
	return p.underlying.HasTool(toolName)
}
