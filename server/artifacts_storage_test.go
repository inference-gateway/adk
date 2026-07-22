package server

import (
	"context"
	"io"
	"strings"
	"testing"

	config "github.com/inference-gateway/adk/server/config"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestFilesystemArtifactStorage_NewFilesystemArtifactStorage(t *testing.T) {
	cfg := &config.ArtifactsStorageConfig{
		BasePath: "./test-artifacts",
		BaseURL:  "http://localhost:8081",
	}
	storage, err := NewFilesystemArtifactStorage(cfg)
	require.NoError(t, err)
	require.NotNil(t, storage)

	defer func() { _ = storage.Close() }()

	assert.Equal(t, "./test-artifacts", storage.basePath)
	assert.Equal(t, "http://localhost:8081", storage.baseURL)
}

func TestFilesystemArtifactStorage_Store(t *testing.T) {
	cfg := &config.ArtifactsStorageConfig{
		BasePath: "./test-artifacts",
		BaseURL:  "http://localhost:8081",
	}
	storage, err := NewFilesystemArtifactStorage(cfg)
	require.NoError(t, err)
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	data := strings.NewReader("test content")

	url, err := storage.Store(ctx, "test-context", "test-artifact", "test.txt", data)
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:8081/artifacts/test-context/test-artifact/test.txt", url)

	exists, err := storage.Exists(ctx, "test-context", "test-artifact", "test.txt")
	assert.NoError(t, err)
	assert.True(t, exists)

	err = storage.Delete(ctx, "test-context", "test-artifact", "test.txt")
	assert.NoError(t, err)
}

func TestFilesystemArtifactStorage_Retrieve(t *testing.T) {
	cfg := &config.ArtifactsStorageConfig{
		BasePath: "./test-artifacts",
		BaseURL:  "http://localhost:8081",
	}
	storage, err := NewFilesystemArtifactStorage(cfg)
	require.NoError(t, err)
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	testContent := "test content for retrieval"

	_, err = storage.Store(ctx, "test-context", "test-artifact", "test.txt", strings.NewReader(testContent))
	require.NoError(t, err)

	reader, err := storage.Retrieve(ctx, "test-context", "test-artifact", "test.txt")
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()

	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))

	err = storage.Delete(ctx, "test-context", "test-artifact", "test.txt")
	assert.NoError(t, err)
}

func TestFilesystemArtifactStorage_GetURL(t *testing.T) {
	cfg := &config.ArtifactsStorageConfig{
		BasePath: "./test-artifacts",
		BaseURL:  "http://localhost:8081",
	}
	storage, err := NewFilesystemArtifactStorage(cfg)
	require.NoError(t, err)
	defer func() { _ = storage.Close() }()

	url := storage.GetURL("test-context", "test-artifact", "test.txt")
	assert.Equal(t, "http://localhost:8081/artifacts/test-context/test-artifact/test.txt", url)
}

func TestFilesystemArtifactStorage_InvalidInputs(t *testing.T) {
	cfg := &config.ArtifactsStorageConfig{
		BasePath: "./test-artifacts",
		BaseURL:  "http://localhost:8081",
	}
	storage, err := NewFilesystemArtifactStorage(cfg)
	require.NoError(t, err)
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	_, err = storage.Store(ctx, "test-context", "", "test.txt", strings.NewReader("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid context ID, artifact ID or filename")

	_, err = storage.Store(ctx, "test-context", "test-artifact", "", strings.NewReader("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid context ID, artifact ID or filename")

	_, err = storage.Store(ctx, "", "test-artifact", "test.txt", strings.NewReader("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid context ID, artifact ID or filename")
}

func TestFilesystemArtifactStorage_ContextIsolation(t *testing.T) {
	cfg := &config.ArtifactsStorageConfig{
		BasePath: "./test-artifacts-isolation",
		BaseURL:  "http://localhost:8081",
	}
	storage, err := NewFilesystemArtifactStorage(cfg)
	require.NoError(t, err)
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Same artifactID + filename in two different contexts must not collide.
	_, err = storage.Store(ctx, "context-a", "artifact-1", "report.md", strings.NewReader("from A"))
	require.NoError(t, err)
	_, err = storage.Store(ctx, "context-b", "artifact-1", "report.md", strings.NewReader("from B"))
	require.NoError(t, err)

	readerA, err := storage.Retrieve(ctx, "context-a", "artifact-1", "report.md")
	require.NoError(t, err)
	defer func() { _ = readerA.Close() }()
	contentA, err := io.ReadAll(readerA)
	require.NoError(t, err)
	assert.Equal(t, "from A", string(contentA))

	// A file stored under context-b is invisible under context-a's grouping.
	exists, err := storage.Exists(ctx, "context-a", "artifact-1", "other.md")
	require.NoError(t, err)
	assert.False(t, exists)

	_ = storage.Delete(ctx, "context-a", "artifact-1", "report.md")
	_ = storage.Delete(ctx, "context-b", "artifact-1", "report.md")
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal-filename", "normal-filename"},
		{"../../../etc/passwd", "etcpasswd"},
		{"file/with/slashes", "filewithslashes"},
		{"file\\with\\backslashes", "filewithbackslashes"},
		{"  spaced  ", "spaced"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
