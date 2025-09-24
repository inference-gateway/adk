package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FilesystemArtifactStorage implements ArtifactStorageProvider using local filesystem
type FilesystemArtifactStorage struct {
	basePath string
	baseURL  string
}

// NewFilesystemArtifactStorage creates a new filesystem-based artifact storage provider
func NewFilesystemArtifactStorage(basePath, baseURL string) (*FilesystemArtifactStorage, error) {
	// Ensure the base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	// Clean the baseURL to ensure consistent format
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &FilesystemArtifactStorage{
		basePath: basePath,
		baseURL:  baseURL,
	}, nil
}

// Store stores an artifact to the local filesystem
func (fs *FilesystemArtifactStorage) Store(ctx context.Context, artifactID string, filename string, data io.Reader) (string, error) {
	// Sanitize inputs to prevent directory traversal
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if artifactID == "" || filename == "" {
		return "", fmt.Errorf("invalid artifact ID or filename")
	}

	// Create directory for artifact
	artifactDir := filepath.Join(fs.basePath, artifactID)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artifact directory: %w", err)
	}

	// Create the file
	filePath := filepath.Join(artifactDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create artifact file: %w", err)
	}
	defer func() {
		_ = file.Close() // Ignore close errors in defer
	}()

	// Copy data to file
	_, err = io.Copy(file, data)
	if err != nil {
		_ = os.Remove(filePath) // Clean up on error, ignore cleanup errors
		return "", fmt.Errorf("failed to write artifact data: %w", err)
	}

	// Return the URL for accessing this artifact
	url := fs.GetURL(artifactID, filename)
	return url, nil
}

// Retrieve retrieves an artifact from the local filesystem
func (fs *FilesystemArtifactStorage) Retrieve(ctx context.Context, artifactID string, filename string) (io.ReadCloser, error) {
	// Sanitize inputs
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if artifactID == "" || filename == "" {
		return nil, fmt.Errorf("invalid artifact ID or filename")
	}

	filePath := filepath.Join(fs.basePath, artifactID, filename)
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact not found")
		}
		return nil, fmt.Errorf("failed to open artifact: %w", err)
	}

	return file, nil
}

// Delete removes an artifact from the filesystem
func (fs *FilesystemArtifactStorage) Delete(ctx context.Context, artifactID string, filename string) error {
	// Sanitize inputs
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if artifactID == "" || filename == "" {
		return fmt.Errorf("invalid artifact ID or filename")
	}

	filePath := filepath.Join(fs.basePath, artifactID, filename)
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete artifact: %w", err)
	}

	// Try to remove the artifact directory if it's empty
	artifactDir := filepath.Join(fs.basePath, artifactID)
	_ = os.Remove(artifactDir) // Ignore errors - directory might not be empty

	return nil
}

// Exists checks if an artifact exists in the filesystem
func (fs *FilesystemArtifactStorage) Exists(ctx context.Context, artifactID string, filename string) (bool, error) {
	// Sanitize inputs
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if artifactID == "" || filename == "" {
		return false, fmt.Errorf("invalid artifact ID or filename")
	}

	filePath := filepath.Join(fs.basePath, artifactID, filename)
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check artifact existence: %w", err)
	}
	return true, nil
}

// GetURL returns the public URL for accessing an artifact
func (fs *FilesystemArtifactStorage) GetURL(artifactID string, filename string) string {
	// Sanitize inputs
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)
	return fmt.Sprintf("%s/artifacts/%s/%s", fs.baseURL, artifactID, filename)
}

// Close cleans up the filesystem storage (no-op for filesystem)
func (fs *FilesystemArtifactStorage) Close() error {
	return nil
}

// sanitizePath removes dangerous characters and path traversal attempts
func sanitizePath(path string) string {
	// Remove any path separators and dangerous characters
	path = strings.ReplaceAll(path, "/", "")
	path = strings.ReplaceAll(path, "\\", "")
	path = strings.ReplaceAll(path, "..", "")
	path = strings.TrimSpace(path)
	return path
}
