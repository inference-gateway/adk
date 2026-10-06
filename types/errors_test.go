package types_test

import (
	"errors"
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/assert"

	types "github.com/inference-gateway/adk/types"
)

func TestJSONRPCErrorIsAndAs(t *testing.T) {
	tests := []struct {
		name     string
		err      *types.JSONRPCError
		message  string
		sentinel error
	}{
		{
			name:     "method not found",
			err:      &types.JSONRPCError{Code: -32601, Message: "method not found: GetTask"},
			message:  "A2A error: method not found: GetTask (code: -32601)",
			sentinel: types.ErrMethodNotFound,
		},
		{
			name:     "task not found",
			err:      &types.JSONRPCError{Code: -32001, Message: "task not found"},
			message:  "A2A error: task not found (code: -32001)",
			sentinel: types.ErrTaskNotFound,
		},
		{
			name:    "unknown code has no sentinel",
			err:     &types.JSONRPCError{Code: -31999, Message: "boom"},
			message: "A2A error: boom (code: -31999)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := fmt.Errorf("sending message: %w", tt.err)
			assert.Equal(t, tt.message, tt.err.Error())

			var rpcErr *types.JSONRPCError
			assert.True(t, errors.As(wrapped, &rpcErr))
			assert.Equal(t, tt.err.Code, rpcErr.Code)

			if tt.sentinel == nil {
				assert.NoError(t, errors.Unwrap(tt.err))
				return
			}
			assert.True(t, errors.Is(wrapped, tt.sentinel))
		})
	}
}

func TestHTTPStatusErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      *types.HTTPStatusError
		expected string
	}{
		{
			name:     "no operation and no body",
			err:      &types.HTTPStatusError{StatusCode: 403},
			expected: "unexpected status code: 403",
		},
		{
			name:     "body only",
			err:      &types.HTTPStatusError{StatusCode: 500, Body: "boom"},
			expected: "unexpected status code: 500, body: boom",
		},
		{
			name:     "operation and body",
			err:      &types.HTTPStatusError{Operation: "agent card", StatusCode: 401, Body: "denied"},
			expected: "unexpected status code for agent card: 401, body: denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())

			var statusErr *types.HTTPStatusError
			assert.True(t, errors.As(fmt.Errorf("request: %w", tt.err), &statusErr))
			assert.Equal(t, tt.err.StatusCode, statusErr.StatusCode)
		})
	}
}
