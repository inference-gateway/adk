package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestStreamingMessageAccumulation(t *testing.T) {
	tests := []struct {
		name                     string
		streamingMessages        []types.Message
		expectedConsolidatedText string
		expectedTaskState        types.TaskState
		description              string
	}{
		{
			name: "streaming_chunks_accumulated_correctly",
			streamingMessages: []types.Message{
				{
					Kind:      "message",
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Hello ",
						},
					},
				},
				{
					Kind:      "message",
					MessageID: "chunk-2",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "world!",
						},
					},
				},
				{
					Kind:      "message",
					MessageID: "chunk-3",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": " How are you?",
						},
					},
				},
			},
			expectedConsolidatedText: "Hello world! How are you?",
			expectedTaskState:        types.TaskStateCompleted,
			description:              "Multiple streaming chunks should be accumulated into a single consolidated message",
		},
		{
			name: "single_streaming_message",
			streamingMessages: []types.Message{
				{
					Kind:      "message",
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Single message response",
						},
					},
				},
			},
			expectedConsolidatedText: "Single message response",
			expectedTaskState:        types.TaskStateCompleted,
			description:              "Single streaming chunk should be consolidated correctly",
		},
		{
			name: "consolidated_message_with_final_assistant_message",
			streamingMessages: []types.Message{
				{
					Kind:      "message",
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Streaming ",
						},
					},
				},
				{
					Kind:      "message",
					MessageID: "chunk-2",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "content...",
						},
					},
				},
				{
					Kind:      "message",
					MessageID: "assistant-final",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Final consolidated response",
						},
					},
				},
			},
			expectedConsolidatedText: "Final consolidated response",
			expectedTaskState:        types.TaskStateCompleted,
			description:              "Non-chunk assistant message should be preferred over streaming chunks",
		},
		{
			name: "input_required_message_handling",
			streamingMessages: []types.Message{
				{
					Kind:      "message",
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Processing your request...",
						},
					},
				},
				{
					Kind:      "input_required",
					MessageID: "input-req-1",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Please provide additional information",
						},
					},
				},
			},
			expectedConsolidatedText: "Please provide additional information",
			expectedTaskState:        types.TaskStateInputRequired,
			description:              "Input required message should set task state correctly",
		},
		{
			name:                     "empty_streaming_messages",
			streamingMessages:        []types.Message{},
			expectedConsolidatedText: "",
			expectedTaskState:        types.TaskStateCompleted,
			description:              "Empty message list should be handled gracefully",
		},
		{
			name: "mixed_chunk_and_non_chunk_messages",
			streamingMessages: []types.Message{
				{
					Kind:      "message",
					MessageID: "user-1",
					Role:      "user",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "User message",
						},
					},
				},
				{
					Kind:      "message",
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Assistant ",
						},
					},
				},
				{
					Kind:      "message",
					MessageID: "chunk-2",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "response chunks",
						},
					},
				},
			},
			expectedConsolidatedText: "Assistant response chunks",
			expectedTaskState:        types.TaskStateCompleted,
			description:              "Mixed message types should accumulate assistant chunks correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &types.Task{
				ID:        "test-task-123",
				ContextID: "test-context-123",
				Status: types.TaskStatus{
					State: types.TaskStateWorking,
				},
				History: []types.Message{},
			}

			allMessages := tt.streamingMessages
			var lastMessage *types.Message
			if len(allMessages) > 0 {
				lastMessage = &allMessages[len(allMessages)-1]
			}

			if len(allMessages) > 0 {
				var consolidatedMessage *types.Message

				for i := len(allMessages) - 1; i >= 0; i-- {
					msg := &allMessages[i]
					if msg.Role == "assistant" && !strings.HasPrefix(msg.MessageID, "chunk-") {
						consolidatedMessage = msg
						break
					}
				}

				if consolidatedMessage == nil {
					var fullContent string
					var finalMessageID string

					for _, msg := range allMessages {
						if msg.Role == "assistant" && strings.HasPrefix(msg.MessageID, "chunk-") {
							for _, part := range msg.Parts {
								if partMap, ok := part.(map[string]any); ok {
									if text, exists := partMap["text"].(string); exists {
										fullContent += text
									}
								}
							}
							finalMessageID = msg.MessageID
						}
					}

					if fullContent != "" {
						consolidatedMessage = &types.Message{
							Kind:      "message",
							MessageID: strings.Replace(finalMessageID, "chunk-", "assistant-", 1),
							Role:      "assistant",
							Parts: []types.Part{
								map[string]any{
									"kind": "text",
									"text": fullContent,
								},
							},
						}
					}
				}

				if consolidatedMessage != nil {
					task.History = append(task.History, *consolidatedMessage)
				}
			}

			if lastMessage != nil && lastMessage.Kind == "input_required" {
				task.Status.State = types.TaskStateInputRequired
				task.Status.Message = lastMessage
			} else {
				task.Status.State = types.TaskStateCompleted
				if len(task.History) > 0 {
					task.Status.Message = &task.History[len(task.History)-1]
				} else {
					task.Status.Message = lastMessage
				}
			}

			assert.Equal(t, tt.expectedTaskState, task.Status.State, "Task state should match expected")

			if tt.expectedConsolidatedText != "" && task.Status.Message != nil {
				parts := task.Status.Message.Parts
				require.NotEmpty(t, parts, "Status message should have parts")

				if partMap, ok := parts[0].(map[string]any); ok {
					if text, exists := partMap["text"].(string); exists {
						assert.Equal(t, tt.expectedConsolidatedText, text, "Consolidated text should match expected")
					} else {
						t.Fatalf("Status message part should contain text field")
					}
				} else {
					t.Fatalf("Status message part should be a map")
				}
			} else if tt.expectedConsolidatedText == "" && len(allMessages) == 0 {
				assert.Nil(t, task.Status.Message, "Status message should be nil for empty message list")
			}
		})
	}
}

func TestStreamingMessageAccumulationPerformance(t *testing.T) {
	const numMessages = 1000
	messages := make([]types.Message, numMessages)

	for i := 0; i < numMessages; i++ {
		messages[i] = types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("chunk-%d", i+1),
			Role:      "assistant",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": fmt.Sprintf("Message %d ", i+1),
				},
			},
		}
	}

	start := time.Now()

	var fullContent string
	for _, msg := range messages {
		if msg.Role == "assistant" && strings.HasPrefix(msg.MessageID, "chunk-") {
			for _, part := range msg.Parts {
				if partMap, ok := part.(map[string]any); ok {
					if text, exists := partMap["text"].(string); exists {
						fullContent += text
					}
				}
			}
		}
	}

	duration := time.Since(start)
	t.Logf("Processing %d streaming messages took %v", numMessages, duration)

	assert.Less(t, duration, time.Second, "Performance test should complete quickly")
	assert.NotEmpty(t, fullContent, "Consolidated content should not be empty")
}

func TestStreamingMessageAccumulationEdgeCases(t *testing.T) {
	tests := []struct {
		name                     string
		streamingMessages        []types.Message
		expectedConsolidatedText string
		description              string
	}{
		{
			name: "malformed_message_parts",
			streamingMessages: []types.Message{
				{
					Kind:      "message",
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
						},
					},
				},
				{
					Kind:      "message",
					MessageID: "chunk-2",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Valid text",
						},
					},
				},
			},
			expectedConsolidatedText: "Valid text",
			description:              "Should handle malformed message parts gracefully",
		},
		{
			name: "non_string_text_field",
			streamingMessages: []types.Message{
				{
					Kind:      "message",
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": 123,
						},
					},
				},
			},
			expectedConsolidatedText: "",
			description:              "Should handle non-string text fields gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fullContent string
			for _, msg := range tt.streamingMessages {
				if msg.Role == "assistant" && strings.HasPrefix(msg.MessageID, "chunk-") {
					for _, part := range msg.Parts {
						if partMap, ok := part.(map[string]any); ok {
							if text, exists := partMap["text"].(string); exists {
								fullContent += text
							}
						}
					}
				}
			}

			assert.Equal(t, tt.expectedConsolidatedText, fullContent, "Consolidated text should match expected")
		})
	}
}

func TestToolCallAccumulator(t *testing.T) {
	tests := []struct {
		name              string
		toolCallChunks    [][]sdk.ChatCompletionMessageToolCallChunk
		expectedToolCalls []sdk.ChatCompletionMessageToolCall
		description       string
	}{
		{
			name: "single_tool_call_in_one_chunk",
			toolCallChunks: [][]sdk.ChatCompletionMessageToolCallChunk{
				{
					{
						Index: 0,
						ID:    "call_abc123",
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      "get_weather",
							Arguments: `{"location":"New York"}`,
						},
					},
				},
			},
			expectedToolCalls: []sdk.ChatCompletionMessageToolCall{
				{
					Id:   "call_abc123",
					Type: "function",
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"New York"}`,
					},
				},
			},
			description: "Single tool call delivered in one complete chunk",
		},
		{
			name: "tool_call_with_arguments_spread_across_deltas",
			toolCallChunks: [][]sdk.ChatCompletionMessageToolCallChunk{
				{
					{
						Index: 0,
						ID:    "call_xyz789",
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      "search_database",
							Arguments: `{"query":`,
						},
					},
				},
				{
					{
						Index: 0,
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Arguments: `"user data",`,
						},
					},
				},
				{
					{
						Index: 0,
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Arguments: `"limit":100}`,
						},
					},
				},
			},
			expectedToolCalls: []sdk.ChatCompletionMessageToolCall{
				{
					Id:   "call_xyz789",
					Type: "function",
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "search_database",
						Arguments: `{"query":"user data","limit":100}`,
					},
				},
			},
			description: "Tool call with arguments spread across multiple deltas (common with streaming providers)",
		},
		{
			name: "tool_call_without_id_initially",
			toolCallChunks: [][]sdk.ChatCompletionMessageToolCallChunk{
				{
					{
						Index: 0,
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      "calculate_sum",
							Arguments: `{"a":5,`,
						},
					},
				},
				{
					{
						Index: 0,
						ID:    "call_late_id",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Arguments: `"b":10}`,
						},
					},
				},
			},
			expectedToolCalls: []sdk.ChatCompletionMessageToolCall{
				{
					Id:   "call_late_id",
					Type: "function",
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "calculate_sum",
						Arguments: `{"a":5,"b":10}`,
					},
				},
			},
			description: "Tool call where ID is provided in a later chunk (some providers do this)",
		},
		{
			name: "multiple_tool_calls_interleaved",
			toolCallChunks: [][]sdk.ChatCompletionMessageToolCallChunk{
				{
					{
						Index: 0,
						ID:    "call_first",
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      "get_time",
							Arguments: `{"timezone":`,
						},
					},
					{
						Index: 1,
						ID:    "call_second",
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      "get_date",
							Arguments: `{"format":`,
						},
					},
				},
				{
					{
						Index: 0,
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Arguments: `"UTC"}`,
						},
					},
					{
						Index: 1,
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Arguments: `"ISO8601"}`,
						},
					},
				},
			},
			expectedToolCalls: []sdk.ChatCompletionMessageToolCall{
				{
					Id:   "call_first",
					Type: "function",
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "get_time",
						Arguments: `{"timezone":"UTC"}`,
					},
				},
				{
					Id:   "call_second",
					Type: "function",
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "get_date",
						Arguments: `{"format":"ISO8601"}`,
					},
				},
			},
			description: "Multiple tool calls with interleaved chunks using index to track them",
		},
		{
			name: "tool_call_with_empty_chunks",
			toolCallChunks: [][]sdk.ChatCompletionMessageToolCallChunk{
				{
					{
						Index: 0,
						ID:    "call_with_gaps",
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name: "process_data",
						},
					},
				},
				{},
				{
					{
						Index: 0,
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Arguments: `{"input":"test"}`,
						},
					},
				},
			},
			expectedToolCalls: []sdk.ChatCompletionMessageToolCall{
				{
					Id:   "call_with_gaps",
					Type: "function",
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "process_data",
						Arguments: `{"input":"test"}`,
					},
				},
			},
			description: "Tool call with empty chunks in between (network delays or provider quirks)",
		},
		{
			name: "tool_call_name_provided_later",
			toolCallChunks: [][]sdk.ChatCompletionMessageToolCallChunk{
				{
					{
						Index: 0,
						ID:    "call_name_later",
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Arguments: `{"param":`,
						},
					},
				},
				{
					{
						Index: 0,
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      "delayed_function",
							Arguments: `"value"}`,
						},
					},
				},
			},
			expectedToolCalls: []sdk.ChatCompletionMessageToolCall{
				{
					Id:   "call_name_later",
					Type: "function",
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "delayed_function",
						Arguments: `{"param":"value"}`,
					},
				},
			},
			description: "Tool call where function name is provided in a later chunk",
		},
		{
			name: "complex_json_arguments_spread",
			toolCallChunks: [][]sdk.ChatCompletionMessageToolCallChunk{
				{
					{
						Index: 0,
						ID:    "call_complex",
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      "process_order",
							Arguments: `{"items":[`,
						},
					},
				},
				{
					{
						Index: 0,
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Arguments: `{"id":1,"qty":2},`,
						},
					},
				},
				{
					{
						Index: 0,
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Arguments: `{"id":2,"qty":1}],`,
						},
					},
				},
				{
					{
						Index: 0,
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Arguments: `"total":99.99}`,
						},
					},
				},
			},
			expectedToolCalls: []sdk.ChatCompletionMessageToolCall{
				{
					Id:   "call_complex",
					Type: "function",
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "process_order",
						Arguments: `{"items":[{"id":1,"qty":2},{"id":2,"qty":1}],"total":99.99}`,
					},
				},
			},
			description: "Complex nested JSON arguments spread across multiple chunks",
		},
		{
			name: "tool_call_no_id_ever_provided",
			toolCallChunks: [][]sdk.ChatCompletionMessageToolCallChunk{
				{
					{
						Index: 0,
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      "anonymous_function",
							Arguments: `{"data":"test"}`,
						},
					},
				},
			},
			expectedToolCalls: []sdk.ChatCompletionMessageToolCall{
				{
					Id:   "",
					Type: "function",
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "anonymous_function",
						Arguments: `{"data":"test"}`,
					},
				},
			},
			description: "Tool call where no ID is ever provided (some providers omit this)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolCallAccumulator := make(map[string]*sdk.ChatCompletionMessageToolCall)

			for _, chunkBatch := range tt.toolCallChunks {
				for _, toolCallChunk := range chunkBatch {
					key := fmt.Sprintf("%d", toolCallChunk.Index)

					if toolCallAccumulator[key] == nil {
						toolCallAccumulator[key] = &sdk.ChatCompletionMessageToolCall{
							Type:     "function",
							Function: sdk.ChatCompletionMessageToolCallFunction{},
						}
					}

					toolCall := toolCallAccumulator[key]
					if toolCallChunk.ID != "" {
						toolCall.Id = toolCallChunk.ID
					}
					if toolCallChunk.Function.Name != "" {
						toolCall.Function.Name = toolCallChunk.Function.Name
					}
					if toolCallChunk.Function.Arguments != "" {
						if toolCall.Function.Arguments == "" {
							toolCall.Function.Arguments = toolCallChunk.Function.Arguments
						} else if !isCompleteJSON(toolCall.Function.Arguments) {
							toolCall.Function.Arguments += toolCallChunk.Function.Arguments
						}
					}
				}
			}

			var actualToolCalls []sdk.ChatCompletionMessageToolCall
			for i := 0; i < len(toolCallAccumulator); i++ {
				key := fmt.Sprintf("%d", i)
				if toolCall, exists := toolCallAccumulator[key]; exists {
					actualToolCalls = append(actualToolCalls, *toolCall)
				}
			}

			require.Equal(t, len(tt.expectedToolCalls), len(actualToolCalls),
				"Number of tool calls should match expected")

			for i, expected := range tt.expectedToolCalls {
				assert.Equal(t, expected.Id, actualToolCalls[i].Id,
					"Tool call ID should match")
				assert.Equal(t, expected.Type, actualToolCalls[i].Type,
					"Tool call type should match")
				assert.Equal(t, expected.Function.Name, actualToolCalls[i].Function.Name,
					"Function name should match")
				assert.Equal(t, expected.Function.Arguments, actualToolCalls[i].Function.Arguments,
					"Function arguments should match")
			}
		})
	}
}
