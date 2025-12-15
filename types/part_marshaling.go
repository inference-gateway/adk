package types

import (
	"encoding/json"
	"fmt"
)

// messageUnmarshalHelper is a wrapper for Message that ensures Parts are properly unmarshaled
type messageUnmarshalHelper struct {
	ContextID        *string           `json:"contextId,omitempty"`
	Extensions       []string          `json:"extensions,omitempty"`
	MessageID        string            `json:"messageId"`
	Metadata         *Struct           `json:"metadata,omitempty"`
	Parts            []json.RawMessage `json:"parts"`
	ReferenceTaskIds []string          `json:"referenceTaskIds,omitempty"`
	Role             string            `json:"role"`
	TaskID           *string           `json:"taskId,omitempty"`
}

// UnmarshalJSON custom unmarshaler for Message that properly handles typed Parts
func (m *Message) UnmarshalJSON(data []byte) error {
	var helper messageUnmarshalHelper
	if err := json.Unmarshal(data, &helper); err != nil {
		return err
	}

	parts := make([]Part, len(helper.Parts))
	for i, rawPart := range helper.Parts {
		part, err := UnmarshalPart(rawPart)
		if err != nil {
			return fmt.Errorf("failed to unmarshal part at index %d: %w", i, err)
		}
		parts[i] = part
	}

	m.ContextID = helper.ContextID
	m.Extensions = helper.Extensions
	m.MessageID = helper.MessageID
	m.Metadata = helper.Metadata
	m.Parts = parts
	m.ReferenceTaskIds = helper.ReferenceTaskIds
	m.Role = helper.Role
	m.TaskID = helper.TaskID

	return nil
}

// UnmarshalPart unmarshals a single Part from JSON with proper type handling
func UnmarshalPart(data []byte) (Part, error) {
	var part Part
	if err := json.Unmarshal(data, &part); err != nil {
		return Part{}, fmt.Errorf("failed to unmarshal Part: %w", err)
	}
	return part, nil
}

// UnmarshalParts is a utility function to unmarshal a slice of Parts with proper type handling
func UnmarshalParts(data []byte) ([]Part, error) {
	var rawParts []json.RawMessage
	if err := json.Unmarshal(data, &rawParts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw parts: %w", err)
	}

	parts := make([]Part, len(rawParts))
	for i, rawPart := range rawParts {
		part, err := UnmarshalPart(rawPart)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal part at index %d: %w", i, err)
		}
		parts[i] = part
	}

	return parts, nil
}

// MarshalParts is a utility function to marshal a slice of Parts
func MarshalParts(parts []Part) ([]byte, error) {
	return json.Marshal(parts)
}

// CreateTextPart creates a Part with text content
func CreateTextPart(text string, metadata ...*Struct) Part {
	part := Part{
		Text: &text,
	}
	if len(metadata) > 0 {
		part.Metadata = metadata[0]
	}
	return part
}

// CreateDataPart creates a Part with data content
func CreateDataPart(data *DataPart, metadata ...*Struct) Part {
	part := Part{
		Data: data,
	}
	if len(metadata) > 0 {
		part.Metadata = metadata[0]
	}
	return part
}

// CreateFilePart creates a Part with file content
func CreateFilePart(file *FilePart, metadata ...*Struct) Part {
	part := Part{
		File: file,
	}
	if len(metadata) > 0 {
		part.Metadata = metadata[0]
	}
	return part
}
