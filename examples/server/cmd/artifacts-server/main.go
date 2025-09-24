package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/inference-gateway/adk/server"
	"github.com/inference-gateway/adk/server/config"
	"go.uber.org/zap"
)

func main() {
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create artifacts server configuration
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port:         "8081",
			Host:         "",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		StorageConfig: config.ArtifactsStorageConfig{
			Provider: "filesystem",
			BasePath: "./artifacts",
		},
	}

	// Create the artifacts server using the builder
	artifactsServer, err := server.NewArtifactsServerBuilder(cfg, logger).
		WithFilesystemStorage("./artifacts", "http://localhost:8081").
		Build()
	if err != nil {
		logger.Fatal("Failed to build artifacts server", zap.Error(err))
	}

	// Start the artifacts server
	ctx := context.Background()
	logger.Info("Starting artifacts server",
		zap.String("port", cfg.ServerConfig.Port),
		zap.String("storage", cfg.StorageConfig.Provider),
		zap.String("base_path", cfg.StorageConfig.BasePath))

	fmt.Printf(`
Artifacts Server Started!

The server is running on http://localhost:%s

This server provides:
- Static artifact download endpoints
- RESTful API for artifact access
- Pluggable storage backend support

Available endpoints:
- GET /health - Health check
- GET /artifacts/{artifactId}/{filename} - Download artifact

Storage Configuration:
- Provider: %s
- Base Path: %s

Example usage:
1. Store artifacts using the A2A protocol in your main server
2. Access artifacts via: http://localhost:%s/artifacts/ARTIFACT_ID/FILENAME

Press Ctrl+C to stop the server.
`, cfg.ServerConfig.Port, cfg.StorageConfig.Provider, cfg.StorageConfig.BasePath, cfg.ServerConfig.Port)

	if err := artifactsServer.Start(ctx); err != nil {
		logger.Fatal("Failed to start artifacts server", zap.Error(err))
	}
}
