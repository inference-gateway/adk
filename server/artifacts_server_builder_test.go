package server_test

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
	zaptest "go.uber.org/zap/zaptest"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	mocks "github.com/inference-gateway/adk/server/mocks"
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

func TestArtifactsServerBuilder_WithFilesystemStorage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8081",
		},
	}

	builder := server.NewArtifactsServerBuilder(cfg, logger)
	result := builder.WithFilesystemStorage("./test-artifacts", "http://localhost:8081")

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

	mockStorage := &mocks.FakeArtifactStorageProvider{}

	builder := server.NewArtifactsServerBuilder(cfg, logger)
	result := builder.WithCustomStorage(mockStorage)

	assert.Equal(t, builder, result)
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

func TestArtifactsServerBuilder_Build_WithMockStorage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8081",
		},
	}

	mockStorage := &mocks.FakeArtifactStorageProvider{}

	builder := server.NewArtifactsServerBuilder(cfg, logger)
	builder = builder.WithCustomStorage(mockStorage)

	srv, err := builder.Build()
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	assert.NotNil(t, srv.GetStorage())
}

func TestSimpleArtifactsServerWithFilesystem(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8081",
		},
	}

	srv, err := server.SimpleArtifactsServerWithFilesystem(cfg, logger, "./test-artifacts", "http://localhost:8081")
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	assert.NotNil(t, srv.GetStorage())
}
