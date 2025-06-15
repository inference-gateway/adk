package adk

// MessagePartKind represents the different types of message parts supported by A2A protocol.
// Based on the A2A specification: https://google-a2a.github.io/A2A/latest/
type MessagePartKind string

// MessagePartKind enum values for the three official message part types
const (
	// MessagePartKindText represents a text segment within message parts
	MessagePartKindText MessagePartKind = "text"

	// MessagePartKindFile represents a file segment within message parts
	MessagePartKindFile MessagePartKind = "file"

	// MessagePartKindData represents a structured data segment within message parts
	MessagePartKindData MessagePartKind = "data"
)

// String returns the string representation of the MessagePartKind
func (k MessagePartKind) String() string {
	return string(k)
}

// IsValid checks if the MessagePartKind is one of the supported values
func (k MessagePartKind) IsValid() bool {
	switch k {
	case MessagePartKindText, MessagePartKindFile, MessagePartKindData:
		return true
	default:
		return false
	}
}
