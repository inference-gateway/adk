package types

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

// A2AProtocolVersion is the A2A protocol version this ADK speaks, sent and checked as the
// A2A-Version header (spec section 3.6).
const A2AProtocolVersion = "1.0"

// Tool name constants
const (
	ToolInputRequired = "input_required"
)

// GetContextID returns the task's context ID, or "" when it is unset.
// The ADK always assigns one when it creates a task; the accessor exists because
// A2A v1.0 made the field optional on the wire.
func (t *Task) GetContextID() string {
	if t == nil || t.ContextID == nil {
		return ""
	}
	return *t.ContextID
}

// IsTerminal reports whether a task in this state will never change state again.
// A2A v1.0 dropped the `final` flag from status updates; terminal state is the signal.
func (s TaskState) IsTerminal() bool {
	switch s {
	case TaskStateCompleted, TaskStateFailed, TaskStateCanceled, TaskStateRejected:
		return true
	}
	return false
}
