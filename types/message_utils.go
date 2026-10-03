package types

import (
	"fmt"
	"maps"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const eventSource = "adk/agent"

// NewToolResultMessage creates a standardized tool result message
func NewToolResultMessage(toolCallID string, toolName string, result any, hasError bool) *Message {
	return &Message{
		MessageID: fmt.Sprintf("tool-result-%s", toolCallID),
		Role:      RoleAgent,
		Parts: []Part{
			CreateDataPart(map[string]any{
				"tool_call_id": toolCallID,
				"tool_name":    toolName,
				"result":       result,
				"error":        hasError,
			}),
		},
	}
}

// NewAssistantMessage creates a standardized assistant message
func NewAssistantMessage(messageID string, parts []Part) *Message {
	return &Message{
		MessageID: messageID,
		Role:      RoleAgent,
		Parts:     parts,
	}
}

// NewTextPart creates a text part for a message
func NewTextPart(text string) Part {
	return Part{
		Text: &text,
	}
}

// NewToolCallPart creates a tool call part for a message
func NewToolCallPart(toolCallID, toolName string, arguments map[string]any) Part {
	return CreateDataPart(map[string]any{
		"tool_call": map[string]any{
			"id":        toolCallID,
			"name":      toolName,
			"arguments": arguments,
		},
	})
}

// NewDataPart creates a generic data part for a message
func NewDataPart(data map[string]any) Part {
	return CreateDataPart(data)
}

// NewStreamingStatusMessage creates a status message for streaming
func NewStreamingStatusMessage(messageID, status string, metadata map[string]any) *Message {
	data := map[string]any{
		"status": status,
	}
	maps.Copy(data, metadata)

	return &Message{
		MessageID: messageID,
		Role:      RoleAgent,
		Parts: []Part{
			NewDataPart(data),
		},
	}
}

// NewInputRequiredMessage creates an input required message
func NewInputRequiredMessage(toolCallID, message string) *Message {
	return &Message{
		MessageID: fmt.Sprintf("input-required-%s", toolCallID),
		Role:      RoleAgent,
		Parts: []Part{
			NewTextPart(message),
		},
	}
}

func newAgentEvent(eventType, eventID string, data any) cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(eventID)
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetTime(time.Now())
	_ = event.SetData(cloudevents.ApplicationJSON, data)

	return event
}

// NewAgentEvent creates a CloudEvent for agent lifecycle events
func NewAgentEvent(eventType, eventID string, data map[string]any) cloudevents.Event {
	return newAgentEvent(eventType, eventID, data)
}

// NewDeltaEvent creates a CloudEvent for streaming deltas, with the message in the data field
func NewDeltaEvent(message *Message) cloudevents.Event {
	return newAgentEvent(EventDelta, message.MessageID, message)
}

// NewIterationCompletedEvent creates a CloudEvent for iteration completed with the final message
func NewIterationCompletedEvent(iteration int, taskID string, finalMessage *Message) cloudevents.Event {
	eventID := fmt.Sprintf("iteration-completed-%s-%d", taskID, iteration)
	event := newAgentEvent(EventIterationCompleted, eventID, finalMessage)
	event.SetExtension("iteration", iteration)
	event.SetExtension("task_id", taskID)

	return event
}

// NewMessageEvent creates a CloudEvent with a message payload and custom event type
func NewMessageEvent(eventType, eventID string, message *Message) cloudevents.Event {
	return newAgentEvent(eventType, eventID, message)
}
