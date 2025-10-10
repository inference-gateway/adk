package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/inference-gateway/adk/server/testutils"
	"github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestCallbackExecutor_ExecuteBeforeAgent(t *testing.T) {
	tests := []struct {
		name                      string
		setupBeforeAgentCallbacks func(counter testutils.Counter) []BeforeAgentCallback
		expected                  *types.Message
		expectedCallbackCalls     int
	}{
		{
			name:                      "no callback configured",
			setupBeforeAgentCallbacks: func(counter testutils.Counter) []BeforeAgentCallback { return nil },
			expected:                  nil,
		},
		{
			name: "callback returns nil (allow execution)",
			setupBeforeAgentCallbacks: func(counter testutils.Counter) []BeforeAgentCallback {
				counter.Increment()
				return []BeforeAgentCallback{
					func(ctx context.Context, callbackContext *CallbackContext) *types.Message {
						return nil
					},
				}
			},
			expected:              nil,
			expectedCallbackCalls: 1,
		},
		{
			name: "callback returns message (skip execution)",
			setupBeforeAgentCallbacks: func(counter testutils.Counter) []BeforeAgentCallback {
				return []BeforeAgentCallback{
					func(ctx context.Context, callbackContext *CallbackContext) *types.Message {
						counter.Increment()
						return &types.Message{
							Kind:      "message",
							MessageID: "test-skip",
							Role:      "assistant",
							Parts: []types.Part{
								map[string]any{
									"kind": "text",
									"text": "Execution skipped by callback",
								},
							},
						}
					},
				}
			},
			expected: &types.Message{
				Kind:      "message",
				MessageID: "test-skip",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Execution skipped by callback",
					},
				},
			},
			expectedCallbackCalls: 1,
		},
		{
			name: "when a callback panics it should continue with normal execution",
			setupBeforeAgentCallbacks: func(counter testutils.Counter) []BeforeAgentCallback {
				return []BeforeAgentCallback{
					func(ctx context.Context, callbackContext *CallbackContext) *types.Message {
						counter.Increment()
						panic("test panic")
					},
					func(ctx context.Context, callbackContext *CallbackContext) *types.Message {
						counter.Increment()
						return nil
					},
				}
			},
			expected:              nil,
			expectedCallbackCalls: 2,
		},
		{
			name: "when a callback returns a message it short-circuits by exiting immediately and skips the rest of the callbacks",
			setupBeforeAgentCallbacks: func(counter testutils.Counter) []BeforeAgentCallback {
				return []BeforeAgentCallback{
					func(ctx context.Context, callbackContext *CallbackContext) *types.Message {
						counter.Increment()
						return &types.Message{
							ContextID: stringPtr("1"),
						}
					},
					func(ctx context.Context, callbackContext *CallbackContext) *types.Message {
						counter.Increment()
						t.Errorf("this callback should not be called")
						return &types.Message{
							ContextID: stringPtr("2"),
						}
					},
				}
			},
			expected: &types.Message{
				ContextID: stringPtr("1"),
			},
			expectedCallbackCalls: 1,
		},
		{
			name: "callback context is modified and passed onto next callback",
			setupBeforeAgentCallbacks: func(counter testutils.Counter) []BeforeAgentCallback {
				return []BeforeAgentCallback{
					func(ctx context.Context, callbackContext *CallbackContext) *types.Message {
						counter.Increment()
						callbackContext.AgentName = "Loki"
						return nil
					},
					func(ctx context.Context, callbackContext *CallbackContext) *types.Message {
						counter.Increment()
						assert.Equal(t, "Loki", callbackContext.AgentName)
						return nil
					},
				}
			},
			expectedCallbackCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			counter := testutils.NewCounter()
			beforeAgentCallbacks := tt.setupBeforeAgentCallbacks(counter)
			config := &CallbackConfig{
				BeforeAgent: beforeAgentCallbacks,
			}
			executor := NewCallbackExecutor(config, logger)

			callbackContext := &CallbackContext{
				AgentName:    "test-agent",
				InvocationID: "test-invocation",
				Logger:       logger,
			}

			result := executor.ExecuteBeforeAgent(context.Background(), callbackContext)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedCallbackCalls, counter.Get())
		})
	}
}

func TestCallbackExecutor_ExecuteAfterAgent(t *testing.T) {
	tests := []struct {
		name                     string
		setupAfterAgentCallbacks func(counter testutils.Counter) []AfterAgentCallback
		agentOutput              *types.Message
		expected                 *types.Message
		expectedCallbackCalls    int
	}{
		{
			name: "no callback configured",
			setupAfterAgentCallbacks: func(counter testutils.Counter) []AfterAgentCallback {
				return []AfterAgentCallback{}
			},
			agentOutput:           &types.Message{},
			expected:              nil,
			expectedCallbackCalls: 0,
		},
		{
			name: "callback returns nil (use original response)",
			setupAfterAgentCallbacks: func(counter testutils.Counter) []AfterAgentCallback {
				return []AfterAgentCallback{func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
					counter.Increment()
					return nil
				}}
			},
			agentOutput:           &types.Message{},
			expected:              nil,
			expectedCallbackCalls: 1,
		},
		{
			name: "callback returns modified response",
			setupAfterAgentCallbacks: func(counter testutils.Counter) []AfterAgentCallback {
				return []AfterAgentCallback{
					func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
						counter.Increment()
						return &types.Message{
							ReferenceTaskIds: agentOutput.ReferenceTaskIds,
							Kind:             "message",
							MessageID:        "test-modified",
							Role:             "assistant",
							Parts: []types.Part{
								map[string]any{
									"kind": "text",
									"text": "Response modified by callback",
								},
							},
						}
					},
				}
			},
			agentOutput: &types.Message{
				ReferenceTaskIds: []string{"1"},
				Kind:             "message",
				MessageID:        "original",
				Role:             "assistant",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Original response",
					},
				},
			},
			expected: &types.Message{
				ReferenceTaskIds: []string{"1"},
				Kind:             "message",
				MessageID:        "test-modified",
				Role:             "assistant",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Response modified by callback",
					},
				},
			},
			expectedCallbackCalls: 1,
		},
		{
			name: "when a callback panics it passes on the current response to the next callback",
			setupAfterAgentCallbacks: func(counter testutils.Counter) []AfterAgentCallback {
				return []AfterAgentCallback{
					func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
						counter.Increment()
						agentOutput.Metadata = map[string]any{
							"test": "test-data",
						}
						return agentOutput
					},
					func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
						counter.Increment()
						panic("something went wrong")
					},
					func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
						counter.Increment()
						agentOutput.Kind = "message"
						agentOutput.MessageID = "test-modified"
						agentOutput.Role = "assistant"
						return agentOutput
					}}
			},
			agentOutput: &types.Message{
				Kind:      "message",
				MessageID: "original",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Original response",
					},
				},
			},
			expected: &types.Message{
				Kind:      "message",
				MessageID: "test-modified",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Original response",
					},
				},
				Metadata: map[string]any{
					"test": "test-data",
				},
			},
			expectedCallbackCalls: 3,
		},
		{
			name: "all callbacks are invoked even when they return nil",
			setupAfterAgentCallbacks: func(counter testutils.Counter) []AfterAgentCallback {
				return []AfterAgentCallback{
					func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
						counter.Increment()
						return nil
					},
					func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
						counter.Increment()
						return nil
					},
				}
			},
			expectedCallbackCalls: 2,
		},
		{
			name: "returns the final modified response",
			setupAfterAgentCallbacks: func(counter testutils.Counter) []AfterAgentCallback {
				return []AfterAgentCallback{
					func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
						counter.Increment()
						agentOutput.Kind = "test"
						return agentOutput
					},
					func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
						counter.Increment()
						return nil
					},
				}
			},
			agentOutput: &types.Message{
				Kind:      "message",
				MessageID: "original",
			},
			expected: &types.Message{
				Kind:      "test",
				MessageID: "original",
			},
			expectedCallbackCalls: 2,
		},
		{
			name: "callback context is modified and passed onto next callback",
			setupAfterAgentCallbacks: func(counter testutils.Counter) []AfterAgentCallback {
				return []AfterAgentCallback{
					func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
						counter.Increment()
						callbackContext.TaskID = "123456"
						return nil
					},
					func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
						counter.Increment()
						assert.Equal(t, "123456", callbackContext.TaskID)
						return nil
					},
				}
			},
			expectedCallbackCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			counter := testutils.NewCounter()
			config := &CallbackConfig{
				AfterAgent: tt.setupAfterAgentCallbacks(counter),
			}
			executor := NewCallbackExecutor(config, logger)

			callbackContext := &CallbackContext{
				AgentName:    "test-agent",
				InvocationID: "test-invocation",
				Logger:       logger,
			}

			result := executor.ExecuteAfterAgent(context.Background(), callbackContext, tt.agentOutput)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedCallbackCalls, counter.Get())
		})
	}
}

func TestCallbackExecutor_ExecuteBeforeModel(t *testing.T) {
	temperature := 0.2

	tests := []struct {
		name                      string
		setupBeforeModelCallbacks func(counter testutils.Counter) []BeforeModelCallback
		request                   *LLMRequest
		expected                  *LLMResponse
		expectedCallbackCalls     int
	}{
		{
			name:                      "no callback configured",
			setupBeforeModelCallbacks: func(counter testutils.Counter) []BeforeModelCallback { return nil },
			request:                   &LLMRequest{},
			expected:                  nil,
			expectedCallbackCalls:     0,
		},
		{
			name: "callback returns nil (allow LLM call)",
			setupBeforeModelCallbacks: func(counter testutils.Counter) []BeforeModelCallback {
				return []BeforeModelCallback{
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						return nil
					},
				}
			},
			request:               &LLMRequest{},
			expected:              nil,
			expectedCallbackCalls: 1,
		},
		{
			name: "callback returns response (skip LLM call)",
			setupBeforeModelCallbacks: func(counter testutils.Counter) []BeforeModelCallback {
				return []BeforeModelCallback{
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						return &LLMResponse{
							Content: &types.Message{
								Kind:      "message",
								MessageID: "test-blocked",
								Role:      "assistant",
								Parts: []types.Part{
									map[string]any{
										"kind": "text",
										"text": "LLM call blocked by callback",
									},
								},
							},
						}
					},
				}
			},
			request: &LLMRequest{},
			expected: &LLMResponse{
				Content: &types.Message{
					Kind:      "message",
					MessageID: "test-blocked",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "LLM call blocked by callback",
						},
					},
				},
			},
			expectedCallbackCalls: 1,
		},
		{
			name: "callback modifies request and returns nil",
			setupBeforeModelCallbacks: func(counter testutils.Counter) []BeforeModelCallback {
				return []BeforeModelCallback{
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						if llmRequest.Config == nil {
							llmRequest.Config = &LLMConfig{}
						}
						llmRequest.Config.SystemInstruction = &types.Message{
							Role: "system",
							Parts: []types.Part{
								map[string]any{
									"kind": "text",
									"text": "[Modified by Callback] You are a helpful assistant.",
								},
							},
						}
						return nil // Allow LLM call to proceed with modified request
					},
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						if llmRequest.Config == nil {
							t.Error("expected config to be defined from previous callback")
						}
						llmRequest.Config.Temperature = &temperature
						return nil
					},
				}
			},
			request: &LLMRequest{
				Config: &LLMConfig{},
			},
			expected:              nil,
			expectedCallbackCalls: 2,
		},
		{
			name: "panics in callbacks are handled and execution continues",
			setupBeforeModelCallbacks: func(counter testutils.Counter) []BeforeModelCallback {
				return []BeforeModelCallback{
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						panic("fatal error occurred")
					},
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						return &LLMResponse{Content: &types.Message{Role: "Admin"}}
					},
				}
			},
			request:               &LLMRequest{},
			expected:              &LLMResponse{Content: &types.Message{Role: "Admin"}},
			expectedCallbackCalls: 2,
		},
		{
			name: "callback context is modified and passed onto next callback",
			setupBeforeModelCallbacks: func(counter testutils.Counter) []BeforeModelCallback {
				return []BeforeModelCallback{
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						callbackContext.TaskID = "123456"
						return nil
					},
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						assert.Equal(t, "123456", callbackContext.TaskID)
						return nil
					},
				}
			},
			expectedCallbackCalls: 2,
		},
		{
			name: "returns first callback with non-nil result and skips remaining callbacks",
			setupBeforeModelCallbacks: func(counter testutils.Counter) []BeforeModelCallback {
				return []BeforeModelCallback{
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						return nil
					},
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						return &LLMResponse{}
					},
					func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
						counter.Increment()
						t.Errorf("this callback should not be called")
						return nil
					},
				}
			},
			expected:              &LLMResponse{},
			expectedCallbackCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			counter := testutils.NewCounter()
			config := &CallbackConfig{
				BeforeModel: tt.setupBeforeModelCallbacks(counter),
			}
			executor := NewCallbackExecutor(config, logger)

			callbackContext := &CallbackContext{
				AgentName:    "test-agent",
				InvocationID: "test-invocation",
				Logger:       logger,
			}

			result := executor.ExecuteBeforeModel(context.Background(), callbackContext, tt.request)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedCallbackCalls, counter.Get())

			// For the modification test, check that the request was actually modified
			if tt.name == "callback modifies request and returns nil" {
				assert.NotNil(t, tt.request.Config.SystemInstruction)
				assert.Equal(t, temperature, *tt.request.Config.Temperature)
				assert.Contains(t, tt.request.Config.SystemInstruction.Parts[0].(map[string]any)["text"], "[Modified by Callback]")
			}
		})
	}
}

func TestCallbackExecutor_ExecuteAfterModel(t *testing.T) {
	tests := []struct {
		name                     string
		setupAfterModelCallbacks func(counter testutils.Counter) []AfterModelCallback
		response                 *LLMResponse
		expected                 *LLMResponse
		expectedCallbackCalls    int
	}{
		{
			name: "no callback configured",
			setupAfterModelCallbacks: func(counter testutils.Counter) []AfterModelCallback {
				return []AfterModelCallback{}
			},
			response:              &LLMResponse{},
			expected:              nil,
			expectedCallbackCalls: 0,
		},
		{
			name: "callback returns nil (use original response)",
			setupAfterModelCallbacks: func(counter testutils.Counter) []AfterModelCallback {
				return []AfterModelCallback{func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
					counter.Increment()
					return nil
				}}
			},
			response:              &LLMResponse{},
			expected:              nil,
			expectedCallbackCalls: 1,
		},
		{
			name: "callback returns modified response",
			setupAfterModelCallbacks: func(counter testutils.Counter) []AfterModelCallback {
				return []AfterModelCallback{func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
					counter.Increment()
					return &LLMResponse{
						Content: &types.Message{
							ReferenceTaskIds: llmResponse.Content.ReferenceTaskIds,
							Kind:             "message",
							MessageID:        "test-modified",
							Role:             "assistant",
							Parts: []types.Part{
								map[string]any{
									"kind": "text",
									"text": "Response modified by callback",
								},
							},
						},
					}
				}}
			},
			response: &LLMResponse{
				Content: &types.Message{
					ReferenceTaskIds: []string{"1"},
					Kind:             "message",
					MessageID:        "original",
					Role:             "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Original response",
						},
					},
				},
			},
			expected: &LLMResponse{
				Content: &types.Message{
					ReferenceTaskIds: []string{"1"},
					Kind:             "message",
					MessageID:        "test-modified",
					Role:             "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Response modified by callback",
						},
					},
				},
			},
			expectedCallbackCalls: 1,
		},
		{
			name: "when a callback panics it passes on the current response to the next callback",
			setupAfterModelCallbacks: func(counter testutils.Counter) []AfterModelCallback {
				return []AfterModelCallback{
					func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
						counter.Increment()
						llmResponse.Content.Metadata = map[string]any{
							"test": "test-data",
						}
						return llmResponse
					},
					func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
						counter.Increment()
						panic("something went wrong")
					},
					func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
						counter.Increment()
						llmResponse.Content.Kind = "message"
						llmResponse.Content.MessageID = "test-modified"
						llmResponse.Content.Role = "assistant"
						return llmResponse
					}}
			},
			response: &LLMResponse{
				Content: &types.Message{
					Kind:      "message",
					MessageID: "original",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Original response",
						},
					},
				},
			},
			expected: &LLMResponse{
				Content: &types.Message{
					Kind:      "message",
					MessageID: "test-modified",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Original response",
						},
					},
					Metadata: map[string]any{
						"test": "test-data",
					},
				},
			},
			expectedCallbackCalls: 3,
		},
		{
			name: "all callbacks are invoked even when they return nil",
			setupAfterModelCallbacks: func(counter testutils.Counter) []AfterModelCallback {
				return []AfterModelCallback{
					func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
						counter.Increment()
						return nil
					},
					func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
						counter.Increment()
						return nil
					},
				}
			},
			expectedCallbackCalls: 2,
		},
		{
			name: "returns the final modified response",
			setupAfterModelCallbacks: func(counter testutils.Counter) []AfterModelCallback {
				return []AfterModelCallback{
					func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
						counter.Increment()
						llmResponse.Content.Kind = "test"
						return llmResponse
					},
					func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
						counter.Increment()
						return nil
					},
				}
			},
			response: &LLMResponse{
				Content: &types.Message{
					Kind:      "message",
					MessageID: "original",
				},
			},
			expected: &LLMResponse{
				Content: &types.Message{
					Kind:      "test",
					MessageID: "original",
				},
			},
			expectedCallbackCalls: 2,
		},
		{
			name: "callback context is modified and passed onto next callback",
			setupAfterModelCallbacks: func(counter testutils.Counter) []AfterModelCallback {
				return []AfterModelCallback{
					func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
						counter.Increment()
						callbackContext.TaskID = "123456"
						return nil
					},
					func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
						counter.Increment()
						assert.Equal(t, "123456", callbackContext.TaskID)
						return nil
					},
				}
			},
			expectedCallbackCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			counter := testutils.NewCounter()
			config := &CallbackConfig{
				AfterModel: tt.setupAfterModelCallbacks(counter),
			}
			executor := NewCallbackExecutor(config, logger)

			callbackContext := &CallbackContext{
				AgentName:    "test-agent",
				InvocationID: "test-invocation",
				Logger:       logger,
			}

			result := executor.ExecuteAfterModel(context.Background(), callbackContext, tt.response)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedCallbackCalls, counter.Get())
		})
	}
}

func TestCallbackExecutor_ExecuteBeforeTool(t *testing.T) {

	basicTool := NewBasicTool(
		"test_tool",
		"tool used for testing",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "Clear, specific message explaining exactly what additional information you need from the user to complete their request.",
				},
			},
			"required": []string{"message"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			message := args["message"].(string)
			return fmt.Sprintf("Input requested from user: %s", message), nil
		},
	)

	tests := []struct {
		name                     string
		setupBeforeToolsCallback func(counter testutils.Counter) []BeforeToolCallback
		tool                     Tool
		args                     map[string]interface{}
		expected                 map[string]interface{}
		expectedCallbackCalls    int
	}{
		{
			name:                     "no callback configured",
			setupBeforeToolsCallback: func(counter testutils.Counter) []BeforeToolCallback { return nil },
			tool:                     basicTool,
			args:                     map[string]interface{}{"input": "test"},
			expected:                 nil,
			expectedCallbackCalls:    0,
		},
		{
			name: "callback returns nil (allow tool execution)",
			setupBeforeToolsCallback: func(counter testutils.Counter) []BeforeToolCallback {
				return []BeforeToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{} {
						counter.Increment()
						return nil
					},
				}
			},
			tool:                  basicTool,
			args:                  map[string]interface{}{"input": "test"},
			expected:              nil,
			expectedCallbackCalls: 1,
		},
		{
			name: "when a callback returns data it short circuits and skips remaining callbacks",
			setupBeforeToolsCallback: func(counter testutils.Counter) []BeforeToolCallback {
				return []BeforeToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{} {
						counter.Increment()
						return map[string]interface{}{
							"result":  "Tool execution skipped by callback",
							"skipped": true,
						}
					},
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{} {
						counter.Increment()
						t.Errorf("this callback should not be called")
						return nil
					},
				}
			},
			tool: basicTool,
			args: map[string]interface{}{"input": "test"},
			expected: map[string]interface{}{
				"result":  "Tool execution skipped by callback",
				"skipped": true,
			},
			expectedCallbackCalls: 1,
		},
		{
			name: "callback continues when the result is nil",
			setupBeforeToolsCallback: func(counter testutils.Counter) []BeforeToolCallback {
				return []BeforeToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{} {
						counter.Increment()
						return nil
					},
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{} {
						counter.Increment()
						return nil
					},
				}
			},
			tool:                  basicTool,
			expected:              nil,
			expectedCallbackCalls: 2,
		},
		{
			name: "callback continues when it encounters a panic",
			setupBeforeToolsCallback: func(counter testutils.Counter) []BeforeToolCallback {
				return []BeforeToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{} {
						counter.Increment()
						panic("unkown error")
					},
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{} {
						counter.Increment()
						return nil
					},
				}
			},
			tool:                  basicTool,
			expected:              nil,
			expectedCallbackCalls: 2,
		},
		{
			name: "callback passes on modified toolContext to the next callback",
			setupBeforeToolsCallback: func(counter testutils.Counter) []BeforeToolCallback {
				return []BeforeToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{} {
						counter.Increment()
						toolContext.AgentName = "Jason Bourne"
						return nil
					},
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{} {
						counter.Increment()
						assert.Equal(t, "Jason Bourne", toolContext.AgentName)
						return nil
					},
				}
			},
			tool:                  basicTool,
			expected:              nil,
			expectedCallbackCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			counter := testutils.NewCounter()
			config := &CallbackConfig{
				BeforeTool: tt.setupBeforeToolsCallback(counter),
			}
			executor := NewCallbackExecutor(config, logger)

			toolContext := &ToolContext{
				AgentName:    "test-agent",
				InvocationID: "test-invocation",
				Logger:       logger,
			}

			result := executor.ExecuteBeforeTool(context.Background(), tt.tool, tt.args, toolContext)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedCallbackCalls, counter.Get())
		})
	}
}

func TestCallbackExecutor_ExecuteAfterTool(t *testing.T) {
	basicTool := NewBasicTool(
		"test_tool",
		"tool used for testing",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "Clear, specific message explaining exactly what additional information you need from the user to complete their request.",
				},
			},
			"required": []string{"message"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			message := args["message"].(string)
			return fmt.Sprintf("Input requested from user: %s", message), nil
		},
	)

	tests := []struct {
		name                    string
		setupAfterToolCallbacks func(counter testutils.Counter) []AfterToolCallback
		tool                    Tool
		args                    map[string]interface{}
		toolResult              map[string]interface{}
		expected                map[string]interface{}
		expectedCallbackCalls   int
	}{
		{
			name:                    "no callback configured",
			setupAfterToolCallbacks: func(counter testutils.Counter) []AfterToolCallback { return nil },
			tool:                    basicTool,
			args:                    map[string]interface{}{"input": "test"},
			toolResult:              map[string]interface{}{"result": "original"},
			expected:                nil,
		},
		{
			name: "callback returns nil (use original result)",
			setupAfterToolCallbacks: func(counter testutils.Counter) []AfterToolCallback {
				return []AfterToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						return nil
					},
				}
			},
			tool:                  basicTool,
			args:                  map[string]interface{}{"input": "test"},
			toolResult:            map[string]interface{}{"result": "original"},
			expected:              nil,
			expectedCallbackCalls: 1,
		},
		{
			name: "callback returns modified result",
			setupAfterToolCallbacks: func(counter testutils.Counter) []AfterToolCallback {
				return []AfterToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						return map[string]interface{}{
							"result":   "Modified by callback",
							"original": toolResult["result"],
							"modified": true,
						}
					},
				}
			},
			tool:       basicTool,
			args:       map[string]interface{}{"input": "test"},
			toolResult: map[string]interface{}{"result": "original"},
			expected: map[string]interface{}{
				"result":   "Modified by callback",
				"original": "original",
				"modified": true,
			},
			expectedCallbackCalls: 1,
		},
		{
			name: "when a callback modifies a result and additional callbacks return nil it will return the final modified result",
			setupAfterToolCallbacks: func(counter testutils.Counter) []AfterToolCallback {
				return []AfterToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						return map[string]interface{}{
							"modified": true,
						}
					},
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						return nil
					},
				}
			},
			tool:       basicTool,
			args:       map[string]interface{}{"input": "test"},
			toolResult: map[string]interface{}{"result": "original"},
			expected: map[string]interface{}{
				"modified": true,
			},
			expectedCallbackCalls: 2,
		},
		{
			name: "each callback receives the previous result and can modify it",
			setupAfterToolCallbacks: func(counter testutils.Counter) []AfterToolCallback {
				return []AfterToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						return map[string]interface{}{
							"callback1": true,
						}
					},
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						toolResult["callback2"] = true
						return toolResult
					},
				}
			},
			tool:                  basicTool,
			args:                  map[string]interface{}{"input": "test"},
			toolResult:            map[string]interface{}{"result": "original"},
			expected:              map[string]interface{}{"callback1": true, "callback2": true},
			expectedCallbackCalls: 2,
		},
		{
			name: "callback continues when it encounters a panic",
			setupAfterToolCallbacks: func(counter testutils.Counter) []AfterToolCallback {
				return []AfterToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						panic("something went wrong")
					},
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						return nil
					},
				}
			},
			tool:                  basicTool,
			expected:              nil,
			expectedCallbackCalls: 2,
		},
		{
			name: "returns the final modified result",
			setupAfterToolCallbacks: func(counter testutils.Counter) []AfterToolCallback {
				return []AfterToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						return map[string]interface{}{"pii-data": "secret-value"}
					},
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						return nil
					},
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						return map[string]interface{}{"use-this": 1}
					},
				}
			},
			tool:                  basicTool,
			toolResult:            map[string]interface{}{"result": "original"},
			expected:              map[string]interface{}{"use-this": 1},
			expectedCallbackCalls: 3,
		},
		{
			name: "toolContext is modified and passed through to next callback",
			setupAfterToolCallbacks: func(counter testutils.Counter) []AfterToolCallback {
				return []AfterToolCallback{
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						counter.Increment()
						toolContext.AgentName = "James Bond"
						return nil
					},
					func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
						assert.Equal(t, "James Bond", toolContext.AgentName)
						counter.Increment()
						return nil
					},
				}
			},
			tool:                  basicTool,
			expected:              nil,
			expectedCallbackCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			counter := testutils.NewCounter()
			config := &CallbackConfig{
				AfterTool: tt.setupAfterToolCallbacks(counter),
			}
			executor := NewCallbackExecutor(config, logger)

			toolContext := &ToolContext{
				AgentName:    "test-agent",
				InvocationID: "test-invocation",
				Logger:       logger,
			}

			result := executor.ExecuteAfterTool(context.Background(), tt.tool, tt.args, toolContext, tt.toolResult)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedCallbackCalls, counter.Get())
		})
	}
}
