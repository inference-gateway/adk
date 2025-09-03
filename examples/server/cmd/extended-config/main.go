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
	zap "go.uber.org/zap"
)

// CustomAppConfig extends the base A2A config with application-specific settings
type CustomAppConfig struct {
	config.Config                                    // Embed the base A2A configuration
	DatabaseURL       string                         `env:"DATABASE_URL" description:"PostgreSQL database connection URL"`
	RedisURL          string                         `env:"REDIS_URL" description:"Redis cache connection URL"`
	AppName           string                         `env:"APP_NAME,default=ExtendedConfigExample" description:"Application name"`
	MaxConnections    int                            `env:"MAX_CONNECTIONS,default=100" description:"Maximum database connections"`
	EnableRateLimiter bool                           `env:"ENABLE_RATE_LIMITER,default=true" description:"Enable API rate limiting"`
	CustomHeaders     map[string]string              `env:"CUSTOM_HEADERS" description:"Custom HTTP headers to add to responses"`
	FeatureFlags      CustomFeatureFlags             `env:",prefix=FEATURE_" description:"Feature flag configuration"`
}

// CustomFeatureFlags contains feature-specific configuration
type CustomFeatureFlags struct {
	EnableNewUI        bool   `env:"ENABLE_NEW_UI,default=false" description:"Enable the new UI experience"`
	EnableAdvancedAuth bool   `env:"ENABLE_ADVANCED_AUTH,default=false" description:"Enable advanced authentication features"`
	MaxFileSize        int64  `env:"MAX_FILE_SIZE,default=10485760" description:"Maximum file upload size in bytes (default: 10MB)"`
	TempDirectory      string `env:"TEMP_DIRECTORY,default=/tmp/app" description:"Temporary directory for file processing"`
}

// Validate implements the ExtendableConfig interface for custom validation
func (c *CustomAppConfig) Validate() error {
	if c.DatabaseURL != "" {
		// Basic URL validation
		if len(c.DatabaseURL) < 10 {
			return fmt.Errorf("DATABASE_URL appears to be too short: %s", c.DatabaseURL)
		}
	}

	if c.MaxConnections < 1 {
		return fmt.Errorf("MAX_CONNECTIONS must be at least 1, got: %d", c.MaxConnections)
	}

	if c.FeatureFlags.MaxFileSize < 0 {
		return fmt.Errorf("FEATURE_MAX_FILE_SIZE cannot be negative: %d", c.FeatureFlags.MaxFileSize)
	}

	return nil
}

// GetBaseConfig implements the ExtendableConfig interface
func (c *CustomAppConfig) GetBaseConfig() *config.Config {
	return &c.Config
}

// CustomTaskHandler demonstrates a task handler that uses the extended configuration
type CustomTaskHandler struct {
	logger    *zap.Logger
	appConfig *CustomAppConfig
	agent     server.OpenAICompatibleAgent
}

// NewCustomTaskHandler creates a new task handler with extended config
func NewCustomTaskHandler(logger *zap.Logger, appConfig *CustomAppConfig) *CustomTaskHandler {
	return &CustomTaskHandler{
		logger:    logger,
		appConfig: appConfig,
	}
}

// HandleTask processes tasks with access to custom configuration
func (h *CustomTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	userInput := ""
	if message != nil {
		for _, part := range message.Parts {
			if partMap, ok := part.(map[string]any); ok {
				if text, ok := partMap["text"].(string); ok {
					userInput = text
					break
				}
			}
		}
	}

	// Example of using custom configuration in task processing
	responseText := fmt.Sprintf("Hello from %s! You said: %s", h.appConfig.AppName, userInput)
	
	if h.appConfig.FeatureFlags.EnableNewUI {
		responseText += "\n[New UI Enabled] Enhanced response formatting available"
	}
	
	if h.appConfig.DatabaseURL != "" {
		responseText += "\n[Database] Connected to database for persistent operations"
	}
	
	responseText += fmt.Sprintf("\n[Config] Max connections: %d, Rate limiter: %t",
		h.appConfig.MaxConnections, h.appConfig.EnableRateLimiter)

	// Log with custom configuration context
	h.logger.Info("Processing task",
		zap.String("app_name", h.appConfig.AppName),
		zap.String("task_id", task.ID),
		zap.Bool("new_ui_enabled", h.appConfig.FeatureFlags.EnableNewUI),
		zap.Int("max_connections", h.appConfig.MaxConnections),
	)

	task.History = append(task.History, types.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("response-%s", task.ID),
		Role:      "assistant",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": responseText,
			},
		},
	})

	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &task.History[len(task.History)-1]

	return task, nil
}

// SetAgent sets the OpenAI-compatible agent
func (h *CustomTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) {
	h.agent = agent
}

// GetAgent returns the configured OpenAI-compatible agent
func (h *CustomTaskHandler) GetAgent() server.OpenAICompatibleAgent {
	return h.agent
}

// Extended Configuration Example
//
// This example demonstrates how to extend the A2A server configuration
// with custom application-specific settings while maintaining compatibility
// with the base A2A configuration system.
//
// To run with custom configuration:
// 
//   export DATABASE_URL="postgresql://localhost/myapp"
//   export REDIS_URL="redis://localhost:6379"
//   export APP_NAME="MyCustomApp"
//   export MAX_CONNECTIONS="50"
//   export FEATURE_ENABLE_NEW_UI="true"
//   export DEBUG="true"
//   go run main.go
func main() {
	fmt.Println("🤖 Starting A2A Server with Extended Configuration...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create extended configuration instance
	appConfig := &CustomAppConfig{}

	// Load configuration using the new extended configuration API
	ctx := context.Background()
	if err := config.LoadExtended(ctx, appConfig); err != nil {
		logger.Fatal("failed to load extended configuration", zap.Error(err))
	}

	// Log loaded configuration for demonstration
	logger.Info("Configuration loaded successfully",
		zap.String("agent_name", appConfig.AgentName),
		zap.String("app_name", appConfig.AppName),
		zap.String("database_url", maskSensitiveURL(appConfig.DatabaseURL)),
		zap.String("redis_url", maskSensitiveURL(appConfig.RedisURL)),
		zap.Int("max_connections", appConfig.MaxConnections),
		zap.Bool("enable_rate_limiter", appConfig.EnableRateLimiter),
		zap.Bool("feature_new_ui", appConfig.FeatureFlags.EnableNewUI),
		zap.Bool("feature_advanced_auth", appConfig.FeatureFlags.EnableAdvancedAuth),
		zap.String("server_port", appConfig.ServerConfig.Port),
		zap.Bool("debug", appConfig.Debug),
	)

	// Create custom task handler with access to extended configuration
	taskHandler := NewCustomTaskHandler(logger, appConfig)

	// Build A2A server using the base config from extended configuration
	// The server builder automatically extracts the base config
	baseConfig := appConfig.Config
	a2aServer, err := server.NewA2AServerBuilder(baseConfig, logger).
		WithBackgroundTaskHandler(taskHandler).
		WithAgentCard(types.AgentCard{
			Name:            getValueOrDefault(appConfig.AgentName, appConfig.AppName),
			Description:     fmt.Sprintf("%s - A2A server with extended configuration", appConfig.AppName),
			Version:         getValueOrDefault(appConfig.AgentVersion, "1.0.0"),
			URL:             fmt.Sprintf("http://localhost:%s", appConfig.ServerConfig.Port),
			ProtocolVersion: "1.0.0",
			Capabilities: types.AgentCapabilities{
				Streaming:              &[]bool{appConfig.CapabilitiesConfig.Streaming}[0],
				PushNotifications:      &[]bool{appConfig.CapabilitiesConfig.PushNotifications}[0],
				StateTransitionHistory: &[]bool{appConfig.CapabilitiesConfig.StateTransitionHistory}[0],
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills:             []types.AgentSkill{},
		}).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	logger.Info("✅ A2A server with extended configuration created")

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("🌐 server running on port " + appConfig.ServerConfig.Port)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	} else {
		logger.Info("✅ goodbye!")
	}
}

// maskSensitiveURL masks sensitive information in URLs for logging
func maskSensitiveURL(url string) string {
	if url == "" {
		return ""
	}
	if len(url) > 20 {
		return url[:10] + "***" + url[len(url)-7:]
	}
	return "***"
}

// getValueOrDefault returns the first non-empty string
func getValueOrDefault(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}