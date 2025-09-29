package server

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOArtifactStorage implements ArtifactStorageProvider using MinIO/S3
type MinIOArtifactStorage struct {
	client     *minio.Client
	bucketName string
	baseURL    string
}

// NewMinIOArtifactStorage creates a new MinIO-based artifact storage provider
func NewMinIOArtifactStorage(endpoint, accessKey, secretKey, bucketName, baseURL string, useSSL bool) (*MinIOArtifactStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	storage := &MinIOArtifactStorage{
		client:     client,
		bucketName: bucketName,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return storage, nil
}

// Store stores an artifact to MinIO
func (m *MinIOArtifactStorage) Store(ctx context.Context, artifactID string, filename string, data io.Reader) (string, error) {
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if artifactID == "" || filename == "" {
		return "", fmt.Errorf("invalid artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s", artifactID, filename)

	_, err := m.client.PutObject(ctx, m.bucketName, objectName, data, -1, minio.PutObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to store artifact in MinIO: %w", err)
	}

	url := m.GetURL(artifactID, filename)
	return url, nil
}

// Retrieve retrieves an artifact from MinIO
func (m *MinIOArtifactStorage) Retrieve(ctx context.Context, artifactID string, filename string) (io.ReadCloser, error) {
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if artifactID == "" || filename == "" {
		return nil, fmt.Errorf("invalid artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s", artifactID, filename)

	object, err := m.client.GetObject(ctx, m.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve artifact from MinIO: %w", err)
	}

	return object, nil
}

// Delete removes an artifact from MinIO
func (m *MinIOArtifactStorage) Delete(ctx context.Context, artifactID string, filename string) error {
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if artifactID == "" || filename == "" {
		return fmt.Errorf("invalid artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s", artifactID, filename)

	err := m.client.RemoveObject(ctx, m.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete artifact from MinIO: %w", err)
	}

	return nil
}

// Exists checks if an artifact exists in MinIO
func (m *MinIOArtifactStorage) Exists(ctx context.Context, artifactID string, filename string) (bool, error) {
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)

	if artifactID == "" || filename == "" {
		return false, fmt.Errorf("invalid artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s", artifactID, filename)

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
func (m *MinIOArtifactStorage) GetURL(artifactID string, filename string) string {
	artifactID = sanitizePath(artifactID)
	filename = sanitizePath(filename)
	return fmt.Sprintf("%s/artifacts/%s/%s", m.baseURL, artifactID, filename)
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

// CleanupOldestArtifacts removes old artifacts keeping only maxCount per artifact ID
func (m *MinIOArtifactStorage) CleanupOldestArtifacts(ctx context.Context, maxCount int) (int, error) {
	if maxCount <= 0 {
		return 0, nil
	}

	objectCh := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	artifactGroups := make(map[string][]minio.ObjectInfo)
	for object := range objectCh {
		if object.Err != nil {
			continue
		}

		parts := strings.Split(object.Key, "/")
		if len(parts) >= 2 {
			artifactID := parts[0]
			artifactGroups[artifactID] = append(artifactGroups[artifactID], object)
		}
	}

	removedCount := 0
	for _, objects := range artifactGroups {
		if len(objects) <= maxCount {
			continue
		}

		sort.Slice(objects, func(i, j int) bool {
			return objects[i].LastModified.After(objects[j].LastModified)
		})

		for i := maxCount; i < len(objects); i++ {
			err := m.client.RemoveObject(ctx, m.bucketName, objects[i].Key, minio.RemoveObjectOptions{})
			if err == nil {
				removedCount++
			}
		}
	}

	return removedCount, nil
}
