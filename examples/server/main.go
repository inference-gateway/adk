package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inference-gateway/a2a/adk/server"
	"github.com/inference-gateway/a2a/adk/server/config"
	"github.com/inference-gateway/a2a/adk/server/otel"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("🚀 Running Standard A2A Server Example")

	// Load environment variables using envconfig
	var envConfig config.Config
	if err := envconfig.Process(context.Background(), &envConfig); err != nil {
		log.Fatalf("failed to load environment variables: %v", err)
	}

	// Create a basic logger
	var logger *zap.Logger
	var err error
	if envConfig.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}

	// Configure the server with environment variables
	cfg := config.Config{
		AgentName:        envConfig.AgentName,
		AgentDescription: envConfig.AgentDescription,
		AgentURL:         envConfig.AgentURL,
		AgentVersion:     envConfig.AgentVersion,
		Port:             envConfig.Port,
		Debug:            envConfig.Debug,
		CapabilitiesConfig: &config.CapabilitiesConfig{
			Streaming:              envConfig.CapabilitiesConfig.Streaming,
			PushNotifications:      envConfig.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: envConfig.CapabilitiesConfig.StateTransitionHistory,
		},
		TLSConfig: &config.TLSConfig{
			Enable: envConfig.TLSConfig.Enable,
		},
		AuthConfig: &config.AuthConfig{
			Enable: envConfig.AuthConfig.Enable,
		},
		QueueConfig: &config.QueueConfig{
			MaxSize:         envConfig.QueueConfig.MaxSize,
			CleanupInterval: envConfig.QueueConfig.CleanupInterval,
		},
		ServerConfig: &config.ServerConfig{
			ReadTimeout:  envConfig.ServerConfig.ReadTimeout,
			WriteTimeout: envConfig.ServerConfig.WriteTimeout,
			IdleTimeout:  envConfig.ServerConfig.IdleTimeout,
		},
		TelemetryConfig: &config.TelemetryConfig{
			Enable: envConfig.TelemetryConfig.Enable,
		},
	}

	// Initialize OpenTelemetry for metrics collection
	var telemetryInstance otel.OpenTelemetry
	if cfg.TelemetryConfig.Enable {
		var err error
		telemetryInstance, err = otel.NewOpenTelemetry(&cfg, logger)
		if err != nil {
			logger.Fatal("failed to initialize telemetry", zap.Error(err))
		}
		logger.Info("telemetry enabled - metrics will be available on :9090/metrics")
	}

	// Create the A2A server with telemetry support
	a2aServer := server.NewA2AServer(&cfg, logger, telemetryInstance)

	// Start the server
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
	}()

	fmt.Printf("🌐 Server starting on http://localhost:%s\n", cfg.Port)
	fmt.Println("📋 Available endpoints:")
	fmt.Println("  • GET  /health - Health check")
	fmt.Println("  • GET  /.well-known/agent.json - Agent capabilities")
	fmt.Println("  • POST /a2a - A2A protocol endpoint")
	if cfg.TelemetryConfig.Enable {
		fmt.Println("📊 Telemetry endpoints:")
		fmt.Println("  • GET  :9090/metrics - Prometheus metrics")
		fmt.Println("")
		fmt.Println("🔍 Telemetry features:")
		fmt.Println("  • Request count and duration tracking")
		fmt.Println("  • Response status code monitoring")
		fmt.Println("  • Provider and model metrics")
		fmt.Println("  • Task processing metrics")
	}
	fmt.Println("👋 Press Ctrl+C to stop the server")

	err = a2aServer.Start(context.Background())
	if err != nil && err != http.ErrServerClosed {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}
