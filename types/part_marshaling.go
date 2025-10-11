package types

import (
	"encoding/json"
	"fmt"
)

// UnmarshalPart unmarshals a single Part from JSON with proper type handling
func UnmarshalPart(data []byte) (Part, error) {
	// First, unmarshal into a temporary struct to get the kind field
	var temp struct {
		Kind string `json:"kind"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal part kind: %w", err)
	}

	// Based on the kind, unmarshal into the appropriate concrete type
	switch temp.Kind {
	case "text":
		var textPart TextPart
		if err := json.Unmarshal(data, &textPart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal TextPart: %w", err)
		}
		return textPart, nil

	case "data":
		var dataPart DataPart
		if err := json.Unmarshal(data, &dataPart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal DataPart: %w", err)
		}
		return dataPart, nil

	case "file":
		var filePart FilePart
		if err := json.Unmarshal(data, &filePart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal FilePart: %w", err)
		}
		return filePart, nil

	default:
		// Fall back to map[string]any for unknown or unsupported kinds
		var mapPart map[string]any
		if err := json.Unmarshal(data, &mapPart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal as map[string]any: %w", err)
		}
		return mapPart, nil
	}
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

// CreateTextPart creates a properly typed TextPart
func CreateTextPart(text string, metadata ...map[string]any) TextPart {
	part := TextPart{
		Kind: "text",
		Text: text,
	}
	if len(metadata) > 0 {
		part.Metadata = metadata[0]
	}
	return part
}

// CreateDataPart creates a properly typed DataPart
func CreateDataPart(data map[string]any, metadata ...map[string]any) DataPart {
	part := DataPart{
		Kind: "data",
		Data: data,
	}
	if len(metadata) > 0 {
		part.Metadata = metadata[0]
	}
	return part
}

// CreateFilePart creates a properly typed FilePart
func CreateFilePart(file any, metadata ...map[string]any) FilePart {
	part := FilePart{
		Kind: "file",
		File: file,
	}
	if len(metadata) > 0 {
		part.Metadata = metadata[0]
	}
	return part
}
