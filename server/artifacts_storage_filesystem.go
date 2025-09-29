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
)

// FilesystemArtifactStorage implements ArtifactStorageProvider using local filesystem
type FilesystemArtifactStorage struct {
	basePath string
	baseURL  string
}

// NewFilesystemArtifactStorage creates a new filesystem-based artifact storage provider
func NewFilesystemArtifactStorage(basePath, baseURL string) (*FilesystemArtifactStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	baseURL = strings.TrimSuffix(baseURL, "/")

	return &FilesystemArtifactStorage{
		basePath: basePath,
		baseURL:  baseURL,
	}, nil
}

// Store stores an artifact to the local filesystem
func (fs *FilesystemArtifactStorage) Store(ctx context.Context, artifactID string, filename string, data io.Reader) (string, error) {
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if artifactID == "" || filename == "" {
		return "", fmt.Errorf("invalid artifact ID or filename")
	}

	artifactDir := filepath.Join(fs.basePath, artifactID)
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

	url := fs.GetURL(artifactID, filename)
	return url, nil
}

// Retrieve retrieves an artifact from the local filesystem
func (fs *FilesystemArtifactStorage) Retrieve(ctx context.Context, artifactID string, filename string) (io.ReadCloser, error) {
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

	artifactDir := filepath.Join(fs.basePath, artifactID)
	_ = os.Remove(artifactDir)

	return nil
}

// Exists checks if an artifact exists in the filesystem
func (fs *FilesystemArtifactStorage) Exists(ctx context.Context, artifactID string, filename string) (bool, error) {
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
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)
	return fmt.Sprintf("%s/artifacts/%s/%s", fs.baseURL, artifactID, filename)
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

// CleanupOldestArtifacts removes old artifacts keeping only maxCount per artifact ID
func (fs *FilesystemArtifactStorage) CleanupOldestArtifacts(ctx context.Context, maxCount int) (int, error) {
	if maxCount <= 0 {
		return 0, nil
	}

	removedCount := 0

	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read artifacts directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		artifactDir := filepath.Join(fs.basePath, entry.Name())
		cleaned, err := fs.cleanupArtifactDirectory(artifactDir, maxCount)
		if err != nil {
			continue
		}
		removedCount += cleaned
	}

	fs.cleanupEmptyDirectories()
	return removedCount, nil
}

// cleanupArtifactDirectory removes oldest files in a directory, keeping only maxCount files
func (fs *FilesystemArtifactStorage) cleanupArtifactDirectory(artifactDir string, maxCount int) (int, error) {
	files, err := os.ReadDir(artifactDir)
	if err != nil {
		return 0, err
	}

	if len(files) <= maxCount {
		return 0, nil
	}

	type fileInfo struct {
		name    string
		modTime time.Time
	}

	var fileInfos []fileInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		fileInfos = append(fileInfos, fileInfo{
			name:    file.Name(),
			modTime: info.ModTime(),
		})
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	removedCount := 0
	for i := maxCount; i < len(fileInfos); i++ {
		filePath := filepath.Join(artifactDir, fileInfos[i].name)
		if err := os.Remove(filePath); err == nil {
			removedCount++
		}
	}

	return removedCount, nil
}

// cleanupEmptyDirectories removes empty artifact directories
func (fs *FilesystemArtifactStorage) cleanupEmptyDirectories() {
	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		artifactDir := filepath.Join(fs.basePath, entry.Name())
		files, err := os.ReadDir(artifactDir)
		if err != nil {
			continue
		}

		if len(files) == 0 {
			_ = os.Remove(artifactDir)
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
