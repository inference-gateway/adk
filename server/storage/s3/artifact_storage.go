package s3

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	minio "github.com/minio/minio-go/v7"
	credentials "github.com/minio/minio-go/v7/pkg/credentials"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
)

// ArtifactStorage implements server.ArtifactStorageProvider using an S3-compatible
// object store such as RustFS, MinIO or AWS S3
type ArtifactStorage struct {
	client     *minio.Client
	bucketName string
	baseURL    string
}

var _ server.ArtifactStorageProvider = (*ArtifactStorage)(nil)

// NewArtifactStorage creates a new S3-based artifact storage provider
func NewArtifactStorage(cfg *serverConfig.ArtifactsStorageConfig) (*ArtifactStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	storage := &ArtifactStorage{
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

// Store stores an artifact in S3 storage
func (s *ArtifactStorage) Store(ctx context.Context, contextID string, artifactID string, filename string, data io.Reader) (string, error) {
	contextID = server.SanitizeArtifactPath(contextID)
	artifactID = server.SanitizeArtifactPath(artifactID)
	filename = server.SanitizeArtifactPath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return "", fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s/%s", contextID, artifactID, filename)

	_, err := s.client.PutObject(ctx, s.bucketName, objectName, data, -1, minio.PutObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to store artifact in S3 storage: %w", err)
	}

	url := s.GetURL(contextID, artifactID, filename)
	return url, nil
}

// Retrieve retrieves an artifact from S3 storage
func (s *ArtifactStorage) Retrieve(ctx context.Context, contextID string, artifactID string, filename string) (io.ReadCloser, error) {
	contextID = server.SanitizeArtifactPath(contextID)
	artifactID = server.SanitizeArtifactPath(artifactID)
	filename = server.SanitizeArtifactPath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return nil, fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s/%s", contextID, artifactID, filename)

	object, err := s.client.GetObject(ctx, s.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve artifact from S3 storage: %w", err)
	}

	return object, nil
}

// Delete removes an artifact from S3 storage
func (s *ArtifactStorage) Delete(ctx context.Context, contextID string, artifactID string, filename string) error {
	contextID = server.SanitizeArtifactPath(contextID)
	artifactID = server.SanitizeArtifactPath(artifactID)
	filename = server.SanitizeArtifactPath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s/%s", contextID, artifactID, filename)

	err := s.client.RemoveObject(ctx, s.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete artifact from S3 storage: %w", err)
	}

	return nil
}

// Exists checks if an artifact exists in S3 storage
func (s *ArtifactStorage) Exists(ctx context.Context, contextID string, artifactID string, filename string) (bool, error) {
	contextID = server.SanitizeArtifactPath(contextID)
	artifactID = server.SanitizeArtifactPath(artifactID)
	filename = server.SanitizeArtifactPath(filename)

	if contextID == "" || artifactID == "" || filename == "" {
		return false, fmt.Errorf("invalid context ID, artifact ID or filename")
	}

	objectName := fmt.Sprintf("%s/%s/%s", contextID, artifactID, filename)

	_, err := s.client.StatObject(ctx, s.bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check artifact existence in S3 storage: %w", err)
	}

	return true, nil
}

// GetURL returns the public URL for accessing an artifact
func (s *ArtifactStorage) GetURL(contextID string, artifactID string, filename string) string {
	contextID = server.SanitizeArtifactPath(contextID)
	artifactID = server.SanitizeArtifactPath(artifactID)
	filename = server.SanitizeArtifactPath(filename)
	return fmt.Sprintf("%s/artifacts/%s/%s/%s", s.baseURL, contextID, artifactID, filename)
}

// Close closes the S3 connection
func (s *ArtifactStorage) Close() error {
	return nil
}

// CleanupExpiredArtifacts removes artifacts older than maxAge
func (s *ArtifactStorage) CleanupExpiredArtifacts(ctx context.Context, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, nil
	}

	cutoffTime := time.Now().Add(-maxAge)
	removedCount := 0

	objectCh := s.client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
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
		err := s.client.RemoveObject(ctx, s.bucketName, obj.Key, minio.RemoveObjectOptions{})
		if err == nil {
			removedCount++
		}
	}

	return removedCount, nil
}

type artifactGroup struct {
	keys    []string
	modTime time.Time
}

// CleanupOldestArtifacts removes the oldest artifacts keeping only maxCount per context
func (s *ArtifactStorage) CleanupOldestArtifacts(ctx context.Context, maxCount int) (int, error) {
	if maxCount <= 0 {
		return 0, nil
	}

	removedCount := 0
	for _, artifacts := range s.groupObjectsByContext(ctx) {
		if len(artifacts) <= maxCount {
			continue
		}

		sort.Slice(artifacts, func(i, j int) bool {
			return artifacts[i].modTime.After(artifacts[j].modTime)
		})

		for _, artifact := range artifacts[maxCount:] {
			if s.removeObjects(ctx, artifact.keys) {
				removedCount++
			}
		}
	}

	return removedCount, nil
}

// groupObjectsByContext groups stored objects into artifacts per context ID,
// following the {contextID}/{artifactID}/{filename} key layout
func (s *ArtifactStorage) groupObjectsByContext(ctx context.Context) map[string][]*artifactGroup {
	objectCh := s.client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	contexts := make(map[string][]*artifactGroup)
	artifactsByID := make(map[string]*artifactGroup)

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
			artifact = &artifactGroup{}
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
func (s *ArtifactStorage) removeObjects(ctx context.Context, keys []string) bool {
	removed := false
	for _, key := range keys {
		if err := s.client.RemoveObject(ctx, s.bucketName, key, minio.RemoveObjectOptions{}); err == nil {
			removed = true
		}
	}
	return removed
}

// init registers the S3 backend under its canonical provider name and the
// legacy minio alias. The backend is opt-in: it is only linked into programs
// that import this package, e.g. _ "github.com/inference-gateway/adk/server/storage/s3"
func init() {
	factory := func(cfg *serverConfig.ArtifactsStorageConfig, logger *zap.Logger) (server.ArtifactStorageProvider, error) {
		storage, err := NewArtifactStorage(cfg)
		if err != nil {
			return nil, err
		}

		logger.Info("artifact service initialized with S3 storage",
			zap.String("endpoint", cfg.Endpoint),
			zap.String("bucket", cfg.BucketName))

		return storage, nil
	}

	server.RegisterArtifactStorageProvider("s3", factory)
	server.RegisterArtifactStorageProvider("minio", factory)
}
