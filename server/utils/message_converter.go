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
					if err := c.processDataPart(typedPart.Data, role, &content, &toolCallId, &toolCalls); err != nil {
						c.logger.Warn("failed to process typed data part",
							zap.String("message_id", msg.MessageID),
							zap.Error(err))
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
						if err := c.processDataPart(dataMap, role, &content, &toolCallId, &toolCalls); err != nil {
							c.logger.Warn("failed to process map data part",
								zap.String("message_id", msg.MessageID),
								zap.Error(err))
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

// processDataPart handles the extraction of data from data parts for both typed and map formats
func (c *OptimizedMessageConverter) processDataPart(
	data map[string]interface{},
	role string,
	content *string,
	toolCallId **string,
	toolCalls **[]sdk.ChatCompletionMessageToolCall,
) error {
	if role == "tool" {
		if result, exists := data["result"]; exists {
			if resultStr, ok := result.(string); ok {
				*content += resultStr
			}
		}

		if id, exists := data["tool_call_id"]; exists {
			if idStr, ok := id.(string); ok {
				*toolCallId = &idStr
			}
		}
		return nil
	}

	if role == "assistant" {
		if toolCallData, exists := data["tool_call"]; exists {
			if err := c.extractSingleToolCall(toolCallData, toolCalls); err != nil {
				return fmt.Errorf("failed to extract single tool call: %w", err)
			}
			return nil
		}

		if toolCallsData, exists := data["tool_calls"]; exists {
			if err := c.extractToolCallsArray(toolCallsData, toolCalls); err != nil {
				return fmt.Errorf("failed to extract tool calls array: %w", err)
			}
		}

		if contentData, exists := data["content"]; exists {
			if contentStr, ok := contentData.(string); ok {
				*content += contentStr
			}
		}

		if result, exists := data["result"]; exists {
			if resultStr, ok := result.(string); ok {
				*content += resultStr
			}
		}
		return nil
	}

	if result, exists := data["result"]; exists {
		if resultStr, ok := result.(string); ok {
			*content += resultStr
		}
	}

	if contentData, exists := data["content"]; exists {
		if contentStr, ok := contentData.(string); ok {
			*content += contentStr
		}
	}

	return nil
}

// extractSingleToolCall extracts a single tool call from data part
func (c *OptimizedMessageConverter) extractSingleToolCall(
	toolCallData interface{},
	toolCalls **[]sdk.ChatCompletionMessageToolCall,
) error {
	if toolCallMap, ok := toolCallData.(map[string]interface{}); ok {
		toolCall, err := c.mapToToolCall(toolCallMap)
		if err != nil {
			return err
		}

		if *toolCalls == nil {
			*toolCalls = &[]sdk.ChatCompletionMessageToolCall{}
		}
		**toolCalls = append(**toolCalls, toolCall)
		return nil
	}

	if toolCallStruct, ok := toolCallData.(sdk.ChatCompletionMessageToolCall); ok {
		if *toolCalls == nil {
			*toolCalls = &[]sdk.ChatCompletionMessageToolCall{}
		}
		**toolCalls = append(**toolCalls, toolCallStruct)
		return nil
	}

	return fmt.Errorf("unsupported tool call data type: %T", toolCallData)
}

// extractToolCallsArray extracts tool calls from an array in data part
func (c *OptimizedMessageConverter) extractToolCallsArray(
	toolCallsData interface{},
	toolCalls **[]sdk.ChatCompletionMessageToolCall,
) error {
	if toolCallsSlice, ok := toolCallsData.([]sdk.ChatCompletionMessageToolCall); ok {
		*toolCalls = &toolCallsSlice
		return nil
	}

	if toolCallsInterface, ok := toolCallsData.([]interface{}); ok {
		var extractedToolCalls []sdk.ChatCompletionMessageToolCall
		for _, item := range toolCallsInterface {
			if toolCallMap, ok := item.(map[string]interface{}); ok {
				toolCall, err := c.mapToToolCall(toolCallMap)
				if err != nil {
					return err
				}
				extractedToolCalls = append(extractedToolCalls, toolCall)
			}
		}
		if len(extractedToolCalls) > 0 {
			*toolCalls = &extractedToolCalls
		}
		return nil
	}

	return fmt.Errorf("unsupported tool calls array type: %T", toolCallsData)
}

// mapToToolCall converts a map to sdk.ChatCompletionMessageToolCall
func (c *OptimizedMessageConverter) mapToToolCall(toolCallMap map[string]interface{}) (sdk.ChatCompletionMessageToolCall, error) {
	var toolCall sdk.ChatCompletionMessageToolCall

	if id, exists := toolCallMap["id"]; exists {
		if idStr, ok := id.(string); ok {
			toolCall.Id = idStr
		}
	}

	if typeField, exists := toolCallMap["type"]; exists {
		if typeStr, ok := typeField.(string); ok {
			toolCall.Type = sdk.ChatCompletionToolType(typeStr)
		}
	}

	if function, exists := toolCallMap["function"]; exists {
		if functionMap, ok := function.(map[string]interface{}); ok {
			if name, exists := functionMap["name"]; exists {
				if nameStr, ok := name.(string); ok {
					toolCall.Function.Name = nameStr
				}
			}

			if arguments, exists := functionMap["arguments"]; exists {
				if argsStr, ok := arguments.(string); ok {
					toolCall.Function.Arguments = argsStr
				}
			}
		}
	}

	return toolCall, nil
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

	switch role {
	case "tool":
		toolData := map[string]interface{}{
			"result": response.Content,
		}

		if response.ToolCallId != nil {
			toolData["tool_call_id"] = *response.ToolCallId
		}

		if response.ToolCalls != nil && len(*response.ToolCalls) > 0 {
			toolData["tool_name"] = (*response.ToolCalls)[0].Function.Name
		} else {
			toolData["tool_name"] = ""
		}

		message.Parts = append(message.Parts, map[string]interface{}{
			"kind": string(types.MessagePartKindData),
			"data": toolData,
		})

	case "assistant":
		hasToolCalls := response.ToolCalls != nil && len(*response.ToolCalls) > 0

		if response.Content != "" {
			message.Parts = append(message.Parts, map[string]interface{}{
				"kind": string(types.MessagePartKindText),
				"text": response.Content,
			})
		}

		if hasToolCalls {
			toolCallData := map[string]interface{}{
				"tool_calls": *response.ToolCalls,
			}
			message.Parts = append(message.Parts, map[string]interface{}{
				"kind": string(types.MessagePartKindData),
				"data": toolCallData,
			})
		}

		if response.ReasoningContent != nil && *response.ReasoningContent != "" {
			message.Parts = append(message.Parts, map[string]interface{}{
				"kind": string(types.MessagePartKindText),
				"text": *response.ReasoningContent,
			})
		} else if response.Reasoning != nil && *response.Reasoning != "" {
			message.Parts = append(message.Parts, map[string]interface{}{
				"kind": string(types.MessagePartKindText),
				"text": *response.Reasoning,
			})
		}

	default:
		if response.Content != "" {
			message.Parts = append(message.Parts, map[string]interface{}{
				"kind": string(types.MessagePartKindText),
				"text": response.Content,
			})
		}
	}

	if len(message.Parts) == 0 {
		message.Parts = append(message.Parts, map[string]interface{}{
			"kind": string(types.MessagePartKindText),
			"text": "",
		})
	}

	hasReasoning := (response.ReasoningContent != nil && *response.ReasoningContent != "") ||
		(response.Reasoning != nil && *response.Reasoning != "")

	c.logger.Debug("converted SDK message to A2A format",
		zap.String("role", role),
		zap.String("content", response.Content),
		zap.Bool("has_tool_calls", response.ToolCalls != nil),
		zap.Bool("has_reasoning", hasReasoning),
		zap.Int("parts_count", len(message.Parts)))

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
