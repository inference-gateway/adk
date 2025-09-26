package server_test

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	server "github.com/inference-gateway/adk/server"
	mocks "github.com/inference-gateway/adk/server/mocks"
	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
	zap "go.uber.org/zap"
)

func TestDefaultTaskHandler_HandleTask(t *testing.T) {
	tests := []struct {
		name        string
		task        *types.Task
		expectError bool
		expectedMsg string
	}{
		{
			name: "default handler provides basic response - task with message",
			task: &types.Task{
				ID:        "test-task-1",
				ContextID: "test-context",
				Status: types.TaskStatus{
					State: types.TaskStateSubmitted,
					Message: &types.Message{
						Kind:      "message",
						MessageID: "test-msg",
						Role:      "user",
						Parts: []types.Part{
							map[string]any{
								"kind": "text",
								"text": "Hello from task",
							},
						},
					},
				},
			},
			expectError: false,
			expectedMsg: "",
		},
		{
			name: "default handler provides basic response - task with nil message",
			task: &types.Task{
				ID:        "test-task-2",
				ContextID: "test-context",
				Status: types.TaskStatus{
					State:   types.TaskStateSubmitted,
					Message: nil,
				},
			},
			expectError: false,
			expectedMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			taskHandler := server.NewDefaultTaskHandler(logger)

			ctx := context.Background()
			message := tt.task.Status.Message
			result, err := taskHandler.HandleTask(ctx, tt.task, message)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.expectedMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, types.TaskStateCompleted, result.Status.State)
				assert.NotNil(t, result.Status.Message)
				assert.Equal(t, "assistant", result.Status.Message.Role)
				assert.GreaterOrEqual(t, len(result.History), 1)
			}
		})
	}
}

func TestDefaultBackgroundTaskHandler_HandleTask(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	tests := []struct {
		name           string
		task           *types.Task
		agent          server.OpenAICompatibleAgent
		expectedState  types.TaskState
		expectInputReq bool
	}{
		{
			name: "successful polling task without agent",
			task: &types.Task{
				ID:        "test-task-1",
				ContextID: "test-context-1",
				Status:    types.TaskStatus{State: types.TaskStateSubmitted},
				History:   []types.Message{},
			},
			agent:          nil,
			expectedState:  types.TaskStateCompleted,
			expectInputReq: false,
		},
		{
			name: "polling task with agent requiring input",
			task: &types.Task{
				ID:        "test-task-2",
				ContextID: "test-context-2",
				Status:    types.TaskStatus{State: types.TaskStateSubmitted},
				History:   []types.Message{},
			},
			agent:          createMockAgentWithInputRequired(),
			expectedState:  types.TaskStateInputRequired,
			expectInputReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskHandler := server.NewDefaultBackgroundTaskHandler(logger, tt.agent)
			if tt.agent != nil {
				taskHandler.SetAgent(tt.agent)
			}
			message := &types.Message{
				Kind: "message",
				Role: "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Test message",
					},
				},
			}

			result, err := taskHandler.HandleTask(ctx, tt.task, message)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedState, result.Status.State)

			if tt.expectInputReq {
				assert.Equal(t, types.TaskStateInputRequired, result.Status.State)
				assert.NotNil(t, result.Status.Message)
			}

			assert.Greater(t, len(result.History), 0)
		})
	}
}

func TestDefaultStreamingTaskHandler_HandleStreamingTask(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	tests := []struct {
		name           string
		task           *types.Task
		agent          server.OpenAICompatibleAgent
		expectError    bool
		expectInputReq bool
	}{
		{
			name: "streaming task without agent should error",
			task: &types.Task{
				ID:        "test-task-1",
				ContextID: "test-context-1",
				Status:    types.TaskStatus{State: types.TaskStateSubmitted},
				History:   []types.Message{},
			},
			agent:       nil,
			expectError: true,
		},
		{
			name: "streaming task with agent requiring input",
			task: &types.Task{
				ID:        "test-task-2",
				ContextID: "test-context-2",
				Status:    types.TaskStatus{State: types.TaskStateSubmitted},
				History:   []types.Message{},
			},
			agent:          createMockAgentWithInputRequired(),
			expectError:    false,
			expectInputReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskHandler := server.NewDefaultStreamingTaskHandler(logger, tt.agent)
			message := &types.Message{
				Kind: "message",
				Role: "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Test streaming message",
					},
				},
			}

			eventsChan, err := taskHandler.HandleStreamingTask(ctx, tt.task, message)

			assert.NoError(t, err)
			assert.NotNil(t, eventsChan)

			var events []server.StreamEvent
			for event := range eventsChan {
				events = append(events, event)
			}

			if tt.expectError {
				hasErrorEvent := false
				for _, event := range events {
					if event.GetEventType() == "error" {
						hasErrorEvent = true
						break
					}
				}
				assert.True(t, hasErrorEvent, "Expected an error event")
			} else {
				hasCompleteEvent := false
				for _, event := range events {
					if event.GetEventType() == "task_complete" {
						hasCompleteEvent = true
						if tt.expectInputReq {
							if taskData, ok := event.GetData().(*types.Task); ok {
								assert.Equal(t, types.TaskStateInputRequired, taskData.Status.State)
							}
						}
						break
					}
				}
				assert.True(t, hasCompleteEvent, "Expected a task completion event")
			}
		})
	}
}

// createMockAgentWithInputRequired creates a mock agent that returns an input_required response
func createMockAgentWithInputRequired() server.OpenAICompatibleAgent {
	mockAgent := &mocks.FakeOpenAICompatibleAgent{}

	inputRequiredResponse := &server.AgentResponse{
		Response: &types.Message{
			Kind:      "input_required",
			MessageID: "input-req-123",
			Role:      "assistant",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "I need more information from you to continue.",
				},
			},
		},
		AdditionalMessages: []types.Message{},
	}

	mockAgent.RunReturns(inputRequiredResponse, nil)

	streamChan := make(chan cloudevents.Event, 1)

	inputMessage := &types.Message{
		Kind:      "input_required",
		MessageID: "stream-input-req-123",
		Role:      "assistant",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "I need more information from you to continue.",
			},
		},
	}

	event := types.NewMessageEvent("adk.agent.input.required", "stream-input-req-123", inputMessage, nil)
	streamChan <- event
	close(streamChan)

	mockAgent.RunWithStreamReturns(streamChan, nil)
	return mockAgent
}

func TestDefaultA2AProtocolHandler_ContextHistoryHandling(t *testing.T) {
	tests := []struct {
		name                            string
		contextID                       *string
		existingHistory                 []types.Message
		expectGetHistoryCall            bool
		expectCreateTaskCall            bool
		expectCreateTaskWithHistoryCall bool
		expectedHistoryCount            int
	}{
		{
			name:                            "new context without history should use CreateTask",
			contextID:                       stringPtr("new-context"),
			existingHistory:                 []types.Message{},
			expectGetHistoryCall:            true,
			expectCreateTaskCall:            true,
			expectCreateTaskWithHistoryCall: false,
			expectedHistoryCount:            0,
		},
		{
			name:      "existing context with history should use CreateTaskWithHistory",
			contextID: stringPtr("existing-context"),
			existingHistory: []types.Message{
				{
					Kind:      "message",
					MessageID: "msg-1",
					Role:      "user",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Previous message from conversation",
						},
					},
				},
			},
			expectGetHistoryCall:            true,
			expectCreateTaskCall:            false,
			expectCreateTaskWithHistoryCall: true,
			expectedHistoryCount:            1,
		},
		{
			name:                            "nil context ID should generate new ID and use CreateTask",
			contextID:                       nil,
			existingHistory:                 []types.Message{},
			expectGetHistoryCall:            false,
			expectCreateTaskCall:            true,
			expectCreateTaskWithHistoryCall: false,
			expectedHistoryCount:            0,
		},
		{
			name:      "context with multiple history messages should use CreateTaskWithHistory",
			contextID: stringPtr("multi-history-context"),
			existingHistory: []types.Message{
				{
					Kind:      "message",
					MessageID: "msg-1",
					Role:      "user",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "First message",
						},
					},
				},
				{
					Kind:      "message",
					MessageID: "msg-2",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Assistant response",
						},
					},
				},
			},
			expectGetHistoryCall:            true,
			expectCreateTaskCall:            false,
			expectCreateTaskWithHistoryCall: true,
			expectedHistoryCount:            2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTaskManager := &mocks.FakeTaskManager{}

			mockTaskManager.GetConversationHistoryReturns(tt.existingHistory)

			expectedTask := &types.Task{
				ID:        "test-task-id",
				ContextID: "test-context",
				Status: types.TaskStatus{
					State: types.TaskStateSubmitted,
				},
			}

			mockTaskManager.CreateTaskWithHistoryReturns(expectedTask)
			mockTaskManager.CreateTaskReturns(expectedTask)

			testMessage := types.Message{
				Kind:      "message",
				MessageID: "test-msg",
				Role:      "user",
				ContextID: tt.contextID,
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Test message for context history",
					},
				},
			}

			originalContextID := tt.contextID
			contextIDValue := ""
			if tt.contextID != nil {
				contextIDValue = *tt.contextID
			} else {
				contextIDValue = "generated-context-id"
			}

			if originalContextID != nil {
				history := mockTaskManager.GetConversationHistory(contextIDValue)
				if len(history) > 0 {
					mockTaskManager.CreateTaskWithHistory(contextIDValue, types.TaskStateSubmitted, &testMessage, history)
				} else {
					mockTaskManager.CreateTask(contextIDValue, types.TaskStateSubmitted, &testMessage)
				}
			} else {
				mockTaskManager.CreateTask(contextIDValue, types.TaskStateSubmitted, &testMessage)
			}

			if tt.expectGetHistoryCall {
				assert.GreaterOrEqual(t, mockTaskManager.GetConversationHistoryCallCount(), 1,
					"GetConversationHistory should be called to check for existing history")
			} else {
				assert.Equal(t, 0, mockTaskManager.GetConversationHistoryCallCount(),
					"GetConversationHistory should not be called when context ID is nil (optimization)")
			}

			if tt.expectCreateTaskCall {
				assert.Equal(t, 1, mockTaskManager.CreateTaskCallCount(),
					"CreateTask should be called for new contexts without history")
				assert.Equal(t, 0, mockTaskManager.CreateTaskWithHistoryCallCount(),
					"CreateTaskWithHistory should not be called for new contexts")
			}

			if tt.expectCreateTaskWithHistoryCall {
				assert.Equal(t, 1, mockTaskManager.CreateTaskWithHistoryCallCount(),
					"CreateTaskWithHistory should be called for existing contexts with history")
				assert.Equal(t, 0, mockTaskManager.CreateTaskCallCount(),
					"CreateTask should not be called for existing contexts with history")

				_, _, _, history := mockTaskManager.CreateTaskWithHistoryArgsForCall(0)
				assert.Equal(t, tt.expectedHistoryCount, len(history),
					"History should be passed with correct number of messages")
				assert.Equal(t, tt.existingHistory, history,
					"History should match the existing conversation history")
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
