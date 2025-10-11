package types

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

// Health status constants
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusDegraded  = "degraded"
	HealthStatusUnhealthy = "unhealthy"
)

// CloudEvent type constants for agent streaming operations
const (
	EventDelta              = "adk.agent.delta"
	EventIterationCompleted = "adk.agent.iteration.completed"
	EventToolStarted        = "adk.agent.tool.started"
	EventToolCompleted      = "adk.agent.tool.completed"
	EventToolFailed         = "adk.agent.tool.failed"
	EventToolResult         = "adk.agent.tool.result"
	EventInputRequired      = "adk.agent.input.required"
	EventTaskInterrupted    = "adk.agent.task.interrupted"
	EventTaskStatusChanged  = "adk.agent.task.status.changed"
	EventStreamFailed       = "adk.agent.stream.failed"
)

// Tool name constants
const (
	ToolInputRequired = "input_required"
)
