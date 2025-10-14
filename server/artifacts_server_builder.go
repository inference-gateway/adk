package server

import (
	"fmt"

	"github.com/inference-gateway/adk/server/config"
	"go.uber.org/zap"
)

// ArtifactsServerBuilder provides a fluent interface for building artifacts servers with custom configurations.
// This interface allows for flexible server construction with optional components and settings.
// Use NewArtifactsServerBuilder to create an instance, then chain method calls to configure the server.
//
// Example:
//
//	artifactsServer := NewArtifactsServerBuilder(cfg, logger).
//	  Build()
type ArtifactsServerBuilder interface {
	// WithLogger sets a custom logger for the builder and resulting server
	WithLogger(logger *zap.Logger) ArtifactsServerBuilder

	// Build creates and returns the configured artifacts server
	Build() (ArtifactsServer, error)
}

var _ ArtifactsServerBuilder = (*ArtifactsServerBuilderImpl)(nil)

// ArtifactsServerBuilderImpl is the concrete implementation of the ArtifactsServerBuilder interface.
// It provides a fluent interface for building artifacts servers with custom configurations.
// This struct holds the configuration and optional components that will be used to create the server.
type ArtifactsServerBuilderImpl struct {
	config *config.ArtifactsConfig
	logger *zap.Logger
}

// NewArtifactsServerBuilder creates a new artifacts server builder with required dependencies.
// The configuration passed here will be used to configure the server.
//
// Parameters:
//   - cfg: The artifacts configuration for the server
//   - logger: Logger instance to use for the server
//
// Returns:
//
//	ArtifactsServerBuilder interface that can be used to further configure the server before building.
//
// Example:
//
//	cfg := &config.ArtifactsConfig{
//	  Enable: true,
//	  ServerConfig: config.ArtifactsServerConfig{
//	    Port: "8081",
//	  },
//	  StorageConfig: config.ArtifactsStorageConfig{
//	    Provider: "filesystem",
//	    BasePath: "./artifacts",
//	  },
//	}
//	logger, _ := zap.NewDevelopment()
//	server := NewArtifactsServerBuilder(cfg, logger).
//	  Build()
func NewArtifactsServerBuilder(cfg *config.ArtifactsConfig, logger *zap.Logger) ArtifactsServerBuilder {
	return &ArtifactsServerBuilderImpl{
		config: cfg,
		logger: logger,
	}
}

// WithLogger sets a custom logger for the builder
func (b *ArtifactsServerBuilderImpl) WithLogger(logger *zap.Logger) ArtifactsServerBuilder {
	b.logger = logger
	return b
}

// Build creates and returns the configured artifacts server
func (b *ArtifactsServerBuilderImpl) Build() (ArtifactsServer, error) {
	if b.config == nil {
		return nil, fmt.Errorf("artifacts configuration must be provided")
	}

	if !b.config.Enable {
		return nil, fmt.Errorf("artifacts server is not enabled in configuration")
	}

	artifactService, err := NewArtifactService(b.config, b.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact service: %w", err)
	}

	return NewArtifactsServer(b.config, b.logger, artifactService), nil
}
