package server

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	minio "github.com/minio/minio-go/v7"
	credentials "github.com/minio/minio-go/v7/pkg/credentials"

	serverConfig "github.com/inference-gateway/adk/server/config"
)

// MinIOArtifactStorage implements ArtifactStorageProvider using MinIO/S3
type MinIOArtifactStorage struct {
	client     *minio.Client
	bucketName string
	baseURL    string
}

// NewMinIOArtifactStorage creates a new MinIO-based artifact storage provider
func NewMinIOArtifactStorage(cfg *serverConfig.ArtifactsStorageConfig) (*MinIOArtifactStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	storage := &MinIOArtifactStorage{
		client:     client,
		bucketName: cfg.BucketName,
		baseURL:    strings.TrimSuffix(cfg.BaseURL, "/"),
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return storage, nil
}

// Store stores an artifact to MinIO
func (m *MinIOArtifactStorage) Store(ctx context.Context, contextID string, artifactID string, filename string, data io.Reader) (string, error) {
	contextID = sanitizePath(contextID)
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return "", fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s/%s", contextID, artifactID, filename)

	_, err := m.client.PutObject(ctx, m.bucketName, objectName, data, -1, minio.PutObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to store artifact in MinIO: %w", err)
	}

	url := m.GetURL(contextID, artifactID, filename)
	return url, nil
}

// Retrieve retrieves an artifact from MinIO
func (m *MinIOArtifactStorage) Retrieve(ctx context.Context, contextID string, artifactID string, filename string) (io.ReadCloser, error) {
	contextID = sanitizePath(contextID)
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return nil, fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s/%s", contextID, artifactID, filename)

	object, err := m.client.GetObject(ctx, m.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve artifact from MinIO: %w", err)
	}

	return object, nil
}

// Delete removes an artifact from MinIO
func (m *MinIOArtifactStorage) Delete(ctx context.Context, contextID string, artifactID string, filename string) error {
	contextID = sanitizePath(contextID)
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s/%s", contextID, artifactID, filename)

	err := m.client.RemoveObject(ctx, m.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete artifact from MinIO: %w", err)
	}

	return nil
}

// Exists checks if an artifact exists in MinIO
func (m *MinIOArtifactStorage) Exists(ctx context.Context, contextID string, artifactID string, filename string) (bool, error) {
	contextID = sanitizePath(contextID)
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return false, fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s/%s", contextID, artifactID, filename)

	_, err := m.client.StatObject(ctx, m.bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check artifact existence in MinIO: %w", err)
	}

	return true, nil
}

// GetURL returns the public URL for accessing an artifact
func (m *MinIOArtifactStorage) GetURL(contextID string, artifactID string, filename string) string {
	contextID = sanitizePath(contextID)
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)
	return fmt.Sprintf("%s/artifacts/%s/%s/%s", m.baseURL, contextID, artifactID, filename)
}

// Close closes the MinIO connection
func (m *MinIOArtifactStorage) Close() error {
	return nil
}

// CleanupExpiredArtifacts removes artifacts older than maxAge
func (m *MinIOArtifactStorage) CleanupExpiredArtifacts(ctx context.Context, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, nil
	}

	cutoffTime := time.Now().Add(-maxAge)
	removedCount := 0

	objectCh := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	var objectsToDelete []minio.ObjectInfo
	for object := range objectCh {
		if object.Err != nil {
			continue
		}

		if object.LastModified.Before(cutoffTime) {
			objectsToDelete = append(objectsToDelete, object)
		}
	}

	for _, obj := range objectsToDelete {
		err := m.client.RemoveObject(ctx, m.bucketName, obj.Key, minio.RemoveObjectOptions{})
		if err == nil {
			removedCount++
		}
	}

	return removedCount, nil
}

type minioArtifactGroup struct {
	keys    []string
	modTime time.Time
}

// CleanupOldestArtifacts removes the oldest artifacts keeping only maxCount per context
func (m *MinIOArtifactStorage) CleanupOldestArtifacts(ctx context.Context, maxCount int) (int, error) {
	if maxCount <= 0 {
		return 0, nil
	}

	removedCount := 0
	for _, artifacts := range m.groupObjectsByContext(ctx) {
		if len(artifacts) <= maxCount {
			continue
		}

		sort.Slice(artifacts, func(i, j int) bool {
			return artifacts[i].modTime.After(artifacts[j].modTime)
		})

		for _, artifact := range artifacts[maxCount:] {
			if m.removeObjects(ctx, artifact.keys) {
				removedCount++
			}
		}
	}

	return removedCount, nil
}

// groupObjectsByContext groups stored objects into artifacts per context ID,
// following the {contextID}/{artifactID}/{filename} key layout
func (m *MinIOArtifactStorage) groupObjectsByContext(ctx context.Context) map[string][]*minioArtifactGroup {
	objectCh := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	contexts := make(map[string][]*minioArtifactGroup)
	artifactsByID := make(map[string]*minioArtifactGroup)

	for object := range objectCh {
		if object.Err != nil {
			continue
		}

		parts := strings.Split(object.Key, "/")
		if len(parts) < 3 {
			continue
		}

		contextID := parts[0]
		artifactKey := contextID + "/" + parts[1]

		artifact, ok := artifactsByID[artifactKey]
		if !ok {
			artifact = &minioArtifactGroup{}
			artifactsByID[artifactKey] = artifact
			contexts[contextID] = append(contexts[contextID], artifact)
		}

		artifact.keys = append(artifact.keys, object.Key)
		if object.LastModified.After(artifact.modTime) {
			artifact.modTime = object.LastModified
		}
	}

	return contexts
}

// removeObjects deletes the given object keys, reporting whether anything was removed
func (m *MinIOArtifactStorage) removeObjects(ctx context.Context, keys []string) bool {
	removed := false
	for _, key := range keys {
		if err := m.client.RemoveObject(ctx, m.bucketName, key, minio.RemoveObjectOptions{}); err == nil {
			removed = true
		}
	}
	return removed
}
