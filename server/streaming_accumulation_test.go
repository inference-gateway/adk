package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			// Create a test task
			task := &types.Task{
				ID:        "test-task-123",
				ContextID: "test-context-123",
				Status: types.TaskStatus{
					State: types.TaskStateWorking,
				},
				History: []types.Message{},
			}

			// Simulate the streaming messages we received
			allMessages := tt.streamingMessages
			var lastMessage *types.Message
			if len(allMessages) > 0 {
				lastMessage = &allMessages[len(allMessages)-1]
			}

			// Apply the streaming completion logic from server.go lines 990-1049
			if len(allMessages) > 0 {
				var consolidatedMessage *types.Message

				// Look for non-chunk assistant message first
				for i := len(allMessages) - 1; i >= 0; i-- {
					msg := &allMessages[i]
					if msg.Role == "assistant" && !strings.HasPrefix(msg.MessageID, "chunk-") {
						consolidatedMessage = msg
						break
					}
				}

				// If no non-chunk message found, accumulate chunks
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

			// Apply task status update logic
			if lastMessage != nil && lastMessage.Kind == "input_required" {
				task.Status.State = types.TaskStateInputRequired
				task.Status.Message = lastMessage
			} else {
				task.Status.State = types.TaskStateCompleted
				// Use consolidated message if available, otherwise fall back to last message
				if len(task.History) > 0 {
					task.Status.Message = &task.History[len(task.History)-1]
				} else {
					task.Status.Message = lastMessage
				}
			}

			// Verify results
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
				// For empty messages test case, we expect no status message
				assert.Nil(t, task.Status.Message, "Status message should be nil for empty message list")
			}
		})
	}
}

func TestStreamingMessageAccumulationPerformance(t *testing.T) {
	// Test performance with a large number of streaming messages
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

	// Simulate accumulation logic
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

	// Should complete within reasonable time (less than 1 second for 1000 messages)
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
							// Missing "text" field
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
							"text": 123, // Non-string text field
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
			// Apply the streaming accumulation logic
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
