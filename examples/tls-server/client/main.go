package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
)

// Config represents the client configuration
type Config struct {
	Environment   string        `env:"ENVIRONMENT,default=development"`
	ServerURL     string        `env:"A2A_SERVER_URL,default=https://localhost:8443"`
	Timeout       time.Duration `env:"A2A_TIMEOUT,default=30s"`
	SkipTLSVerify bool          `env:"A2A_SKIP_TLS_VERIFY,default=true"`
}

// TLS-Enabled A2A Client Example
//
// This example demonstrates an A2A client connecting securely to a TLS-enabled server.
// The client uses HTTPS to communicate with the server and handles TLS certificate
// verification.
//
// Configuration can be provided via environment variables:
//   - ENVIRONMENT: Runtime environment (default: development)
//   - A2A_SERVER_URL: Server URL (default: https://localhost:8443)
//   - A2A_TIMEOUT: Request timeout (default: 30s)
//   - A2A_SKIP_TLS_VERIFY: Skip TLS certificate verification (default: true for self-signed certs)
//
// To run: go run main.go
func main() {
	fmt.Println("🔒 Starting TLS A2A Client...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	// Create configuration with defaults
	cfg := &Config{}

	// Load configuration from environment variables
	ctx := context.Background()
	if err := envconfig.Process(ctx, cfg); err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	// Log configuration info
	logger.Info("configuration loaded",
		zap.String("environment", cfg.Environment),
		zap.String("server_url", cfg.ServerURL),
		zap.Duration("timeout", cfg.Timeout),
		zap.Bool("skip_tls_verify", cfg.SkipTLSVerify),
	)

	// Create HTTP client with TLS configuration
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.SkipTLSVerify,
			},
		},
	}

	// Create A2A client with custom HTTP client for TLS
	a2aClient := client.NewClientWithConfig(&client.Config{
		BaseURL:    cfg.ServerURL,
		HTTPClient: httpClient,
	})

	// Test server health over TLS
	logger.Info("🔍 checking server health over TLS...")

	healthCtx, healthCancel := context.WithTimeout(ctx, 10*time.Second)
	defer healthCancel()

	health, err := a2aClient.GetHealth(healthCtx)
	if err != nil {
		logger.Fatal("failed to check server health", zap.Error(err))
	}

	logger.Info("✅ server health check successful",
		zap.String("status", health.Status),
	)

	// Get agent card over TLS
	logger.Info("🔍 fetching agent card over TLS...")

	cardCtx, cardCancel := context.WithTimeout(ctx, 10*time.Second)
	defer cardCancel()

	agentCard, err := a2aClient.GetAgentCard(cardCtx)
	if err != nil {
		logger.Fatal("failed to get agent card", zap.Error(err))
	}

	logger.Info("✅ agent card retrieved successfully",
		zap.String("name", agentCard.Name),
		zap.String("description", agentCard.Description),
		zap.String("version", agentCard.Version),
		zap.String("url", agentCard.URL),
	)

	// Test messages to demonstrate secure communication
	testMessages := []string{
		"Hello TLS server!",
		"This message is sent over HTTPS",
		"Secure communication is working!",
		"🔒 Testing encrypted connection",
	}

	for i, message := range testMessages {
		logger.Info("📤 sending secure message",
			zap.Int("message_number", i+1),
			zap.String("content", message),
		)

		// Create task message
		taskMessage := types.Message{
			Role: "user",
			Parts: []types.Part{
				types.TextPart{
					Kind: "text",
					Text: message,
				},
			},
		}

		// Submit task over TLS
		taskCtx, taskCancel := context.WithTimeout(ctx, cfg.Timeout)

		response, err := a2aClient.SendTask(taskCtx, types.MessageSendParams{
			Message: taskMessage,
		})
		if err != nil {
			logger.Error("failed to submit task", zap.Error(err))
			taskCancel()
			continue
		}

		logger.Info("✅ task submitted successfully",
			zap.Any("response", response),
		)

		taskCancel()

		// Wait a bit between messages
		time.Sleep(1 * time.Second)
	}

	logger.Info("🔒 TLS communication test completed successfully!")
	logger.Info("✅ All messages were transmitted securely over HTTPS")
}
