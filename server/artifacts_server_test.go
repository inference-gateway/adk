package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/inference-gateway/adk/server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewArtifactsServer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8082",
		},
	}

	server := NewArtifactsServer(cfg, logger)
	assert.NotNil(t, server)

	impl, ok := server.(*ArtifactsServerImpl)
	require.True(t, ok)
	assert.Equal(t, cfg, impl.config)
	assert.Equal(t, logger, impl.logger)
}

func TestArtifactsServer_SetGetStorage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
	}

	server := NewArtifactsServer(cfg, logger)
	mockStorage := &mockArtifactStorageProvider{}

	// Initially no storage
	assert.Nil(t, server.GetStorage())

	// Set storage
	server.SetStorage(mockStorage)
	assert.Equal(t, mockStorage, server.GetStorage())
}

func TestArtifactsServer_StartWithoutStorage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8083",
		},
	}

	server := NewArtifactsServer(cfg, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storage provider must be set")
}

func TestArtifactsServer_HealthEndpoint(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8084",
		},
	}

	server := NewArtifactsServer(cfg, logger)
	mockStorage := &mockArtifactStorageProvider{}
	server.SetStorage(mockStorage)

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint
	resp, err := http.Get("http://localhost:8084/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Should contain JSON response
	assert.Contains(t, string(body), "status")
	assert.Contains(t, string(body), "ok")
}

func TestArtifactsServer_ArtifactDownload(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8085",
		},
	}

	server := NewArtifactsServer(cfg, logger)

	testContent := "test artifact content"
	mockStorage := &mockArtifactStorageProvider{
		existsFunc: func(ctx context.Context, artifactID string, filename string) (bool, error) {
			return artifactID == "test-artifact" && filename == "test.txt", nil
		},
		retrieveFunc: func(ctx context.Context, artifactID string, filename string) (io.ReadCloser, error) {
			if artifactID == "test-artifact" && filename == "test.txt" {
				return io.NopCloser(strings.NewReader(testContent)), nil
			}
			return nil, fmt.Errorf("artifact not found")
		},
	}
	server.SetStorage(mockStorage)

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	// Test artifact download
	resp, err := http.Get("http://localhost:8085/artifacts/test-artifact/test.txt")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(body))

	// Check content type header
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
}

func TestArtifactsServer_ArtifactNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8086",
		},
	}

	server := NewArtifactsServer(cfg, logger)
	mockStorage := &mockArtifactStorageProvider{
		existsFunc: func(ctx context.Context, artifactID string, filename string) (bool, error) {
			return false, nil // Artifact doesn't exist
		},
	}
	server.SetStorage(mockStorage)

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	// Test artifact not found
	resp, err := http.Get("http://localhost:8086/artifacts/nonexistent/file.txt")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "artifact not found")
}

func TestArtifactsServer_BadRequest(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8087",
		},
	}

	server := NewArtifactsServer(cfg, logger)
	mockStorage := &mockArtifactStorageProvider{}
	server.SetStorage(mockStorage)

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	// Test missing filename
	resp, err := http.Get("http://localhost:8087/artifacts/test-artifact/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode) // Gin returns 404 for missing route params
}

func TestArtifactsServer_StorageError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8088",
		},
	}

	server := NewArtifactsServer(cfg, logger)
	mockStorage := &mockArtifactStorageProvider{
		existsFunc: func(ctx context.Context, artifactID string, filename string) (bool, error) {
			return false, fmt.Errorf("storage error")
		},
	}
	server.SetStorage(mockStorage)

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	// Test storage error
	resp, err := http.Get("http://localhost:8088/artifacts/test/file.txt")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "failed to check artifact existence")
}
