package utils

import (
	"testing"

	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zap "go.uber.org/zap"
)

func TestOptimizedMessageConverter_ConvertToSDK(t *testing.T) {
	logger := zap.NewNop()
	converter := NewOptimizedMessageConverter(logger)

	tests := []struct {
		name           string
		input          []types.Message
		expectedOutput []sdk.Message
		expectError    bool
	}{
		{
			name: "convert simple text message",
			input: []types.Message{
				{
					Kind:      "message",
					MessageID: "test-msg-1",
					Role:      "user",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Hello, world!",
						},
					},
				},
			},
			expectedOutput: []sdk.Message{
				{
					Role:    sdk.User,
					Content: "Hello, world!",
				},
			},
			expectError: false,
		},
		{
			name: "convert assistant message",
			input: []types.Message{
				{
					Kind:      "message",
					MessageID: "test-msg-2",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Hi there!",
						},
					},
				},
			},
			expectedOutput: []sdk.Message{
				{
					Role:    sdk.Assistant,
					Content: "Hi there!",
				},
			},
			expectError: false,
		},
		{
			name: "convert system message",
			input: []types.Message{
				{
					Kind:      "message",
					MessageID: "test-msg-3",
					Role:      "system",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "You are a helpful assistant.",
						},
					},
				},
			},
			expectedOutput: []sdk.Message{
				{
					Role:    sdk.System,
					Content: "You are a helpful assistant.",
				},
			},
			expectError: false,
		},
		{
			name: "convert message with empty role defaults to user",
			input: []types.Message{
				{
					Kind:      "message",
					MessageID: "test-msg-4",
					Role:      "",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Default role test",
						},
					},
				},
			},
			expectedOutput: []sdk.Message{
				{
					Role:    sdk.User,
					Content: "Default role test",
				},
			},
			expectError: false,
		},
		{
			name: "convert message with multiple text parts",
			input: []types.Message{
				{
					Kind:      "message",
					MessageID: "test-msg-5",
					Role:      "user",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Part 1. ",
						},
						map[string]interface{}{
							"kind": "text",
							"text": "Part 2.",
						},
					},
				},
			},
			expectedOutput: []sdk.Message{
				{
					Role:    sdk.User,
					Content: "Part 1. Part 2.",
				},
			},
			expectError: false,
		},
		{
			name: "convert message with data part",
			input: []types.Message{
				{
					Kind:      "message",
					MessageID: "test-msg-6",
					Role:      "tool",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "data",
							"data": map[string]interface{}{
								"tool_call_id": "call_test_function",
								"tool_name":    "test_function",
								"result":       "Tool execution result",
							},
						},
					},
				},
			},
			expectedOutput: []sdk.Message{
				{
					Role:       sdk.Tool,
					Content:    "Tool execution result",
					ToolCallId: stringPtr("call_test_function"),
				},
			},
			expectError: false,
		},
		{
			name: "convert strongly-typed message part",
			input: []types.Message{
				{
					Kind:      "message",
					MessageID: "test-msg-7",
					Role:      "user",
					Parts: []types.Part{
						types.OptimizedMessagePart{
							Kind: types.MessagePartKindText,
							Text: stringPtr("Strongly typed message"),
						},
					},
				},
			},
			expectedOutput: []sdk.Message{
				{
					Role:    sdk.User,
					Content: "Strongly typed message",
				},
			},
			expectError: false,
		},
		{
			name: "convert message with file part (no content extraction)",
			input: []types.Message{
				{
					Kind:      "message",
					MessageID: "test-msg-8",
					Role:      "user",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Please analyze this file: ",
						},
						types.OptimizedMessagePart{
							Kind: types.MessagePartKindFile,
							File: &types.FileData{
								Name:     stringPtr("test.txt"),
								MIMEType: stringPtr("text/plain"),
								Bytes:    stringPtr("base64encodedcontent"),
							},
						},
					},
				},
			},
			expectedOutput: []sdk.Message{
				{
					Role:    sdk.User,
					Content: "Please analyze this file: ",
				},
			},
			expectError: false,
		},
		{
			name: "convert multiple messages",
			input: []types.Message{
				{
					Kind:      "message",
					MessageID: "test-msg-9",
					Role:      "user",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "First message",
						},
					},
				},
				{
					Kind:      "message",
					MessageID: "test-msg-10",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Second message",
						},
					},
				},
			},
			expectedOutput: []sdk.Message{
				{
					Role:    sdk.User,
					Content: "First message",
				},
				{
					Role:    sdk.Assistant,
					Content: "Second message",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToSDK(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, len(tt.expectedOutput), len(result))

			for i, expected := range tt.expectedOutput {
				assert.Equal(t, expected.Role, result[i].Role)
				assert.Equal(t, expected.Content, result[i].Content)
			}
		})
	}
}

func TestOptimizedMessageConverter_ConvertFromSDK(t *testing.T) {
	logger := zap.NewNop()
	converter := NewOptimizedMessageConverter(logger)

	tests := []struct {
		name           string
		input          sdk.Message
		expectedOutput *types.Message
		expectError    bool
	}{
		{
			name: "convert SDK user message",
			input: sdk.Message{
				Role:    sdk.User,
				Content: "Hello from SDK",
			},
			expectedOutput: &types.Message{
				Kind: "message",
				Role: "user",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Hello from SDK",
					},
				},
			},
			expectError: false,
		},
		{
			name: "convert SDK assistant message",
			input: sdk.Message{
				Role:    sdk.Assistant,
				Content: "Response from assistant",
			},
			expectedOutput: &types.Message{
				Kind: "message",
				Role: "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Response from assistant",
					},
				},
			},
			expectError: false,
		},
		{
			name: "convert SDK system message",
			input: sdk.Message{
				Role:    sdk.System,
				Content: "System instructions",
			},
			expectedOutput: &types.Message{
				Kind: "message",
				Role: "system",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "System instructions",
					},
				},
			},
			expectError: false,
		},
		{
			name: "convert SDK tool message",
			input: sdk.Message{
				Role:       sdk.Tool,
				Content:    "Tool response",
				ToolCallId: func() *string { s := "call_123"; return &s }(),
			},
			expectedOutput: &types.Message{
				Kind: "message",
				Role: "tool",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "data",
						"data": map[string]interface{}{
							"tool_call_id": "call_123",
							"tool_name":    "",
							"result":       "Tool response",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "convert SDK message with tool calls",
			input: sdk.Message{
				Role:    sdk.Assistant,
				Content: "I'll help you with that",
				ToolCalls: &[]sdk.ChatCompletionMessageToolCall{
					{
						Id:   "call_123",
						Type: "function",
						Function: sdk.ChatCompletionMessageToolCallFunction{
							Name:      "get_weather",
							Arguments: `{"location": "New York"}`,
						},
					},
				},
			},
			expectedOutput: &types.Message{
				Kind: "message",
				Role: "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "I'll help you with that",
					},
					map[string]interface{}{
						"kind": "data",
						"data": map[string]interface{}{
							"tool_calls": []sdk.ChatCompletionMessageToolCall{
								{
									Id:   "call_123",
									Type: "function",
									Function: sdk.ChatCompletionMessageToolCallFunction{
										Name:      "get_weather",
										Arguments: `{"location": "New York"}`,
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "convert SDK message with empty content",
			input: sdk.Message{
				Role:    sdk.Assistant,
				Content: "",
			},
			expectedOutput: &types.Message{
				Kind: "message",
				Role: "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "",
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertFromSDK(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedOutput.Kind, result.Kind)
			assert.Equal(t, tt.expectedOutput.Role, result.Role)
			assert.Equal(t, len(tt.expectedOutput.Parts), len(result.Parts))

			for i, expectedPart := range tt.expectedOutput.Parts {
				if partMap, ok := expectedPart.(map[string]interface{}); ok {
					resultPartMap, ok := result.Parts[i].(map[string]interface{})
					require.True(t, ok, "Expected result part to be map[string]interface{}")
					assert.Equal(t, partMap["kind"], resultPartMap["kind"])

					switch partMap["kind"] {
					case "text":
						assert.Equal(t, partMap["text"], resultPartMap["text"])
					case "data":
						expectedData := partMap["data"].(map[string]interface{})
						resultData := resultPartMap["data"].(map[string]interface{})
						assert.Equal(t, expectedData, resultData)
					}
				}
			}
		})
	}
}

func TestOptimizedMessageConverter_ValidateMessagePart(t *testing.T) {
	logger := zap.NewNop()
	converter := NewOptimizedMessageConverter(logger)

	tests := []struct {
		name        string
		input       types.Part
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid strongly-typed text part",
			input: types.OptimizedMessagePart{
				Kind: types.MessagePartKindText,
				Text: stringPtr("Valid text"),
			},
			expectError: false,
		},
		{
			name: "valid strongly-typed file part",
			input: types.OptimizedMessagePart{
				Kind: types.MessagePartKindFile,
				File: &types.FileData{
					Name:     stringPtr("test.txt"),
					MIMEType: stringPtr("text/plain"),
				},
			},
			expectError: false,
		},
		{
			name: "valid strongly-typed data part",
			input: types.OptimizedMessagePart{
				Kind: types.MessagePartKindData,
				Data: map[string]interface{}{
					"key": "value",
				},
			},
			expectError: false,
		},
		{
			name: "invalid strongly-typed text part (missing text)",
			input: types.OptimizedMessagePart{
				Kind: types.MessagePartKindText,
				Text: nil,
			},
			expectError: true,
			errorMsg:    "text part missing text field",
		},
		{
			name: "invalid strongly-typed file part (missing file)",
			input: types.OptimizedMessagePart{
				Kind: types.MessagePartKindFile,
				File: nil,
			},
			expectError: true,
			errorMsg:    "file part missing file field",
		},
		{
			name: "invalid strongly-typed data part (missing data)",
			input: types.OptimizedMessagePart{
				Kind: types.MessagePartKindData,
				Data: nil,
			},
			expectError: true,
			errorMsg:    "data part missing data field",
		},
		{
			name: "valid map-based text part",
			input: map[string]interface{}{
				"kind": "text",
				"text": "Valid text content",
			},
			expectError: false,
		},
		{
			name: "valid map-based data part",
			input: map[string]interface{}{
				"kind": "data",
				"data": map[string]interface{}{
					"result": "some result",
				},
			},
			expectError: false,
		},
		{
			name: "valid map-based file part",
			input: map[string]interface{}{
				"kind": "file",
				"file": map[string]interface{}{
					"name":     "test.txt",
					"mimeType": "text/plain",
				},
			},
			expectError: false,
		},
		{
			name: "invalid map-based part (missing kind)",
			input: map[string]interface{}{
				"text": "Missing kind field",
			},
			expectError: true,
			errorMsg:    "message part missing kind field",
		},
		{
			name: "invalid map-based part (non-string kind)",
			input: map[string]interface{}{
				"kind": 123,
				"text": "Invalid kind type",
			},
			expectError: true,
			errorMsg:    "message part kind must be string",
		},
		{
			name: "invalid map-based part (invalid kind value)",
			input: map[string]interface{}{
				"kind": "invalid_kind",
				"text": "Invalid kind value",
			},
			expectError: true,
			errorMsg:    "invalid message part kind: invalid_kind",
		},
		{
			name:        "unsupported part type",
			input:       "unsupported string part",
			expectError: true,
			errorMsg:    "unsupported message part type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := converter.ValidateMessagePart(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOptimizedMessageConverter_RoundTrip(t *testing.T) {
	logger := zap.NewNop()
	converter := NewOptimizedMessageConverter(logger)

	originalMessage := types.Message{
		Kind:      "message",
		MessageID: "round-trip-test",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Round trip test message",
			},
		},
	}

	sdkMessages, err := converter.ConvertToSDK([]types.Message{originalMessage})
	require.NoError(t, err)
	require.Len(t, sdkMessages, 1)

	convertedMessage, err := converter.ConvertFromSDK(sdkMessages[0])
	require.NoError(t, err)

	assert.Equal(t, originalMessage.Kind, convertedMessage.Kind)
	assert.Equal(t, originalMessage.Role, convertedMessage.Role)
	assert.Len(t, convertedMessage.Parts, 1)

	originalPart := originalMessage.Parts[0].(map[string]interface{})
	convertedPart := convertedMessage.Parts[0].(map[string]interface{})
	assert.Equal(t, originalPart["kind"], convertedPart["kind"])
	assert.Equal(t, originalPart["text"], convertedPart["text"])
}

func TestOptimizedMessageConverter_PerformanceWithManyMessages(t *testing.T) {
	logger := zap.NewNop()
	converter := NewOptimizedMessageConverter(logger)

	messages := make([]types.Message, 1000)
	for i := 0; i < 1000; i++ {
		messages[i] = types.Message{
			Kind:      "message",
			MessageID: "perf-test-" + string(rune(i)),
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Performance test message number " + string(rune(i)),
				},
			},
		}
	}

	result, err := converter.ConvertToSDK(messages)
	require.NoError(t, err)
	assert.Len(t, result, 1000)

	for _, sdkMsg := range result {
		assert.Equal(t, sdk.User, sdkMsg.Role)
		assert.Contains(t, sdkMsg.Content, "Performance test message")
	}
}

func TestOptimizedMessageConverter_ConvertToSDK_ToolCalls(t *testing.T) {
	logger := zap.NewNop()
	converter := NewOptimizedMessageConverter(logger)

	tests := []struct {
		name              string
		inputMessage      types.Message
		expectedToolCalls *[]sdk.ChatCompletionMessageToolCall
		expectedContent   string
	}{
		{
			name: "assistant message with tool_calls in data part",
			inputMessage: types.Message{
				MessageID: "test-assistant-msg",
				Role:      "assistant",
				Kind:      "message",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "data",
						"data": map[string]interface{}{
							"tool_calls": []sdk.ChatCompletionMessageToolCall{
								{
									Id:   "call_123",
									Type: sdk.ChatCompletionToolType("function"),
									Function: sdk.ChatCompletionMessageToolCallFunction{
										Name:      "test_tool",
										Arguments: `{"param":"value"}`,
									},
								},
							},
							"content": "I'll help you with that.",
						},
					},
				},
			},
			expectedToolCalls: &[]sdk.ChatCompletionMessageToolCall{
				{
					Id:   "call_123",
					Type: sdk.ChatCompletionToolType("function"),
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "test_tool",
						Arguments: `{"param":"value"}`,
					},
				},
			},
			expectedContent: "I'll help you with that.",
		},
		{
			name: "assistant message with only content, no tool_calls",
			inputMessage: types.Message{
				MessageID: "test-assistant-msg-2",
				Role:      "assistant",
				Kind:      "message",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Hello, how can I help you?",
					},
				},
			},
			expectedToolCalls: nil,
			expectedContent:   "Hello, how can I help you?",
		},
		{
			name: "user message with text (should not extract tool_calls)",
			inputMessage: types.Message{
				MessageID: "test-user-msg",
				Role:      "user",
				Kind:      "message",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "data",
						"data": map[string]interface{}{
							"tool_calls": []sdk.ChatCompletionMessageToolCall{
								{
									Id:   "call_456",
									Type: sdk.ChatCompletionToolType("function"),
								},
							},
							"result": "User content",
						},
					},
				},
			},
			expectedToolCalls: nil,
			expectedContent:   "User content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToSDK([]types.Message{tt.inputMessage})
			require.NoError(t, err)
			require.Len(t, result, 1)

			sdkMsg := result[0]
			assert.Equal(t, tt.expectedContent, sdkMsg.Content)

			if tt.expectedToolCalls == nil {
				assert.Nil(t, sdkMsg.ToolCalls)
			} else {
				require.NotNil(t, sdkMsg.ToolCalls)
				assert.Equal(t, *tt.expectedToolCalls, *sdkMsg.ToolCalls)
			}
		})
	}
}

func TestOptimizedMessageConverter_ConvertToSDK_ToolCallsSequence(t *testing.T) {
	logger := zap.NewNop()
	converter := NewOptimizedMessageConverter(logger)

	messages := []types.Message{
		{
			MessageID: "user-msg",
			Role:      "user",
			Kind:      "message",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "What's on my calendar today?",
				},
			},
		},
		{
			MessageID: "assistant-msg",
			Role:      "assistant",
			Kind:      "message",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "data",
					"data": map[string]interface{}{
						"tool_calls": []sdk.ChatCompletionMessageToolCall{
							{
								Id:   "call_0_2e5a532f-06e2-4ced-8434-31e25019e144",
								Type: sdk.ChatCompletionToolType("function"),
								Function: sdk.ChatCompletionMessageToolCallFunction{
									Name:      "list_calendar_events",
									Arguments: `{"start_date":"2025-06-16","end_date":"2025-06-16"}`,
								},
							},
						},
						"content": "",
					},
				},
			},
		},
		{
			MessageID: "tool-result-msg",
			Role:      "tool",
			Kind:      "message",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "data",
					"data": map[string]interface{}{
						"tool_call_id": "call_0_2e5a532f-06e2-4ced-8434-31e25019e144",
						"tool_name":    "list_calendar_events",
						"result":       `{"message":"Found 0 events between 2025-06-16 00:00 and 2025-06-16 23:59","success":true}`,
					},
				},
			},
		},
	}

	result, err := converter.ConvertToSDK(messages)
	require.NoError(t, err)
	require.Len(t, result, 3)

	userMsg := result[0]
	assert.Equal(t, sdk.User, userMsg.Role)
	assert.Equal(t, "What's on my calendar today?", userMsg.Content)
	assert.Nil(t, userMsg.ToolCalls)
	assert.Nil(t, userMsg.ToolCallId)

	assistantMsg := result[1]
	assert.Equal(t, sdk.Assistant, assistantMsg.Role)
	assert.Equal(t, "", assistantMsg.Content)
	require.NotNil(t, assistantMsg.ToolCalls)
	require.Len(t, *assistantMsg.ToolCalls, 1)

	toolCall := (*assistantMsg.ToolCalls)[0]
	assert.Equal(t, "call_0_2e5a532f-06e2-4ced-8434-31e25019e144", toolCall.Id)
	assert.Equal(t, sdk.ChatCompletionToolType("function"), toolCall.Type)
	assert.Equal(t, "list_calendar_events", toolCall.Function.Name)
	assert.Equal(t, `{"start_date":"2025-06-16","end_date":"2025-06-16"}`, toolCall.Function.Arguments)

	toolMsg := result[2]
	assert.Equal(t, sdk.Tool, toolMsg.Role)
	assert.Contains(t, toolMsg.Content, "Found 0 events between 2025-06-16 00:00 and 2025-06-16 23:59")
	assert.Nil(t, toolMsg.ToolCalls)
	require.NotNil(t, toolMsg.ToolCallId)
	assert.Equal(t, "call_0_2e5a532f-06e2-4ced-8434-31e25019e144", *toolMsg.ToolCallId)
}

func TestConversationHistoryPreservationIssue(t *testing.T) {
	logger := zap.NewNop()
	converter := NewOptimizedMessageConverter(logger)

	tests := []struct {
		name                    string
		maxConversationHistory  int
		conversationHistory     []types.Message
		systemPrompt            string
		newUserMessage          types.Message
		expectedMessageCount    int
		expectedSystemPrompt    bool
		expectedHistoryMessages int
		expectContextPreserved  bool
	}{
		{
			name:                   "context preserved with proper max history",
			maxConversationHistory: 20,
			conversationHistory: []types.Message{
				{
					MessageID: "msg-1",
					Role:      "user",
					Kind:      "message",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Can you move my meeting tomorrow from 14:00 to 15:00?",
						},
					},
				},
				{
					MessageID: "assistant-1",
					Role:      "assistant",
					Kind:      "message",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "I found a scheduled meeting with John Doe at 2:00 PM to 3:00 PM. Let me know if you'd like to modify it!",
						},
					},
				},
			},
			systemPrompt: "You are a helpful Google Calendar assistant.",
			newUserMessage: types.Message{
				MessageID: "msg-2",
				Role:      "user",
				Kind:      "message",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Just move it to 15:00?",
					},
				},
			},
			expectedMessageCount:    4,
			expectedSystemPrompt:    true,
			expectedHistoryMessages: 2,
			expectContextPreserved:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := make([]types.Message, 0)

			if tt.systemPrompt != "" {
				systemMessage := types.Message{
					Kind:      "message",
					MessageID: "system-prompt",
					Role:      "system",
					Parts: []types.Part{
						map[string]interface{}{
							"kind": "text",
							"text": tt.systemPrompt,
						},
					},
				}
				messages = append(messages, systemMessage)
			}

			trimmedHistory := tt.conversationHistory
			if tt.maxConversationHistory <= 0 {
				trimmedHistory = []types.Message{}
			} else if len(tt.conversationHistory) > tt.maxConversationHistory {
				startIndex := len(tt.conversationHistory) - tt.maxConversationHistory
				trimmedHistory = tt.conversationHistory[startIndex:]
			}

			messages = append(messages, trimmedHistory...)

			messages = append(messages, tt.newUserMessage)

			sdkMessages, err := converter.ConvertToSDK(messages)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedMessageCount, len(sdkMessages),
				"Expected %d messages but got %d", tt.expectedMessageCount, len(sdkMessages))

			if tt.expectedSystemPrompt {
				assert.Equal(t, sdk.System, sdkMessages[0].Role, "First message should be system prompt")
				assert.Contains(t, sdkMessages[0].Content, "Google Calendar assistant",
					"System prompt content should be preserved")
			}

			actualHistoryMessages := len(sdkMessages) - 1
			if tt.expectedSystemPrompt {
				actualHistoryMessages--
			}
			assert.Equal(t, tt.expectedHistoryMessages, actualHistoryMessages,
				"Expected %d history messages but got %d", tt.expectedHistoryMessages, actualHistoryMessages)

			if tt.expectContextPreserved {
				foundContext := false
				for _, msg := range sdkMessages {
					if msg.Role == sdk.Assistant && (contains(msg.Content, "John Doe") ||
						contains(msg.Content, "2:00 PM") ||
						contains(msg.Content, "scheduled meeting")) {
						foundContext = true
						break
					}
				}
				assert.True(t, foundContext, "Previous conversation context should be preserved")
			} else {
				foundContext := false
				for _, msg := range sdkMessages {
					if msg.Role == sdk.Assistant && (contains(msg.Content, "John Doe") ||
						contains(msg.Content, "2:00 PM") ||
						contains(msg.Content, "scheduled meeting")) {
						foundContext = true
						break
					}
				}
				assert.False(t, foundContext, "Previous conversation context should be lost when maxHistory=0")
			}
		})
	}
}

// Helper function to check if string contains substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringPtr(s string) *string {
	return &s
}
