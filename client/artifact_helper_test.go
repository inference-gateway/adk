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

	tests := []struct {
		name       string
		setup      func() *types.JSONRPCSuccessResponse
		wantErr    bool
		errMsg     string
		assertions func(t *testing.T, task *types.Task)
	}{
		{
			name: "success with artifacts",
			setup: func() *types.JSONRPCSuccessResponse {
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
				taskBytes, _ := json.Marshal(task)
				return &types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      "req-1",
					Result:  taskBytes,
				}
			},
			wantErr: false,
			assertions: func(t *testing.T, task *types.Task) {
				assert.Equal(t, "task-123", task.ID)
				assert.Equal(t, "context-456", task.ContextID)
				assert.Len(t, task.Artifacts, 1)
			},
		},
		{
			name:    "nil response",
			setup:   func() *types.JSONRPCSuccessResponse { return nil },
			wantErr: true,
			errMsg:  "response or result is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := tt.setup()
			task, err := helper.ExtractTaskFromResponse(response)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				if tt.assertions != nil {
					tt.assertions(t, task)
				}
			}
		})
	}
}

func TestArtifactHelper_ExtractArtifactsFromTask(t *testing.T) {
	helper := NewArtifactHelper()

	tests := []struct {
		name          string
		task          *types.Task
		expectedCount int
		expectedIDs   []string
	}{
		{
			name: "task with multiple artifacts",
			task: &types.Task{
				ID: "task-123",
				Artifacts: []types.Artifact{
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
				},
			},
			expectedCount: 2,
			expectedIDs:   []string{"artifact-1", "artifact-2"},
		},
		{
			name:          "nil task",
			task:          nil,
			expectedCount: 0,
		},
		{
			name:          "task without artifacts",
			task:          &types.Task{ID: "task-123"},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts := helper.ExtractArtifactsFromTask(tt.task)
			assert.Len(t, artifacts, tt.expectedCount)

			if tt.expectedIDs != nil {
				for i, id := range tt.expectedIDs {
					assert.Equal(t, id, artifacts[i].ArtifactID)
				}
			}
		})
	}
}

func TestArtifactHelper_GetArtifactByID(t *testing.T) {
	helper := NewArtifactHelper()

	task := &types.Task{
		Artifacts: []types.Artifact{
			{ArtifactID: "artifact-1", Name: stringPtr("Artifact 1")},
			{ArtifactID: "artifact-2", Name: stringPtr("Artifact 2")},
		},
	}

	tests := []struct {
		name       string
		task       *types.Task
		artifactID string
		wantExists bool
		wantID     string
	}{
		{
			name:       "existing artifact",
			task:       task,
			artifactID: "artifact-1",
			wantExists: true,
			wantID:     "artifact-1",
		},
		{
			name:       "non-existent artifact",
			task:       task,
			artifactID: "non-existent",
			wantExists: false,
		},
		{
			name:       "nil task",
			task:       nil,
			artifactID: "artifact-1",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, exists := helper.GetArtifactByID(tt.task, tt.artifactID)
			assert.Equal(t, tt.wantExists, exists)
			if tt.wantExists {
				assert.Equal(t, tt.wantID, found.ArtifactID)
			}
		})
	}
}

func TestArtifactHelper_GetArtifactsByType(t *testing.T) {
	helper := NewArtifactHelper()

	task := &types.Task{
		Artifacts: []types.Artifact{
			{
				ArtifactID: "text-artifact",
				Parts: []types.Part{
					types.TextPart{Kind: "text", Text: "Hello"},
				},
			},
			{
				ArtifactID: "file-artifact",
				Parts: []types.Part{
					types.FilePart{Kind: "file", File: types.FileWithBytes{Bytes: "dGVzdA=="}},
				},
			},
			{
				ArtifactID: "data-artifact",
				Parts: []types.Part{
					types.DataPart{Kind: "data", Data: map[string]any{"key": "value"}},
				},
			},
			{
				ArtifactID: "mixed-artifact",
				Parts: []types.Part{
					types.TextPart{Kind: "text", Text: "Mixed"},
					types.DataPart{Kind: "data", Data: map[string]any{}},
				},
			},
		},
	}

	tests := []struct {
		name         string
		task         *types.Task
		artifactType string
		expectedIDs  []string
	}{
		{
			name:         "text artifacts",
			task:         task,
			artifactType: "text",
			expectedIDs:  []string{"text-artifact", "mixed-artifact"},
		},
		{
			name:         "file artifacts",
			task:         task,
			artifactType: "file",
			expectedIDs:  []string{"file-artifact"},
		},
		{
			name:         "data artifacts",
			task:         task,
			artifactType: "data",
			expectedIDs:  []string{"data-artifact", "mixed-artifact"},
		},
		{
			name:         "unknown type",
			task:         task,
			artifactType: "unknown",
			expectedIDs:  []string{},
		},
		{
			name:         "nil task",
			task:         nil,
			artifactType: "text",
			expectedIDs:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts := helper.GetArtifactsByType(tt.task, tt.artifactType)
			assert.Len(t, artifacts, len(tt.expectedIDs))

			for i, expectedID := range tt.expectedIDs {
				assert.Equal(t, expectedID, artifacts[i].ArtifactID)
			}
		})
	}
}

func TestArtifactHelper_GetTextArtifacts(t *testing.T) {
	helper := NewArtifactHelper()

	tests := []struct {
		name        string
		task        *types.Task
		expectedIDs []string
	}{
		{
			name: "task with text and non-text artifacts",
			task: &types.Task{
				Artifacts: []types.Artifact{
					{
						ArtifactID: "text-artifact",
						Parts: []types.Part{
							types.TextPart{Kind: "text", Text: "Hello"},
						},
					},
					{
						ArtifactID: "data-artifact",
						Parts: []types.Part{
							types.DataPart{Kind: "data", Data: map[string]any{"key": "value"}},
						},
					},
				},
			},
			expectedIDs: []string{"text-artifact"},
		},
		{
			name:        "nil task",
			task:        nil,
			expectedIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts := helper.GetTextArtifacts(tt.task)
			assert.Len(t, artifacts, len(tt.expectedIDs))

			for i, expectedID := range tt.expectedIDs {
				assert.Equal(t, expectedID, artifacts[i].ArtifactID)
			}
		})
	}
}

func TestArtifactHelper_ExtractTextFromArtifact(t *testing.T) {
	helper := NewArtifactHelper()

	tests := []struct {
		name          string
		artifact      *types.Artifact
		expectedTexts []string
	}{
		{
			name: "artifact with multiple text parts",
			artifact: &types.Artifact{
				ArtifactID: "multi-text-artifact",
				Parts: []types.Part{
					types.TextPart{Kind: "text", Text: "First text"},
					types.TextPart{Kind: "text", Text: "Second text"},
					types.DataPart{Kind: "data", Data: map[string]any{"key": "value"}},
				},
			},
			expectedTexts: []string{"First text", "Second text"},
		},
		{
			name: "artifact with no text parts",
			artifact: &types.Artifact{
				ArtifactID: "no-text",
				Parts: []types.Part{
					types.DataPart{Kind: "data", Data: map[string]any{}},
				},
			},
			expectedTexts: []string{},
		},
		{
			name:          "nil artifact",
			artifact:      nil,
			expectedTexts: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			texts := helper.ExtractTextFromArtifact(tt.artifact)
			assert.Equal(t, tt.expectedTexts, texts)
		})
	}
}

func TestArtifactHelper_ExtractFileDataFromArtifact(t *testing.T) {
	helper := NewArtifactHelper()

	testData := []byte("Hello, World!")
	encodedData := base64.StdEncoding.EncodeToString(testData)
	fileName := "test.txt"
	mimeType := "text/plain"
	uri := "https://example.com/file.pdf"
	pdfName := "remote.pdf"
	pdfMime := "application/pdf"

	tests := []struct {
		name       string
		artifact   *types.Artifact
		wantErr    bool
		fileCount  int
		assertions func(t *testing.T, files []FileData)
	}{
		{
			name: "artifact with byte file",
			artifact: &types.Artifact{
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
			},
			wantErr:   false,
			fileCount: 1,
			assertions: func(t *testing.T, files []FileData) {
				file := files[0]
				assert.Equal(t, fileName, *file.Name)
				assert.Equal(t, mimeType, *file.MIMEType)
				assert.Equal(t, testData, file.Data)
				assert.True(t, file.IsDataFile())
				assert.False(t, file.IsURIFile())
			},
		},
		{
			name: "artifact with URI file",
			artifact: &types.Artifact{
				ArtifactID: "uri-artifact",
				Parts: []types.Part{
					types.FilePart{
						Kind: "file",
						File: types.FileWithUri{
							Name:     &pdfName,
							MIMEType: &pdfMime,
							URI:      uri,
						},
					},
				},
			},
			wantErr:   false,
			fileCount: 1,
			assertions: func(t *testing.T, files []FileData) {
				file := files[0]
				assert.Equal(t, pdfName, *file.Name)
				assert.Equal(t, pdfMime, *file.MIMEType)
				assert.Equal(t, uri, *file.URI)
				assert.False(t, file.IsDataFile())
				assert.True(t, file.IsURIFile())
			},
		},
		{
			name: "artifact with multiple files",
			artifact: &types.Artifact{
				ArtifactID: "multi-file",
				Parts: []types.Part{
					types.FilePart{
						Kind: "file",
						File: types.FileWithBytes{Bytes: encodedData},
					},
					types.FilePart{
						Kind: "file",
						File: types.FileWithUri{URI: uri},
					},
				},
			},
			wantErr:   false,
			fileCount: 2,
		},
		{
			name:      "nil artifact",
			artifact:  nil,
			wantErr:   false,
			fileCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := helper.ExtractFileDataFromArtifact(tt.artifact)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, files, tt.fileCount)

				if tt.assertions != nil {
					tt.assertions(t, files)
				}
			}
		})
	}
}

func TestArtifactHelper_ExtractDataFromArtifact(t *testing.T) {
	helper := NewArtifactHelper()

	data1 := map[string]any{"key1": "value1", "count": 42}
	data2 := map[string]any{"key2": "value2", "items": []string{"a", "b", "c"}}

	tests := []struct {
		name         string
		artifact     *types.Artifact
		expectedData []map[string]any
	}{
		{
			name: "artifact with multiple data parts",
			artifact: &types.Artifact{
				ArtifactID: "data-artifact",
				Parts: []types.Part{
					types.DataPart{Kind: "data", Data: data1},
					types.DataPart{Kind: "data", Data: data2},
					types.TextPart{Kind: "text", Text: "Should be ignored"},
				},
			},
			expectedData: []map[string]any{data1, data2},
		},
		{
			name: "artifact with no data parts",
			artifact: &types.Artifact{
				ArtifactID: "no-data",
				Parts: []types.Part{
					types.TextPart{Kind: "text", Text: "Only text"},
				},
			},
			expectedData: []map[string]any{},
		},
		{
			name:         "nil artifact",
			artifact:     nil,
			expectedData: []map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataList := helper.ExtractDataFromArtifact(tt.artifact)
			assert.Equal(t, tt.expectedData, dataList)
		})
	}
}

func TestArtifactHelper_ExtractArtifactUpdateFromStreamEvent(t *testing.T) {
	helper := NewArtifactHelper()

	tests := []struct {
		name       string
		event      any
		wantOk     bool
		assertions func(t *testing.T, event *types.TaskArtifactUpdateEvent)
	}{
		{
			name: "valid TaskArtifactUpdateEvent",
			event: types.TaskArtifactUpdateEvent{
				Kind:      "artifact-update",
				TaskID:    "task-123",
				ContextID: "context-456",
				Artifact: types.Artifact{
					ArtifactID: "stream-artifact",
					Parts: []types.Part{
						types.TextPart{Kind: "text", Text: "Streaming content"},
					},
				},
			},
			wantOk: true,
			assertions: func(t *testing.T, event *types.TaskArtifactUpdateEvent) {
				assert.Equal(t, "task-123", event.TaskID)
				assert.Equal(t, "stream-artifact", event.Artifact.ArtifactID)
			},
		},
		{
			name: "valid map representation",
			event: map[string]any{
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
			},
			wantOk: true,
			assertions: func(t *testing.T, event *types.TaskArtifactUpdateEvent) {
				assert.Equal(t, "task-123", event.TaskID)
				assert.Equal(t, "context-456", event.ContextID)
				assert.Equal(t, "stream-artifact", event.Artifact.ArtifactID)
			},
		},
		{
			name: "non-artifact event",
			event: map[string]any{
				"kind": "status-update",
				"data": "some status",
			},
			wantOk: false,
		},
		{
			name:   "unsupported type",
			event:  "invalid event",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractedEvent, ok := helper.ExtractArtifactUpdateFromStreamEvent(tt.event)
			assert.Equal(t, tt.wantOk, ok)

			if tt.wantOk && tt.assertions != nil {
				tt.assertions(t, extractedEvent)
			}
		})
	}
}

func TestArtifactHelper_HasArtifacts(t *testing.T) {
	helper := NewArtifactHelper()

	tests := []struct {
		name     string
		task     *types.Task
		expected bool
	}{
		{
			name: "task with artifacts",
			task: &types.Task{
				Artifacts: []types.Artifact{{ArtifactID: "artifact-1"}},
			},
			expected: true,
		},
		{
			name:     "task without artifacts",
			task:     &types.Task{},
			expected: false,
		},
		{
			name:     "nil task",
			task:     nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.HasArtifacts(tt.task)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArtifactHelper_GetArtifactCount(t *testing.T) {
	helper := NewArtifactHelper()

	tests := []struct {
		name     string
		task     *types.Task
		expected int
	}{
		{
			name: "task with multiple artifacts",
			task: &types.Task{
				Artifacts: []types.Artifact{
					{ArtifactID: "artifact-1"},
					{ArtifactID: "artifact-2"},
					{ArtifactID: "artifact-3"},
				},
			},
			expected: 3,
		},
		{
			name:     "empty task",
			task:     &types.Task{},
			expected: 0,
		},
		{
			name:     "nil task",
			task:     nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := helper.GetArtifactCount(tt.task)
			assert.Equal(t, tt.expected, count)
		})
	}
}

func TestArtifactHelper_GetArtifactSummary(t *testing.T) {
	helper := NewArtifactHelper()

	tests := []struct {
		name     string
		task     *types.Task
		expected map[string]int
	}{
		{
			name: "task with mixed artifact types",
			task: &types.Task{
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
			},
			expected: map[string]int{"text": 2, "file": 1, "data": 2},
		},
		{
			name:     "empty task",
			task:     &types.Task{},
			expected: map[string]int{},
		},
		{
			name:     "nil task",
			task:     nil,
			expected: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := helper.GetArtifactSummary(tt.task)
			assert.Equal(t, tt.expected, summary)
		})
	}
}

func TestArtifactHelper_FilterArtifactsByName(t *testing.T) {
	helper := NewArtifactHelper()

	task := &types.Task{
		Artifacts: []types.Artifact{
			{ArtifactID: "artifact-1", Name: stringPtr("User Report")},
			{ArtifactID: "artifact-2", Name: stringPtr("System Log")},
			{ArtifactID: "artifact-3", Name: stringPtr("User Guide")},
			{ArtifactID: "artifact-4", Name: nil},
		},
	}

	tests := []struct {
		name          string
		task          *types.Task
		substring     string
		expectedNames []string
	}{
		{
			name:          "filter by 'user'",
			task:          task,
			substring:     "user",
			expectedNames: []string{"User Report", "User Guide"},
		},
		{
			name:          "filter by 'log'",
			task:          task,
			substring:     "log",
			expectedNames: []string{"System Log"},
		},
		{
			name:          "no matches",
			task:          task,
			substring:     "xyz",
			expectedNames: []string{},
		},
		{
			name:          "nil task",
			task:          nil,
			substring:     "user",
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := helper.FilterArtifactsByName(tt.task, tt.substring)
			assert.Len(t, filtered, len(tt.expectedNames))

			if len(tt.expectedNames) > 0 {
				foundNames := make([]string, len(filtered))
				for i, artifact := range filtered {
					foundNames[i] = *artifact.Name
				}
				for _, expectedName := range tt.expectedNames {
					assert.Contains(t, foundNames, expectedName)
				}
			}
		})
	}
}

func TestFileData_Methods(t *testing.T) {
	uri := "https://example.com/file.pdf"

	tests := []struct {
		name               string
		file               FileData
		expectedIsDataFile bool
		expectedIsURIFile  bool
		expectedFileName   string
		expectedMIMEType   string
	}{
		{
			name: "data file",
			file: FileData{
				Name:     stringPtr("test.txt"),
				MIMEType: stringPtr("text/plain"),
				Data:     []byte("test content"),
			},
			expectedIsDataFile: true,
			expectedIsURIFile:  false,
			expectedFileName:   "test.txt",
			expectedMIMEType:   "text/plain",
		},
		{
			name: "URI file",
			file: FileData{
				Name:     stringPtr("remote.pdf"),
				MIMEType: stringPtr("application/pdf"),
				URI:      &uri,
			},
			expectedIsDataFile: false,
			expectedIsURIFile:  true,
			expectedFileName:   "remote.pdf",
			expectedMIMEType:   "application/pdf",
		},
		{
			name:               "default file",
			file:               FileData{},
			expectedIsDataFile: false,
			expectedIsURIFile:  false,
			expectedFileName:   "unnamed_file",
			expectedMIMEType:   "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedIsDataFile, tt.file.IsDataFile())
			assert.Equal(t, tt.expectedIsURIFile, tt.file.IsURIFile())
			assert.Equal(t, tt.expectedFileName, tt.file.GetFileName())
			assert.Equal(t, tt.expectedMIMEType, tt.file.GetMIMEType())
		})
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
