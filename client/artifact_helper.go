package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	types "github.com/inference-gateway/adk/types"
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

	var taskBytes []byte
	switch result := response.Result.(type) {
	case []byte:
		taskBytes = result
	case json.RawMessage:
		taskBytes = result
	default:
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
		if part.Text != nil {
			texts = append(texts, *part.Text)
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
		if part.File != nil {
			fileData, err := ah.extractFileFromPart(*part.File)
			if err != nil {
				return nil, fmt.Errorf("failed to extract file from part: %w", err)
			}
			files = append(files, fileData)
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
		if part.Data != nil {
			dataList = append(dataList, part.Data.Data)
		}
	}

	return dataList
}

// FileData represents extracted file information from an artifact
type FileData struct {
	Name     *string
	MIMEType *string
	Data     []byte
	URI      *string
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
	switch kind {
	case "text":
		return part.Text != nil
	case "file":
		return part.File != nil
	case "data":
		return part.Data != nil
	}
	return false
}

// extractFileFromPart extracts file data from a FilePart
func (ah *ArtifactHelper) extractFileFromPart(filePart types.FilePart) (FileData, error) {
	fileData := FileData{
		Name:     &filePart.Name,
		MIMEType: &filePart.MediaType,
	}

	if filePart.FileWithBytes != nil && *filePart.FileWithBytes != "" {
		data, err := base64.StdEncoding.DecodeString(*filePart.FileWithBytes)
		if err != nil {
			return FileData{}, fmt.Errorf("failed to decode base64 file data: %w", err)
		}
		fileData.Data = data
		return fileData, nil
	}

	if filePart.FileWithURI != nil && *filePart.FileWithURI != "" {
		fileData.URI = filePart.FileWithURI
		return fileData, nil
	}

	return FileData{}, fmt.Errorf("file part contains neither bytes nor URI")
}

// ExtractArtifactUpdateFromStreamEvent extracts an artifact update event from a streaming event
func (ah *ArtifactHelper) ExtractArtifactUpdateFromStreamEvent(eventData any) (*types.TaskArtifactUpdateEvent, bool) {
	switch event := eventData.(type) {
	case types.TaskArtifactUpdateEvent:
		return &event, true
	case map[string]any:
		if kind, exists := event["kind"].(string); exists && kind == "artifact-update" {
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
			if part.Text != nil {
				summary["text"]++
			}
			if part.File != nil {
				summary["file"]++
			}
			if part.Data != nil {
				summary["data"]++
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

// DownloadConfig holds configuration for downloading artifacts
type DownloadConfig struct {
	// OutputDir is the directory where files will be saved (default: current directory)
	OutputDir string
	// HTTPClient is the HTTP client to use for downloads (default: http.DefaultClient)
	HTTPClient *http.Client
	// OverwriteExisting allows overwriting existing files (default: false)
	OverwriteExisting bool
	// OrganizeByArtifactID creates subdirectories by artifact ID to prevent collisions (default: true)
	OrganizeByArtifactID bool
}

// DownloadResult represents the result of a file download
type DownloadResult struct {
	// FileName is the name of the downloaded file
	FileName string
	// FilePath is the full path where the file was saved
	FilePath string
	// BytesWritten is the number of bytes written to disk
	BytesWritten int64
	// Error contains any error that occurred during download
	Error error
}

// DownloadFileData downloads a FileData object to disk
func (ah *ArtifactHelper) DownloadFileData(ctx context.Context, fileData FileData, config *DownloadConfig) (*DownloadResult, error) {
	if config == nil {
		config = &DownloadConfig{
			OutputDir:  ".",
			HTTPClient: http.DefaultClient,
		}
	}

	if config.OutputDir == "" {
		config.OutputDir = "."
	}

	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}

	fileName := fileData.GetFileName()
	filePath := filepath.Join(config.OutputDir, fileName)

	if !config.OverwriteExisting {
		if _, err := os.Stat(filePath); err == nil {
			return nil, fmt.Errorf("file already exists: %s (use OverwriteExisting to allow overwriting)", filePath)
		}
	}

	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	var data []byte
	var err error

	if fileData.IsDataFile() {
		data = fileData.Data
	} else if fileData.IsURIFile() {
		data, err = ah.downloadFromURI(ctx, *fileData.URI, config.HTTPClient)
		if err != nil {
			return &DownloadResult{
				FileName: fileName,
				FilePath: filePath,
				Error:    err,
			}, err
		}
	} else {
		return nil, fmt.Errorf("file data contains neither bytes nor URI")
	}

	bytesWritten, err := ah.writeFile(filePath, data)
	if err != nil {
		return &DownloadResult{
			FileName: fileName,
			FilePath: filePath,
			Error:    err,
		}, err
	}

	return &DownloadResult{
		FileName:     fileName,
		FilePath:     filePath,
		BytesWritten: bytesWritten,
	}, nil
}

// DownloadArtifact downloads all files from an artifact
func (ah *ArtifactHelper) DownloadArtifact(ctx context.Context, artifact *types.Artifact, config *DownloadConfig) ([]*DownloadResult, error) {
	files, err := ah.ExtractFileDataFromArtifact(artifact)
	if err != nil {
		return nil, fmt.Errorf("failed to extract file data: %w", err)
	}

	artifactConfig := config
	if config != nil && config.OrganizeByArtifactID && artifact != nil {
		artifactConfig = &DownloadConfig{
			OutputDir:         filepath.Join(config.OutputDir, artifact.ArtifactID),
			HTTPClient:        config.HTTPClient,
			OverwriteExisting: config.OverwriteExisting,
		}
	}

	results := make([]*DownloadResult, 0, len(files))
	for _, file := range files {
		result, err := ah.DownloadFileData(ctx, file, artifactConfig)
		if err != nil {
			results = append(results, &DownloadResult{
				FileName: file.GetFileName(),
				Error:    err,
			})
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// DownloadAllArtifacts downloads all files from all artifacts in a task
func (ah *ArtifactHelper) DownloadAllArtifacts(ctx context.Context, task *types.Task, config *DownloadConfig) ([]*DownloadResult, error) {
	if !ah.HasArtifacts(task) {
		return []*DownloadResult{}, nil
	}

	if config == nil {
		config = &DownloadConfig{
			OutputDir:            ".",
			OrganizeByArtifactID: true,
		}
	}

	results := make([]*DownloadResult, 0)
	for _, artifact := range task.Artifacts {
		artifactResults, err := ah.DownloadArtifact(ctx, &artifact, config)
		if err != nil {
			return results, fmt.Errorf("failed to download artifact %s: %w", artifact.ArtifactID, err)
		}
		results = append(results, artifactResults...)
	}

	return results, nil
}

// downloadFromURI downloads content from a URI
func (ah *ArtifactHelper) downloadFromURI(ctx context.Context, uri string, client *http.Client) (data []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download from %s: %w", uri, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close response body: %w", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// writeFile writes data to a file and returns the number of bytes written
func (ah *ArtifactHelper) writeFile(filePath string, data []byte) (bytesWritten int64, err error) {
	file, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()

	n, err := file.Write(data)
	if err != nil {
		return 0, fmt.Errorf("failed to write file: %w", err)
	}

	return int64(n), nil
}
