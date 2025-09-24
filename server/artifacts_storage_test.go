package server

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockArtifactStorageProvider implements ArtifactStorageProvider for testing
type mockArtifactStorageProvider struct {
	storeFunc    func(ctx context.Context, artifactID string, filename string, data io.Reader) (string, error)
	retrieveFunc func(ctx context.Context, artifactID string, filename string) (io.ReadCloser, error)
	deleteFunc   func(ctx context.Context, artifactID string, filename string) error
	existsFunc   func(ctx context.Context, artifactID string, filename string) (bool, error)
	getURLFunc   func(artifactID string, filename string) string
	closeFunc    func() error
}

func (m *mockArtifactStorageProvider) Store(ctx context.Context, artifactID string, filename string, data io.Reader) (string, error) {
	if m.storeFunc != nil {
		return m.storeFunc(ctx, artifactID, filename, data)
	}
	return "", nil
}

func (m *mockArtifactStorageProvider) Retrieve(ctx context.Context, artifactID string, filename string) (io.ReadCloser, error) {
	if m.retrieveFunc != nil {
		return m.retrieveFunc(ctx, artifactID, filename)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockArtifactStorageProvider) Delete(ctx context.Context, artifactID string, filename string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, artifactID, filename)
	}
	return nil
}

func (m *mockArtifactStorageProvider) Exists(ctx context.Context, artifactID string, filename string) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, artifactID, filename)
	}
	return true, nil
}

func (m *mockArtifactStorageProvider) GetURL(artifactID string, filename string) string {
	if m.getURLFunc != nil {
		return m.getURLFunc(artifactID, filename)
	}
	return "http://localhost:8081/artifacts/" + artifactID + "/" + filename
}

func (m *mockArtifactStorageProvider) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestFilesystemArtifactStorage_NewFilesystemArtifactStorage(t *testing.T) {
	storage, err := NewFilesystemArtifactStorage("./test-artifacts", "http://localhost:8081")
	require.NoError(t, err)
	require.NotNil(t, storage)

	defer func() { _ = storage.Close() }()

	assert.Equal(t, "./test-artifacts", storage.basePath)
	assert.Equal(t, "http://localhost:8081", storage.baseURL)
}

func TestFilesystemArtifactStorage_Store(t *testing.T) {
	storage, err := NewFilesystemArtifactStorage("./test-artifacts", "http://localhost:8081")
	require.NoError(t, err)
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	data := strings.NewReader("test content")

	url, err := storage.Store(ctx, "test-artifact", "test.txt", data)
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:8081/artifacts/test-artifact/test.txt", url)

	// Verify file exists
	exists, err := storage.Exists(ctx, "test-artifact", "test.txt")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Clean up
	err = storage.Delete(ctx, "test-artifact", "test.txt")
	assert.NoError(t, err)
}

func TestFilesystemArtifactStorage_Retrieve(t *testing.T) {
	storage, err := NewFilesystemArtifactStorage("./test-artifacts", "http://localhost:8081")
	require.NoError(t, err)
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	testContent := "test content for retrieval"

	// Store test data
	_, err = storage.Store(ctx, "test-artifact", "test.txt", strings.NewReader(testContent))
	require.NoError(t, err)

	// Retrieve the data
	reader, err := storage.Retrieve(ctx, "test-artifact", "test.txt")
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()

	// Read content
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))

	// Clean up
	err = storage.Delete(ctx, "test-artifact", "test.txt")
	assert.NoError(t, err)
}

func TestFilesystemArtifactStorage_GetURL(t *testing.T) {
	storage, err := NewFilesystemArtifactStorage("./test-artifacts", "http://localhost:8081")
	require.NoError(t, err)
	defer func() { _ = storage.Close() }()

	url := storage.GetURL("test-artifact", "test.txt")
	assert.Equal(t, "http://localhost:8081/artifacts/test-artifact/test.txt", url)
}

func TestFilesystemArtifactStorage_InvalidInputs(t *testing.T) {
	storage, err := NewFilesystemArtifactStorage("./test-artifacts", "http://localhost:8081")
	require.NoError(t, err)
	defer func() { _ = storage.Close() }()

	ctx := context.Background()

	// Test empty artifact ID
	_, err = storage.Store(ctx, "", "test.txt", strings.NewReader("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid artifact ID or filename")

	// Test empty filename
	_, err = storage.Store(ctx, "test-artifact", "", strings.NewReader("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid artifact ID or filename")
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
