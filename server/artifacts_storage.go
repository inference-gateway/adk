package server

import (
	"context"
	"io"
)

// ArtifactStorageProvider defines the interface for artifact storage backends
type ArtifactStorageProvider interface {
	// Store stores an artifact and returns its URL for retrieval
	Store(ctx context.Context, artifactID string, filename string, data io.Reader) (string, error)

	// Retrieve retrieves an artifact by its ID and filename
	Retrieve(ctx context.Context, artifactID string, filename string) (io.ReadCloser, error)

	// Delete removes an artifact from storage
	Delete(ctx context.Context, artifactID string, filename string) error

	// Exists checks if an artifact exists in storage
	Exists(ctx context.Context, artifactID string, filename string) (bool, error)

	// GetURL returns the public URL for accessing an artifact
	GetURL(artifactID string, filename string) string

	// Close closes the storage provider and cleans up resources
	Close() error
}

// ArtifactMetadata holds metadata about stored artifacts
type ArtifactMetadata struct {
	ArtifactID  string `json:"artifact_id"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	UploadedAt  string `json:"uploaded_at"`
}
