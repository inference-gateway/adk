package utils

import (
	"testing"

	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zap "go.uber.org/zap"
)

func TestMessageConverter_ConvertToSDK(t *testing.T) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

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
					MessageID: "test-msg-1",
					Role:      types.RoleUser,
					Parts: []types.Part{
						types.CreateTextPart("Hello, world!"),
					},
				},
			},
			expectedOutput: []sdk.Message{
				func() sdk.Message {
					msg, _ := sdk.NewTextMessage(sdk.User, "Hello, world!")
					return msg
				}(),
			},
			expectError: false,
		},
		{
			name: "convert assistant message",
			input: []types.Message{
				{
					MessageID: "test-msg-2",
					Role:      types.RoleAgent,
					Parts: []types.Part{
						types.CreateTextPart("Hi there!"),
					},
				},
			},
			expectedOutput: []sdk.Message{
				func() sdk.Message {
					msg, _ := sdk.NewTextMessage(sdk.Assistant, "Hi there!")
					return msg
				}(),
			},
			expectError: false,
		},
		{
			name: "convert agent message treated as system-like (A2A doesn't have system role)",
			input: []types.Message{
				{
					MessageID: "test-msg-3",
					Role:      types.RoleAgent,
					Parts: []types.Part{
						types.CreateTextPart("You are a helpful assistant."),
					},
				},
			},
			expectedOutput: []sdk.Message{
				func() sdk.Message {
					msg, _ := sdk.NewTextMessage(sdk.Assistant, "You are a helpful assistant.")
					return msg
				}(),
			},
			expectError: false,
		},
		{
			name: "convert message with empty role defaults to user",
			input: []types.Message{
				{
					MessageID: "test-msg-4",
					Role:      "",
					Parts: []types.Part{
						types.CreateTextPart("Default role test"),
					},
				},
			},
			expectedOutput: []sdk.Message{
				func() sdk.Message {
					msg, _ := sdk.NewTextMessage(sdk.User, "Default role test")
					return msg
				}(),
			},
			expectError: false,
		},
		{
			name: "convert message with multiple text parts",
			input: []types.Message{
				{
					MessageID: "test-msg-5",
					Role:      types.RoleUser,
					Parts: []types.Part{
						types.CreateTextPart("Part 1. "),
						types.CreateTextPart("Part 2."),
					},
				},
			},
			expectedOutput: []sdk.Message{
				func() sdk.Message {
					msg, _ := sdk.NewTextMessage(sdk.User, "Part 1. Part 2.")
					return msg
				}(),
			},
			expectError: false,
		},
		{
			name: "convert message with data part (A2A doesn't have tool role, uses agent with tool_call_id)",
			input: []types.Message{
				{
					MessageID: "test-msg-6",
					Role:      types.RoleAgent,
					Parts: []types.Part{
						types.CreateDataPart(map[string]any{
							"tool_call_id": "call_test_function",
							"tool_name":    "test_function",
							"result":       "Tool execution result",
						}),
					},
				},
			},
			expectedOutput: []sdk.Message{
				func() sdk.Message {
					msg := sdk.Message{
						Role:       sdk.Tool,
						ToolCallId: stringPtr("call_test_function"),
					}
					_ = msg.Content.FromMessageContent0("Tool execution result")
					return msg
				}(),
			},
			expectError: false,
		},
		{
			name: "convert strongly-typed message part",
			input: []types.Message{
				{
					MessageID: "test-msg-7",
					Role:      types.RoleUser,
					Parts: []types.Part{
						types.CreateTextPart("Strongly typed message"),
					},
				},
			},
			expectedOutput: []sdk.Message{
				func() sdk.Message {
					msg, _ := sdk.NewTextMessage(sdk.User, "Strongly typed message")
					return msg
				}(),
			},
			expectError: false,
		},
		{
			name: "convert message with file part (no content extraction)",
			input: []types.Message{
				{
					MessageID: "test-msg-8",
					Role:      types.RoleUser,
					Parts: []types.Part{
						types.CreateTextPart("Please analyze this file: "),
						types.CreateFilePart("test.txt", "text/plain", stringPtr("base64encodedcontent"), nil),
					},
				},
			},
			expectedOutput: []sdk.Message{
				func() sdk.Message {
					msg, _ := sdk.NewTextMessage(sdk.User, "Please analyze this file: ")
					return msg
				}(),
			},
			expectError: false,
		},
		{
			name: "convert multiple messages",
			input: []types.Message{
				{
					MessageID: "test-msg-9",
					Role:      types.RoleUser,
					Parts: []types.Part{
						types.CreateTextPart("First message"),
					},
				},
				{
					MessageID: "test-msg-10",
					Role:      types.RoleAgent,
					Parts: []types.Part{
						types.CreateTextPart("Second message"),
					},
				},
			},
			expectedOutput: []sdk.Message{
				func() sdk.Message {
					msg, _ := sdk.NewTextMessage(sdk.User, "First message")
					return msg
				}(),
				func() sdk.Message {
					msg, _ := sdk.NewTextMessage(sdk.Assistant, "Second message")
					return msg
				}(),
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

func TestMessageConverter_ConvertFromSDK(t *testing.T) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	tests := []struct {
		name           string
		input          sdk.Message
		expectedOutput *types.Message
		expectError    bool
	}{
		{
			name: "convert SDK user message",
			input: func() sdk.Message {
				msg, _ := sdk.NewTextMessage(sdk.User, "Hello from SDK")
				return msg
			}(),
			expectedOutput: &types.Message{
				Role: types.RoleUser,
				Parts: []types.Part{
					types.CreateTextPart("Hello from SDK"),
				},
			},
			expectError: false,
		},
		{
			name: "convert SDK assistant message",
			input: func() sdk.Message {
				msg, _ := sdk.NewTextMessage(sdk.Assistant, "Response from assistant")
				return msg
			}(),
			expectedOutput: &types.Message{
				Role: types.RoleAgent,
				Parts: []types.Part{
					types.CreateTextPart("Response from assistant"),
				},
			},
			expectError: false,
		},
		{
			name: "convert SDK system message (A2A maps to agent role)",
			input: func() sdk.Message {
				msg, _ := sdk.NewTextMessage(sdk.System, "System instructions")
				return msg
			}(),
			expectedOutput: &types.Message{
				Role: types.RoleAgent,
				Parts: []types.Part{
					types.CreateTextPart("System instructions"),
				},
			},
			expectError: false,
		},
		{
			name: "convert SDK tool message (A2A maps to agent role)",
			input: func() sdk.Message {
				msg := sdk.Message{
					Role:       sdk.Tool,
					ToolCallId: func() *string { s := "call_123"; return &s }(),
				}
				_ = msg.Content.FromMessageContent0("Tool response")
				return msg
			}(),
			expectedOutput: &types.Message{
				Role: types.RoleAgent,
				Parts: []types.Part{
					types.CreateDataPart(map[string]any{
						"tool_call_id": "call_123",
						"tool_name":    "",
						"result":       "Tool response",
					}),
				},
			},
			expectError: false,
		},
		{
			name: "convert SDK message with tool calls",
			input: func() sdk.Message {
				msg := sdk.Message{
					Role: sdk.Assistant,
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
				}
				_ = msg.Content.FromMessageContent0("I'll help you with that")
				return msg
			}(),
			expectedOutput: &types.Message{
				Role: types.RoleAgent,
				Parts: []types.Part{
					types.CreateTextPart("I'll help you with that"),
					types.CreateDataPart(map[string]any{
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
					}),
				},
			},
			expectError: false,
		},
		{
			name: "convert SDK message with empty content",
			input: func() sdk.Message {
				msg, _ := sdk.NewTextMessage(sdk.Assistant, "")
				return msg
			}(),
			expectedOutput: &types.Message{
				Role: types.RoleAgent,
				Parts: []types.Part{
					types.CreateTextPart(""),
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
			assert.Equal(t, tt.expectedOutput.Role, result.Role)
			assert.Equal(t, len(tt.expectedOutput.Parts), len(result.Parts))

			for i, expectedPart := range tt.expectedOutput.Parts {
				resultPart := result.Parts[i]

				if expectedPart.Text != nil {
					require.NotNil(t, resultPart.Text, "Expected result part to have Text")
					assert.Equal(t, *expectedPart.Text, *resultPart.Text)
				}

				if expectedPart.Data != nil {
					require.NotNil(t, resultPart.Data, "Expected result part to have Data")
					assert.Equal(t, expectedPart.Data.Data, resultPart.Data.Data)
				}

				if expectedPart.File != nil {
					require.NotNil(t, resultPart.File, "Expected result part to have File")
					assert.Equal(t, expectedPart.File.Name, resultPart.File.Name)
					assert.Equal(t, expectedPart.File.MediaType, resultPart.File.MediaType)
				}

				if expectedPart.Metadata != nil {
					assert.Equal(t, expectedPart.Metadata, resultPart.Metadata)
				}
			}
		})
	}
}

func TestMessageConverter_ValidateMessagePart(t *testing.T) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	tests := []struct {
		name        string
		input       types.Part
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid text part",
			input:       types.CreateTextPart("Valid text"),
			expectError: false,
		},
		{
			name:        "valid file part",
			input:       types.CreateFilePart("test.txt", "text/plain", nil, nil),
			expectError: false,
		},
		{
			name: "valid data part",
			input: types.CreateDataPart(map[string]any{
				"key": "value",
			}),
			expectError: false,
		},
		{
			name:        "invalid text part (empty text)",
			input:       types.CreateTextPart(""),
			expectError: true,
			errorMsg:    "text part has empty text field",
		},
		{
			name: "invalid file part (missing name)",
			input: types.Part{
				File: &types.FilePart{
					Name:      "",
					MediaType: "text/plain",
				},
			},
			expectError: true,
			errorMsg:    "file part missing name",
		},
		{
			name: "invalid data part (nil data)",
			input: types.Part{
				Data: &types.DataPart{
					Data: nil,
				},
			},
			expectError: true,
			errorMsg:    "data part missing data field",
		},
		{
			name: "invalid part with no fields set",
			input: types.Part{
				Text: nil,
				Data: nil,
				File: nil,
			},
			expectError: true,
			errorMsg:    "part must have at least one field set",
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

func TestMessageConverter_RoundTrip(t *testing.T) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	originalMessage := types.Message{
		MessageID: "round-trip-test",
		Role:      types.RoleUser,
		Parts: []types.Part{
			types.CreateTextPart("Round trip test message"),
		},
	}

	sdkMessages, err := converter.ConvertToSDK([]types.Message{originalMessage})
	require.NoError(t, err)
	require.Len(t, sdkMessages, 1)

	convertedMessage, err := converter.ConvertFromSDK(sdkMessages[0])
	require.NoError(t, err)

	assert.Equal(t, originalMessage.Role, convertedMessage.Role)
	assert.Len(t, convertedMessage.Parts, 1)

	convertedPart := convertedMessage.Parts[0]
	require.NotNil(t, convertedPart.Text, "Expected converted part to have Text")
	assert.Equal(t, "Round trip test message", *convertedPart.Text)
}

func TestMessageConverter_PerformanceWithManyMessages(t *testing.T) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	messages := make([]types.Message, 1000)
	for i := 0; i < 1000; i++ {
		messages[i] = types.Message{
			MessageID: "perf-test-" + string(rune(i)),
			Role:      types.RoleUser,
			Parts: []types.Part{
				types.CreateTextPart("Performance test message number " + string(rune(i))),
			},
		}
	}

	result, err := converter.ConvertToSDK(messages)
	require.NoError(t, err)
	assert.Len(t, result, 1000)

	for _, sdkMsg := range result {
		assert.Equal(t, sdk.User, sdkMsg.Role)
		content, _ := sdkMsg.Content.AsMessageContent0()
		assert.Contains(t, content, "Performance test message")
	}
}

func TestMessageConverter_ConvertToSDK_ToolCalls(t *testing.T) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

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
				Role:      types.RoleAgent,
				Parts: []types.Part{
					types.CreateDataPart(map[string]any{
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
					}),
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
				Role:      types.RoleAgent,
				Parts: []types.Part{
					types.CreateTextPart("Hello, how can I help you?"),
				},
			},
			expectedToolCalls: nil,
			expectedContent:   "Hello, how can I help you?",
		},
		{
			name: "user message with text (should not extract tool_calls)",
			inputMessage: types.Message{
				MessageID: "test-user-msg",
				Role:      types.RoleUser,
				Parts: []types.Part{
					types.CreateDataPart(map[string]any{
						"tool_calls": []sdk.ChatCompletionMessageToolCall{
							{
								Id:   "call_456",
								Type: sdk.ChatCompletionToolType("function"),
							},
						},
						"result": "User content",
					}),
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
			content, err := sdkMsg.Content.AsMessageContent0()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedContent, content)

			if tt.expectedToolCalls == nil {
				assert.Nil(t, sdkMsg.ToolCalls)
			} else {
				require.NotNil(t, sdkMsg.ToolCalls)
				assert.Equal(t, *tt.expectedToolCalls, *sdkMsg.ToolCalls)
			}
		})
	}
}

func TestMessageConverter_ConvertToSDK_ToolCallsSequence(t *testing.T) {
	logger := zap.NewNop()
	converter := NewMessageConverter(logger)

	messages := []types.Message{
		{
			MessageID: "user-msg",
			Role:      types.RoleUser,
			Parts: []types.Part{
				types.CreateTextPart("What's on my calendar today?"),
			},
		},
		{
			MessageID: "assistant-msg",
			Role:      types.RoleAgent,
			Parts: []types.Part{
				types.CreateDataPart(map[string]any{
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
				}),
			},
		},
		{
			MessageID: "tool-result-msg",
			Role:      types.RoleAgent,
			Parts: []types.Part{
				types.CreateDataPart(map[string]any{
					"tool_call_id": "call_0_2e5a532f-06e2-4ced-8434-31e25019e144",
					"tool_name":    "list_calendar_events",
					"result":       `{"message":"Found 0 events between 2025-06-16 00:00 and 2025-06-16 23:59","success":true}`,
				}),
			},
		},
	}

	result, err := converter.ConvertToSDK(messages)
	require.NoError(t, err)
	require.Len(t, result, 3)

	userMsg := result[0]
	assert.Equal(t, sdk.User, userMsg.Role)
	userContent, _ := userMsg.Content.AsMessageContent0()
	assert.Equal(t, "What's on my calendar today?", userContent)
	assert.Nil(t, userMsg.ToolCalls)
	assert.Nil(t, userMsg.ToolCallId)

	assistantMsg := result[1]
	assert.Equal(t, sdk.Assistant, assistantMsg.Role)
	assistantContent, _ := assistantMsg.Content.AsMessageContent0()
	assert.Equal(t, "", assistantContent)
	require.NotNil(t, assistantMsg.ToolCalls)
	require.Len(t, *assistantMsg.ToolCalls, 1)

	toolCall := (*assistantMsg.ToolCalls)[0]
	assert.Equal(t, "call_0_2e5a532f-06e2-4ced-8434-31e25019e144", toolCall.Id)
	assert.Equal(t, sdk.ChatCompletionToolType("function"), toolCall.Type)
	assert.Equal(t, "list_calendar_events", toolCall.Function.Name)
	assert.Equal(t, `{"start_date":"2025-06-16","end_date":"2025-06-16"}`, toolCall.Function.Arguments)

	toolMsg := result[2]
	assert.Equal(t, sdk.Tool, toolMsg.Role)
	toolContent, _ := toolMsg.Content.AsMessageContent0()
	assert.Contains(t, toolContent, "Found 0 events between 2025-06-16 00:00 and 2025-06-16 23:59")
	assert.Nil(t, toolMsg.ToolCalls)
	require.NotNil(t, toolMsg.ToolCallId)
	assert.Equal(t, "call_0_2e5a532f-06e2-4ced-8434-31e25019e144", *toolMsg.ToolCallId)
}

func stringPtr(s string) *string {
	return &s
}
