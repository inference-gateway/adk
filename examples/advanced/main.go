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

	"github.com/inference-gateway/a2a/adk"
	"github.com/inference-gateway/a2a/adk/server"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("🔧 Running Advanced A2A Server Example")

	// Create a development logger with more detailed output
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Create advanced configuration
	cfg := server.Config{
		AgentName:                     "advanced-example-agent",
		AgentDescription:              "An advanced example A2A agent with custom handlers and enhanced capabilities",
		AgentURL:                      "http://localhost:8080",
		AgentVersion:                  "2.0.0",
		Port:                          "8080",
		Debug:                         true,
		StreamingStatusUpdateInterval: 2 * time.Second,
		// Advanced LLM provider client configuration with custom settings
		LLMProviderClientConfig: &server.LLMProviderClientConfig{
			Provider:                    "openai", // LLM clients can now choose their provider
			Model:                       "gpt-4",
			BaseURL:                     "https://api.openai.com/v1",
			APIKey:                      os.Getenv("OPENAI_API_KEY"),
			Timeout:                     45 * time.Second,
			MaxRetries:                  5,
			MaxChatCompletionIterations: 5,
			MaxTokens:                   8192,
			Temperature:                 0.3,
			TopP:                        0.9,
			CustomHeaders: map[string]string{
				"User-Agent": "advanced-a2a-agent/2.0.0",
			},
			TLSConfig: &server.ClientTLSConfig{
				InsecureSkipVerify: false,
			},
		},
		CapabilitiesConfig: &server.CapabilitiesConfig{
			Streaming:              true,
			PushNotifications:      true,
			StateTransitionHistory: true,
		},
		TLSConfig: &server.TLSConfig{
			Enable: false, // Disabled for example - enable in production with proper certs
		},
		AuthConfig: &server.AuthConfig{
			Enable: false, // Disabled for example - enable in production
		},
		QueueConfig: &server.QueueConfig{
			MaxSize:         500,
			CleanupInterval: 15 * time.Second,
		},
		ServerConfig: &server.ServerConfig{
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}

	// Create the A2A server
	a2aServer := server.NewDefaultA2AServer(cfg, logger)

	// Set custom task handler
	customTaskHandler := &CustomTaskHandler{
		logger:    logger,
		llmClient: "mock-llm-client", // In real implementation, this would be an actual LLM client
	}
	a2aServer.SetTaskHandler(customTaskHandler)

	// Set custom task result processor
	customProcessor := &CustomTaskResultProcessor{
		logger: logger,
	}
	a2aServer.SetTaskResultProcessor(customProcessor)

	// Set custom agent info provider
	customAgentProvider := &CustomAgentInfoProvider{
		logger: logger,
	}
	a2aServer.SetAgentInfoProvider(customAgentProvider)

	// Start the server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("shutting down advanced server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := a2aServer.Stop(shutdownCtx); err != nil {
			logger.Error("error during shutdown", zap.Error(err))
		}
		cancel()
	}()

	logger.Info("starting advanced A2A server",
		zap.String("port", cfg.Port),
		zap.Bool("tls_enabled", cfg.TLSConfig.Enable),
		zap.Bool("auth_enabled", cfg.AuthConfig.Enable),
		zap.String("llm_provider", func() string {
			if cfg.LLMProviderClientConfig != nil {
				return cfg.LLMProviderClientConfig.Provider
			}
			return "not configured"
		}()),
		zap.String("llm_model", func() string {
			if cfg.LLMProviderClientConfig != nil {
				return cfg.LLMProviderClientConfig.Model
			}
			return "not configured"
		}()))

	fmt.Printf("🌐 Advanced server starting on http://localhost:%s\n", cfg.Port)
	fmt.Println("🔧 Enhanced features enabled:")
	fmt.Printf("  • Custom Task Handler: %T\n", customTaskHandler)
	fmt.Printf("  • Custom Result Processor: %T\n", customProcessor)
	fmt.Printf("  • Custom Agent Provider: %T\n", customAgentProvider)
	fmt.Println("📋 Available endpoints:")
	fmt.Println("  • GET  /health - Health check")
	fmt.Println("  • GET  /.well-known/agent.json - Enhanced agent capabilities")
	fmt.Println("  • POST /a2a - A2A protocol endpoint with custom processing")
	fmt.Println("👋 Press Ctrl+C to stop the server")

	if err := a2aServer.Start(ctx); err != nil {
		logger.Fatal("failed to start advanced server", zap.Error(err))
	}
}

// CustomTaskHandler demonstrates a custom task handler implementation
type CustomTaskHandler struct {
	logger    *zap.Logger
	llmClient interface{} // In real implementation, this would be your LLM client
}

func (h *CustomTaskHandler) HandleTask(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error) {
	h.logger.Info("processing task with custom handler",
		zap.String("task_id", task.ID),
		zap.String("message_role", message.Role))

	// Extract message content for processing
	var messageContent string
	for _, part := range message.Parts {
		if partMap, ok := part.(map[string]interface{}); ok {
			if text, exists := partMap["text"]; exists {
				if textStr, ok := text.(string); ok {
					messageContent = textStr
					break
				}
			}
		}
	}

	// Simulate custom processing logic with different responses based on input
	time.Sleep(100 * time.Millisecond) // Simulate processing time

	var responseText string
	switch {
	case messageContent == "":
		responseText = "🤔 I received an empty message. How can I help you today?"
	case strings.Contains(messageContent, "help"):
		responseText = "🆘 I'm here to help! You can ask me questions about A2A protocol, send tasks, or request information."
	case strings.Contains(messageContent, "hello") || strings.Contains(messageContent, "hi"):
		responseText = "👋 Hello! Welcome to the advanced A2A agent. I'm ready to assist you with enhanced capabilities."
	case len(messageContent) > 100:
		responseText = "📝 I received a detailed message. Let me process that comprehensively for you..."
	default:
		responseText = fmt.Sprintf("✅ I processed your message: '%s'. This response was generated by the custom task handler with enhanced logic.", messageContent)
	}

	// Update task status
	task.Status.State = adk.TaskStateCompleted

	// Create response message with enhanced formatting
	responseMessage := &adk.Message{
		Kind:      "message",
		MessageID: "custom-response-" + task.ID,
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": responseText,
				"metadata": map[string]interface{}{
					"processed_by":    "CustomTaskHandler",
					"processing_time": "100ms",
					"enhanced":        true,
				},
			},
		},
	}

	// Add messages to task history
	if task.History == nil {
		task.History = []adk.Message{}
	}
	task.History = append(task.History, *message)
	task.History = append(task.History, *responseMessage)

	h.logger.Info("task processed by custom handler",
		zap.String("task_id", task.ID),
		zap.String("response_preview", truncate(responseText, 50)))

	return task, nil
}

// CustomTaskResultProcessor demonstrates custom task result processing
type CustomTaskResultProcessor struct {
	logger *zap.Logger
}

func (p *CustomTaskResultProcessor) ProcessToolResult(toolCallResult string) *adk.Message {
	p.logger.Info("processing tool result with custom logic", zap.String("result", toolCallResult))

	// Custom logic to determine if task should be completed
	if len(toolCallResult) > 0 {
		return &adk.Message{
			Kind:      "message",
			MessageID: "tool-completion-" + fmt.Sprintf("%d", time.Now().Unix()),
			Role:      "assistant",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": fmt.Sprintf("🔧 Tool execution completed successfully: %s", toolCallResult),
					"metadata": map[string]interface{}{
						"processor":       "CustomTaskResultProcessor",
						"completion_time": time.Now().Format(time.RFC3339),
					},
				},
			},
		}
	}

	return nil // Continue processing
}

// CustomAgentInfoProvider demonstrates custom agent metadata
type CustomAgentInfoProvider struct {
	logger *zap.Logger
}

func (p *CustomAgentInfoProvider) GetAgentCard(baseConfig server.Config) adk.AgentCard {
	p.logger.Info("providing enhanced custom agent card")

	return adk.AgentCard{
		Name:        "🤖 Advanced Custom A2A Agent",
		Description: "A sophisticated A2A agent with enhanced capabilities, custom task processing, and intelligent response generation",
		URL:         baseConfig.AgentURL,
		Version:     "2.0.0-custom-enhanced",
		Capabilities: adk.AgentCapabilities{
			Streaming:              &baseConfig.CapabilitiesConfig.Streaming,
			PushNotifications:      &baseConfig.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: &baseConfig.CapabilitiesConfig.StateTransitionHistory,
		},
		DefaultInputModes:  []string{"text/plain", "application/json", "text/markdown"},
		DefaultOutputModes: []string{"text/plain", "application/json", "text/markdown", "text/html"},
		Skills: []adk.AgentSkill{
			{
				Name:        "enhanced-text-processing",
				Description: "Advanced text processing with context awareness and intelligent responses",
			},
			{
				Name:        "conversational-ai",
				Description: "Natural language conversation with enhanced understanding and personalized responses",
			},
			{
				Name:        "task-automation",
				Description: "Automated task processing with custom business logic and result handling",
			},
			{
				Name:        "intelligent-routing",
				Description: "Smart message routing and processing based on content analysis",
			},
		},
	}
}

// Helper functions
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
