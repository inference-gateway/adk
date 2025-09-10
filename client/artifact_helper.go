package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/inference-gateway/adk/types"
)

// ArtifactHelper provides utility functions for working with artifacts in client responses
type ArtifactHelper struct{}

// NewArtifactHelper creates a new client-side artifact helper instance
func NewArtifactHelper() *ArtifactHelper {
	return &ArtifactHelper{}
}

// ExtractTaskFromResponse extracts a task from a JSON-RPC response
func (ah *ArtifactHelper) ExtractTaskFromResponse(response *types.JSONRPCSuccessResponse) (*types.Task, error) {
	if response == nil || response.Result == nil {
		return nil, fmt.Errorf("response or result is nil")
	}

	// Handle both []byte and json.RawMessage
	var taskBytes []byte
	switch result := response.Result.(type) {
	case []byte:
		taskBytes = result
	case json.RawMessage:
		taskBytes = result
	default:
		// Try to marshal the interface{} to bytes
		var err error
		taskBytes, err = json.Marshal(response.Result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result to bytes: %w", err)
		}
	}

	var task types.Task
	if err := json.Unmarshal(taskBytes, &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task from response: %w", err)
	}

	return &task, nil
}

// ExtractArtifactsFromTask extracts all artifacts from a task
func (ah *ArtifactHelper) ExtractArtifactsFromTask(task *types.Task) []types.Artifact {
	if task == nil || task.Artifacts == nil {
		return []types.Artifact{}
	}
	return task.Artifacts
}

// GetArtifactByID retrieves a specific artifact by its ID from a task
func (ah *ArtifactHelper) GetArtifactByID(task *types.Task, artifactID string) (*types.Artifact, bool) {
	if task == nil {
		return nil, false
	}
	for i, artifact := range task.Artifacts {
		if artifact.ArtifactID == artifactID {
			return &task.Artifacts[i], true
		}
	}
	return nil, false
}

// GetArtifactsByType retrieves all artifacts containing parts of a specific type
func (ah *ArtifactHelper) GetArtifactsByType(task *types.Task, partKind string) []types.Artifact {
	matchingArtifacts := make([]types.Artifact, 0)

	if task == nil {
		return matchingArtifacts
	}

	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			if ah.isPartOfKind(part, partKind) {
				matchingArtifacts = append(matchingArtifacts, artifact)
				break
			}
		}
	}

	return matchingArtifacts
}

// GetTextArtifacts retrieves all artifacts that contain text parts
func (ah *ArtifactHelper) GetTextArtifacts(task *types.Task) []types.Artifact {
	return ah.GetArtifactsByType(task, "text")
}

// GetFileArtifacts retrieves all artifacts that contain file parts
func (ah *ArtifactHelper) GetFileArtifacts(task *types.Task) []types.Artifact {
	return ah.GetArtifactsByType(task, "file")
}

// GetDataArtifacts retrieves all artifacts that contain data parts
func (ah *ArtifactHelper) GetDataArtifacts(task *types.Task) []types.Artifact {
	return ah.GetArtifactsByType(task, "data")
}

// ExtractTextFromArtifact extracts all text content from an artifact
func (ah *ArtifactHelper) ExtractTextFromArtifact(artifact *types.Artifact) []string {
	texts := make([]string, 0)

	if artifact == nil {
		return texts
	}

	for _, part := range artifact.Parts {
		switch p := part.(type) {
		case types.TextPart:
			if p.Kind == "text" {
				texts = append(texts, p.Text)
			}
		case map[string]any:
			if kind, ok := p["kind"].(string); ok && kind == "text" {
				if text, exists := p["text"].(string); exists {
					texts = append(texts, text)
				}
			}
		}
	}

	return texts
}

// ExtractFileDataFromArtifact extracts file data from an artifact
func (ah *ArtifactHelper) ExtractFileDataFromArtifact(artifact *types.Artifact) ([]FileData, error) {
	files := make([]FileData, 0)

	if artifact == nil {
		return files, nil
	}

	for _, part := range artifact.Parts {
		switch p := part.(type) {
		case types.FilePart:
			if p.Kind == "file" {
				fileData, err := ah.extractFileFromPart(p)
				if err != nil {
					return nil, fmt.Errorf("failed to extract file from part: %w", err)
				}
				files = append(files, fileData)
			}
		case map[string]any:
			if kind, ok := p["kind"].(string); ok && kind == "file" {
				fileData, err := ah.extractFileFromMap(p)
				if err != nil {
					return nil, fmt.Errorf("failed to extract file from map part: %w", err)
				}
				files = append(files, fileData)
			}
		}
	}

	return files, nil
}

// ExtractDataFromArtifact extracts structured data from an artifact
func (ah *ArtifactHelper) ExtractDataFromArtifact(artifact *types.Artifact) []map[string]any {
	dataList := make([]map[string]any, 0)

	if artifact == nil {
		return dataList
	}

	for _, part := range artifact.Parts {
		switch p := part.(type) {
		case types.DataPart:
			if p.Kind == "data" {
				dataList = append(dataList, p.Data)
			}
		case map[string]any:
			if kind, ok := p["kind"].(string); ok && kind == "data" {
				if data, exists := p["data"].(map[string]any); exists {
					dataList = append(dataList, data)
				}
			}
		}
	}

	return dataList
}

// FileData represents extracted file information from an artifact
type FileData struct {
	Name     *string
	MIMEType *string
	Data     []byte  // For FileWithBytes
	URI      *string // For FileWithUri
}

// IsDataFile returns true if this file contains data (bytes), false if it's URI-based
func (fd *FileData) IsDataFile() bool {
	return len(fd.Data) > 0
}

// IsURIFile returns true if this file is URI-based, false if it contains data
func (fd *FileData) IsURIFile() bool {
	return fd.URI != nil && *fd.URI != ""
}

// GetFileName returns the file name or a default if none is set
func (fd *FileData) GetFileName() string {
	if fd.Name != nil && *fd.Name != "" {
		return *fd.Name
	}
	return "unnamed_file"
}

// GetMIMEType returns the MIME type or a default if none is set
func (fd *FileData) GetMIMEType() string {
	if fd.MIMEType != nil && *fd.MIMEType != "" {
		return *fd.MIMEType
	}
	return "application/octet-stream"
}

// isPartOfKind checks if a part is of a specific kind
func (ah *ArtifactHelper) isPartOfKind(part types.Part, kind string) bool {
	switch p := part.(type) {
	case types.TextPart:
		return p.Kind == kind
	case types.FilePart:
		return p.Kind == kind
	case types.DataPart:
		return p.Kind == kind
	case map[string]any:
		if partKind, ok := p["kind"].(string); ok {
			return partKind == kind
		}
	}
	return false
}

// extractFileFromPart extracts file data from a FilePart
func (ah *ArtifactHelper) extractFileFromPart(filePart types.FilePart) (FileData, error) {
	switch file := filePart.File.(type) {
	case types.FileWithBytes:
		data, err := base64.StdEncoding.DecodeString(file.Bytes)
		if err != nil {
			return FileData{}, fmt.Errorf("failed to decode base64 file data: %w", err)
		}
		return FileData{
			Name:     file.Name,
			MIMEType: file.MIMEType,
			Data:     data,
		}, nil
	case types.FileWithUri:
		return FileData{
			Name:     file.Name,
			MIMEType: file.MIMEType,
			URI:      &file.URI,
		}, nil
	default:
		return FileData{}, fmt.Errorf("unsupported file type: %T", file)
	}
}

// extractFileFromMap extracts file data from a map representation
func (ah *ArtifactHelper) extractFileFromMap(fileMap map[string]any) (FileData, error) {
	fileData := FileData{}

	// Extract file content from the "file" field
	fileContent, exists := fileMap["file"]
	if !exists {
		return FileData{}, fmt.Errorf("file map missing 'file' field")
	}

	fileContentMap, ok := fileContent.(map[string]any)
	if !ok {
		return FileData{}, fmt.Errorf("file content is not a map")
	}

	// Extract name if present
	if name, exists := fileContentMap["name"].(string); exists {
		fileData.Name = &name
	}

	// Extract MIME type if present
	if mimeType, exists := fileContentMap["mimeType"].(string); exists {
		fileData.MIMEType = &mimeType
	}

	// Check if it's a file with bytes
	if bytes, exists := fileContentMap["bytes"].(string); exists {
		data, err := base64.StdEncoding.DecodeString(bytes)
		if err != nil {
			return FileData{}, fmt.Errorf("failed to decode base64 file data: %w", err)
		}
		fileData.Data = data
		return fileData, nil
	}

	// Check if it's a file with URI
	if uri, exists := fileContentMap["uri"].(string); exists {
		fileData.URI = &uri
		return fileData, nil
	}

	return FileData{}, fmt.Errorf("file content contains neither 'bytes' nor 'uri'")
}

// ExtractArtifactUpdateFromStreamEvent extracts an artifact update event from a streaming event
func (ah *ArtifactHelper) ExtractArtifactUpdateFromStreamEvent(eventData any) (*types.TaskArtifactUpdateEvent, bool) {
	switch event := eventData.(type) {
	case types.TaskArtifactUpdateEvent:
		return &event, true
	case map[string]any:
		if kind, exists := event["kind"].(string); exists && kind == "artifact-update" {
			// Convert map to TaskArtifactUpdateEvent
			eventBytes, err := json.Marshal(event)
			if err != nil {
				return nil, false
			}

			var artifactEvent types.TaskArtifactUpdateEvent
			if err := json.Unmarshal(eventBytes, &artifactEvent); err != nil {
				return nil, false
			}

			return &artifactEvent, true
		}
	}
	return nil, false
}

// HasArtifacts returns true if the task contains any artifacts
func (ah *ArtifactHelper) HasArtifacts(task *types.Task) bool {
	return task != nil && len(task.Artifacts) > 0
}

// GetArtifactCount returns the number of artifacts in a task
func (ah *ArtifactHelper) GetArtifactCount(task *types.Task) int {
	if task == nil {
		return 0
	}
	return len(task.Artifacts)
}

// GetArtifactSummary returns a summary of artifacts by type
func (ah *ArtifactHelper) GetArtifactSummary(task *types.Task) map[string]int {
	summary := make(map[string]int)

	if task == nil {
		return summary
	}

	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			switch p := part.(type) {
			case types.TextPart:
				summary[p.Kind]++
			case types.FilePart:
				summary[p.Kind]++
			case types.DataPart:
				summary[p.Kind]++
			case map[string]any:
				if kind, ok := p["kind"].(string); ok {
					summary[kind]++
				}
			}
		}
	}

	return summary
}

// FilterArtifactsByName returns artifacts that match a name pattern (case-insensitive)
func (ah *ArtifactHelper) FilterArtifactsByName(task *types.Task, namePattern string) []types.Artifact {
	matchingArtifacts := make([]types.Artifact, 0)

	if task == nil {
		return matchingArtifacts
	}

	pattern := strings.ToLower(namePattern)

	for _, artifact := range task.Artifacts {
		if artifact.Name != nil && strings.Contains(strings.ToLower(*artifact.Name), pattern) {
			matchingArtifacts = append(matchingArtifacts, artifact)
		}
	}

	return matchingArtifacts
}
