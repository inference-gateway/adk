package server

// Additional error types for the new interface-based design

// EmptyMessagePartsError represents an error for empty message parts
type EmptyMessagePartsError struct{}

func (e *EmptyMessagePartsError) Error() string {
	return "empty message parts"
}

// NewEmptyMessagePartsError creates a new EmptyMessagePartsError
func NewEmptyMessagePartsError() error {
	return &EmptyMessagePartsError{}
}

// StreamingNotImplementedError represents an error for unimplemented streaming
type StreamingNotImplementedError struct{}

func (e *StreamingNotImplementedError) Error() string {
	return "streaming not implemented"
}

// NewStreamingNotImplementedError creates a new StreamingNotImplementedError
func NewStreamingNotImplementedError() error {
	return &StreamingNotImplementedError{}
}
