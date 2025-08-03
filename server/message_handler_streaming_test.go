package server_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	mocks "github.com/inference-gateway/adk/server/mocks"
	types "github.com/inference-gateway/adk/types"
	sdk "github.com/inference-gateway/sdk"
	assert "github.com/stretchr/testify/assert"
	zap "go.uber.org/zap"
)

func TestMessageHandler_HandleMessageStream_AgentStreaming_Success(t *testing.T) {
	logger := zap.NewNop()
	mockTaskManager := &mocks.FakeTaskManager{}

	expectedTask := &types.Task{
		ID:        "stream-test-task",
		ContextID: "stream-test-context",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		History: []types.Message{},
	}
	mockTaskManager.CreateTaskReturns(expectedTask)
	mockTaskManager.UpdateStateReturns(nil)
	mockTaskManager.UpdateConversationHistoryCalls(func(string, []types.Message) {})

	cfg := &config.Config{
		AgentConfig: config.AgentConfig{
			MaxChatCompletionIterations: 10,
		},
	}

	agent := &mocks.FakeOpenAICompatibleAgent{}
	streamChan := make(chan *types.Message, 3)
	go func() {
		defer close(streamChan)

		streamChan <- &types.Message{
			Kind:      "message",
			MessageID: "stream-1",
			Role:      "assistant",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hello, ",
				},
			},
		}

		streamChan <- &types.Message{
			Kind:      "message",
			MessageID: "stream-2",
			Role:      "assistant",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "I'm processing your request!",
				},
			},
		}
	}()

	agent.RunWithStreamReturns(streamChan, nil)
	messageHandler := server.NewDefaultMessageHandlerWithAgent(logger, mockTaskManager, agent, cfg)

	params := types.MessageSendParams{
		Message: types.Message{
			Kind:      "message",
			MessageID: "agent-stream-test",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hello agent, stream me a response",
				},
			},
		},
	}

	responseChan := make(chan types.SendStreamingMessageResponse, 10)
	defer close(responseChan)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var responses []types.SendStreamingMessageResponse
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			select {
			case response, ok := <-responseChan:
				if !ok {
					return
				}
				responses = append(responses, response)

				if statusEvent, ok := response.(types.TaskStatusUpdateEvent); ok && statusEvent.Final {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	err := messageHandler.HandleMessageStream(ctx, params, responseChan)
	assert.NoError(t, err)

	wg.Wait()

	assert.GreaterOrEqual(t, len(responses), 3, "Should have at least 3 events: initial + streaming + final")

	first, ok := responses[0].(types.TaskStatusUpdateEvent)
	assert.True(t, ok)
	assert.Equal(t, types.TaskStateWorking, first.Status.State)
	assert.False(t, first.Final)

	last, ok := responses[len(responses)-1].(types.TaskStatusUpdateEvent)
	assert.True(t, ok)
	assert.Equal(t, types.TaskStateCompleted, last.Status.State)
	assert.True(t, last.Final)

	assert.Equal(t, 1, mockTaskManager.CreateTaskCallCount())
}

func TestMessageHandler_HandleMessageStream_AgentStreamingError(t *testing.T) {
	logger := zap.NewNop()
	mockTaskManager := &mocks.FakeTaskManager{}

	expectedTask := &types.Task{
		ID:        "stream-error-task",
		ContextID: "stream-error-context",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		History: []types.Message{},
	}
	mockTaskManager.CreateTaskReturns(expectedTask)
	mockTaskManager.UpdateStateReturns(nil)
	mockTaskManager.UpdateConversationHistoryCalls(func(string, []types.Message) {})

	cfg := &config.Config{
		AgentConfig: config.AgentConfig{
			MaxChatCompletionIterations: 10,
		},
	}

	agent := &mocks.FakeOpenAICompatibleAgent{}
	agent.RunWithStreamReturns(nil, fmt.Errorf("agent streaming failed"))

	messageHandler := server.NewDefaultMessageHandlerWithAgent(logger, mockTaskManager, agent, cfg)

	params := types.MessageSendParams{
		Message: types.Message{
			Kind:      "message",
			MessageID: "agent-error-test",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "This should fail",
				},
			},
		},
	}

	responseChan := make(chan types.SendStreamingMessageResponse, 10)
	defer close(responseChan)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var responses []types.SendStreamingMessageResponse
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			select {
			case response, ok := <-responseChan:
				if !ok {
					return
				}
				responses = append(responses, response)

				if statusEvent, ok := response.(types.TaskStatusUpdateEvent); ok && statusEvent.Final {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	err := messageHandler.HandleMessageStream(ctx, params, responseChan)
	assert.NoError(t, err)

	wg.Wait()

	assert.GreaterOrEqual(t, len(responses), 2)

	last, ok := responses[len(responses)-1].(types.TaskStatusUpdateEvent)
	assert.True(t, ok)
	assert.Equal(t, types.TaskStateFailed, last.Status.State)
	assert.True(t, last.Final)

	assert.NotNil(t, last.Status.Message)
	if len(last.Status.Message.Parts) > 0 {
		if part, ok := last.Status.Message.Parts[0].(map[string]interface{}); ok {
			if text, exists := part["text"]; exists {
				assert.Contains(t, text, "Agent streaming failed")
			}
		}
	}
}

func TestMessageHandler_HandleMessageStream_IterativeStreaming(t *testing.T) {
	logger := zap.NewNop()
	mockTaskManager := &mocks.FakeTaskManager{}

	expectedTask := &types.Task{
		ID:        "iterative-test-task",
		ContextID: "iterative-test-context",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		History: []types.Message{},
	}
	mockTaskManager.CreateTaskReturns(expectedTask)
	mockTaskManager.UpdateStateReturns(nil)
	mockTaskManager.UpdateConversationHistoryCalls(func(string, []types.Message) {})

	cfg := &config.Config{
		AgentConfig: config.AgentConfig{
			MaxChatCompletionIterations: 3,
		},
	}

	messageHandler := server.NewDefaultMessageHandler(logger, mockTaskManager, cfg)

	llmClient := &mocks.FakeLLMClient{}
	respChan := make(chan *sdk.CreateChatCompletionStreamResponse, 3)
	errChan := make(chan error, 1)

	go func() {
		defer close(respChan)
		defer close(errChan)

		respChan <- &sdk.CreateChatCompletionStreamResponse{
			Choices: []sdk.ChatCompletionStreamChoice{
				{
					Delta: sdk.ChatCompletionStreamResponseDelta{
						Content: "Hello, ",
					},
				},
			},
		}

		respChan <- &sdk.CreateChatCompletionStreamResponse{
			Choices: []sdk.ChatCompletionStreamChoice{
				{
					Delta: sdk.ChatCompletionStreamResponseDelta{
						Content: "how can I help you?",
					},
				},
			},
		}

		respChan <- &sdk.CreateChatCompletionStreamResponse{
			Choices: []sdk.ChatCompletionStreamChoice{
				{
					Delta: sdk.ChatCompletionStreamResponseDelta{
						Content: "",
					},
					FinishReason: "stop",
				},
			},
		}
	}()

	llmClient.CreateStreamingChatCompletionReturns(respChan, errChan)

	params := types.MessageSendParams{
		Message: types.Message{
			Kind:      "message",
			MessageID: "iterative-simple",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hello, how are you?",
				},
			},
		},
	}

	responseChan := make(chan types.SendStreamingMessageResponse, 10)
	defer close(responseChan)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var responses []types.SendStreamingMessageResponse
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			select {
			case response, ok := <-responseChan:
				if !ok {
					return
				}
				responses = append(responses, response)

				if statusEvent, ok := response.(types.TaskStatusUpdateEvent); ok && statusEvent.Final {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	err := messageHandler.HandleMessageStream(ctx, params, responseChan)
	assert.NoError(t, err)

	wg.Wait()

	assert.GreaterOrEqual(t, len(responses), 3)

	assert.Equal(t, 1, mockTaskManager.CreateTaskCallCount())
}

func TestMessageHandler_HandleMessageStream_MockStreaming(t *testing.T) {
	logger := zap.NewNop()
	mockTaskManager := &mocks.FakeTaskManager{}

	expectedTask := &types.Task{
		ID:        "mock-stream-task",
		ContextID: "mock-stream-context",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		History: []types.Message{},
	}
	mockTaskManager.CreateTaskReturns(expectedTask)
	mockTaskManager.UpdateStateReturns(nil)

	cfg := &config.Config{
		AgentConfig: config.AgentConfig{
			MaxChatCompletionIterations: 10,
		},
	}

	messageHandler := server.NewDefaultMessageHandler(logger, mockTaskManager, cfg)

	params := types.MessageSendParams{
		Message: types.Message{
			Kind:      "message",
			MessageID: "mock-stream-test",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Test mock streaming",
				},
			},
		},
	}

	responseChan := make(chan types.SendStreamingMessageResponse, 10)
	defer close(responseChan)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var responses []types.SendStreamingMessageResponse
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			select {
			case response, ok := <-responseChan:
				if !ok {
					return
				}
				responses = append(responses, response)

				if statusEvent, ok := response.(types.TaskStatusUpdateEvent); ok && statusEvent.Final {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	err := messageHandler.HandleMessageStream(ctx, params, responseChan)
	assert.NoError(t, err)

	wg.Wait()

	assert.GreaterOrEqual(t, len(responses), 5, "Mock streaming should generate at least 5 events")

	first, ok := responses[0].(types.TaskStatusUpdateEvent)
	assert.True(t, ok)
	assert.Equal(t, types.TaskStateWorking, first.Status.State)
	assert.False(t, first.Final)

	mockContentFound := false
	for i := 1; i < len(responses)-1; i++ {
		event, ok := responses[i].(types.TaskStatusUpdateEvent)
		assert.True(t, ok)
		assert.Equal(t, types.TaskStateWorking, event.Status.State)
		assert.False(t, event.Final)

		if event.Status.Message != nil && len(event.Status.Message.Parts) > 0 {
			if part, ok := event.Status.Message.Parts[0].(map[string]interface{}); ok {
				if text, exists := part["text"]; exists {
					if textStr, ok := text.(string); ok {
						if textStr == "Starting to process your request..." {
							mockContentFound = true
						}
					}
				}
			}
		}
	}
	assert.True(t, mockContentFound, "Should find mock streaming content")

	last, ok := responses[len(responses)-1].(types.TaskStatusUpdateEvent)
	assert.True(t, ok)
	assert.Equal(t, types.TaskStateCompleted, last.Status.State)
	assert.True(t, last.Final)
}

func TestMessageHandler_HandleMessageStream_ContextCancellation(t *testing.T) {
	logger := zap.NewNop()
	mockTaskManager := &mocks.FakeTaskManager{}

	expectedTask := &types.Task{
		ID:        "cancel-test-task",
		ContextID: "cancel-test-context",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		History: []types.Message{},
	}
	mockTaskManager.CreateTaskReturns(expectedTask)
	mockTaskManager.UpdateStateReturns(nil)

	cfg := &config.Config{
		AgentConfig: config.AgentConfig{
			MaxChatCompletionIterations: 10,
		},
	}

	messageHandler := server.NewDefaultMessageHandler(logger, mockTaskManager, cfg)

	params := types.MessageSendParams{
		Message: types.Message{
			Kind:      "message",
			MessageID: "cancel-test",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "This will be cancelled",
				},
			},
		},
	}

	responseChan := make(chan types.SendStreamingMessageResponse, 10)
	defer close(responseChan)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := messageHandler.HandleMessageStream(ctx, params, responseChan)

	if err != nil {
		assert.Contains(t, err.Error(), "context deadline exceeded")
	}

	assert.Equal(t, 1, mockTaskManager.CreateTaskCallCount())
}

func TestMessageHandler_HandleMessageStream_EmptyParts(t *testing.T) {
	logger := zap.NewNop()
	mockTaskManager := &mocks.FakeTaskManager{}

	cfg := &config.Config{
		AgentConfig: config.AgentConfig{
			MaxChatCompletionIterations: 10,
		},
	}

	messageHandler := server.NewDefaultMessageHandler(logger, mockTaskManager, cfg)

	params := types.MessageSendParams{
		Message: types.Message{
			Kind:      "message",
			MessageID: "empty-parts-test",
			Role:      "user",
			Parts:     []types.Part{},
		},
	}

	responseChan := make(chan types.SendStreamingMessageResponse, 10)
	defer close(responseChan)

	ctx := context.Background()

	err := messageHandler.HandleMessageStream(ctx, params, responseChan)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty message parts")

	assert.Equal(t, 0, mockTaskManager.CreateTaskCallCount())
}

func TestMessageHandler_HandleMessageStream_WithToolCalls(t *testing.T) {
	logger := zap.NewNop()
	mockTaskManager := &mocks.FakeTaskManager{}

	expectedTask := &types.Task{
		ID:        "tool-test-task",
		ContextID: "tool-test-context",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		History: []types.Message{},
	}
	mockTaskManager.CreateTaskReturns(expectedTask)
	mockTaskManager.UpdateStateReturns(nil)
	mockTaskManager.UpdateConversationHistoryCalls(func(string, []types.Message) {})

	cfg := &config.Config{
		AgentConfig: config.AgentConfig{
			MaxChatCompletionIterations: 10,
		},
	}

	agent := &mocks.FakeOpenAICompatibleAgent{}
	streamChan := make(chan *types.Message, 2)
	go func() {
		defer close(streamChan)

		streamChan <- &types.Message{
			Kind:      "message",
			MessageID: "tool-call-1",
			Role:      "assistant",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "data",
					"data": map[string]interface{}{
						"tool_calls": []sdk.ChatCompletionMessageToolCall{
							{
								Id:   "call_123",
								Type: "function",
								Function: sdk.ChatCompletionMessageToolCallFunction{
									Name:      "test_tool",
									Arguments: `{"param": "value"}`,
								},
							},
						},
					},
				},
			},
		}

		streamChan <- &types.Message{
			Kind:      "message",
			MessageID: "final-response",
			Role:      "assistant",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Tool executed successfully!",
				},
			},
		}
	}()

	agent.RunWithStreamReturns(streamChan, nil)
	messageHandler := server.NewDefaultMessageHandlerWithAgent(logger, mockTaskManager, agent, cfg)

	params := types.MessageSendParams{
		Message: types.Message{
			Kind:      "message",
			MessageID: "tool-stream-test",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Use a tool please",
				},
			},
		},
	}

	responseChan := make(chan types.SendStreamingMessageResponse, 10)
	defer close(responseChan)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var responses []types.SendStreamingMessageResponse
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			select {
			case response, ok := <-responseChan:
				if !ok {
					return
				}
				responses = append(responses, response)

				if statusEvent, ok := response.(types.TaskStatusUpdateEvent); ok && statusEvent.Final {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	err := messageHandler.HandleMessageStream(ctx, params, responseChan)
	assert.NoError(t, err)

	wg.Wait()

	assert.GreaterOrEqual(t, len(responses), 3)

	foundToolCall := false
	for _, event := range responses {
		if statusEvent, ok := event.(types.TaskStatusUpdateEvent); ok {
			if statusEvent.Status.Message != nil && len(statusEvent.Status.Message.Parts) > 0 {
				if part, ok := statusEvent.Status.Message.Parts[0].(map[string]interface{}); ok {
					if part["kind"] == "data" {
						if data, ok := part["data"].(map[string]interface{}); ok {
							if _, exists := data["tool_calls"]; exists {
								foundToolCall = true
							}
						}
					}
				}
			}
		}
	}

	assert.True(t, foundToolCall, "Should find tool call in streaming events")

	assert.Equal(t, 1, mockTaskManager.CreateTaskCallCount())
}
