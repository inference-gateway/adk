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
//	  WithFilesystemStorage("./artifacts", "http://localhost:8081").
//	  Build()
type ArtifactsServerBuilder interface {
	// WithFilesystemStorage configures filesystem-based artifact storage
	WithFilesystemStorage(basePath, baseURL string) ArtifactsServerBuilder

	// WithMinIOStorage configures MinIO-based artifact storage
	WithMinIOStorage(endpoint, accessKey, secretKey, bucketName, baseURL string, useSSL bool) ArtifactsServerBuilder

	// WithCustomStorage sets a custom storage provider
	WithCustomStorage(storage ArtifactStorageProvider) ArtifactsServerBuilder

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
	config  *config.ArtifactsConfig
	logger  *zap.Logger
	storage ArtifactStorageProvider
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
//	  WithFilesystemStorage("./artifacts", "http://localhost:8081").
//	  Build()
func NewArtifactsServerBuilder(cfg *config.ArtifactsConfig, logger *zap.Logger) ArtifactsServerBuilder {
	return &ArtifactsServerBuilderImpl{
		config: cfg,
		logger: logger,
	}
}

// WithFilesystemStorage configures filesystem-based artifact storage
func (b *ArtifactsServerBuilderImpl) WithFilesystemStorage(basePath, baseURL string) ArtifactsServerBuilder {
	storage, err := NewFilesystemArtifactStorage(basePath, baseURL)
	if err != nil {
		b.logger.Error("failed to create filesystem storage", zap.Error(err))
		return b
	}
	b.storage = storage
	return b
}

// WithMinIOStorage configures MinIO-based artifact storage
func (b *ArtifactsServerBuilderImpl) WithMinIOStorage(endpoint, accessKey, secretKey, bucketName, baseURL string, useSSL bool) ArtifactsServerBuilder {
	storage, err := NewMinIOArtifactStorage(endpoint, accessKey, secretKey, bucketName, baseURL, useSSL)
	if err != nil {
		b.logger.Error("failed to create MinIO storage", zap.Error(err))
		return b
	}
	b.storage = storage
	return b
}

// WithCustomStorage sets a custom storage provider
func (b *ArtifactsServerBuilderImpl) WithCustomStorage(storage ArtifactStorageProvider) ArtifactsServerBuilder {
	b.storage = storage
	return b
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

	if b.storage == nil {
		// Try to auto-configure storage based on config
		if err := b.autoConfigureStorage(); err != nil {
			return nil, fmt.Errorf("no storage provider configured and failed to auto-configure: %w", err)
		}
	}

	server := NewArtifactsServer(b.config, b.logger)
	server.SetStorage(b.storage)

	return server, nil
}

// autoConfigureStorage attempts to configure storage based on the configuration
func (b *ArtifactsServerBuilderImpl) autoConfigureStorage() error {
	storageConfig := b.config.StorageConfig

	switch storageConfig.Provider {
	case "filesystem":
		baseURL := fmt.Sprintf("http://%s:%s",
			b.config.ServerConfig.Host,
			b.config.ServerConfig.Port)
		if b.config.ServerConfig.Host == "" {
			baseURL = fmt.Sprintf("http://localhost:%s", b.config.ServerConfig.Port)
		}

		storage, err := NewFilesystemArtifactStorage(storageConfig.BasePath, baseURL)
		if err != nil {
			return fmt.Errorf("failed to create filesystem storage: %w", err)
		}
		b.storage = storage
		b.logger.Info("configured filesystem storage",
			zap.String("base_path", storageConfig.BasePath),
			zap.String("base_url", baseURL))

	case "minio":
		if storageConfig.Endpoint == "" || storageConfig.AccessKey == "" || storageConfig.SecretKey == "" {
			return fmt.Errorf("MinIO storage requires endpoint, access key, and secret key")
		}

		baseURL := fmt.Sprintf("http://%s:%s",
			b.config.ServerConfig.Host,
			b.config.ServerConfig.Port)
		if b.config.ServerConfig.Host == "" {
			baseURL = fmt.Sprintf("http://localhost:%s", b.config.ServerConfig.Port)
		}

		storage, err := NewMinIOArtifactStorage(
			storageConfig.Endpoint,
			storageConfig.AccessKey,
			storageConfig.SecretKey,
			storageConfig.BucketName,
			baseURL,
			storageConfig.UseSSL,
		)
		if err != nil {
			return fmt.Errorf("failed to create MinIO storage: %w", err)
		}
		b.storage = storage
		b.logger.Info("configured MinIO storage",
			zap.String("endpoint", storageConfig.Endpoint),
			zap.String("bucket", storageConfig.BucketName),
			zap.Bool("ssl", storageConfig.UseSSL))

	default:
		return fmt.Errorf("unsupported storage provider: %s", storageConfig.Provider)
	}

	return nil
}

// SimpleArtifactsServerWithFilesystem creates a basic artifacts server with filesystem storage
// This is a convenience function for filesystem-based use cases
func SimpleArtifactsServerWithFilesystem(cfg *config.ArtifactsConfig, logger *zap.Logger, basePath, baseURL string) (ArtifactsServer, error) {
	return NewArtifactsServerBuilder(cfg, logger).
		WithFilesystemStorage(basePath, baseURL).
		Build()
}

// SimpleArtifactsServerWithMinIO creates a basic artifacts server with MinIO storage
// This is a convenience function for MinIO-based use cases
func SimpleArtifactsServerWithMinIO(cfg *config.ArtifactsConfig, logger *zap.Logger, endpoint, accessKey, secretKey, bucketName, baseURL string, useSSL bool) (ArtifactsServer, error) {
	return NewArtifactsServerBuilder(cfg, logger).
		WithMinIOStorage(endpoint, accessKey, secretKey, bucketName, baseURL, useSSL).
		Build()
}
