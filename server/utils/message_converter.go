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

// messageConverter provides efficient conversion with type safety
type messageConverter struct {
	logger *zap.Logger
}

// NewMessageConverter creates a new message converter
func NewMessageConverter(logger *zap.Logger) MessageConverter {
	return &messageConverter{
		logger: logger,
	}
}

// ConvertToSDK converts A2A messages to SDK format with validation
func (c *messageConverter) ConvertToSDK(messages []types.Message) ([]sdk.Message, error) {
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
func (c *messageConverter) convertSingleMessage(msg types.Message) (sdk.Message, error) {
	role := msg.Role
	if role == "" {
		role = "user"
	}

	var content string
	var toolCallId *string
	var toolCalls *[]sdk.ChatCompletionMessageToolCall

	for _, part := range msg.Parts {
		switch p := part.(type) {
		case types.TextPart:
			content += p.Text

		case types.DataPart:
			if err := c.processDataPart(p.Data, role, &content, &toolCallId, &toolCalls); err != nil {
				c.logger.Warn("failed to process DataPart",
					zap.String("message_id", msg.MessageID),
					zap.Error(err))
			}

		case types.FilePart:
			c.logger.Debug("file part detected in message",
				zap.String("message_id", msg.MessageID))

		case map[string]any:
			if err := c.processMapPart(p, role, &content, &toolCallId, &toolCalls); err != nil {
				c.logger.Warn("failed to process map part",
					zap.String("message_id", msg.MessageID),
					zap.Error(err))
			}

		default:
			c.logger.Warn("unsupported part type",
				zap.String("message_id", msg.MessageID),
				zap.String("type", fmt.Sprintf("%T", part)))
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

	sdkMsg := sdk.Message{
		Role:       sdkRole,
		ToolCallId: toolCallId,
		ToolCalls:  toolCalls,
	}

	if err := sdkMsg.Content.FromMessageContent0(content); err != nil {
		return sdk.Message{}, fmt.Errorf("failed to set message content: %w", err)
	}

	return sdkMsg, nil
}

// processDataPart handles the extraction of data from data parts for both typed and map formats
func (c *messageConverter) processDataPart(
	data map[string]any,
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

// processMapPart handles map[string]any fallback (for backward compatibility)
func (c *messageConverter) processMapPart(
	partMap map[string]any,
	role string,
	content *string,
	toolCallId **string,
	toolCalls **[]sdk.ChatCompletionMessageToolCall,
) error {
	kind, hasKind := partMap["kind"]
	if !hasKind {
		return nil
	}

	switch kind {
	case "text":
		if text, exists := partMap["text"]; exists {
			if textStr, ok := text.(string); ok {
				*content += textStr
			}
		}

	case "data":
		if data, exists := partMap["data"]; exists {
			if dataMap, ok := data.(map[string]any); ok {
				return c.processDataPart(dataMap, role, content, toolCallId, toolCalls)
			}
		}
	}

	return nil
}

// extractSingleToolCall extracts a single tool call from data part
func (c *messageConverter) extractSingleToolCall(
	toolCallData any,
	toolCalls **[]sdk.ChatCompletionMessageToolCall,
) error {
	if toolCallMap, ok := toolCallData.(map[string]any); ok {
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
func (c *messageConverter) extractToolCallsArray(
	toolCallsData any,
	toolCalls **[]sdk.ChatCompletionMessageToolCall,
) error {
	if toolCallsSlice, ok := toolCallsData.([]sdk.ChatCompletionMessageToolCall); ok {
		*toolCalls = &toolCallsSlice
		return nil
	}

	if toolCallsInterface, ok := toolCallsData.([]any); ok {
		var extractedToolCalls []sdk.ChatCompletionMessageToolCall
		for _, item := range toolCallsInterface {
			if toolCallMap, ok := item.(map[string]any); ok {
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
func (c *messageConverter) mapToToolCall(toolCallMap map[string]any) (sdk.ChatCompletionMessageToolCall, error) {
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
		if functionMap, ok := function.(map[string]any); ok {
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
func (c *messageConverter) ConvertFromSDK(response sdk.Message) (*types.Message, error) {
	role := string(response.Role)
	messageID := fmt.Sprintf("%s-%d", role, time.Now().UnixNano())

	message := &types.Message{
		Kind:      "message",
		MessageID: messageID,
		Role:      role,
		Parts:     []types.Part{},
	}

	content, err := response.Content.AsMessageContent0()
	if err != nil {
		c.logger.Debug("content is not a string, treating as empty",
			zap.Error(err))
		content = ""
	}

	switch role {
	case "tool":
		toolData := map[string]any{
			"result": content,
		}

		if response.ToolCallId != nil {
			toolData["tool_call_id"] = *response.ToolCallId
		}

		if response.ToolCalls != nil && len(*response.ToolCalls) > 0 {
			toolData["tool_name"] = (*response.ToolCalls)[0].Function.Name
		} else {
			toolData["tool_name"] = ""
		}

		message.Parts = append(message.Parts, types.CreateDataPart(toolData))

	case "assistant":
		hasToolCalls := response.ToolCalls != nil && len(*response.ToolCalls) > 0

		if content != "" {
			message.Parts = append(message.Parts, types.CreateTextPart(content))
		}

		if hasToolCalls {
			toolCallData := map[string]any{
				"tool_calls": *response.ToolCalls,
			}
			message.Parts = append(message.Parts, types.CreateDataPart(toolCallData))
		}

		if response.ReasoningContent != nil && *response.ReasoningContent != "" {
			message.Parts = append(message.Parts, types.CreateTextPart(*response.ReasoningContent))
		} else if response.Reasoning != nil && *response.Reasoning != "" {
			message.Parts = append(message.Parts, types.CreateTextPart(*response.Reasoning))
		}

	default:
		if content != "" {
			message.Parts = append(message.Parts, types.CreateTextPart(content))
		}
	}

	if len(message.Parts) == 0 {
		message.Parts = append(message.Parts, types.CreateTextPart(""))
	}

	hasReasoning := (response.ReasoningContent != nil && *response.ReasoningContent != "") ||
		(response.Reasoning != nil && *response.Reasoning != "")

	c.logger.Debug("converted SDK message to A2A format",
		zap.String("role", role),
		zap.String("content", content),
		zap.Bool("has_tool_calls", response.ToolCalls != nil),
		zap.Bool("has_reasoning", hasReasoning),
		zap.Int("parts_count", len(message.Parts)))

	return message, nil
}

// ValidateMessagePart validates message part structure and type
func (c *messageConverter) ValidateMessagePart(part types.Part) error {
	switch p := part.(type) {
	case types.TextPart:
		if p.Text == "" {
			return fmt.Errorf("text part missing text field")
		}
		return nil

	case types.DataPart:
		if p.Data == nil {
			return fmt.Errorf("data part missing data field")
		}
		return nil

	case types.FilePart:
		if p.File == nil {
			return fmt.Errorf("file part missing file field")
		}
		return nil

	case map[string]any:
		kind, hasKind := p["kind"]
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

	default:
		return fmt.Errorf("unsupported message part type: %T", part)
	}
}
