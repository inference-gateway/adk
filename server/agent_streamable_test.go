package server_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	server "github.com/inference-gateway/adk/server"
	mocks "github.com/inference-gateway/adk/server/mocks"
	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zap "go.uber.org/zap"
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
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("Hello "),
					},
				},
				{
					MessageID: "chunk-2",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("world!"),
					},
				},
				{
					MessageID: "chunk-3",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart(" How are you?"),
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
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("Single message response"),
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
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("Streaming "),
					},
				},
				{
					MessageID: "chunk-2",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("content..."),
					},
				},
				{
					MessageID: "assistant-final",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("Final consolidated response"),
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
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("Processing your request..."),
					},
				},
				{
					MessageID: "input-req-1",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("Please provide additional information"),
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
					MessageID: "user-1",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("User message"),
					},
				},
				{
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("Assistant "),
					},
				},
				{
					MessageID: "chunk-2",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("response chunks"),
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
								if part.Text != nil {
									fullContent += *part.Text
								}
							}
							finalMessageID = msg.MessageID
						}
					}

					if fullContent != "" {
						consolidatedMessage = &types.Message{
							MessageID: strings.Replace(finalMessageID, "chunk-", "assistant-", 1),
							Role:      "assistant",
							Parts: []types.Part{
								types.CreateTextPart(fullContent),
							},
						}
					}
				}

				if consolidatedMessage != nil {
					task.History = append(task.History, *consolidatedMessage)
				}
			}

			if lastMessage != nil && strings.HasPrefix(lastMessage.MessageID, "input-req") {
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

				if parts[0].Text != nil {
					assert.Equal(t, tt.expectedConsolidatedText, *parts[0].Text, "Consolidated text should match expected")
				} else {
					t.Fatalf("Status message part should contain text field")
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
			MessageID: fmt.Sprintf("chunk-%d", i+1),
			Role:      "assistant",
			Parts: []types.Part{
				types.CreateTextPart(fmt.Sprintf("Message %d ", i+1)),
			},
		}
	}

	start := time.Now()

	var fullContent string
	for _, msg := range messages {
		if msg.Role == "assistant" && strings.HasPrefix(msg.MessageID, "chunk-") {
			for _, part := range msg.Parts {
				if part.Text != nil {
					fullContent += *part.Text
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
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart(""),
					},
				},
				{
					MessageID: "chunk-2",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("Valid text"),
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
					MessageID: "chunk-1",
					Role:      "assistant",
					Parts: []types.Part{
						types.CreateTextPart("123"),
					},
				},
			},
			expectedConsolidatedText: "123",
			description:              "Should handle non-string text fields gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fullContent string
			for _, msg := range tt.streamingMessages {
				if msg.Role == "assistant" && strings.HasPrefix(msg.MessageID, "chunk-") {
					for _, part := range msg.Parts {
						if part.Text != nil {
							fullContent += *part.Text
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
						} else {
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

func TestRunWithStream_ContextCancellation(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	ctx, cancel := context.WithCancel(context.Background())

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{Delta: sdk.ChatCompletionStreamResponseDelta{Content: "Partial "}},
				},
			}
			time.Sleep(50 * time.Millisecond)
		}()

		return responseChan, errorChan
	}

	agent, err := server.NewAgentBuilder(logger).WithLLMClient(mockLLMClient).Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				types.CreateTextPart("Hello"),
			},
		},
	}

	eventChan, err := agent.RunWithStream(ctx, messages)
	require.NoError(t, err)

	cancel()

	var receivedInterrupted bool
	for event := range eventChan {
		if event.Type() == "adk.agent.task.interrupted" {
			receivedInterrupted = true
		}
	}

	assert.True(t, receivedInterrupted, "Should receive task.interrupted event when context is cancelled")
}

func TestRunWithStream_WithInputRequiredTool(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			toolCallChunks := []sdk.ChatCompletionMessageToolCallChunk{
				{
					Index: 0,
					ID:    "call_input",
					Type:  "function",
					Function: struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					}{
						Name:      "input_required",
						Arguments: `{"message":"Please provide more details"}`,
					},
				},
			}
			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta:        sdk.ChatCompletionStreamResponseDelta{ToolCalls: toolCallChunks},
						FinishReason: "tool_calls",
					},
				},
			}
		}()

		return responseChan, errorChan
	}

	toolBox := server.NewDefaultToolBox(nil)
	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		WithToolBox(toolBox).
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				types.CreateTextPart("Hello"),
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)

	var hasInputRequired bool
	for event := range eventChan {
		if event.Type() == "adk.agent.input.required" {
			hasInputRequired = true
		}
	}

	assert.True(t, hasInputRequired, "Should emit input.required event when input_required tool is called")
}

func TestRunWithStream_MaxIterationsReached(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			toolCallChunks := []sdk.ChatCompletionMessageToolCallChunk{
				{
					Index: 0,
					ID:    fmt.Sprintf("call_%d", time.Now().UnixNano()),
					Type:  "function",
					Function: struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					}{
						Name:      "test_tool",
						Arguments: `{"param":"value"}`,
					},
				},
			}
			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta:        sdk.ChatCompletionStreamResponseDelta{ToolCalls: toolCallChunks},
						FinishReason: "tool_calls",
					},
				},
			}
		}()

		return responseChan, errorChan
	}

	toolBox := server.NewDefaultToolBox(nil)
	testTool := server.NewBasicTool(
		"test_tool",
		"Test tool",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param": map[string]any{"type": "string"},
			},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			return "result", nil
		},
	)
	toolBox.AddTool(testTool)

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		WithToolBox(toolBox).
		WithMaxChatCompletion(3).
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				types.CreateTextPart("Keep calling the tool"),
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)

	iterationCount := 0
	for event := range eventChan {
		if event.Type() == "adk.agent.iteration.completed" {
			iterationCount++
		}
	}

	assert.Equal(t, 3, iterationCount, "Should execute exactly max iterations")
}

func TestRunWithStream_ToolExecutionError(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			toolCallChunks := []sdk.ChatCompletionMessageToolCallChunk{
				{
					Index: 0,
					ID:    "call_fail",
					Type:  "function",
					Function: struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					}{
						Name:      "failing_tool",
						Arguments: `{"param":"value"}`,
					},
				},
			}
			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta:        sdk.ChatCompletionStreamResponseDelta{ToolCalls: toolCallChunks},
						FinishReason: "tool_calls",
					},
				},
			}
		}()

		return responseChan, errorChan
	}

	toolBox := server.NewDefaultToolBox(nil)
	failingTool := server.NewBasicTool(
		"failing_tool",
		"Tool that fails",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param": map[string]any{"type": "string"},
			},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			return "", fmt.Errorf("tool execution failed")
		},
	)
	toolBox.AddTool(failingTool)

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		WithToolBox(toolBox).
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				types.CreateTextPart("Execute the failing tool"),
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)

	var hasToolFailed bool
	for event := range eventChan {
		if event.Type() == "adk.agent.tool.failed" {
			hasToolFailed = true
		}
	}

	assert.True(t, hasToolFailed, "Should emit tool.failed event when tool execution fails")
}

func TestRunWithStream_InvalidToolArguments(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			toolCallChunks := []sdk.ChatCompletionMessageToolCallChunk{
				{
					Index: 0,
					ID:    "call_invalid",
					Type:  "function",
					Function: struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					}{
						Name:      "test_tool",
						Arguments: `{invalid json`,
					},
				},
			}
			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta:        sdk.ChatCompletionStreamResponseDelta{ToolCalls: toolCallChunks},
						FinishReason: "tool_calls",
					},
				},
			}
		}()

		return responseChan, errorChan
	}

	toolBox := server.NewDefaultToolBox(nil)
	testTool := server.NewBasicTool(
		"test_tool",
		"Test tool",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param": map[string]any{"type": "string"},
			},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			return "result", nil
		},
	)
	toolBox.AddTool(testTool)

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		WithToolBox(toolBox).
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				types.CreateTextPart("Test"),
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)

	var hasToolFailed bool
	for event := range eventChan {
		if event.Type() == "adk.agent.tool.failed" {
			hasToolFailed = true
		}
	}

	assert.True(t, hasToolFailed, "Should emit tool.failed event when tool arguments are invalid JSON")
}

func TestRunWithStream_WithSystemPrompt(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	var capturedMessages []sdk.Message
	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		capturedMessages = messages
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta:        sdk.ChatCompletionStreamResponseDelta{Content: "Response"},
						FinishReason: "stop",
					},
				},
			}
		}()

		return responseChan, errorChan
	}

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		WithSystemPrompt("You are a helpful assistant").
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				types.CreateTextPart("Hello"),
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)

	for range eventChan {
	}

	require.NotEmpty(t, capturedMessages, "Should have captured messages")
	assert.Equal(t, sdk.System, capturedMessages[0].Role, "First message should be system message")
	systemContent, _ := capturedMessages[0].Content.AsMessageContent0()
	assert.Equal(t, "You are a helpful assistant", systemContent, "System prompt should match")
}

func TestRunWithStream_MultipleIterations(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	callCount := 0
	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			callCount++
			if callCount <= 2 {
				toolCallChunks := []sdk.ChatCompletionMessageToolCallChunk{
					{
						Index: 0,
						ID:    fmt.Sprintf("call_%d", callCount),
						Type:  "function",
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      "iteration_tool",
							Arguments: `{"iteration":` + fmt.Sprintf("%d", callCount) + `}`,
						},
					},
				}
				responseChan <- &sdk.CreateChatCompletionStreamResponse{
					Choices: []sdk.ChatCompletionStreamChoice{
						{
							Delta:        sdk.ChatCompletionStreamResponseDelta{ToolCalls: toolCallChunks},
							FinishReason: "tool_calls",
						},
					},
				}
			} else {
				responseChan <- &sdk.CreateChatCompletionStreamResponse{
					Choices: []sdk.ChatCompletionStreamChoice{
						{
							Delta:        sdk.ChatCompletionStreamResponseDelta{Content: "Final response"},
							FinishReason: "stop",
						},
					},
				}
			}
		}()

		return responseChan, errorChan
	}

	toolBox := server.NewDefaultToolBox(nil)
	iterationTool := server.NewBasicTool(
		"iteration_tool",
		"Tool for iterations",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"iteration": map[string]any{"type": "number"},
			},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			return fmt.Sprintf("Iteration %v completed", args["iteration"]), nil
		},
	)
	toolBox.AddTool(iterationTool)

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		WithToolBox(toolBox).
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				types.CreateTextPart("Run iterations"),
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)

	iterationCount := 0
	for event := range eventChan {
		if event.Type() == "adk.agent.iteration.completed" {
			iterationCount++
		}
	}

	assert.Equal(t, 3, iterationCount, "Should complete 3 iterations (2 with tools, 1 final)")
	assert.Equal(t, 3, callCount, "Should call LLM 3 times")
}

func TestRunWithStream_AllEventTypesEmitted(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			toolCallChunks := []sdk.ChatCompletionMessageToolCallChunk{
				{
					Index: 0,
					ID:    "call_success",
					Type:  "function",
					Function: struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					}{
						Name:      "success_tool",
						Arguments: `{"param":"value"}`,
					},
				},
			}
			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta:        sdk.ChatCompletionStreamResponseDelta{ToolCalls: toolCallChunks},
						FinishReason: "tool_calls",
					},
				},
			}
		}()

		return responseChan, errorChan
	}

	toolBox := server.NewDefaultToolBox(nil)
	successTool := server.NewBasicTool(
		"success_tool",
		"Tool that succeeds",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param": map[string]any{"type": "string"},
			},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			return "success result", nil
		},
	)
	toolBox.AddTool(successTool)

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		WithToolBox(toolBox).
		WithMaxChatCompletion(1).
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				types.CreateTextPart("Execute tool"),
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)

	eventTypes := make(map[string]int)
	for event := range eventChan {
		eventTypes[event.Type()]++
	}

	assert.Greater(t, eventTypes[types.EventToolStarted], 0, "Should emit tool.started event")
	assert.Greater(t, eventTypes[types.EventToolCompleted], 0, "Should emit tool.completed event")
	assert.Greater(t, eventTypes[types.EventToolResult], 0, "Should emit tool.result event")
	assert.Greater(t, eventTypes["adk.agent.iteration.completed"], 0, "Should emit iteration.completed event")
}

func TestRunWithStream_ToolFailedEventEmitted(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			toolCallChunks := []sdk.ChatCompletionMessageToolCallChunk{
				{
					Index: 0,
					ID:    "call_fail",
					Type:  "function",
					Function: struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					}{
						Name:      "fail_tool",
						Arguments: `{"param":"value"}`,
					},
				},
			}
			responseChan <- &sdk.CreateChatCompletionStreamResponse{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta:        sdk.ChatCompletionStreamResponseDelta{ToolCalls: toolCallChunks},
						FinishReason: "tool_calls",
					},
				},
			}
		}()

		return responseChan, errorChan
	}

	toolBox := server.NewDefaultToolBox(nil)
	failTool := server.NewBasicTool(
		"fail_tool",
		"Tool that fails",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param": map[string]any{"type": "string"},
			},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			return "", fmt.Errorf("intentional failure")
		},
	)
	toolBox.AddTool(failTool)

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		WithToolBox(toolBox).
		WithMaxChatCompletion(1).
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				types.CreateTextPart("Execute failing tool"),
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)

	eventTypes := make(map[string]int)
	for event := range eventChan {
		eventTypes[event.Type()]++
	}

	assert.Greater(t, eventTypes[types.EventToolStarted], 0, "Should emit tool.started event")
	assert.Greater(t, eventTypes[types.EventToolFailed], 0, "Should emit tool.failed event")
	assert.Greater(t, eventTypes[types.EventToolResult], 0, "Should emit tool.result event even on failure")
}

func TestRunWithStream_StreamFailedEventEmitted(t *testing.T) {
	logger := zap.NewNop()
	mockLLMClient := &mocks.FakeLLMClient{}

	mockLLMClient.CreateStreamingChatCompletionStub = func(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responseChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
		errorChan := make(chan error, 1)

		go func() {
			defer close(responseChan)
			defer close(errorChan)

			errorChan <- fmt.Errorf("streaming connection error")
		}()

		return responseChan, errorChan
	}

	agent, err := server.NewAgentBuilder(logger).
		WithLLMClient(mockLLMClient).
		Build()
	require.NoError(t, err)

	messages := []types.Message{
		{
			Role: "user",
			Parts: []types.Part{
				types.CreateTextPart("Trigger error"),
			},
		},
	}

	eventChan, err := agent.RunWithStream(context.Background(), messages)
	require.NoError(t, err)

	eventTypes := make(map[string]int)
	for event := range eventChan {
		eventTypes[event.Type()]++
	}

	assert.Greater(t, eventTypes[types.EventStreamFailed], 0, "Should emit stream.failed event on error")
}
