package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inference-gateway/a2a/adk/server"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("🚀 Running Standard A2A Server Example")

	// Create a basic logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Load environment variables using envconfig
	var envConfig server.Config
	if err := envconfig.Process(context.Background(), &envConfig); err != nil {
		log.Fatalf("failed to load environment variables: %v", err)
	}

	// Configure the server with environment variables
	cfg := server.Config{
		AgentName:        envConfig.AgentName,
		AgentDescription: envConfig.AgentDescription,
		AgentURL:         envConfig.AgentURL,
		AgentVersion:     envConfig.AgentVersion,
		Port:             envConfig.Port,
		Debug:            envConfig.Debug,
		CapabilitiesConfig: &server.CapabilitiesConfig{
			Streaming:              envConfig.CapabilitiesConfig.Streaming,
			PushNotifications:      envConfig.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: envConfig.CapabilitiesConfig.StateTransitionHistory,
		},
		TLSConfig: &server.TLSConfig{
			Enable: envConfig.TLSConfig.Enable,
		},
		AuthConfig: &server.AuthConfig{
			Enable: envConfig.AuthConfig.Enable,
		},
		QueueConfig: &server.QueueConfig{
			MaxSize:         envConfig.QueueConfig.MaxSize,
			CleanupInterval: envConfig.QueueConfig.CleanupInterval,
		},
		ServerConfig: &server.ServerConfig{
			ReadTimeout:  envConfig.ServerConfig.ReadTimeout,
			WriteTimeout: envConfig.ServerConfig.WriteTimeout,
			IdleTimeout:  envConfig.ServerConfig.IdleTimeout,
		},
	}

	// Create the A2A server with default handlers
	a2aServer := server.NewDefaultA2AServer(cfg, logger)

	// Start the server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := a2aServer.Stop(shutdownCtx); err != nil {
			logger.Error("error during shutdown", zap.Error(err))
		}
		cancel()
	}()

	logger.Info("starting standard A2A server",
		zap.String("port", cfg.Port),
		zap.String("agent_name", cfg.AgentName))

	fmt.Printf("🌐 Server starting on http://localhost:%s\n", cfg.Port)
	fmt.Println("📋 Available endpoints:")
	fmt.Println("  • GET  /health - Health check")
	fmt.Println("  • GET  /.well-known/agent.json - Agent capabilities")
	fmt.Println("  • POST /a2a - A2A protocol endpoint")
	fmt.Println("👋 Press Ctrl+C to stop the server")

	if err := a2aServer.Start(ctx); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}
