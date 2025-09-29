package server_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zaptest "go.uber.org/zap/zaptest"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	mocks "github.com/inference-gateway/adk/server/mocks"
)

func TestNewArtifactsServer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8082",
		},
	}

	srv := server.NewArtifactsServer(cfg, logger)
	assert.NotNil(t, srv)
}

func TestArtifactsServer_SetGetStorage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
	}

	server := server.NewArtifactsServer(cfg, logger)
	mockStorage := &mocks.FakeArtifactStorageProvider{}

	assert.Nil(t, server.GetStorage())

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

	server := server.NewArtifactsServer(cfg, logger)
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

	server := server.NewArtifactsServer(cfg, logger)
	mockStorage := &mocks.FakeArtifactStorageProvider{}
	server.SetStorage(mockStorage)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:8084/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

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

	server := server.NewArtifactsServer(cfg, logger)

	testContent := "test artifact content"
	mockStorage := &mocks.FakeArtifactStorageProvider{}
	mockStorage.ExistsStub = func(ctx context.Context, artifactID string, filename string) (bool, error) {
		return artifactID == "test-artifact" && filename == "test.txt", nil
	}
	mockStorage.RetrieveStub = func(ctx context.Context, artifactID string, filename string) (io.ReadCloser, error) {
		if artifactID == "test-artifact" && filename == "test.txt" {
			return io.NopCloser(strings.NewReader(testContent)), nil
		}
		return nil, fmt.Errorf("artifact not found")
	}
	server.SetStorage(mockStorage)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:8085/artifacts/test-artifact/test.txt")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(body))

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

	server := server.NewArtifactsServer(cfg, logger)
	mockStorage := &mocks.FakeArtifactStorageProvider{}
	mockStorage.ExistsStub = func(ctx context.Context, artifactID string, filename string) (bool, error) {
		return false, nil
	}
	server.SetStorage(mockStorage)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

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

	server := server.NewArtifactsServer(cfg, logger)
	mockStorage := &mocks.FakeArtifactStorageProvider{}
	server.SetStorage(mockStorage)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:8087/artifacts/test-artifact/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestArtifactsServer_StorageError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.ArtifactsConfig{
		Enable: true,
		ServerConfig: config.ArtifactsServerConfig{
			Port: "8088",
		},
	}

	server := server.NewArtifactsServer(cfg, logger)
	mockStorage := &mocks.FakeArtifactStorageProvider{}
	mockStorage.ExistsStub = func(ctx context.Context, artifactID string, filename string) (bool, error) {
		return false, fmt.Errorf("storage error")
	}
	server.SetStorage(mockStorage)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:8088/artifacts/test/file.txt")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "failed to check artifact existence")
}
