package server

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	zap "go.uber.org/zap"

	sdk "github.com/inference-gateway/sdk"

	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

// TestUsageMetadata_BackgroundTaskHandler tests usage metadata in background task processing
func TestUsageMetadata_BackgroundTaskHandler(t *testing.T) {
	logger := zap.NewNop()

	mockLLMClient := &MockLLMClient{
		streamResponses: []*sdk.CreateChatCompletionStreamResponse{
			{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta: sdk.ChatCompletionStreamResponseDelta{
							Content: "Hello! I can help you.",
						},
					},
				},
			},
			{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta: sdk.ChatCompletionStreamResponseDelta{
							Content: "",
						},
						FinishReason: "stop",
					},
				},
				Usage: &sdk.CompletionUsage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
			},
		},
	}

	agent := NewOpenAICompatibleAgentWithConfig(logger, &serverConfig.AgentConfig{
		MaxChatCompletionIterations: 10,
		SystemPrompt:                "You are a test assistant",
	})
	agent.SetLLMClient(mockLLMClient)

	handler := NewDefaultBackgroundTaskHandler(logger, agent)

	task := &types.Task{
		ID:        "test-task-123",
		ContextID: new("test-context-456"),
		Status: types.TaskStatus{
			State: types.TaskStateSubmitted,
		},
		History: []types.Message{
			{
				Role: "user",
				Parts: []types.Part{
					types.NewTextPart("Hello, can you help me?"),
				},
			},
		},
	}

	resultTask, err := handler.HandleTask(context.Background(), task, nil)
	require.NoError(t, err)
	require.NotNil(t, resultTask)

	require.NotNil(t, resultTask.Metadata, "Task metadata should not be nil")

	assert.Contains(t, *resultTask.Metadata, types.UsageMetadataKey, "Metadata should contain 'usage' field")
	usageMap, ok := (*resultTask.Metadata)[types.UsageMetadataKey].(map[string]any)
	require.True(t, ok, "Usage should be a map")
	assert.Equal(t, int64(100), usageMap["prompt_tokens"])
	assert.Equal(t, int64(50), usageMap["completion_tokens"])
	assert.Equal(t, int64(150), usageMap["total_tokens"])

	assert.Contains(t, *resultTask.Metadata, types.ExecutionStatsMetadataKey, "Metadata should contain 'execution_stats' field")
	execStats, ok := (*resultTask.Metadata)[types.ExecutionStatsMetadataKey].(map[string]any)
	require.True(t, ok, "Execution stats should be a map")
	assert.Greater(t, execStats["iterations"], 0, "Should have at least one iteration")
	assert.GreaterOrEqual(t, execStats["messages"], 0, "Should have message count")
}

// TestUsageMetadata_StreamingTaskHandler tests usage metadata in streaming task processing
func TestUsageMetadata_StreamingTaskHandler(t *testing.T) {
	logger := zap.NewNop()

	mockLLMClient := &MockLLMClient{
		streamResponses: []*sdk.CreateChatCompletionStreamResponse{
			{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta: sdk.ChatCompletionStreamResponseDelta{
							Content: "I'm here to help!",
						},
					},
				},
			},
			{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta: sdk.ChatCompletionStreamResponseDelta{
							Content: "",
						},
						FinishReason: "stop",
					},
				},
				Usage: &sdk.CompletionUsage{
					PromptTokens:     200,
					CompletionTokens: 75,
					TotalTokens:      275,
				},
			},
		},
	}

	agent := NewOpenAICompatibleAgentWithConfig(logger, &serverConfig.AgentConfig{
		MaxChatCompletionIterations: 10,
		SystemPrompt:                "You are a test assistant",
	})
	agent.SetLLMClient(mockLLMClient)

	handler := NewDefaultStreamingTaskHandler(logger, agent)

	task := &types.Task{
		ID:        "test-streaming-task-123",
		ContextID: new("test-streaming-context-456"),
		Status: types.TaskStatus{
			State: types.TaskStateSubmitted,
		},
		History: []types.Message{
			{
				Role: "user",
				Parts: []types.Part{
					types.NewTextPart("Can you assist me?"),
				},
			},
		},
	}

	eventChan, err := handler.HandleStreamingTask(context.Background(), task, nil)
	require.NoError(t, err)
	require.NotNil(t, eventChan)

	var completedEvent *cloudevents.Event
	for event := range eventChan {
		if event.Type() == types.EventTaskStatusChanged {
			var statusData types.TaskStatus
			if err := event.DataAs(&statusData); err == nil {
				if statusData.State == types.TaskStateCompleted {
					evt := event
					completedEvent = &evt
				}
			}
		}
	}

	require.NotNil(t, completedEvent, "Should receive completed event")

	require.NotNil(t, task.Metadata, "Task metadata should not be nil")

	assert.Contains(t, *task.Metadata, types.UsageMetadataKey, "Metadata should contain 'usage' field")
	usageMap, ok := (*task.Metadata)[types.UsageMetadataKey].(map[string]any)
	require.True(t, ok, "Usage should be a map")
	assert.Equal(t, int64(200), usageMap["prompt_tokens"])
	assert.Equal(t, int64(75), usageMap["completion_tokens"])
	assert.Equal(t, int64(275), usageMap["total_tokens"])

	assert.Contains(t, *task.Metadata, types.ExecutionStatsMetadataKey, "Metadata should contain 'execution_stats' field")
	execStats, ok := (*task.Metadata)[types.ExecutionStatsMetadataKey].(map[string]any)
	require.True(t, ok, "Execution stats should be a map")
	assert.Greater(t, execStats["iterations"], 0, "Should have at least one iteration")
}

// TestUsageMetadata_BackgroundTaskHandler_Disabled verifies that when the
// EnableUsageMetadata flag is false on the background handler, no usage
// metadata is attached to the completed task.
func TestUsageMetadata_BackgroundTaskHandler_Disabled(t *testing.T) {
	logger := zap.NewNop()

	mockLLMClient := &MockLLMClient{
		streamResponses: []*sdk.CreateChatCompletionStreamResponse{
			{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta: sdk.ChatCompletionStreamResponseDelta{
							Content: "Disabled response.",
						},
					},
				},
			},
			{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta:        sdk.ChatCompletionStreamResponseDelta{Content: ""},
						FinishReason: "stop",
					},
				},
				Usage: &sdk.CompletionUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
		},
	}

	agent := NewOpenAICompatibleAgentWithConfig(logger, &serverConfig.AgentConfig{
		MaxChatCompletionIterations: 10,
		SystemPrompt:                "You are a test assistant",
	})
	agent.SetLLMClient(mockLLMClient)

	handler := NewDefaultBackgroundTaskHandler(logger, agent)
	require.True(t, handler.IsUsageMetadataEnabled(), "default should be enabled")
	handler.SetEnableUsageMetadata(false)
	require.False(t, handler.IsUsageMetadataEnabled(), "should be disabled after setter")

	task := &types.Task{
		ID:        "test-task-disabled",
		ContextID: new("test-context-disabled"),
		Status:    types.TaskStatus{State: types.TaskStateSubmitted},
		History: []types.Message{
			{
				Role:  "user",
				Parts: []types.Part{types.NewTextPart("Hello")},
			},
		},
	}

	resultTask, err := handler.HandleTask(context.Background(), task, nil)
	require.NoError(t, err)
	require.NotNil(t, resultTask)

	if resultTask.Metadata != nil {
		assert.NotContains(t, *resultTask.Metadata, types.UsageMetadataKey, "usage metadata should not be attached when disabled")
		assert.NotContains(t, *resultTask.Metadata, types.ExecutionStatsMetadataKey, "execution_stats should not be attached when disabled")
	}
}

// TestUsageMetadata_StreamingTaskHandler_Disabled verifies the streaming
// handler honors the disabled flag.
func TestUsageMetadata_StreamingTaskHandler_Disabled(t *testing.T) {
	logger := zap.NewNop()

	mockLLMClient := &MockLLMClient{
		streamResponses: []*sdk.CreateChatCompletionStreamResponse{
			{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta: sdk.ChatCompletionStreamResponseDelta{Content: "Disabled streaming."},
					},
				},
			},
			{
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Delta:        sdk.ChatCompletionStreamResponseDelta{Content: ""},
						FinishReason: "stop",
					},
				},
				Usage: &sdk.CompletionUsage{
					PromptTokens:     20,
					CompletionTokens: 10,
					TotalTokens:      30,
				},
			},
		},
	}

	agent := NewOpenAICompatibleAgentWithConfig(logger, &serverConfig.AgentConfig{
		MaxChatCompletionIterations: 10,
		SystemPrompt:                "You are a test assistant",
	})
	agent.SetLLMClient(mockLLMClient)

	handler := NewDefaultStreamingTaskHandler(logger, agent)
	require.True(t, handler.IsUsageMetadataEnabled(), "default should be enabled")
	handler.SetEnableUsageMetadata(false)
	require.False(t, handler.IsUsageMetadataEnabled(), "should be disabled after setter")

	task := &types.Task{
		ID:        "test-streaming-disabled",
		ContextID: new("test-streaming-context-disabled"),
		Status:    types.TaskStatus{State: types.TaskStateSubmitted},
		History: []types.Message{
			{
				Role:  "user",
				Parts: []types.Part{types.NewTextPart("Hello")},
			},
		},
	}

	eventChan, err := handler.HandleStreamingTask(context.Background(), task, nil)
	require.NoError(t, err)
	require.NotNil(t, eventChan)

	for event := range eventChan {
		_ = event
	}

	if task.Metadata != nil {
		assert.NotContains(t, *task.Metadata, types.UsageMetadataKey, "usage metadata should not be attached when disabled")
		assert.NotContains(t, *task.Metadata, types.ExecutionStatsMetadataKey, "execution_stats should not be attached when disabled")
	}
}

// TestRunWithStream_CountsTrailingUsageChunk streams the include_usage shape:
// the finish_reason chunk, a repeated one, then a chunk with no choices that
// carries the usage. The usage must be counted once and the turn finish once.
func TestRunWithStream_CountsTrailingUsageChunk(t *testing.T) {
	finish := &sdk.CreateChatCompletionStreamResponse{
		Choices: []sdk.ChatCompletionStreamChoice{{FinishReason: "stop"}},
	}
	mockLLMClient := &MockLLMClient{
		streamResponses: []*sdk.CreateChatCompletionStreamResponse{
			{Choices: []sdk.ChatCompletionStreamChoice{{Delta: sdk.ChatCompletionStreamResponseDelta{Content: "Hi"}}}},
			finish,
			finish,
			{
				Choices: []sdk.ChatCompletionStreamChoice{},
				Usage:   &sdk.CompletionUsage{PromptTokens: 120, CompletionTokens: 30, TotalTokens: 150},
			},
		},
	}
	agent := NewOpenAICompatibleAgentWithConfig(zap.NewNop(), &serverConfig.AgentConfig{MaxChatCompletionIterations: 10})
	agent.SetLLMClient(mockLLMClient)

	tracker := NewUsageTracker()
	ctx := context.WithValue(context.Background(), UsageTrackerContextKey, tracker)
	eventChan, err := agent.RunWithStream(ctx, []types.Message{
		{Role: "user", Parts: []types.Part{types.NewTextPart("hello")}},
	})
	require.NoError(t, err)

	iterations := 0
	for event := range eventChan {
		if event.Type() == types.EventIterationCompleted {
			iterations++
		}
	}

	assert.Equal(t, 1, iterations, "a repeated finish_reason must not finish the turn twice")
	assert.Equal(t, 1, tracker.llmCalls)
	assert.Equal(t, int64(120), tracker.promptTokens)
	assert.Equal(t, int64(30), tracker.completionTokens)
}

// MockLLMClient is a simple mock for testing
type MockLLMClient struct {
	streamResponses []*sdk.CreateChatCompletionStreamResponse
}

func (m *MockLLMClient) CreateChatCompletion(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (*sdk.CreateChatCompletionResponse, error) {
	message, _ := sdk.NewTextMessage(sdk.Assistant, "Mock response")
	return &sdk.CreateChatCompletionResponse{
		Choices: []sdk.ChatCompletionChoice{
			{
				Message: message,
			},
		},
	}, nil
}

func (m *MockLLMClient) CreateStreamingChatCompletion(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
	responseChan := make(chan *sdk.CreateChatCompletionStreamResponse)
	errorChan := make(chan error, 1)

	go func() {
		defer close(responseChan)
		defer close(errorChan)

		for _, resp := range m.streamResponses {
			select {
			case responseChan <- resp:
			case <-ctx.Done():
				return
			}
		}
	}()

	return responseChan, errorChan
}
