package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalPart(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected Part
	}{
		{
			name:     "unmarshal TextPart",
			jsonData: `{"kind": "text", "text": "Hello, world!", "metadata": {"key": "value"}}`,
			expected: TextPart{
				Kind:     "text",
				Text:     "Hello, world!",
				Metadata: map[string]any{"key": "value"},
			},
		},
		{
			name:     "unmarshal DataPart",
			jsonData: `{"kind": "data", "data": {"result": "success"}, "metadata": {"source": "test"}}`,
			expected: DataPart{
				Kind:     "data",
				Data:     map[string]any{"result": "success"},
				Metadata: map[string]any{"source": "test"},
			},
		},
		{
			name:     "unmarshal FilePart with FileWithBytes",
			jsonData: `{"kind": "file", "file": {"name": "test.txt", "mimeType": "text/plain", "bytes": "dGVzdA=="}}`,
			expected: FilePart{
				Kind: "file",
				File: map[string]any{
					"name":     "test.txt",
					"mimeType": "text/plain",
					"bytes":    "dGVzdA==",
				},
			},
		},
		{
			name:     "unmarshal unknown kind as map",
			jsonData: `{"kind": "unknown", "customField": "value"}`,
			expected: map[string]any{
				"kind":        "unknown",
				"customField": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			part, err := UnmarshalPart([]byte(tt.jsonData))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, part)
		})
	}
}

func TestMarshalPart(t *testing.T) {
	tests := []struct {
		name     string
		part     Part
		expected string
	}{
		{
			name: "marshal TextPart",
			part: TextPart{
				Kind: "text",
				Text: "Hello, world!",
			},
			expected: `{"kind":"text","text":"Hello, world!"}`,
		},
		{
			name: "marshal DataPart",
			part: DataPart{
				Kind: "data",
				Data: map[string]any{"result": "success"},
			},
			expected: `{"kind":"data","data":{"result":"success"}}`,
		},
		{
			name: "marshal FilePart",
			part: FilePart{
				Kind: "file",
				File: map[string]any{
					"name":     "test.txt",
					"mimeType": "text/plain",
				},
			},
			expected: `{"kind":"file","file":{"mimeType":"text/plain","name":"test.txt"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := json.Marshal(tt.part)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(result))
		})
	}
}

func TestUnmarshalParts(t *testing.T) {
	jsonData := `[
		{"kind": "text", "text": "Hello"},
		{"kind": "data", "data": {"key": "value"}},
		{"kind": "file", "file": {"name": "test.txt"}}
	]`

	parts, err := UnmarshalParts([]byte(jsonData))
	require.NoError(t, err)
	require.Len(t, parts, 3)

	textPart, ok := parts[0].(TextPart)
	require.True(t, ok)
	assert.Equal(t, "text", textPart.Kind)
	assert.Equal(t, "Hello", textPart.Text)

	dataPart, ok := parts[1].(DataPart)
	require.True(t, ok)
	assert.Equal(t, "data", dataPart.Kind)
	assert.Equal(t, map[string]any{"key": "value"}, dataPart.Data)

	filePart, ok := parts[2].(FilePart)
	require.True(t, ok)
	assert.Equal(t, "file", filePart.Kind)
	assert.Equal(t, map[string]any{"name": "test.txt"}, filePart.File)
}

func TestMarshalParts(t *testing.T) {
	parts := []Part{
		TextPart{Kind: "text", Text: "Hello"},
		DataPart{Kind: "data", Data: map[string]any{"key": "value"}},
		FilePart{Kind: "file", File: map[string]any{"name": "test.txt"}},
	}

	result, err := MarshalParts(parts)
	require.NoError(t, err)

	expected := `[
		{"kind":"text","text":"Hello"},
		{"kind":"data","data":{"key":"value"}},
		{"kind":"file","file":{"name":"test.txt"}}
	]`

	assert.JSONEq(t, expected, string(result))
}

func TestCreateTextPart(t *testing.T) {
	part := CreateTextPart("Hello, world!")
	assert.Equal(t, "text", part.Kind)
	assert.Equal(t, "Hello, world!", part.Text)
	assert.Nil(t, part.Metadata)

	// Test with metadata
	metadata := map[string]any{"key": "value"}
	partWithMeta := CreateTextPart("Hello", metadata)
	assert.Equal(t, "text", partWithMeta.Kind)
	assert.Equal(t, "Hello", partWithMeta.Text)
	assert.Equal(t, metadata, partWithMeta.Metadata)
}

func TestCreateDataPart(t *testing.T) {
	data := map[string]any{"result": "success"}
	part := CreateDataPart(data)
	assert.Equal(t, "data", part.Kind)
	assert.Equal(t, data, part.Data)
	assert.Nil(t, part.Metadata)

	// Test with metadata
	metadata := map[string]any{"source": "test"}
	partWithMeta := CreateDataPart(data, metadata)
	assert.Equal(t, "data", partWithMeta.Kind)
	assert.Equal(t, data, partWithMeta.Data)
	assert.Equal(t, metadata, partWithMeta.Metadata)
}

func TestCreateFilePart(t *testing.T) {
	file := map[string]any{"name": "test.txt", "mimeType": "text/plain"}
	part := CreateFilePart(file)
	assert.Equal(t, "file", part.Kind)
	assert.Equal(t, file, part.File)
	assert.Nil(t, part.Metadata)

	// Test with metadata
	metadata := map[string]any{"uploaded": true}
	partWithMeta := CreateFilePart(file, metadata)
	assert.Equal(t, "file", partWithMeta.Kind)
	assert.Equal(t, file, partWithMeta.File)
	assert.Equal(t, metadata, partWithMeta.Metadata)
}

func TestPartMarshalingRoundTrip(t *testing.T) {
	original := []Part{
		TextPart{Kind: "text", Text: "Hello, world!", Metadata: map[string]any{"lang": "en"}},
		DataPart{Kind: "data", Data: map[string]any{"result": "success"}, Metadata: map[string]any{"source": "api"}},
		FilePart{Kind: "file", File: map[string]any{"name": "test.txt", "bytes": "dGVzdA=="}, Metadata: map[string]any{"size": 4}},
	}

	marshaled, err := MarshalParts(original)
	require.NoError(t, err)

	unmarshaled, err := UnmarshalParts(marshaled)
	require.NoError(t, err)

	require.Len(t, unmarshaled, 3)

	textPart, ok := unmarshaled[0].(TextPart)
	require.True(t, ok)
	assert.Equal(t, "text", textPart.Kind)
	assert.Equal(t, "Hello, world!", textPart.Text)
	assert.Equal(t, map[string]any{"lang": "en"}, textPart.Metadata)

	dataPart, ok := unmarshaled[1].(DataPart)
	require.True(t, ok)
	assert.Equal(t, "data", dataPart.Kind)
	assert.Equal(t, map[string]any{"result": "success"}, dataPart.Data)
	assert.Equal(t, map[string]any{"source": "api"}, dataPart.Metadata)

	filePart, ok := unmarshaled[2].(FilePart)
	require.True(t, ok)
	assert.Equal(t, "file", filePart.Kind)
	assert.Equal(t, map[string]any{"name": "test.txt", "bytes": "dGVzdA=="}, filePart.File)
	assert.Equal(t, map[string]any{"size": float64(4)}, filePart.Metadata) // JSON numbers are float64
}
