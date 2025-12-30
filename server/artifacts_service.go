package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

// ArtifactService defines the interface for artifact operations with storage support.
// It provides a clean API for artifact creation, management, and storage operations.
type ArtifactService interface {
	// CreateTextArtifact creates a text artifact
	CreateTextArtifact(name, description, text string) types.Artifact

	// CreateFileArtifact creates a file artifact with URI by storing the content
	CreateFileArtifact(name, description, filename string, data []byte, mimeType *string) (types.Artifact, error)

	// CreateFileArtifactFromURI creates a file artifact from an existing URI
	CreateFileArtifactFromURI(name, description, filename, uri string, mimeType *string) types.Artifact

	// CreateDataArtifact creates a structured data artifact
	CreateDataArtifact(name, description string, data map[string]any) types.Artifact

	// CreateMultiPartArtifact creates an artifact with multiple parts
	CreateMultiPartArtifact(name, description string, parts []types.Part) types.Artifact

	// AddArtifactToTask adds an artifact to a task's artifact collection
	AddArtifactToTask(task *types.Task, artifact types.Artifact)

	// AddArtifactsToTask adds multiple artifacts to a task's artifact collection
	AddArtifactsToTask(task *types.Task, artifacts []types.Artifact)

	// GetArtifactByID retrieves an artifact from a task by its ID
	GetArtifactByID(task *types.Task, artifactID string) (*types.Artifact, bool)

	// GetArtifactsByType retrieves all artifacts from a task that contain parts of a specific type
	GetArtifactsByType(task *types.Task, partKind string) []types.Artifact

	// ValidateArtifact validates that an artifact conforms to the A2A protocol specification
	ValidateArtifact(artifact types.Artifact) error

	// GetMimeTypeFromExtension returns a MIME type based on file extension
	GetMimeTypeFromExtension(filename string) *string

	// CreateTaskArtifactUpdateEvent creates an artifact update event for streaming
	CreateTaskArtifactUpdateEvent(taskID, contextID string, artifact types.Artifact, append, lastChunk *bool) types.TaskArtifactUpdateEvent

	// Storage operations for artifacts server
	// Exists checks if an artifact file exists
	Exists(ctx context.Context, artifactID, filename string) (bool, error)

	// Retrieve retrieves an artifact file
	Retrieve(ctx context.Context, artifactID, filename string) (io.ReadCloser, error)

	// CleanupExpiredArtifacts removes artifacts older than maxAge
	CleanupExpiredArtifacts(ctx context.Context, maxAge time.Duration) (int, error)

	// CleanupOldestArtifacts removes oldest artifacts keeping only maxArtifacts
	CleanupOldestArtifacts(ctx context.Context, maxArtifacts int) (int, error)

	// Close closes the artifact service and releases resources
	Close() error
}

// ArtifactServiceImpl is the concrete implementation of ArtifactService.
// It encapsulates the storage dependency and provides a clean API for artifact creation.
type ArtifactServiceImpl struct {
	storage ArtifactStorageProvider
	logger  *zap.Logger
}

// NewArtifactService creates a new artifact service from configuration.
// It creates and manages its own storage provider internally.
func NewArtifactService(cfg *config.ArtifactsConfig, logger *zap.Logger) (ArtifactService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("artifacts configuration is required")
	}

	if !cfg.Enable {
		return nil, fmt.Errorf("artifacts are not enabled in configuration")
	}

	// Create storage provider based on configuration
	var storage ArtifactStorageProvider
	var err error

	// Generate base URL if not provided
	storageConfig := cfg.StorageConfig
	if storageConfig.BaseURL == "" {
		scheme := "http"
		if cfg.ServerConfig.TLSConfig.Enable {
			scheme = "https"
		}
		host := cfg.ServerConfig.Host
		if host == "" {
			host = "localhost"
		}
		port := cfg.ServerConfig.Port
		if port == "" {
			port = "8081"
		}
		storageConfig.BaseURL = fmt.Sprintf("%s://%s:%s", scheme, host, port)
	}

	switch storageConfig.Provider {
	case "filesystem":
		storage, err = NewFilesystemArtifactStorage(&storageConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create filesystem storage: %w", err)
		}
		logger.Info("artifact service initialized with filesystem storage",
			zap.String("base_path", storageConfig.BasePath),
			zap.String("base_url", storageConfig.BaseURL))

	case "minio":
		storage, err = NewMinIOArtifactStorage(&storageConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create MinIO storage: %w", err)
		}
		logger.Info("artifact service initialized with MinIO storage",
			zap.String("endpoint", storageConfig.Endpoint),
			zap.String("bucket", storageConfig.BucketName))

	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", storageConfig.Provider)
	}

	return &ArtifactServiceImpl{
		storage: storage,
		logger:  logger,
	}, nil
}

// CreateTextArtifact creates a text artifact
func (as *ArtifactServiceImpl) CreateTextArtifact(name, description, text string) types.Artifact {
	return types.Artifact{
		ArtifactID:  uuid.New().String(),
		Name:        &name,
		Description: &description,
		Parts: []types.Part{
			types.CreateTextPart(text),
		},
	}
}

// CreateFileArtifact creates a file artifact with URI by storing the content
func (as *ArtifactServiceImpl) CreateFileArtifact(name, description, filename string, data []byte, mimeType *string) (types.Artifact, error) {
	artifactID := uuid.New().String()

	ctx := context.Background()
	reader := bytes.NewReader(data)
	uri, err := as.storage.Store(ctx, artifactID, filename, reader)
	if err != nil {
		return types.Artifact{}, fmt.Errorf("failed to store artifact: %w", err)
	}

	return types.Artifact{
		ArtifactID:  artifactID,
		Name:        &name,
		Description: &description,
		Parts: []types.Part{
			types.CreateFilePart(filename, *mimeType, nil, &uri),
		},
	}, nil
}

// CreateFileArtifactFromURI creates a file artifact from an existing URI
func (as *ArtifactServiceImpl) CreateFileArtifactFromURI(name, description, filename, uri string, mimeType *string) types.Artifact {
	mediaType := "application/octet-stream"
	if mimeType != nil {
		mediaType = *mimeType
	}

	return types.Artifact{
		ArtifactID:  uuid.New().String(),
		Name:        &name,
		Description: &description,
		Parts: []types.Part{
			types.CreateFilePart(filename, mediaType, nil, &uri),
		},
	}
}

// CreateDataArtifact creates a structured data artifact
func (as *ArtifactServiceImpl) CreateDataArtifact(name, description string, data map[string]any) types.Artifact {
	return types.Artifact{
		ArtifactID:  uuid.New().String(),
		Name:        &name,
		Description: &description,
		Parts: []types.Part{
			types.CreateDataPart(data),
		},
	}
}

// CreateMultiPartArtifact creates an artifact with multiple parts
func (as *ArtifactServiceImpl) CreateMultiPartArtifact(name, description string, parts []types.Part) types.Artifact {
	return types.Artifact{
		ArtifactID:  uuid.New().String(),
		Name:        &name,
		Description: &description,
		Parts:       parts,
	}
}

// AddArtifactToTask adds an artifact to a task's artifact collection
func (as *ArtifactServiceImpl) AddArtifactToTask(task *types.Task, artifact types.Artifact) {
	if task.Artifacts == nil {
		task.Artifacts = []types.Artifact{}
	}
	task.Artifacts = append(task.Artifacts, artifact)
}

// AddArtifactsToTask adds multiple artifacts to a task's artifact collection
func (as *ArtifactServiceImpl) AddArtifactsToTask(task *types.Task, artifacts []types.Artifact) {
	if task.Artifacts == nil {
		task.Artifacts = []types.Artifact{}
	}
	task.Artifacts = append(task.Artifacts, artifacts...)
}

// GetArtifactByID retrieves an artifact from a task by its ID
func (as *ArtifactServiceImpl) GetArtifactByID(task *types.Task, artifactID string) (*types.Artifact, bool) {
	for i, artifact := range task.Artifacts {
		if artifact.ArtifactID == artifactID {
			return &task.Artifacts[i], true
		}
	}
	return nil, false
}

// GetArtifactsByType retrieves all artifacts from a task that contain parts of a specific type
func (as *ArtifactServiceImpl) GetArtifactsByType(task *types.Task, partKind string) []types.Artifact {
	var matchingArtifacts []types.Artifact

	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			matched := false
			switch partKind {
			case "text":
				if part.Text != nil {
					matched = true
				}
			case "file":
				if part.File != nil {
					matched = true
				}
			case "data":
				if part.Data != nil {
					matched = true
				}
			}
			if matched {
				matchingArtifacts = append(matchingArtifacts, artifact)
				break
			}
		}
	}

	return matchingArtifacts
}

// ValidateArtifact validates that an artifact conforms to the A2A protocol specification
func (as *ArtifactServiceImpl) ValidateArtifact(artifact types.Artifact) error {
	if artifact.ArtifactID == "" {
		return fmt.Errorf("artifact must have a non-empty artifactId")
	}

	if len(artifact.Parts) == 0 {
		return fmt.Errorf("artifact must contain at least one part")
	}

	for i, part := range artifact.Parts {
		if err := as.validatePart(part); err != nil {
			return fmt.Errorf("invalid part at index %d: %w", i, err)
		}
	}

	return nil
}

// validatePart validates a single part of an artifact
func (as *ArtifactServiceImpl) validatePart(part types.Part) error {
	// Check which field is populated
	if part.Text != nil {
		if *part.Text == "" {
			return fmt.Errorf("text part must have non-empty text content")
		}
		return nil
	}
	if part.File != nil {
		// File part is valid if the pointer is not nil
		return nil
	}
	if part.Data != nil {
		if part.Data.Data == nil {
			return fmt.Errorf("data part must have non-nil data content")
		}
		return nil
	}

	return fmt.Errorf("part must have at least one populated field (text, file, or data)")
}

// GetMimeTypeFromExtension returns a MIME type based on file extension
func (as *ArtifactServiceImpl) GetMimeTypeFromExtension(filename string) *string {
	ext := filepath.Ext(filename)
	var mimeType string

	switch ext {
	case ".txt":
		mimeType = "text/plain"
	case ".json":
		mimeType = "application/json"
	case ".xml":
		mimeType = "application/xml"
	case ".pdf":
		mimeType = "application/pdf"
	case ".png":
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".gif":
		mimeType = "image/gif"
	case ".svg":
		mimeType = "image/svg+xml"
	case ".html":
		mimeType = "text/html"
	case ".css":
		mimeType = "text/css"
	case ".js":
		mimeType = "application/javascript"
	case ".csv":
		mimeType = "text/csv"
	case ".zip":
		mimeType = "application/zip"
	default:
		mimeType = "application/octet-stream"
	}

	return &mimeType
}

// CreateTaskArtifactUpdateEvent creates an artifact update event for streaming
func (as *ArtifactServiceImpl) CreateTaskArtifactUpdateEvent(taskID, contextID string, artifact types.Artifact, append, lastChunk *bool) types.TaskArtifactUpdateEvent {
	return types.TaskArtifactUpdateEvent{
		TaskID:    taskID,
		ContextID: contextID,
		Artifact:  artifact,
		Append:    append,
		LastChunk: lastChunk,
	}
}

// Exists checks if an artifact file exists
func (as *ArtifactServiceImpl) Exists(ctx context.Context, artifactID, filename string) (bool, error) {
	return as.storage.Exists(ctx, artifactID, filename)
}

// Retrieve retrieves an artifact file
func (as *ArtifactServiceImpl) Retrieve(ctx context.Context, artifactID, filename string) (io.ReadCloser, error) {
	return as.storage.Retrieve(ctx, artifactID, filename)
}

// CleanupExpiredArtifacts removes artifacts older than maxAge
func (as *ArtifactServiceImpl) CleanupExpiredArtifacts(ctx context.Context, maxAge time.Duration) (int, error) {
	return as.storage.CleanupExpiredArtifacts(ctx, maxAge)
}

// CleanupOldestArtifacts removes oldest artifacts keeping only maxArtifacts
func (as *ArtifactServiceImpl) CleanupOldestArtifacts(ctx context.Context, maxArtifacts int) (int, error) {
	return as.storage.CleanupOldestArtifacts(ctx, maxArtifacts)
}

// Close closes the artifact service and releases resources
func (as *ArtifactServiceImpl) Close() error {
	if as.storage != nil {
		return as.storage.Close()
	}
	return nil
}
