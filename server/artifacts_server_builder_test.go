package server_test

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
	zaptest "go.uber.org/zap/zaptest"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
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

	builder := server.NewArtifactsServerBuilder(cfg, logger)
	assert.NotNil(t, builder)
}

func TestArtifactsServerBuilder_AutoConfigureStorage(t *testing.T) {
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

	builder := server.NewArtifactsServerBuilder(cfg, logger)
	srv, err := builder.Build()

	assert.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestArtifactsServerBuilder_WithLogger(t *testing.T) {
	originalLogger := zaptest.NewLogger(t)
	newLogger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
	}

	builder := server.NewArtifactsServerBuilder(cfg, originalLogger)
	result := builder.WithLogger(newLogger)

	assert.Equal(t, builder, result)
}

func TestSimpleArtifactsServerWithFilesystem(t *testing.T) {
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

	srv, err := server.NewArtifactsServerBuilder(cfg, logger).Build()
	assert.NoError(t, err)
	assert.NotNil(t, srv)
}
