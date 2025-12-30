package server

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zap "go.uber.org/zap"
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

	agent := NewOpenAICompatibleAgentWithConfig(logger, &config.AgentConfig{
		MaxChatCompletionIterations: 10,
		SystemPrompt:                "You are a test assistant",
	})
	agent.SetLLMClient(mockLLMClient)

	handler := NewDefaultBackgroundTaskHandler(logger, agent)

	task := &types.Task{
		ID:        "test-task-123",
		ContextID: "test-context-456",
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

	assert.Contains(t, *resultTask.Metadata, "usage", "Metadata should contain 'usage' field")
	usageMap, ok := (*resultTask.Metadata)["usage"].(map[string]any)
	require.True(t, ok, "Usage should be a map")
	assert.Equal(t, int64(100), usageMap["prompt_tokens"])
	assert.Equal(t, int64(50), usageMap["completion_tokens"])
	assert.Equal(t, int64(150), usageMap["total_tokens"])

	assert.Contains(t, *resultTask.Metadata, "execution_stats", "Metadata should contain 'execution_stats' field")
	execStats, ok := (*resultTask.Metadata)["execution_stats"].(map[string]any)
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

	agent := NewOpenAICompatibleAgentWithConfig(logger, &config.AgentConfig{
		MaxChatCompletionIterations: 10,
		SystemPrompt:                "You are a test assistant",
	})
	agent.SetLLMClient(mockLLMClient)

	handler := NewDefaultStreamingTaskHandler(logger, agent)

	task := &types.Task{
		ID:        "test-streaming-task-123",
		ContextID: "test-streaming-context-456",
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

	assert.Contains(t, *task.Metadata, "usage", "Metadata should contain 'usage' field")
	usageMap, ok := (*task.Metadata)["usage"].(map[string]any)
	require.True(t, ok, "Usage should be a map")
	assert.Equal(t, int64(200), usageMap["prompt_tokens"])
	assert.Equal(t, int64(75), usageMap["completion_tokens"])
	assert.Equal(t, int64(275), usageMap["total_tokens"])

	assert.Contains(t, *task.Metadata, "execution_stats", "Metadata should contain 'execution_stats' field")
	execStats, ok := (*task.Metadata)["execution_stats"].(map[string]any)
	require.True(t, ok, "Execution stats should be a map")
	assert.Greater(t, execStats["iterations"], 0, "Should have at least one iteration")
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
