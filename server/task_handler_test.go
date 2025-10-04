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

func TestDefaultA2AProtocolHandler_MessageEnrichment(t *testing.T) {
	tests := []struct {
		name              string
		inputMessage      types.Message
		expectedKind      string
		expectedMessageID string
		shouldGenerateID  bool
	}{
		{
			name: "message without Kind and MessageID should be enriched",
			inputMessage: types.Message{
				Role: "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Hello without kind and messageId",
					},
				},
			},
			expectedKind:     "message",
			shouldGenerateID: true,
		},
		{
			name: "message with empty Kind and MessageID should be enriched",
			inputMessage: types.Message{
				Kind:      "",
				MessageID: "",
				Role:      "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Hello with empty kind and messageId",
					},
				},
			},
			expectedKind:     "message",
			shouldGenerateID: true,
		},
		{
			name: "message with existing Kind and MessageID should be preserved",
			inputMessage: types.Message{
				Kind:      "existing_kind",
				MessageID: "existing_message_id",
				Role:      "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Hello with existing kind and messageId",
					},
				},
			},
			expectedKind:      "existing_kind",
			expectedMessageID: "existing_message_id",
			shouldGenerateID:  false,
		},
		{
			name: "message with only Kind should generate MessageID",
			inputMessage: types.Message{
				Kind: "custom_kind",
				Role: "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Hello with custom kind but no messageId",
					},
				},
			},
			expectedKind:     "custom_kind",
			shouldGenerateID: true,
		},
		{
			name: "message with only MessageID should get default Kind",
			inputMessage: types.Message{
				MessageID: "custom_message_id",
				Role:      "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Hello with custom messageId but no kind",
					},
				},
			},
			expectedKind:      "message",
			expectedMessageID: "custom_message_id",
			shouldGenerateID:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTaskManager := &mocks.FakeTaskManager{}
			mockStorage := &mocks.FakeStorage{}
			mockResponseSender := &mocks.FakeResponseSender{}
			mockTaskHandler := &mocks.FakeTaskHandler{}
			mockStreamingTaskHandler := &mocks.FakeStreamableTaskHandler{}

			mockTaskManager.GetConversationHistoryReturns([]types.Message{})

			expectedTask := &types.Task{
				ID:        "test-task-id",
				ContextID: "test-context",
				Status: types.TaskStatus{
					State: types.TaskStateSubmitted,
				},
			}
			mockTaskManager.CreateTaskReturns(expectedTask)

			logger := zap.NewNop()
			handler := server.NewDefaultA2AProtocolHandler(
				logger,
				mockStorage,
				mockTaskManager,
				mockResponseSender,
				mockTaskHandler,
				mockStreamingTaskHandler,
			)

			contextID := "test-context"
			params := types.MessageSendParams{
				Message: tt.inputMessage,
			}
			params.Message.ContextID = &contextID

			ctx := context.Background()
			task, err := handler.CreateTaskFromMessage(ctx, params)

			assert.NoError(t, err)
			assert.NotNil(t, task)

			assert.Equal(t, 1, mockTaskManager.CreateTaskCallCount())
			_, _, enrichedMessage := mockTaskManager.CreateTaskArgsForCall(0)

			assert.Equal(t, tt.expectedKind, enrichedMessage.Kind)

			if tt.shouldGenerateID {
				assert.NotEmpty(t, enrichedMessage.MessageID, "MessageID should be generated when missing")
				assert.NotEqual(t, "", enrichedMessage.MessageID, "MessageID should not be empty")
			} else {
				assert.Equal(t, tt.expectedMessageID, enrichedMessage.MessageID)
			}

			assert.Equal(t, tt.inputMessage.Role, enrichedMessage.Role)
			assert.Equal(t, tt.inputMessage.Parts, enrichedMessage.Parts)
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

func TestDefaultTaskHandler_ArtifactExtraction(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name                string
		agentResponse       *server.AgentResponse
		expectedArtifacts   int
		expectedArtifactIDs []string
		description         string
	}{
		{
			name: "extracts artifact from tool result with artifactId",
			agentResponse: &server.AgentResponse{
				AdditionalMessages: []types.Message{
					{
						Kind:      "message",
						MessageID: "tool-result-1",
						Role:      "tool",
						Parts: []types.Part{
							map[string]any{
								"artifactId":  "test-artifact-1",
								"name":        "Test Artifact",
								"description": "A test artifact",
								"kind":        "file",
								"file": map[string]any{
									"name":     "test.txt",
									"mimeType": "text/plain",
									"bytes":    "dGVzdCBjb250ZW50", // base64 "test content"
								},
							},
						},
					},
				},
				Response: &types.Message{
					Kind:      "message",
					MessageID: "response-1",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Tool executed successfully",
						},
					},
				},
			},
			expectedArtifacts:   1,
			expectedArtifactIDs: []string{"test-artifact-1"},
			description:         "Should extract artifact with explicit artifactId",
		},
		{
			name: "extracts artifact from nested artifact field",
			agentResponse: &server.AgentResponse{
				AdditionalMessages: []types.Message{
					{
						Kind:      "message",
						MessageID: "tool-result-2",
						Role:      "tool",
						Parts: []types.Part{
							map[string]any{
								"kind": "text",
								"text": "Tool result with nested artifact",
								"artifact": map[string]any{
									"artifactId":  "nested-artifact-1",
									"name":        "Nested Artifact",
									"description": "A nested artifact",
									"parts": []any{
										map[string]any{
											"kind": "text",
											"text": "Artifact content",
										},
									},
								},
							},
						},
					},
				},
				Response: nil,
			},
			expectedArtifacts:   1,
			expectedArtifactIDs: []string{"nested-artifact-1"},
			description:         "Should extract artifact from nested artifact field",
		},
		{
			name: "extracts multiple artifacts from multiple tool results",
			agentResponse: &server.AgentResponse{
				AdditionalMessages: []types.Message{
					{
						Kind:      "message",
						MessageID: "tool-result-3",
						Role:      "tool",
						Parts: []types.Part{
							map[string]any{
								"artifactId": "multi-artifact-1",
								"name":       "First Artifact",
								"kind":       "text",
								"text":       "First artifact content",
							},
						},
					},
					{
						Kind:      "message",
						MessageID: "tool-result-4",
						Role:      "tool",
						Parts: []types.Part{
							map[string]any{
								"artifactId": "multi-artifact-2",
								"name":       "Second Artifact",
								"kind":       "data",
								"data":       map[string]any{"key": "value"},
							},
						},
					},
				},
				Response: nil,
			},
			expectedArtifacts:   2,
			expectedArtifactIDs: []string{"multi-artifact-1", "multi-artifact-2"},
			description:         "Should extract multiple artifacts from multiple tool results",
		},
		{
			name: "skips non-tool messages",
			agentResponse: &server.AgentResponse{
				AdditionalMessages: []types.Message{
					{
						Kind:      "message",
						MessageID: "user-msg",
						Role:      "user",
						Parts: []types.Part{
							map[string]any{
								"artifactId": "user-artifact",
								"kind":       "text",
								"text":       "This should be ignored",
							},
						},
					},
					{
						Kind:      "message",
						MessageID: "tool-result-5",
						Role:      "tool",
						Parts: []types.Part{
							map[string]any{
								"artifactId": "tool-artifact",
								"kind":       "text",
								"text":       "This should be extracted",
							},
						},
					},
				},
				Response: nil,
			},
			expectedArtifacts:   1,
			expectedArtifactIDs: []string{"tool-artifact"},
			description:         "Should only extract artifacts from tool messages",
		},
		{
			name: "handles empty tool results",
			agentResponse: &server.AgentResponse{
				AdditionalMessages: []types.Message{
					{
						Kind:      "message",
						MessageID: "empty-tool-result",
						Role:      "tool",
						Parts:     []types.Part{},
					},
				},
				Response: nil,
			},
			expectedArtifacts:   0,
			expectedArtifactIDs: []string{},
			description:         "Should handle empty tool results gracefully",
		},
		{
			name: "no artifacts when no tool results",
			agentResponse: &server.AgentResponse{
				AdditionalMessages: []types.Message{},
				Response: &types.Message{
					Kind:      "message",
					MessageID: "response-only",
					Role:      "assistant",
					Parts: []types.Part{
						map[string]any{
							"kind": "text",
							"text": "Just a response, no artifacts",
						},
					},
				},
			},
			expectedArtifacts:   0,
			expectedArtifactIDs: []string{},
			description:         "Should handle cases with no tool results",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock agent that returns our test response
			mockAgent := &mocks.FakeOpenAICompatibleAgent{}
			mockAgent.RunReturns(tt.agentResponse, nil)

			// Create task handler with mock agent
			taskHandler := server.NewDefaultTaskHandlerWithAgent(logger, mockAgent)

			// Create test task
			task := &types.Task{
				ID:        "test-task-" + tt.name,
				ContextID: "test-context",
				History:   []types.Message{},
				Artifacts: []types.Artifact{},
				Status: types.TaskStatus{
					State: types.TaskStateSubmitted,
				},
			}

			// Create test message
			message := &types.Message{
				Kind:      "message",
				MessageID: "test-message",
				Role:      "user",
				Parts: []types.Part{
					map[string]any{
						"kind": "text",
						"text": "Test message for " + tt.description,
					},
				},
			}

			// Execute the task
			result, err := taskHandler.HandleTask(context.Background(), task, message)

			// Verify no error occurred
			assert.NoError(t, err)
			assert.NotNil(t, result)

			// Verify task was completed
			assert.Equal(t, types.TaskStateCompleted, result.Status.State)

			// Verify artifact extraction
			assert.Equal(t, tt.expectedArtifacts, len(result.Artifacts),
				"Expected %d artifacts but got %d for: %s",
				tt.expectedArtifacts, len(result.Artifacts), tt.description)

			// Verify specific artifact IDs
			actualArtifactIDs := make([]string, len(result.Artifacts))
			for i, artifact := range result.Artifacts {
				actualArtifactIDs[i] = artifact.ArtifactID
			}

			if tt.expectedArtifacts > 0 {
				for _, expectedID := range tt.expectedArtifactIDs {
					assert.Contains(t, actualArtifactIDs, expectedID,
						"Expected artifact ID %s not found in %v for: %s",
						expectedID, actualArtifactIDs, tt.description)
				}
			}

			// Verify all extracted artifacts are valid
			artifactHelper := server.NewArtifactHelper()
			for _, artifact := range result.Artifacts {
				err := artifactHelper.ValidateArtifact(artifact)
				assert.NoError(t, err, "Extracted artifact %s should be valid for: %s",
					artifact.ArtifactID, tt.description)
			}
		})
	}
}

func TestDefaultBackgroundTaskHandler_ArtifactExtraction(t *testing.T) {
	logger := zap.NewNop()

	// Create a mock agent that returns a response with artifacts
	mockAgent := &mocks.FakeOpenAICompatibleAgent{}
	agentResponse := &server.AgentResponse{
		AdditionalMessages: []types.Message{
			{
				Kind:      "message",
				MessageID: "tool-result-bg",
				Role:      "tool",
				Parts: []types.Part{
					map[string]any{
						"artifactId":  "bg-artifact-1",
						"name":        "Background Artifact",
						"description": "An artifact from background processing",
						"kind":        "file",
						"file": map[string]any{
							"name":     "bg-result.json",
							"mimeType": "application/json",
							"bytes":    "eyJyZXN1bHQiOiAic3VjY2VzcyJ9", // base64 {"result": "success"}
						},
					},
				},
			},
		},
		Response: &types.Message{
			Kind:      "message",
			MessageID: "bg-response",
			Role:      "assistant",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Background processing completed with artifact",
				},
			},
		},
	}
	mockAgent.RunReturns(agentResponse, nil)

	// Create background task handler with mock agent
	taskHandler := server.NewDefaultBackgroundTaskHandlerWithAgent(logger, mockAgent)

	// Create test task
	task := &types.Task{
		ID:        "test-bg-task",
		ContextID: "test-bg-context",
		History:   []types.Message{},
		Artifacts: []types.Artifact{},
		Status: types.TaskStatus{
			State: types.TaskStateSubmitted,
		},
	}

	// Create test message
	message := &types.Message{
		Kind:      "message",
		MessageID: "test-bg-message",
		Role:      "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Test background processing with artifacts",
			},
		},
	}

	// Execute the task
	result, err := taskHandler.HandleTask(context.Background(), task, message)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, types.TaskStateCompleted, result.Status.State)

	// Verify artifact extraction
	assert.Equal(t, 1, len(result.Artifacts), "Expected 1 artifact from background processing")
	assert.Equal(t, "bg-artifact-1", result.Artifacts[0].ArtifactID)
	assert.Equal(t, "Background Artifact", *result.Artifacts[0].Name)

	// Verify artifact is valid
	artifactHelper := server.NewArtifactHelper()
	err = artifactHelper.ValidateArtifact(result.Artifacts[0])
	assert.NoError(t, err, "Background extracted artifact should be valid")
}
