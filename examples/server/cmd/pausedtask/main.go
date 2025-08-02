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

	// Step 2: Check for required inference gateway URL
	gatewayURL := os.Getenv("INFERENCE_GATEWAY_URL")
	if gatewayURL == "" {
		fmt.Println("\n❌ ERROR: Inference Gateway configuration required!")
		fmt.Println("\n🔗 Please set INFERENCE_GATEWAY_URL environment variable:")
		fmt.Println("\n📋 Example:")
		fmt.Println("  export INFERENCE_GATEWAY_URL=\"http://localhost:3000/v1\"")
		fmt.Println("\n💡 For a mock server without AI, use the mock example instead:")
		fmt.Println("  go run ../pausedtask-mock/main.go")
		os.Exit(1)
	}

	// Set the base URL for the agent configuration
	os.Setenv("AGENT_CLIENT_BASE_URL", gatewayURL)

	// Step 3: Load configuration from environment
	cfg := config.Config{
		AgentName:        server.BuildAgentName,
		AgentDescription: server.BuildAgentDescription,
		AgentVersion:     server.BuildAgentVersion,
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

	// Step 4: Create toolbox with input_required tool
	toolBox := server.NewDefaultToolBox()

	// Add input_required tool that the LLM can call to pause task execution
	// This tool simulates pausing by returning a special response that will trigger the pause
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
			
			logger.Info("LLM requested user input", 
				zap.String("message", message),
				zap.String("reason", reason))
			
			// Return a special marker that triggers input-required state
			// The agent will handle this by calling PauseTaskForInput
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

	// Step 5: Create AI agent with LLM client
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

Always be helpful and provide what you can, but don't hesitate to ask for more information when it would significantly improve your response.`

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

	// Step 6: Create a custom agent wrapper that can pause tasks
	pausableAgent := &PausableAgent{
		agent:  agent,
		logger: logger,
	}

	// Step 7: Create and start server
	a2aServer, err := server.SimpleA2AServerWithAgent(cfg, logger, pausableAgent, types.AgentCard{
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
	})

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
		zap.String("name", server.BuildAgentName),
		zap.String("description", server.BuildAgentDescription),
		zap.String("version", server.BuildAgentVersion))

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

// PausableAgent wraps an OpenAI-compatible agent and adds the ability to pause tasks
// when the agent returns an INPUT_REQUIRED response from the request_user_input tool
type PausableAgent struct {
	agent       server.OpenAICompatibleAgent
	taskManager server.TaskManager
	logger      *zap.Logger
}

func (p *PausableAgent) ProcessTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	p.logger.Info("processing task with pausable agent", 
		zap.String("task_id", task.ID),
		zap.String("message_role", message.Role))
	
	// Process the task with the underlying agent
	result, err := p.agent.ProcessTask(ctx, task, message)
	if err != nil {
		return result, err
	}
	
	// Check if the agent's response contains an INPUT_REQUIRED marker
	if result != nil && len(result.History) > 0 {
		lastMessage := result.History[len(result.History)-1]
		for _, part := range lastMessage.Parts {
			if partMap, ok := part.(map[string]interface{}); ok {
				if textContent, exists := partMap["text"]; exists {
					if textStr, ok := textContent.(string); ok {
						// Check for INPUT_REQUIRED marker
						if len(textStr) > 15 && textStr[:15] == "INPUT_REQUIRED:" {
							userMessage := textStr[15:] // Extract the message after the marker
							
							p.logger.Info("Agent requested input pause", 
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
							
							// Replace the last message in history with the user-facing message  
							result.History[len(result.History)-1] = *inputMessage
							
							p.logger.Info("Task paused for user input", 
								zap.String("task_id", task.ID))
							
							break
						}
					}
				}
			}
		}
	}
	
	return result, nil
}

func (p *PausableAgent) SetTaskManager(taskManager server.TaskManager) {
	p.taskManager = taskManager
}

func (p *PausableAgent) GetLLMClient() server.LLMClient {
	return p.agent.GetLLMClient()
}

func (p *PausableAgent) GetToolBox() server.ToolBox {
	return p.agent.GetToolBox()
}

func (p *PausableAgent) GetSystemPrompt() string {
	return p.agent.GetSystemPrompt()
}