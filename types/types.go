package types

import (
	"maps"
	"strings"
)

// Health status constants
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusDegraded  = "degraded"
	HealthStatusUnhealthy = "unhealthy"
)

// CloudEvent type constants for agent streaming operations
const (
	EventDelta               = "adk.agent.delta"
	EventIterationCompleted  = "adk.agent.iteration.completed"
	EventToolStarted         = "adk.agent.tool.started"
	EventToolCompleted       = "adk.agent.tool.completed"
	EventToolFailed          = "adk.agent.tool.failed"
	EventToolResult          = "adk.agent.tool.result"
	EventInputRequired       = "adk.agent.input.required"
	EventTaskInterrupted     = "adk.agent.task.interrupted"
	EventTaskStatusChanged   = "adk.agent.task.status.changed"
	EventStreamFailed        = "adk.agent.stream.failed"
	EventTaskArtifactUpdated = "adk.agent.task.artifact.updated"
)

// A2AProtocolVersion is the A2A protocol version this ADK speaks, sent and checked as the
// A2A-Version header (spec section 3.6).
const A2AProtocolVersion = "1.0"

// UsageExtensionURI identifies the A2A extension that reports a task's token usage and
// execution stats in its metadata. It is inactive unless the client lists it in the
// A2A-Extensions header.
const UsageExtensionURI = "https://github.com/inference-gateway/schemas/tree/main/a2a/extensions/usage/v1"

// Task metadata keys written by the usage extension.
const (
	UsageMetadataKey          = UsageExtensionURI + "/usage"
	ExecutionStatsMetadataKey = UsageExtensionURI + "/execution_stats"
)

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

// IsInterrupted reports whether a task in this state is paused waiting on the client.
func (s TaskState) IsInterrupted() bool {
	return s == TaskStateInputRequired || s == TaskStateAuthRequired
}

// WithHistoryLength returns a copy of the task keeping only its historyLength most recent
// messages; nil keeps the full history (spec section 3.2.4).
func (t Task) WithHistoryLength(historyLength *int) Task {
	if historyLength == nil || *historyLength >= len(t.History) {
		return t
	}
	t.History = t.History[len(t.History)-max(*historyLength, 0):]
	return t
}

// WithoutExtension returns a copy of the task without the metadata keys of the extension
// identified by uri. The metadata map is cloned, so the original task keeps them.
func (t Task) WithoutExtension(uri string) Task {
	if t.Metadata == nil {
		return t
	}
	metadata := maps.Clone(*t.Metadata)
	maps.DeleteFunc(metadata, func(key string, _ any) bool { return strings.HasPrefix(key, uri+"/") })
	t.Metadata = &metadata
	return t
}

// ApplyArtifactUpdate records an artifact update on the task: parts are appended to the
// artifact with the same id when the update says append, otherwise the artifact is added
// or replaced.
func (t *Task) ApplyArtifactUpdate(update TaskArtifactUpdateEvent) {
	appendParts := update.Append != nil && *update.Append
	for i := range t.Artifacts {
		if t.Artifacts[i].ArtifactID != update.Artifact.ArtifactID {
			continue
		}
		if appendParts {
			t.Artifacts[i].Parts = append(t.Artifacts[i].Parts, update.Artifact.Parts...)
		} else {
			t.Artifacts[i] = update.Artifact
		}
		return
	}
	t.Artifacts = append(t.Artifacts, update.Artifact)
}
