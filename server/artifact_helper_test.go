package server

import (
	"encoding/base64"
	"testing"

	"github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifactHelper_CreateTextArtifact(t *testing.T) {
	helper := NewArtifactHelper()
	
	name := "Test Document"
	description := "A test text artifact"
	text := "Hello, World!"
	
	artifact := helper.CreateTextArtifact(name, description, text)
	
	assert.NotEmpty(t, artifact.ArtifactID)
	assert.Equal(t, name, *artifact.Name)
	assert.Equal(t, description, *artifact.Description)
	require.Len(t, artifact.Parts, 1)
	
	textPart, ok := artifact.Parts[0].(types.TextPart)
	require.True(t, ok)
	assert.Equal(t, "text", textPart.Kind)
	assert.Equal(t, text, textPart.Text)
}

func TestArtifactHelper_CreateFileArtifactFromBytes(t *testing.T) {
	helper := NewArtifactHelper()
	
	name := "Test File"
	description := "A test file artifact"
	filename := "test.txt"
	data := []byte("Hello, World!")
	mimeType := "text/plain"
	
	artifact := helper.CreateFileArtifactFromBytes(name, description, filename, data, &mimeType)
	
	assert.NotEmpty(t, artifact.ArtifactID)
	assert.Equal(t, name, *artifact.Name)
	assert.Equal(t, description, *artifact.Description)
	require.Len(t, artifact.Parts, 1)
	
	filePart, ok := artifact.Parts[0].(types.FilePart)
	require.True(t, ok)
	assert.Equal(t, "file", filePart.Kind)
	
	fileWithBytes, ok := filePart.File.(types.FileWithBytes)
	require.True(t, ok)
	assert.Equal(t, filename, *fileWithBytes.Name)
	assert.Equal(t, mimeType, *fileWithBytes.MIMEType)
	
	decodedData, err := base64.StdEncoding.DecodeString(fileWithBytes.Bytes)
	require.NoError(t, err)
	assert.Equal(t, data, decodedData)
}

func TestArtifactHelper_CreateFileArtifactFromURI(t *testing.T) {
	helper := NewArtifactHelper()
	
	name := "Remote File"
	description := "A remote file artifact"
	filename := "remote.pdf"
	uri := "https://example.com/file.pdf"
	mimeType := "application/pdf"
	
	artifact := helper.CreateFileArtifactFromURI(name, description, filename, uri, &mimeType)
	
	assert.NotEmpty(t, artifact.ArtifactID)
	assert.Equal(t, name, *artifact.Name)
	assert.Equal(t, description, *artifact.Description)
	require.Len(t, artifact.Parts, 1)
	
	filePart, ok := artifact.Parts[0].(types.FilePart)
	require.True(t, ok)
	assert.Equal(t, "file", filePart.Kind)
	
	fileWithURI, ok := filePart.File.(types.FileWithUri)
	require.True(t, ok)
	assert.Equal(t, filename, *fileWithURI.Name)
	assert.Equal(t, mimeType, *fileWithURI.MIMEType)
	assert.Equal(t, uri, fileWithURI.URI)
}

func TestArtifactHelper_CreateDataArtifact(t *testing.T) {
	helper := NewArtifactHelper()
	
	name := "Test Data"
	description := "A test data artifact"
	data := map[string]any{
		"key1": "value1",
		"key2": 42,
		"key3": []string{"a", "b", "c"},
	}
	
	artifact := helper.CreateDataArtifact(name, description, data)
	
	assert.NotEmpty(t, artifact.ArtifactID)
	assert.Equal(t, name, *artifact.Name)
	assert.Equal(t, description, *artifact.Description)
	require.Len(t, artifact.Parts, 1)
	
	dataPart, ok := artifact.Parts[0].(types.DataPart)
	require.True(t, ok)
	assert.Equal(t, "data", dataPart.Kind)
	assert.Equal(t, data, dataPart.Data)
}

func TestArtifactHelper_CreateMultiPartArtifact(t *testing.T) {
	helper := NewArtifactHelper()
	
	name := "Multi-part Artifact"
	description := "An artifact with multiple parts"
	
	parts := []types.Part{
		types.TextPart{
			Kind: "text",
			Text: "Some text content",
		},
		types.DataPart{
			Kind: "data",
			Data: map[string]any{"key": "value"},
		},
	}
	
	artifact := helper.CreateMultiPartArtifact(name, description, parts)
	
	assert.NotEmpty(t, artifact.ArtifactID)
	assert.Equal(t, name, *artifact.Name)
	assert.Equal(t, description, *artifact.Description)
	assert.Len(t, artifact.Parts, 2)
}

func TestArtifactHelper_AddArtifactToTask(t *testing.T) {
	helper := NewArtifactHelper()
	
	task := &types.Task{
		ID: "test-task",
	}
	
	artifact := helper.CreateTextArtifact("Test", "Test artifact", "content")
	
	helper.AddArtifactToTask(task, artifact)
	
	assert.Len(t, task.Artifacts, 1)
	assert.Equal(t, artifact.ArtifactID, task.Artifacts[0].ArtifactID)
}

func TestArtifactHelper_GetArtifactByID(t *testing.T) {
	helper := NewArtifactHelper()
	
	artifact1 := helper.CreateTextArtifact("Test 1", "First artifact", "content1")
	artifact2 := helper.CreateTextArtifact("Test 2", "Second artifact", "content2")
	
	task := &types.Task{
		ID: "test-task",
		Artifacts: []types.Artifact{artifact1, artifact2},
	}
	
	// Test finding existing artifact
	found, exists := helper.GetArtifactByID(task, artifact1.ArtifactID)
	assert.True(t, exists)
	assert.Equal(t, artifact1.ArtifactID, found.ArtifactID)
	
	// Test not finding non-existent artifact
	_, exists = helper.GetArtifactByID(task, "non-existent")
	assert.False(t, exists)
}

func TestArtifactHelper_GetArtifactsByType(t *testing.T) {
	helper := NewArtifactHelper()
	
	textArtifact := helper.CreateTextArtifact("Text", "Text artifact", "content")
	dataArtifact := helper.CreateDataArtifact("Data", "Data artifact", map[string]any{"key": "value"})
	
	task := &types.Task{
		ID: "test-task",
		Artifacts: []types.Artifact{textArtifact, dataArtifact},
	}
	
	// Get text artifacts
	textArtifacts := helper.GetArtifactsByType(task, "text")
	assert.Len(t, textArtifacts, 1)
	assert.Equal(t, textArtifact.ArtifactID, textArtifacts[0].ArtifactID)
	
	// Get data artifacts
	dataArtifacts := helper.GetArtifactsByType(task, "data")
	assert.Len(t, dataArtifacts, 1)
	assert.Equal(t, dataArtifact.ArtifactID, dataArtifacts[0].ArtifactID)
	
	// Get non-existent type
	fileArtifacts := helper.GetArtifactsByType(task, "file")
	assert.Len(t, fileArtifacts, 0)
}

func TestArtifactHelper_ValidateArtifact(t *testing.T) {
	helper := NewArtifactHelper()
	
	// Valid artifact
	validArtifact := helper.CreateTextArtifact("Valid", "Valid artifact", "content")
	err := helper.ValidateArtifact(validArtifact)
	assert.NoError(t, err)
	
	// Invalid artifact - empty ID
	invalidArtifact := types.Artifact{
		ArtifactID: "",
		Parts:      []types.Part{types.TextPart{Kind: "text", Text: "content"}},
	}
	err = helper.ValidateArtifact(invalidArtifact)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty artifactId")
	
	// Invalid artifact - no parts
	invalidArtifact2 := types.Artifact{
		ArtifactID: "test-id",
		Parts:      []types.Part{},
	}
	err = helper.ValidateArtifact(invalidArtifact2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one part")
}

func TestArtifactHelper_ValidatePart(t *testing.T) {
	helper := NewArtifactHelper()
	
	// Valid text part
	textPart := types.TextPart{Kind: "text", Text: "content"}
	err := helper.validatePart(textPart)
	assert.NoError(t, err)
	
	// Invalid text part - wrong kind
	invalidTextPart := types.TextPart{Kind: "wrong", Text: "content"}
	err = helper.validatePart(invalidTextPart)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must have kind 'text'")
	
	// Invalid text part - empty text
	emptyTextPart := types.TextPart{Kind: "text", Text: ""}
	err = helper.validatePart(emptyTextPart)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty text content")
	
	// Valid file part
	filePart := types.FilePart{
		Kind: "file",
		File: types.FileWithBytes{Bytes: "dGVzdA==", Name: stringPtr("test.txt")},
	}
	err = helper.validatePart(filePart)
	assert.NoError(t, err)
	
	// Invalid file part - nil file
	invalidFilePart := types.FilePart{Kind: "file", File: nil}
	err = helper.validatePart(invalidFilePart)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-nil file content")
	
	// Valid data part
	dataPart := types.DataPart{
		Kind: "data",
		Data: map[string]any{"key": "value"},
	}
	err = helper.validatePart(dataPart)
	assert.NoError(t, err)
	
	// Invalid data part - nil data
	invalidDataPart := types.DataPart{Kind: "data", Data: nil}
	err = helper.validatePart(invalidDataPart)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-nil data content")
}

func TestArtifactHelper_GetMimeTypeFromExtension(t *testing.T) {
	helper := NewArtifactHelper()
	
	tests := []struct {
		filename     string
		expectedType string
	}{
		{"test.txt", "text/plain"},
		{"data.json", "application/json"},
		{"document.pdf", "application/pdf"},
		{"image.png", "image/png"},
		{"photo.jpg", "image/jpeg"},
		{"animation.gif", "image/gif"},
		{"page.html", "text/html"},
		{"unknown.xyz", "application/octet-stream"},
		{"noextension", "application/octet-stream"},
	}
	
	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			mimeType := helper.GetMimeTypeFromExtension(test.filename)
			require.NotNil(t, mimeType)
			assert.Equal(t, test.expectedType, *mimeType)
		})
	}
}

func TestArtifactHelper_CreateTaskArtifactUpdateEvent(t *testing.T) {
	helper := NewArtifactHelper()
	
	artifact := helper.CreateTextArtifact("Test", "Test artifact", "content")
	taskID := "task-123"
	contextID := "context-456"
	append := true
	lastChunk := false
	
	event := helper.CreateTaskArtifactUpdateEvent(
		taskID, contextID, artifact, &append, &lastChunk,
	)
	
	assert.Equal(t, "artifact-update", event.Kind)
	assert.Equal(t, taskID, event.TaskID)
	assert.Equal(t, contextID, event.ContextID)
	assert.Equal(t, artifact.ArtifactID, event.Artifact.ArtifactID)
	assert.Equal(t, &append, event.Append)
	assert.Equal(t, &lastChunk, event.LastChunk)
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
