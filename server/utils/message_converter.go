package utils

import (
	"fmt"
	"time"

	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// MessageConverter defines the interface for converting between A2A and SDK formats
type MessageConverter interface {
	// ConvertToSDK converts A2A messages to SDK format with type safety
	ConvertToSDK(messages []types.Message) ([]sdk.Message, error)

	// ConvertFromSDK converts SDK response back to A2A format
	ConvertFromSDK(response sdk.Message) (*types.Message, error)

	// ValidateMessagePart validates message part structure and type
	ValidateMessagePart(part types.Part) error
}

// OptimizedMessageConverter provides efficient conversion with type safety
type OptimizedMessageConverter struct {
	logger *zap.Logger
}

// NewOptimizedMessageConverter creates a new optimized message converter
func NewOptimizedMessageConverter(logger *zap.Logger) *OptimizedMessageConverter {
	return &OptimizedMessageConverter{
		logger: logger,
	}
}

// ConvertToSDK converts A2A messages to SDK format with validation
func (c *OptimizedMessageConverter) ConvertToSDK(messages []types.Message) ([]sdk.Message, error) {
	result := make([]sdk.Message, 0, len(messages))

	for i, msg := range messages {
		sdkMsg, err := c.convertSingleMessage(msg)
		if err != nil {
			c.logger.Error("failed to convert message",
				zap.Int("message_index", i),
				zap.String("message_id", msg.MessageID),
				zap.Error(err))
			return nil, err
		}

		result = append(result, sdkMsg)
	}

	return result, nil
}

// convertSingleMessage converts a single A2A message to SDK format
func (c *OptimizedMessageConverter) convertSingleMessage(msg types.Message) (sdk.Message, error) {
	role := msg.Role
	if role == "" {
		role = "user"
	}

	var content string
	var toolCallId *string
	var toolCalls *[]sdk.ChatCompletionMessageToolCall

	for _, part := range msg.Parts {
		if typedPart, ok := part.(types.OptimizedMessagePart); ok {
			switch typedPart.Kind {
			case types.MessagePartKindText:
				if typedPart.Text != nil {
					content += *typedPart.Text
				}
			case types.MessagePartKindData:
				if typedPart.Data != nil {
					if result, exists := typedPart.Data["result"]; exists {
						if resultStr, ok := result.(string); ok {
							content += resultStr
						}
					}

					if role == "assistant" {
						if toolCallsData, exists := typedPart.Data["tool_calls"]; exists {
							if toolCallsSlice, ok := toolCallsData.([]sdk.ChatCompletionMessageToolCall); ok {
								toolCalls = &toolCallsSlice
							}
						}

						if contentData, exists := typedPart.Data["content"]; exists {
							if contentStr, ok := contentData.(string); ok {
								content += contentStr
							}
						}
					}

					if role == "tool" {
						if id, exists := typedPart.Data["tool_call_id"]; exists {
							if idStr, ok := id.(string); ok {
								toolCallId = &idStr
							}
						}
					}
				}
			case types.MessagePartKindFile:
				if typedPart.File != nil {
					c.logger.Debug("file part detected in message",
						zap.String("message_id", msg.MessageID))
				}
			}
		} else if partMap, ok := part.(map[string]interface{}); ok {
			c.logger.Debug("using fallback map processing for message part",
				zap.String("message_id", msg.MessageID))

			kind, hasKind := partMap["kind"]
			if !hasKind {
				continue
			}

			switch kind {
			case "text":
				if text, exists := partMap["text"]; exists {
					if textStr, ok := text.(string); ok {
						content += textStr
					}
				}
			case "data":
				if data, exists := partMap["data"]; exists {
					if dataMap, ok := data.(map[string]interface{}); ok {
						if result, exists := dataMap["result"]; exists {
							if resultStr, ok := result.(string); ok {
								content += resultStr
							}
						}

						if role == "assistant" {
							if toolCallsData, exists := dataMap["tool_calls"]; exists {
								if toolCallsSlice, ok := toolCallsData.([]sdk.ChatCompletionMessageToolCall); ok {
									toolCalls = &toolCallsSlice
								}
							}

							if contentData, exists := dataMap["content"]; exists {
								if contentStr, ok := contentData.(string); ok {
									content += contentStr
								}
							}
						}

						if role == "tool" {
							if id, exists := dataMap["tool_call_id"]; exists {
								if idStr, ok := id.(string); ok {
									toolCallId = &idStr
								}
							}
						}
					}
				}
			}
		}
	}

	var sdkRole sdk.MessageRole
	switch role {
	case "user":
		sdkRole = sdk.User
	case "assistant":
		sdkRole = sdk.Assistant
	case "system":
		sdkRole = sdk.System
	case "tool":
		sdkRole = sdk.Tool
	default:
		sdkRole = sdk.User
	}

	return sdk.Message{
		Role:       sdkRole,
		Content:    content,
		ToolCallId: toolCallId,
		ToolCalls:  toolCalls,
	}, nil
}

// ConvertFromSDK converts SDK message response back to A2A format
func (c *OptimizedMessageConverter) ConvertFromSDK(response sdk.Message) (*types.Message, error) {
	role := string(response.Role)

	messageID := fmt.Sprintf("%s-%d", role, time.Now().UnixNano())

	message := &types.Message{
		Kind:      "message",
		MessageID: messageID,
		Role:      role,
		Parts:     []types.Part{},
	}

	if role == "tool" {
		toolData := map[string]interface{}{
			"result": response.Content,
		}

		if response.ToolCalls != nil && len(*response.ToolCalls) > 0 {
			toolData["tool_name"] = (*response.ToolCalls)[0].Function.Name
		} else {
			toolData["tool_name"] = ""
		}

		if response.ToolCallId != nil {
			toolData["tool_call_id"] = *response.ToolCallId
		}

		message.Parts = append(message.Parts, map[string]interface{}{
			"kind": string(types.MessagePartKindData),
			"data": toolData,
		})
	} else {
		message.Parts = append(message.Parts, map[string]interface{}{
			"kind": string(types.MessagePartKindText),
			"text": response.Content,
		})
	}

	if response.ToolCalls != nil && len(*response.ToolCalls) > 0 {
		toolCallData := map[string]interface{}{
			"tool_calls": *response.ToolCalls,
		}
		message.Parts = append(message.Parts, map[string]interface{}{
			"kind": string(types.MessagePartKindData),
			"data": toolCallData,
		})
	}

	c.logger.Debug("converted SDK message to A2A format",
		zap.String("role", role),
		zap.String("content", response.Content),
		zap.Bool("has_tool_calls", response.ToolCalls != nil))

	return message, nil
}

// ValidateMessagePart validates message part structure and type
func (c *OptimizedMessageConverter) ValidateMessagePart(part types.Part) error {
	if typedPart, ok := part.(types.OptimizedMessagePart); ok {
		if !typedPart.Kind.IsValid() {
			return fmt.Errorf("invalid message part kind: %s", typedPart.Kind)
		}

		switch typedPart.Kind {
		case types.MessagePartKindText:
			if typedPart.Text == nil {
				return fmt.Errorf("text part missing text field")
			}
		case types.MessagePartKindFile:
			if typedPart.File == nil {
				return fmt.Errorf("file part missing file field")
			}
		case types.MessagePartKindData:
			if typedPart.Data == nil {
				return fmt.Errorf("data part missing data field")
			}
		}
		return nil
	}

	if partMap, ok := part.(map[string]interface{}); ok {
		kind, hasKind := partMap["kind"]
		if !hasKind {
			return fmt.Errorf("message part missing kind field")
		}

		kindStr, ok := kind.(string)
		if !ok {
			return fmt.Errorf("message part kind must be string")
		}

		partKind := types.MessagePartKind(kindStr)
		if !partKind.IsValid() {
			return fmt.Errorf("invalid message part kind: %s", kindStr)
		}

		return nil
	}

	return fmt.Errorf("unsupported message part type")
}
