package server

import (
	"encoding/json"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/adk/types"
)

func TestNormalizeParams(t *testing.T) {
	tests := []struct {
		name     string
		params   *types.Struct
		expected *types.Struct
	}{
		{
			name:     "nil params",
			params:   nil,
			expected: nil,
		},
		{
			name:     "proto names are camelized",
			params:   &types.Struct{"page_size": 10, "context_id": "ctx-1"},
			expected: &types.Struct{"pageSize": 10, "contextId": "ctx-1"},
		},
		{
			name:     "camel names are preserved",
			params:   &types.Struct{"pageSize": 10},
			expected: &types.Struct{"pageSize": 10},
		},
		{
			name:     "multi segment names are camelized",
			params:   &types.Struct{"status_timestamp_after": "now"},
			expected: &types.Struct{"statusTimestampAfter": "now"},
		},
		{
			name:     "unknown params are left in place",
			params:   &types.Struct{"surprise": true},
			expected: &types.Struct{"surprise": true},
		},
		{
			name: "nested objects are camelized but caller owned maps are not",
			params: &types.Struct{"message": map[string]any{
				"message_id": "m1",
				"metadata":   map[string]any{"my_key": 1},
			}},
			expected: &types.Struct{"message": map[string]any{
				"messageId": "m1",
				"metadata":  map[string]any{"my_key": 1},
			}},
		},
		{
			name: "arrays are recursed and part data stays verbatim",
			params: &types.Struct{"parts": []any{
				map[string]any{"data": map[string]any{"raw_key": true}, "part_id": "p1"},
			}},
			expected: &types.Struct{"parts": []any{
				map[string]any{"data": map[string]any{"raw_key": true}, "partId": "p1"},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeParams(tt.params))
		})
	}
}

func TestNormalizeParamsDecodesProtoNamedListTasksRequest(t *testing.T) {
	params := types.Struct{
		"context_id":        "ctx-1",
		"page_size":         float64(1),
		"history_length":    float64(2),
		"surprise":          true,
		"include_artifacts": true,
	}

	paramsBytes, err := json.Marshal(normalizeParams(&params))
	require.NoError(t, err)

	var request types.ListTasksRequest
	require.NoError(t, json.Unmarshal(paramsBytes, &request))

	require.NotNil(t, request.ContextID)
	assert.Equal(t, "ctx-1", *request.ContextID)
	require.NotNil(t, request.PageSize)
	assert.Equal(t, 1, *request.PageSize)
	require.NotNil(t, request.HistoryLength)
	assert.Equal(t, 2, *request.HistoryLength)
	require.NotNil(t, request.IncludeArtifacts)
	assert.True(t, *request.IncludeArtifacts)
}

func TestDecodeParams(t *testing.T) {
	tests := []struct {
		name        string
		params      types.Struct
		expectedID  string
		expectError bool
	}{
		{
			name:       "valid payload",
			params:     types.Struct{"id": "task-1", "historyLength": float64(3)},
			expectedID: "task-1",
		},
		{
			name:        "wrong field type",
			params:      types.Struct{"id": float64(42)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := decodeParams[types.GetTaskRequest](&tt.params)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedID, decoded.ID)
		})
	}
}
