package server

import (
	"encoding/base64"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/inference-gateway/adk/types"
)

// ArtifactHelper provides utility functions for working with artifacts in the A2A protocol
type ArtifactHelper struct{}

// NewArtifactHelper creates a new artifact helper instance
func NewArtifactHelper() *ArtifactHelper {
	return &ArtifactHelper{}
}

// CreateTextArtifact creates a text artifact with the given content
func (ah *ArtifactHelper) CreateTextArtifact(name, description, text string) types.Artifact {
	return types.Artifact{
		ArtifactID:  uuid.New().String(),
		Name:        &name,
		Description: &description,
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: text,
			},
		},
	}
}

// CreateFileArtifactFromBytes creates a file artifact from byte data
func (ah *ArtifactHelper) CreateFileArtifactFromBytes(name, description, filename string, data []byte, mimeType *string) types.Artifact {
	encodedData := base64.StdEncoding.EncodeToString(data)

	fileWithBytes := types.FileWithBytes{
		Name:     &filename,
		MIMEType: mimeType,
		Bytes:    encodedData,
	}

	return types.Artifact{
		ArtifactID:  uuid.New().String(),
		Name:        &name,
		Description: &description,
		Parts: []types.Part{
			types.FilePart{
				Kind: "file",
				File: fileWithBytes,
			},
		},
	}
}

// CreateFileArtifactFromURI creates a file artifact from a URI reference
func (ah *ArtifactHelper) CreateFileArtifactFromURI(name, description, filename, uri string, mimeType *string) types.Artifact {
	fileWithURI := types.FileWithUri{
		Name:     &filename,
		MIMEType: mimeType,
		URI:      uri,
	}

	return types.Artifact{
		ArtifactID:  uuid.New().String(),
		Name:        &name,
		Description: &description,
		Parts: []types.Part{
			types.FilePart{
				Kind: "file",
				File: fileWithURI,
			},
		},
	}
}

// CreateDataArtifact creates a structured data artifact
func (ah *ArtifactHelper) CreateDataArtifact(name, description string, data map[string]any) types.Artifact {
	return types.Artifact{
		ArtifactID:  uuid.New().String(),
		Name:        &name,
		Description: &description,
		Parts: []types.Part{
			types.DataPart{
				Kind: "data",
				Data: data,
			},
		},
	}
}

// CreateMultiPartArtifact creates an artifact with multiple parts (text, files, data)
func (ah *ArtifactHelper) CreateMultiPartArtifact(name, description string, parts []types.Part) types.Artifact {
	return types.Artifact{
		ArtifactID:  uuid.New().String(),
		Name:        &name,
		Description: &description,
		Parts:       parts,
	}
}

// AddArtifactToTask adds an artifact to a task's artifact collection
func (ah *ArtifactHelper) AddArtifactToTask(task *types.Task, artifact types.Artifact) {
	if task.Artifacts == nil {
		task.Artifacts = []types.Artifact{}
	}
	task.Artifacts = append(task.Artifacts, artifact)
}

// AddArtifactsToTask adds multiple artifacts to a task's artifact collection
func (ah *ArtifactHelper) AddArtifactsToTask(task *types.Task, artifacts []types.Artifact) {
	if task.Artifacts == nil {
		task.Artifacts = []types.Artifact{}
	}
	task.Artifacts = append(task.Artifacts, artifacts...)
}

// GetArtifactByID retrieves an artifact from a task by its ID
func (ah *ArtifactHelper) GetArtifactByID(task *types.Task, artifactID string) (*types.Artifact, bool) {
	for i, artifact := range task.Artifacts {
		if artifact.ArtifactID == artifactID {
			return &task.Artifacts[i], true
		}
	}
	return nil, false
}

// GetArtifactsByType retrieves all artifacts from a task that contain parts of a specific type
func (ah *ArtifactHelper) GetArtifactsByType(task *types.Task, partKind string) []types.Artifact {
	var matchingArtifacts []types.Artifact

	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			switch p := part.(type) {
			case types.TextPart:
				if p.Kind == partKind {
					matchingArtifacts = append(matchingArtifacts, artifact)
					break
				}
			case types.FilePart:
				if p.Kind == partKind {
					matchingArtifacts = append(matchingArtifacts, artifact)
					break
				}
			case types.DataPart:
				if p.Kind == partKind {
					matchingArtifacts = append(matchingArtifacts, artifact)
					break
				}
			case map[string]any:
				if kind, ok := p["kind"].(string); ok && kind == partKind {
					matchingArtifacts = append(matchingArtifacts, artifact)
					break
				}
			}
		}
	}

	return matchingArtifacts
}

// ValidateArtifact validates that an artifact conforms to the A2A protocol specification
func (ah *ArtifactHelper) ValidateArtifact(artifact types.Artifact) error {
	if artifact.ArtifactID == "" {
		return fmt.Errorf("artifact must have a non-empty artifactId")
	}

	if len(artifact.Parts) == 0 {
		return fmt.Errorf("artifact must contain at least one part")
	}

	for i, part := range artifact.Parts {
		if err := ah.validatePart(part); err != nil {
			return fmt.Errorf("invalid part at index %d: %w", i, err)
		}
	}

	return nil
}

// validatePart validates a single part of an artifact
func (ah *ArtifactHelper) validatePart(part types.Part) error {
	switch p := part.(type) {
	case types.TextPart:
		if p.Kind != "text" {
			return fmt.Errorf("text part must have kind 'text', got '%s'", p.Kind)
		}
		if p.Text == "" {
			return fmt.Errorf("text part must have non-empty text content")
		}
	case types.FilePart:
		if p.Kind != "file" {
			return fmt.Errorf("file part must have kind 'file', got '%s'", p.Kind)
		}
		if p.File == nil {
			return fmt.Errorf("file part must have non-nil file content")
		}
	case types.DataPart:
		if p.Kind != "data" {
			return fmt.Errorf("data part must have kind 'data', got '%s'", p.Kind)
		}
		if p.Data == nil {
			return fmt.Errorf("data part must have non-nil data content")
		}
	case map[string]any:
		kind, exists := p["kind"].(string)
		if !exists {
			return fmt.Errorf("part must have a 'kind' field")
		}
		if kind == "" {
			return fmt.Errorf("part kind cannot be empty")
		}
	default:
		return fmt.Errorf("unsupported part type: %T", part)
	}

	return nil
}

// GetMimeTypeFromExtension returns a MIME type based on file extension
func (ah *ArtifactHelper) GetMimeTypeFromExtension(filename string) *string {
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
func (ah *ArtifactHelper) CreateTaskArtifactUpdateEvent(taskID, contextID string, artifact types.Artifact, append, lastChunk *bool) types.TaskArtifactUpdateEvent {
	return types.TaskArtifactUpdateEvent{
		Kind:      "artifact-update",
		TaskID:    taskID,
		ContextID: contextID,
		Artifact:  artifact,
		Append:    append,
		LastChunk: lastChunk,
	}
}
