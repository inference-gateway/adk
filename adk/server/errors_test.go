package server_test

import (
	"testing"

	"github.com/inference-gateway/a2a/adk/server"
	"github.com/stretchr/testify/assert"
)

func TestEmptyMessagePartsError(t *testing.T) {
	err := server.NewEmptyMessagePartsError()

	assert.NotNil(t, err)
	assert.Equal(t, "empty message parts", err.Error())

	var _ = err
}

func TestStreamingNotImplementedError(t *testing.T) {
	err := server.NewStreamingNotImplementedError()

	assert.NotNil(t, err)
	assert.Equal(t, "streaming not implemented", err.Error())

	var _ = err
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name        string
		createError func() error
		expectedMsg string
	}{
		{
			name:        "EmptyMessagePartsError",
			createError: server.NewEmptyMessagePartsError,
			expectedMsg: "empty message parts",
		},
		{
			name:        "StreamingNotImplementedError",
			createError: server.NewStreamingNotImplementedError,
			expectedMsg: "streaming not implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			assert.NotNil(t, err)
			assert.Equal(t, tt.expectedMsg, err.Error())
			assert.Implements(t, (*error)(nil), err)
		})
	}
}

func TestErrorTypeAssertion(t *testing.T) {
	emptyPartsErr := server.NewEmptyMessagePartsError()
	_, ok := emptyPartsErr.(*server.EmptyMessagePartsError)
	assert.True(t, ok, "should be able to type assert to *EmptyMessagePartsError")

	streamingErr := server.NewStreamingNotImplementedError()
	_, ok = streamingErr.(*server.StreamingNotImplementedError)
	assert.True(t, ok, "should be able to type assert to *StreamingNotImplementedError")
}

func TestErrorsAreDistinct(t *testing.T) {
	emptyPartsErr := server.NewEmptyMessagePartsError()
	streamingErr := server.NewStreamingNotImplementedError()

	assert.NotEqual(t, emptyPartsErr.Error(), streamingErr.Error())

	_, isEmptyParts := streamingErr.(*server.EmptyMessagePartsError)
	assert.False(t, isEmptyParts)

	_, isStreaming := emptyPartsErr.(*server.StreamingNotImplementedError)
	assert.False(t, isStreaming)
}
