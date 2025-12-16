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
		validate func(t *testing.T, part Part)
	}{
		{
			name:     "unmarshal text part",
			jsonData: `{"text": "Hello, world!", "metadata": {"key": "value"}}`,
			validate: func(t *testing.T, part Part) {
				require.NotNil(t, part.Text)
				assert.Equal(t, "Hello, world!", *part.Text)
				require.NotNil(t, part.Metadata)
				assert.Equal(t, map[string]any{"key": "value"}, *part.Metadata)
			},
		},
		{
			name:     "unmarshal data part",
			jsonData: `{"data": {"data": {"result": "success"}}, "metadata": {"source": "test"}}`,
			validate: func(t *testing.T, part Part) {
				require.NotNil(t, part.Data)
				assert.Equal(t, map[string]any{"result": "success"}, part.Data.Data)
				require.NotNil(t, part.Metadata)
				assert.Equal(t, map[string]any{"source": "test"}, *part.Metadata)
			},
		},
		{
			name:     "unmarshal file part",
			jsonData: `{"file": {"name": "test.txt", "mediaType": "text/plain", "fileWithBytes": "dGVzdA=="}}`,
			validate: func(t *testing.T, part Part) {
				require.NotNil(t, part.File)
				assert.Equal(t, "test.txt", part.File.Name)
				assert.Equal(t, "text/plain", part.File.MediaType)
				require.NotNil(t, part.File.FileWithBytes)
				assert.Equal(t, "dGVzdA==", *part.File.FileWithBytes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			part, err := UnmarshalPart([]byte(tt.jsonData))
			require.NoError(t, err)
			tt.validate(t, part)
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
			name: "marshal text part",
			part: CreateTextPart("Hello, world!"),
			expected: `{"text":"Hello, world!"}`,
		},
		{
			name: "marshal data part",
			part: CreateDataPart(map[string]any{"result": "success"}),
			expected: `{"data":{"data":{"result":"success"}}}`,
		},
		{
			name: "marshal file part",
			part: func() Part {
				bytes := "dGVzdA=="
				return CreateFilePart("test.txt", "text/plain", &bytes, nil)
			}(),
			expected: `{"file":{"name":"test.txt","mediaType":"text/plain","fileWithBytes":"dGVzdA=="}}`,
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
		{"text": "Hello"},
		{"data": {"data": {"key": "value"}}},
		{"file": {"name": "test.txt", "mediaType": "text/plain"}}
	]`

	parts, err := UnmarshalParts([]byte(jsonData))
	require.NoError(t, err)
	require.Len(t, parts, 3)

	// First part should be text
	require.NotNil(t, parts[0].Text)
	assert.Equal(t, "Hello", *parts[0].Text)

	// Second part should be data
	require.NotNil(t, parts[1].Data)
	assert.Equal(t, map[string]any{"key": "value"}, parts[1].Data.Data)

	// Third part should be file
	require.NotNil(t, parts[2].File)
	assert.Equal(t, "test.txt", parts[2].File.Name)
	assert.Equal(t, "text/plain", parts[2].File.MediaType)
}

func TestMarshalParts(t *testing.T) {
	parts := []Part{
		CreateTextPart("Hello"),
		CreateDataPart(map[string]any{"key": "value"}),
		func() Part {
			return CreateFilePart("test.txt", "text/plain", nil, nil)
		}(),
	}

	result, err := MarshalParts(parts)
	require.NoError(t, err)

	expected := `[
		{"text":"Hello"},
		{"data":{"data":{"key":"value"}}},
		{"file":{"name":"test.txt","mediaType":"text/plain"}}
	]`

	assert.JSONEq(t, expected, string(result))
}

func TestCreateTextPart(t *testing.T) {
	part := CreateTextPart("Hello, world!")
	require.NotNil(t, part.Text)
	assert.Equal(t, "Hello, world!", *part.Text)
	assert.Nil(t, part.Metadata)

	metadata := map[string]any{"key": "value"}
	partWithMeta := CreateTextPart("Hello", metadata)
	require.NotNil(t, partWithMeta.Text)
	assert.Equal(t, "Hello", *partWithMeta.Text)
	require.NotNil(t, partWithMeta.Metadata)
	assert.Equal(t, metadata, *partWithMeta.Metadata)
}

func TestCreateDataPart(t *testing.T) {
	data := map[string]any{"result": "success"}
	part := CreateDataPart(data)
	require.NotNil(t, part.Data)
	assert.Equal(t, data, part.Data.Data)
	assert.Nil(t, part.Metadata)

	metadata := map[string]any{"source": "test"}
	partWithMeta := CreateDataPart(data, metadata)
	require.NotNil(t, partWithMeta.Data)
	assert.Equal(t, data, partWithMeta.Data.Data)
	require.NotNil(t, partWithMeta.Metadata)
	assert.Equal(t, metadata, *partWithMeta.Metadata)
}

func TestCreateFilePart(t *testing.T) {
	bytes := "dGVzdA=="
	part := CreateFilePart("test.txt", "text/plain", &bytes, nil)
	require.NotNil(t, part.File)
	assert.Equal(t, "test.txt", part.File.Name)
	assert.Equal(t, "text/plain", part.File.MediaType)
	require.NotNil(t, part.File.FileWithBytes)
	assert.Equal(t, bytes, *part.File.FileWithBytes)
	assert.Nil(t, part.Metadata)

	metadata := map[string]any{"uploaded": true}
	partWithMeta := CreateFilePart("test.txt", "text/plain", &bytes, nil, metadata)
	require.NotNil(t, partWithMeta.File)
	assert.Equal(t, "test.txt", partWithMeta.File.Name)
	require.NotNil(t, partWithMeta.Metadata)
	assert.Equal(t, metadata, *partWithMeta.Metadata)
}

func TestPartMarshalingRoundTrip(t *testing.T) {
	bytes := "dGVzdA=="
	original := []Part{
		CreateTextPart("Hello, world!", map[string]any{"lang": "en"}),
		CreateDataPart(map[string]any{"result": "success"}, map[string]any{"source": "api"}),
		CreateFilePart("test.txt", "text/plain", &bytes, nil, map[string]any{"size": 4}),
	}

	marshaled, err := MarshalParts(original)
	require.NoError(t, err)

	unmarshaled, err := UnmarshalParts(marshaled)
	require.NoError(t, err)

	require.Len(t, unmarshaled, 3)

	// Text part
	require.NotNil(t, unmarshaled[0].Text)
	assert.Equal(t, "Hello, world!", *unmarshaled[0].Text)
	require.NotNil(t, unmarshaled[0].Metadata)
	assert.Equal(t, map[string]any{"lang": "en"}, *unmarshaled[0].Metadata)

	// Data part
	require.NotNil(t, unmarshaled[1].Data)
	assert.Equal(t, map[string]any{"result": "success"}, unmarshaled[1].Data.Data)
	require.NotNil(t, unmarshaled[1].Metadata)
	assert.Equal(t, map[string]any{"source": "api"}, *unmarshaled[1].Metadata)

	// File part
	require.NotNil(t, unmarshaled[2].File)
	assert.Equal(t, "test.txt", unmarshaled[2].File.Name)
	require.NotNil(t, unmarshaled[2].File.FileWithBytes)
	assert.Equal(t, bytes, *unmarshaled[2].File.FileWithBytes)
	require.NotNil(t, unmarshaled[2].Metadata)
	assert.Equal(t, map[string]any{"size": float64(4)}, *unmarshaled[2].Metadata)
}

func TestMessageUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		validate func(t *testing.T, msg Message)
	}{
		{
			name: "message with text parts",
			jsonData: `{
				"messageId": "msg-123",
				"role": "user",
				"parts": [
					{"text": "Hello"},
					{"text": "World"}
				]
			}`,
			validate: func(t *testing.T, msg Message) {
				assert.Equal(t, "msg-123", msg.MessageID)
				assert.Equal(t, "user", msg.Role)
				require.Len(t, msg.Parts, 2)

				require.NotNil(t, msg.Parts[0].Text)
				assert.Equal(t, "Hello", *msg.Parts[0].Text)

				require.NotNil(t, msg.Parts[1].Text)
				assert.Equal(t, "World", *msg.Parts[1].Text)
			},
		},
		{
			name: "message with mixed part types",
			jsonData: `{
				"messageId": "msg-456",
				"role": "assistant",
				"parts": [
					{"text": "Response"},
					{"data": {"data": {"result": "success"}}},
					{"file": {"name": "test.txt", "mediaType": "text/plain"}}
				]
			}`,
			validate: func(t *testing.T, msg Message) {
				assert.Equal(t, "assistant", msg.Role)
				require.Len(t, msg.Parts, 3)

				require.NotNil(t, msg.Parts[0].Text)
				assert.Equal(t, "Response", *msg.Parts[0].Text)

				require.NotNil(t, msg.Parts[1].Data)
				assert.Equal(t, map[string]any{"result": "success"}, msg.Parts[1].Data.Data)

				require.NotNil(t, msg.Parts[2].File)
				assert.Equal(t, "test.txt", msg.Parts[2].File.Name)
			},
		},
		{
			name: "message round-trip with typed parts",
			jsonData: `{
				"messageId": "msg-789",
				"role": "user",
				"parts": [
					{"text": "Test message", "metadata": {"key": "value"}}
				]
			}`,
			validate: func(t *testing.T, msg Message) {
				marshaled, err := json.Marshal(msg)
				require.NoError(t, err)

				var msg2 Message
				err = json.Unmarshal(marshaled, &msg2)
				require.NoError(t, err)

				require.NotNil(t, msg2.Parts[0].Text)
				assert.Equal(t, "Test message", *msg2.Parts[0].Text)
				require.NotNil(t, msg2.Parts[0].Metadata)
				assert.Equal(t, map[string]any{"key": "value"}, *msg2.Parts[0].Metadata)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg Message
			err := json.Unmarshal([]byte(tt.jsonData), &msg)
			require.NoError(t, err)
			tt.validate(t, msg)
		})
	}
}
