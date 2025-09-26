package server

import (
	"testing"

	"github.com/inference-gateway/adk/server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewArtifactsServerBuilder(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8081",
		},
		StorageConfig: config.ArtifactsStorageConfig{
			Provider: "filesystem",
			BasePath: "./test-artifacts",
		},
	}

	builder := NewArtifactsServerBuilder(cfg, logger)
	assert.NotNil(t, builder)

	builderImpl, ok := builder.(*ArtifactsServerBuilderImpl)
	require.True(t, ok)
	assert.Equal(t, cfg, builderImpl.config)
	assert.Equal(t, logger, builderImpl.logger)
}

func TestArtifactsServerBuilder_WithFilesystemStorage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8081",
		},
	}

	builder := NewArtifactsServerBuilder(cfg, logger)
	result := builder.WithFilesystemStorage("./test-artifacts", "http://localhost:8081")

	builderImpl, ok := result.(*ArtifactsServerBuilderImpl)
	require.True(t, ok)
	assert.NotNil(t, builderImpl.storage)

	// Should return the same builder instance (fluent interface)
	assert.Equal(t, builder, result)
}

func TestArtifactsServerBuilder_WithCustomStorage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8081",
		},
	}

	// Create a mock storage
	mockStorage := &mockArtifactStorageProvider{}

	builder := NewArtifactsServerBuilder(cfg, logger)
	result := builder.WithCustomStorage(mockStorage)

	builderImpl, ok := result.(*ArtifactsServerBuilderImpl)
	require.True(t, ok)
	assert.Equal(t, mockStorage, builderImpl.storage)
}

func TestArtifactsServerBuilder_WithLogger(t *testing.T) {
	originalLogger := zaptest.NewLogger(t)
	newLogger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
	}

	builder := NewArtifactsServerBuilder(cfg, originalLogger)
	result := builder.WithLogger(newLogger)

	builderImpl, ok := result.(*ArtifactsServerBuilderImpl)
	require.True(t, ok)
	assert.Equal(t, newLogger, builderImpl.logger)
}

func TestArtifactsServerBuilder_Build_Success(t *testing.T) {
	tests := []struct {
		name         string
		setupBuilder func(*ArtifactsServerBuilderImpl)
	}{
		{
			name: "with custom filesystem storage",
			setupBuilder: func(b *ArtifactsServerBuilderImpl) {
				storage, _ := NewFilesystemArtifactStorage("./test-artifacts", "http://localhost:8081")
				b.storage = storage
			},
		},
		{
			name: "with auto-configured filesystem storage",
			setupBuilder: func(b *ArtifactsServerBuilderImpl) {
				// No custom storage - should auto-configure from config
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			cfg := &config.ArtifactsConfig{
				Enable: true,
				ServerConfig: config.ArtifactsServerConfig{
					Port: "8081",
				},
				StorageConfig: config.ArtifactsStorageConfig{
					Provider: "filesystem",
					BasePath: "./test-artifacts",
				},
			}

			builderImpl := &ArtifactsServerBuilderImpl{
				config: cfg,
				logger: logger,
			}
			tt.setupBuilder(builderImpl)

			server, err := builderImpl.Build()
			assert.NoError(t, err)
			assert.NotNil(t, server)
			assert.NotNil(t, server.GetStorage())
		})
	}
}

func TestArtifactsServerBuilder_Build_Errors(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.ArtifactsConfig
		expectedErr string
	}{
		{
			name:        "nil config",
			config:      nil,
			expectedErr: "artifacts configuration must be provided",
		},
		{
			name: "disabled artifacts server",
			config: &config.ArtifactsConfig{
				Enable: false,
			},
			expectedErr: "artifacts server is not enabled in configuration",
		},
		{
			name: "unsupported storage provider",
			config: &config.ArtifactsConfig{
				Enable: true,
				StorageConfig: config.ArtifactsStorageConfig{
					Provider: "unsupported",
				},
			},
			expectedErr: "unsupported storage provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			builderImpl := &ArtifactsServerBuilderImpl{
				config: tt.config,
				logger: logger,
			}

			server, err := builderImpl.Build()
			assert.Error(t, err)
			assert.Nil(t, server)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestSimpleArtifactsServerWithFilesystem(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8081",
		},
	}

	server, err := SimpleArtifactsServerWithFilesystem(cfg, logger, "./test-artifacts", "http://localhost:8081")
	assert.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.GetStorage())
}

func TestArtifactsServerBuilder_AutoConfigureStorage(t *testing.T) {
	tests := []struct {
		name           string
		storageConfig  config.ArtifactsStorageConfig
		serverConfig   config.ArtifactsServerConfig
		expectError    bool
		expectedLogMsg string
	}{
		{
			name: "filesystem storage",
			storageConfig: config.ArtifactsStorageConfig{
				Provider: "filesystem",
				BasePath: "./test-artifacts",
			},
			serverConfig: config.ArtifactsServerConfig{
				Port: "8081",
			},
			expectError: false,
		},
		{
			name: "minio storage with missing endpoint",
			storageConfig: config.ArtifactsStorageConfig{
				Provider:   "minio",
				AccessKey:  "test",
				SecretKey:  "test",
				BucketName: "artifacts",
			},
			serverConfig: config.ArtifactsServerConfig{
				Port: "8081",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			cfg := &config.ArtifactsConfig{
				Enable:        true,
				ServerConfig:  tt.serverConfig,
				StorageConfig: tt.storageConfig,
			}

			builderImpl := &ArtifactsServerBuilderImpl{
				config: cfg,
				logger: logger,
			}

			err := builderImpl.autoConfigureStorage()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, builderImpl.storage)
			}
		})
	}
}
