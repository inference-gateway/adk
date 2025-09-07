package types

import (
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// NewToolResultMessage creates a standardized tool result message
func NewToolResultMessage(toolCallID string, result any, hasError bool) *Message {
	return &Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("tool-result-%s", toolCallID),
		Role:      "tool",
		Parts: []Part{
			map[string]any{
				"kind": "data",
				"data": map[string]any{
					"tool_call_id": toolCallID,
					"result":       result,
					"error":        hasError,
				},
			},
		},
	}
}

// NewAssistantMessage creates a standardized assistant message
func NewAssistantMessage(messageID string, parts []Part) *Message {
	return &Message{
		Kind:      "message",
		MessageID: messageID,
		Role:      "assistant",
		Parts:     parts,
	}
}

// NewTextPart creates a text part for a message
func NewTextPart(text string) Part {
	return map[string]any{
		"kind": "text",
		"text": text,
	}
}

// NewToolCallPart creates a tool call part for a message
func NewToolCallPart(toolCallID, toolName string, arguments map[string]any) Part {
	return map[string]any{
		"kind": "tool_call",
		"tool_call": map[string]any{
			"id":        toolCallID,
			"name":      toolName,
			"arguments": arguments,
		},
	}
}

// NewDataPart creates a generic data part for a message
func NewDataPart(data map[string]any) Part {
	return map[string]any{
		"kind": "data",
		"data": data,
	}
}

// NewStreamingStatusMessage creates a status message for streaming
func NewStreamingStatusMessage(messageID, status string, metadata map[string]any) *Message {
	data := map[string]any{
		"status": status,
	}
	for k, v := range metadata {
		data[k] = v
	}

	return &Message{
		Kind:      "message",
		MessageID: messageID,
		Role:      "assistant",
		Parts: []Part{
			NewDataPart(data),
		},
	}
}

// NewInputRequiredMessage creates an input required message
func NewInputRequiredMessage(toolCallID, message string) *Message {
	return &Message{
		Kind:      "input_required",
		MessageID: fmt.Sprintf("input-required-%s", toolCallID),
		Role:      "assistant",
		Parts: []Part{
			NewTextPart(message),
		},
	}
}

// NewAgentEvent creates a CloudEvent for agent lifecycle events
func NewAgentEvent(eventType, eventID string, data map[string]any) cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(eventID)
	event.SetType(eventType)
	event.SetSource("adk/agent")
	event.SetTime(time.Now())
	_ = event.SetData(cloudevents.ApplicationJSON, data)

	return event
}

// NewDeltaEvent creates a CloudEvent for streaming deltas, with the message in the data field
func NewDeltaEvent(message *Message) cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(message.MessageID)
	event.SetType("adk.agent.delta")
	event.SetSource("adk/agent")
	event.SetTime(time.Now())
	_ = event.SetData(cloudevents.ApplicationJSON, message)

	return event
}

// NewIterationCompletedEvent creates a CloudEvent for iteration completed with the final message
func NewIterationCompletedEvent(iteration int, taskID string, finalMessage *Message) cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(fmt.Sprintf("iteration-completed-%s-%d", taskID, iteration))
	event.SetType("adk.agent.iteration.completed")
	event.SetSource("adk/agent")
	event.SetTime(time.Now())

	event.SetExtension("iteration", iteration)
	event.SetExtension("task_id", taskID)
	_ = event.SetData(cloudevents.ApplicationJSON, finalMessage)

	return event
}

// NewMessageEvent creates a CloudEvent with a message payload and custom event type
func NewMessageEvent(eventType, eventID string, message *Message, extensions map[string]any) cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(eventID)
	event.SetType(eventType)
	event.SetSource("adk/agent")
	event.SetTime(time.Now())
	_ = event.SetData(cloudevents.ApplicationJSON, message)

	for key, value := range extensions {
		event.SetExtension(key, value)
	}

	return event
}
