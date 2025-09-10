package client

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifactHelper_ExtractTaskFromResponse(t *testing.T) {
	helper := NewArtifactHelper()
	
	task := types.Task{
		ID:        "task-123",
		ContextID: "context-456",
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
		},
		Artifacts: []types.Artifact{
			{
				ArtifactID: "artifact-1",
				Name:       stringPtr("Test Artifact"),
				Parts: []types.Part{
					types.TextPart{Kind: "text", Text: "Hello, World!"},
				},
			},
		},
	}
	
	taskBytes, err := json.Marshal(task)
	require.NoError(t, err)
	
	response := &types.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      "req-1",
		Result:  taskBytes,
	}
	
	extractedTask, err := helper.ExtractTaskFromResponse(response)
	require.NoError(t, err)
	assert.Equal(t, task.ID, extractedTask.ID)
	assert.Equal(t, task.ContextID, extractedTask.ContextID)
	assert.Len(t, extractedTask.Artifacts, 1)
}

func TestArtifactHelper_ExtractTaskFromResponse_NilResponse(t *testing.T) {
	helper := NewArtifactHelper()
	
	_, err := helper.ExtractTaskFromResponse(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "response or result is nil")
}

func TestArtifactHelper_ExtractArtifactsFromTask(t *testing.T) {
	helper := NewArtifactHelper()
	
	artifacts := []types.Artifact{
		{
			ArtifactID: "artifact-1",
			Name:       stringPtr("Artifact 1"),
			Parts: []types.Part{
				types.TextPart{Kind: "text", Text: "Content 1"},
			},
		},
		{
			ArtifactID: "artifact-2",
			Name:       stringPtr("Artifact 2"),
			Parts: []types.Part{
				types.TextPart{Kind: "text", Text: "Content 2"},
			},
		},
	}
	
	task := &types.Task{
		ID:        "task-123",
		Artifacts: artifacts,
	}
	
	extractedArtifacts := helper.ExtractArtifactsFromTask(task)
	assert.Len(t, extractedArtifacts, 2)
	assert.Equal(t, artifacts[0].ArtifactID, extractedArtifacts[0].ArtifactID)
	assert.Equal(t, artifacts[1].ArtifactID, extractedArtifacts[1].ArtifactID)
}

func TestArtifactHelper_ExtractArtifactsFromTask_EmptyTask(t *testing.T) {
	helper := NewArtifactHelper()
	
	// Nil task
	artifacts := helper.ExtractArtifactsFromTask(nil)
	assert.Len(t, artifacts, 0)
	
	// Task with nil artifacts
	task := &types.Task{ID: "task-123"}
	artifacts = helper.ExtractArtifactsFromTask(task)
	assert.Len(t, artifacts, 0)
}

func TestArtifactHelper_GetArtifactByID(t *testing.T) {
	helper := NewArtifactHelper()
	
	artifact1 := types.Artifact{
		ArtifactID: "artifact-1",
		Name:       stringPtr("Artifact 1"),
	}
	artifact2 := types.Artifact{
		ArtifactID: "artifact-2",
		Name:       stringPtr("Artifact 2"),
	}
	
	task := &types.Task{
		Artifacts: []types.Artifact{artifact1, artifact2},
	}
	
	// Test finding existing artifact
	found, exists := helper.GetArtifactByID(task, "artifact-1")
	assert.True(t, exists)
	assert.Equal(t, "artifact-1", found.ArtifactID)
	
	// Test not finding non-existent artifact
	_, exists = helper.GetArtifactByID(task, "non-existent")
	assert.False(t, exists)
}

func TestArtifactHelper_GetArtifactsByType(t *testing.T) {
	helper := NewArtifactHelper()
	
	textArtifact := types.Artifact{
		ArtifactID: "text-artifact",
		Parts: []types.Part{
			types.TextPart{Kind: "text", Text: "Hello"},
		},
	}
	
	fileArtifact := types.Artifact{
		ArtifactID: "file-artifact",
		Parts: []types.Part{
			types.FilePart{
				Kind: "file",
				File: types.FileWithBytes{Bytes: "dGVzdA=="},
			},
		},
	}
	
	dataArtifact := types.Artifact{
		ArtifactID: "data-artifact",
		Parts: []types.Part{
			types.DataPart{
				Kind: "data",
				Data: map[string]any{"key": "value"},
			},
		},
	}
	
	task := &types.Task{
		Artifacts: []types.Artifact{textArtifact, fileArtifact, dataArtifact},
	}
	
	// Get text artifacts
	textArtifacts := helper.GetArtifactsByType(task, "text")
	assert.Len(t, textArtifacts, 1)
	assert.Equal(t, "text-artifact", textArtifacts[0].ArtifactID)
	
	// Get file artifacts
	fileArtifacts := helper.GetArtifactsByType(task, "file")
	assert.Len(t, fileArtifacts, 1)
	assert.Equal(t, "file-artifact", fileArtifacts[0].ArtifactID)
	
	// Get data artifacts
	dataArtifacts := helper.GetArtifactsByType(task, "data")
	assert.Len(t, dataArtifacts, 1)
	assert.Equal(t, "data-artifact", dataArtifacts[0].ArtifactID)
}

func TestArtifactHelper_GetTextArtifacts(t *testing.T) {
	helper := NewArtifactHelper()
	
	textArtifact := types.Artifact{
		ArtifactID: "text-artifact",
		Parts: []types.Part{
			types.TextPart{Kind: "text", Text: "Hello"},
		},
	}
	
	nonTextArtifact := types.Artifact{
		ArtifactID: "data-artifact",
		Parts: []types.Part{
			types.DataPart{Kind: "data", Data: map[string]any{"key": "value"}},
		},
	}
	
	task := &types.Task{
		Artifacts: []types.Artifact{textArtifact, nonTextArtifact},
	}
	
	textArtifacts := helper.GetTextArtifacts(task)
	assert.Len(t, textArtifacts, 1)
	assert.Equal(t, "text-artifact", textArtifacts[0].ArtifactID)
}

func TestArtifactHelper_ExtractTextFromArtifact(t *testing.T) {
	helper := NewArtifactHelper()
	
	artifact := types.Artifact{
		ArtifactID: "multi-text-artifact",
		Parts: []types.Part{
			types.TextPart{Kind: "text", Text: "First text"},
			types.TextPart{Kind: "text", Text: "Second text"},
			types.DataPart{Kind: "data", Data: map[string]any{"key": "value"}}, // Should be ignored
		},
	}
	
	texts := helper.ExtractTextFromArtifact(&artifact)
	assert.Len(t, texts, 2)
	assert.Equal(t, "First text", texts[0])
	assert.Equal(t, "Second text", texts[1])
}

func TestArtifactHelper_ExtractFileDataFromArtifact(t *testing.T) {
	helper := NewArtifactHelper()
	
	testData := []byte("Hello, World!")
	encodedData := base64.StdEncoding.EncodeToString(testData)
	fileName := "test.txt"
	mimeType := "text/plain"
	
	artifact := types.Artifact{
		ArtifactID: "file-artifact",
		Parts: []types.Part{
			types.FilePart{
				Kind: "file",
				File: types.FileWithBytes{
					Name:     &fileName,
					MIMEType: &mimeType,
					Bytes:    encodedData,
				},
			},
		},
	}
	
	files, err := helper.ExtractFileDataFromArtifact(&artifact)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	
	file := files[0]
	assert.Equal(t, fileName, *file.Name)
	assert.Equal(t, mimeType, *file.MIMEType)
	assert.Equal(t, testData, file.Data)
	assert.True(t, file.IsDataFile())
	assert.False(t, file.IsURIFile())
}

func TestArtifactHelper_ExtractFileDataFromArtifact_URI(t *testing.T) {
	helper := NewArtifactHelper()
	
	fileName := "remote.pdf"
	uri := "https://example.com/file.pdf"
	mimeType := "application/pdf"
	
	artifact := types.Artifact{
		ArtifactID: "uri-artifact",
		Parts: []types.Part{
			types.FilePart{
				Kind: "file",
				File: types.FileWithUri{
					Name:     &fileName,
					MIMEType: &mimeType,
					URI:      uri,
				},
			},
		},
	}
	
	files, err := helper.ExtractFileDataFromArtifact(&artifact)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	
	file := files[0]
	assert.Equal(t, fileName, *file.Name)
	assert.Equal(t, mimeType, *file.MIMEType)
	assert.Equal(t, uri, *file.URI)
	assert.False(t, file.IsDataFile())
	assert.True(t, file.IsURIFile())
}

func TestArtifactHelper_ExtractDataFromArtifact(t *testing.T) {
	helper := NewArtifactHelper()
	
	data1 := map[string]any{"key1": "value1", "count": 42}
	data2 := map[string]any{"key2": "value2", "items": []string{"a", "b", "c"}}
	
	artifact := types.Artifact{
		ArtifactID: "data-artifact",
		Parts: []types.Part{
			types.DataPart{Kind: "data", Data: data1},
			types.DataPart{Kind: "data", Data: data2},
			types.TextPart{Kind: "text", Text: "Should be ignored"},
		},
	}
	
	dataList := helper.ExtractDataFromArtifact(&artifact)
	assert.Len(t, dataList, 2)
	assert.Equal(t, data1, dataList[0])
	assert.Equal(t, data2, dataList[1])
}

func TestArtifactHelper_ExtractArtifactUpdateFromStreamEvent(t *testing.T) {
	helper := NewArtifactHelper()
	
	artifact := types.Artifact{
		ArtifactID: "stream-artifact",
		Parts: []types.Part{
			types.TextPart{Kind: "text", Text: "Streaming content"},
		},
	}
	
	event := types.TaskArtifactUpdateEvent{
		Kind:      "artifact-update",
		TaskID:    "task-123",
		ContextID: "context-456",
		Artifact:  artifact,
	}
	
	// Test with direct event type
	extractedEvent, ok := helper.ExtractArtifactUpdateFromStreamEvent(event)
	assert.True(t, ok)
	assert.Equal(t, event.TaskID, extractedEvent.TaskID)
	assert.Equal(t, event.Artifact.ArtifactID, extractedEvent.Artifact.ArtifactID)
	
	// Test with map representation
	eventMap := map[string]any{
		"kind":      "artifact-update",
		"taskId":    "task-123",
		"contextId": "context-456",
		"artifact": map[string]any{
			"artifactId": "stream-artifact",
			"parts": []any{
				map[string]any{
					"kind": "text",
					"text": "Streaming content",
				},
			},
		},
	}
	
	extractedEvent2, ok := helper.ExtractArtifactUpdateFromStreamEvent(eventMap)
	assert.True(t, ok)
	assert.Equal(t, "task-123", extractedEvent2.TaskID)
	assert.Equal(t, "context-456", extractedEvent2.ContextID)
	assert.Equal(t, "stream-artifact", extractedEvent2.Artifact.ArtifactID)
	
	// Test with non-artifact event
	nonArtifactEvent := map[string]any{
		"kind": "status-update",
		"data": "some status",
	}
	
	_, ok = helper.ExtractArtifactUpdateFromStreamEvent(nonArtifactEvent)
	assert.False(t, ok)
}

func TestArtifactHelper_HasArtifacts(t *testing.T) {
	helper := NewArtifactHelper()
	
	// Task with artifacts
	taskWithArtifacts := &types.Task{
		Artifacts: []types.Artifact{
			{ArtifactID: "artifact-1"},
		},
	}
	assert.True(t, helper.HasArtifacts(taskWithArtifacts))
	
	// Task without artifacts
	taskWithoutArtifacts := &types.Task{}
	assert.False(t, helper.HasArtifacts(taskWithoutArtifacts))
	
	// Nil task
	assert.False(t, helper.HasArtifacts(nil))
}

func TestArtifactHelper_GetArtifactCount(t *testing.T) {
	helper := NewArtifactHelper()
	
	// Task with artifacts
	task := &types.Task{
		Artifacts: []types.Artifact{
			{ArtifactID: "artifact-1"},
			{ArtifactID: "artifact-2"},
			{ArtifactID: "artifact-3"},
		},
	}
	assert.Equal(t, 3, helper.GetArtifactCount(task))
	
	// Task without artifacts
	emptyTask := &types.Task{}
	assert.Equal(t, 0, helper.GetArtifactCount(emptyTask))
	
	// Nil task
	assert.Equal(t, 0, helper.GetArtifactCount(nil))
}

func TestArtifactHelper_GetArtifactSummary(t *testing.T) {
	helper := NewArtifactHelper()
	
	task := &types.Task{
		Artifacts: []types.Artifact{
			{
				ArtifactID: "artifact-1",
				Parts: []types.Part{
					types.TextPart{Kind: "text", Text: "Text 1"},
					types.TextPart{Kind: "text", Text: "Text 2"},
				},
			},
			{
				ArtifactID: "artifact-2",
				Parts: []types.Part{
					types.FilePart{Kind: "file", File: types.FileWithBytes{}},
				},
			},
			{
				ArtifactID: "artifact-3",
				Parts: []types.Part{
					types.DataPart{Kind: "data", Data: map[string]any{}},
					types.DataPart{Kind: "data", Data: map[string]any{}},
				},
			},
		},
	}
	
	summary := helper.GetArtifactSummary(task)
	expected := map[string]int{
		"text": 2,
		"file": 1,
		"data": 2,
	}
	
	assert.Equal(t, expected, summary)
}

func TestArtifactHelper_FilterArtifactsByName(t *testing.T) {
	helper := NewArtifactHelper()
	
	artifacts := []types.Artifact{
		{
			ArtifactID: "artifact-1",
			Name:       stringPtr("User Report"),
		},
		{
			ArtifactID: "artifact-2",
			Name:       stringPtr("System Log"),
		},
		{
			ArtifactID: "artifact-3",
			Name:       stringPtr("User Guide"),
		},
		{
			ArtifactID: "artifact-4",
			Name:       nil, // No name
		},
	}
	
	task := &types.Task{Artifacts: artifacts}
	
	// Search for "user" (case-insensitive)
	userArtifacts := helper.FilterArtifactsByName(task, "user")
	assert.Len(t, userArtifacts, 2)
	
	// Check that both "User Report" and "User Guide" are found
	foundNames := make([]string, len(userArtifacts))
	for i, artifact := range userArtifacts {
		foundNames[i] = *artifact.Name
	}
	assert.Contains(t, foundNames, "User Report")
	assert.Contains(t, foundNames, "User Guide")
	
	// Search for "log"
	logArtifacts := helper.FilterArtifactsByName(task, "log")
	assert.Len(t, logArtifacts, 1)
	assert.Equal(t, "System Log", *logArtifacts[0].Name)
	
	// Search for non-existent pattern
	noMatches := helper.FilterArtifactsByName(task, "xyz")
	assert.Len(t, noMatches, 0)
}

func TestFileData_Methods(t *testing.T) {
	// Data file
	dataFile := FileData{
		Name:     stringPtr("test.txt"),
		MIMEType: stringPtr("text/plain"),
		Data:     []byte("test content"),
	}
	
	assert.True(t, dataFile.IsDataFile())
	assert.False(t, dataFile.IsURIFile())
	assert.Equal(t, "test.txt", dataFile.GetFileName())
	assert.Equal(t, "text/plain", dataFile.GetMIMEType())
	
	// URI file
	uri := "https://example.com/file.pdf"
	uriFile := FileData{
		Name:     stringPtr("remote.pdf"),
		MIMEType: stringPtr("application/pdf"),
		URI:      &uri,
	}
	
	assert.False(t, uriFile.IsDataFile())
	assert.True(t, uriFile.IsURIFile())
	assert.Equal(t, "remote.pdf", uriFile.GetFileName())
	assert.Equal(t, "application/pdf", uriFile.GetMIMEType())
	
	// File with defaults
	defaultFile := FileData{}
	assert.Equal(t, "unnamed_file", defaultFile.GetFileName())
	assert.Equal(t, "application/octet-stream", defaultFile.GetMIMEType())
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}