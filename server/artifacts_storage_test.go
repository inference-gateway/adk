package server

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	serverConfig "github.com/inference-gateway/adk/server/config"
)

func TestFilesystemArtifactStorage_NewFilesystemArtifactStorage(t *testing.T) {
	cfg := &serverConfig.ArtifactsStorageConfig{
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
	cfg := &serverConfig.ArtifactsStorageConfig{
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
	cfg := &serverConfig.ArtifactsStorageConfig{
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
	cfg := &serverConfig.ArtifactsStorageConfig{
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
	cfg := &serverConfig.ArtifactsStorageConfig{
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
	cfg := &serverConfig.ArtifactsStorageConfig{
		BasePath: "./test-artifacts-isolation",
		BaseURL:  "http://localhost:8081",
	}
	storage, err := NewFilesystemArtifactStorage(cfg)
	require.NoError(t, err)
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

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

	exists, err := storage.Exists(ctx, "context-a", "artifact-1", "other.md")
	require.NoError(t, err)
	assert.False(t, exists)

	_ = storage.Delete(ctx, "context-a", "artifact-1", "report.md")
	_ = storage.Delete(ctx, "context-b", "artifact-1", "report.md")
}

func TestFilesystemArtifactStorage_CleanupOldestArtifacts(t *testing.T) {
	tests := []struct {
		name            string
		maxCount        int
		expectedRemoved int
		expectedKept    []string
	}{
		{
			name:            "keeps the newest artifacts per context",
			maxCount:        2,
			expectedRemoved: 2,
			expectedKept:    []string{"artifact-3", "artifact-4"},
		},
		{
			name:            "unlimited when maxCount is zero",
			maxCount:        0,
			expectedRemoved: 0,
			expectedKept:    []string{"artifact-1", "artifact-2", "artifact-3", "artifact-4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			basePath := t.TempDir()
			storage, err := NewFilesystemArtifactStorage(&serverConfig.ArtifactsStorageConfig{
				BasePath: basePath,
				BaseURL:  "http://localhost:8081",
			})
			require.NoError(t, err)
			defer func() { _ = storage.Close() }()

			ctx := context.Background()
			now := time.Now()
			for i, artifactID := range []string{"artifact-1", "artifact-2", "artifact-3", "artifact-4"} {
				_, err := storage.Store(ctx, "context-a", artifactID, "report.md", strings.NewReader(artifactID))
				require.NoError(t, err)

				modTime := now.Add(time.Duration(i) * time.Hour)
				require.NoError(t, os.Chtimes(filepath.Join(basePath, "context-a", artifactID), modTime, modTime))
			}

			_, err = storage.Store(ctx, "context-b", "artifact-1", "report.md", strings.NewReader("from B"))
			require.NoError(t, err)

			removed, err := storage.CleanupOldestArtifacts(ctx, tt.maxCount)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedRemoved, removed)

			for _, artifactID := range []string{"artifact-1", "artifact-2", "artifact-3", "artifact-4"} {
				exists, err := storage.Exists(ctx, "context-a", artifactID, "report.md")
				require.NoError(t, err)
				assert.Equal(t, slices.Contains(tt.expectedKept, artifactID), exists, "artifact %s", artifactID)
			}

			exists, err := storage.Exists(ctx, "context-b", "artifact-1", "report.md")
			require.NoError(t, err)
			assert.True(t, exists, "other contexts must not be affected")
		})
	}
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
