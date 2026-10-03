package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	serverConfig "github.com/inference-gateway/adk/server/config"
)

// FilesystemArtifactStorage implements ArtifactStorageProvider using local filesystem
type FilesystemArtifactStorage struct {
	basePath string
	baseURL  string
}

// NewFilesystemArtifactStorage creates a new filesystem-based artifact storage provider
func NewFilesystemArtifactStorage(cfg *serverConfig.ArtifactsStorageConfig) (*FilesystemArtifactStorage, error) {
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")

	return &FilesystemArtifactStorage{
		basePath: cfg.BasePath,
		baseURL:  baseURL,
	}, nil
}

// Store stores an artifact to the local filesystem
func (fs *FilesystemArtifactStorage) Store(ctx context.Context, contextID string, artifactID string, filename string, data io.Reader) (string, error) {
	contextID = sanitizePath(contextID)
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return "", fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	artifactDir := filepath.Join(fs.basePath, contextID, artifactID)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artifact directory: %w", err)
	}

	filePath := filepath.Join(artifactDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create artifact file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = io.Copy(file, data)
	if err != nil {
		_ = os.Remove(filePath)
		return "", fmt.Errorf("failed to write artifact data: %w", err)
	}

	url := fs.GetURL(contextID, artifactID, filename)
	return url, nil
}

// Retrieve retrieves an artifact from the local filesystem
func (fs *FilesystemArtifactStorage) Retrieve(ctx context.Context, contextID string, artifactID string, filename string) (io.ReadCloser, error) {
	contextID = sanitizePath(contextID)
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return nil, fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	filePath := filepath.Join(fs.basePath, contextID, artifactID, filename)
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
func (fs *FilesystemArtifactStorage) Delete(ctx context.Context, contextID string, artifactID string, filename string) error {
	contextID = sanitizePath(contextID)
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	filePath := filepath.Join(fs.basePath, contextID, artifactID, filename)
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete artifact: %w", err)
	}

	// Best-effort prune of now-empty artifact and context directories.
	_ = os.Remove(filepath.Join(fs.basePath, contextID, artifactID))
	_ = os.Remove(filepath.Join(fs.basePath, contextID))

	return nil
}

// Exists checks if an artifact exists in the filesystem
func (fs *FilesystemArtifactStorage) Exists(ctx context.Context, contextID string, artifactID string, filename string) (bool, error) {
	contextID = sanitizePath(contextID)
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return false, fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	filePath := filepath.Join(fs.basePath, contextID, artifactID, filename)
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
func (fs *FilesystemArtifactStorage) GetURL(contextID string, artifactID string, filename string) string {
	contextID = sanitizePath(contextID)
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)
	return fmt.Sprintf("%s/artifacts/%s/%s/%s", fs.baseURL, contextID, artifactID, filename)
}

// Close cleans up the filesystem storage (no-op for filesystem)
func (fs *FilesystemArtifactStorage) Close() error {
	return nil
}

// CleanupExpiredArtifacts removes artifacts older than maxAge
func (fs *FilesystemArtifactStorage) CleanupExpiredArtifacts(ctx context.Context, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, nil
	}

	cutoffTime := time.Now().Add(-maxAge)
	removedCount := 0

	err := filepath.Walk(fs.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err == nil {
				removedCount++
			}
		}

		return nil
	})

	if err != nil {
		return removedCount, fmt.Errorf("failed to cleanup expired artifacts: %w", err)
	}

	fs.cleanupEmptyDirectories()
	return removedCount, nil
}

// CleanupOldestArtifacts removes the oldest artifacts keeping only maxCount per context
func (fs *FilesystemArtifactStorage) CleanupOldestArtifacts(ctx context.Context, maxCount int) (int, error) {
	if maxCount <= 0 {
		return 0, nil
	}

	contexts, err := os.ReadDir(fs.basePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read artifacts directory: %w", err)
	}

	removedCount := 0
	for _, contextEntry := range contexts {
		if !contextEntry.IsDir() {
			continue
		}

		cleaned, err := fs.cleanupContextDirectory(filepath.Join(fs.basePath, contextEntry.Name()), maxCount)
		if err != nil {
			continue
		}
		removedCount += cleaned
	}

	fs.cleanupEmptyDirectories()
	return removedCount, nil
}

// cleanupContextDirectory removes the oldest artifact directories in a context,
// keeping only the newest maxCount of them
func (fs *FilesystemArtifactStorage) cleanupContextDirectory(contextDir string, maxCount int) (int, error) {
	entries, err := os.ReadDir(contextDir)
	if err != nil {
		return 0, err
	}

	type artifactInfo struct {
		path    string
		modTime time.Time
	}

	var artifacts []artifactInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		artifacts = append(artifacts, artifactInfo{
			path:    filepath.Join(contextDir, entry.Name()),
			modTime: info.ModTime(),
		})
	}

	if len(artifacts) <= maxCount {
		return 0, nil
	}

	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].modTime.After(artifacts[j].modTime)
	})

	removedCount := 0
	for _, artifact := range artifacts[maxCount:] {
		if err := os.RemoveAll(artifact.path); err == nil {
			removedCount++
		}
	}

	return removedCount, nil
}

// cleanupEmptyDirectories removes empty artifact and context directories
func (fs *FilesystemArtifactStorage) cleanupEmptyDirectories() {
	contexts, err := os.ReadDir(fs.basePath)
	if err != nil {
		return
	}

	for _, contextEntry := range contexts {
		if !contextEntry.IsDir() {
			continue
		}

		contextDir := filepath.Join(fs.basePath, contextEntry.Name())
		artifacts, err := os.ReadDir(contextDir)
		if err != nil {
			continue
		}

		for _, artifactEntry := range artifacts {
			if !artifactEntry.IsDir() {
				continue
			}

			artifactDir := filepath.Join(contextDir, artifactEntry.Name())
			if files, err := os.ReadDir(artifactDir); err == nil && len(files) == 0 {
				_ = os.Remove(artifactDir)
			}
		}

		if files, err := os.ReadDir(contextDir); err == nil && len(files) == 0 {
			_ = os.Remove(contextDir)
		}
	}
}

// sanitizePath removes dangerous characters and path traversal attempts
func sanitizePath(path string) string {
	path = strings.ReplaceAll(path, "/", "")
	path = strings.ReplaceAll(path, "\\", "")
	path = strings.ReplaceAll(path, "..", "")
	path = strings.TrimSpace(path)
	return path
}
