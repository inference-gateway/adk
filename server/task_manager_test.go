package server_test

import (
	"fmt"
	"testing"
	"time"

	server "github.com/inference-gateway/adk/server"
	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
	zap "go.uber.org/zap"
)

func TestDefaultTaskManager_CreateTask(t *testing.T) {
	tests := []struct {
		name      string
		contextID string
		state     types.TaskState
		message   *types.Message
	}{
		{
			name:      "create task with submitted state",
			contextID: "context-1",
			state:     types.TaskStateSubmitted,
			message: &types.Message{
				Kind:      "message",
				MessageID: "msg-1",
				Role:      "user",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Test message",
					},
				},
			},
		},
		{
			name:      "create task with working state",
			contextID: "context-2",
			state:     types.TaskStateWorking,
			message: &types.Message{
				Kind:      "message",
				MessageID: "msg-2",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Processing...",
					},
				},
			},
		},
		{
			name:      "create task with completed state",
			contextID: "context-3",
			state:     types.TaskStateCompleted,
			message: &types.Message{
				Kind:      "message",
				MessageID: "msg-3",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Task completed",
					},
				},
			},
		},
		{
			name:      "create task with failed state",
			contextID: "context-4",
			state:     types.TaskStateFailed,
			message: &types.Message{
				Kind:      "message",
				MessageID: "msg-4",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Task failed",
					},
				},
			},
		},
		{
			name:      "create task with nil message",
			contextID: "context-5",
			state:     types.TaskStateSubmitted,
			message:   nil,
		},
		{
			name:      "create task with empty context",
			contextID: "",
			state:     types.TaskStateSubmitted,
			message: &types.Message{
				Kind:      "message",
				MessageID: "msg-6",
				Role:      "user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			taskManager := server.NewDefaultTaskManager(logger, 20)

			task := taskManager.CreateTask(tt.contextID, tt.state, tt.message)

			assert.NotNil(t, task)
			assert.NotEmpty(t, task.ID)
			assert.Equal(t, tt.contextID, task.ContextID)
			assert.Equal(t, tt.state, task.Status.State)
			assert.Equal(t, tt.message, task.Status.Message)
			assert.NotNil(t, task.Status.Timestamp)

			if task.Status.Timestamp != nil {
				timestamp, err := time.Parse(time.RFC3339Nano, *task.Status.Timestamp)
				assert.NoError(t, err)
				assert.WithinDuration(t, time.Now(), timestamp, time.Second)
			}
		})
	}
}

func TestDefaultTaskManager_GetTask(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 20)

	message := &types.Message{
		Kind:      "message",
		MessageID: "test-msg",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Test message",
			},
		},
	}

	createdTask := taskManager.CreateTask("test-context", types.TaskStateSubmitted, message)

	retrievedTask, exists := taskManager.GetTask(createdTask.ID)
	assert.True(t, exists)
	assert.NotNil(t, retrievedTask)
	assert.Equal(t, createdTask.ID, retrievedTask.ID)
	assert.Equal(t, createdTask.ContextID, retrievedTask.ContextID)
	assert.Equal(t, createdTask.Status.State, retrievedTask.Status.State)

	nonExistentTask, exists := taskManager.GetTask("non-existent-id")
	assert.False(t, exists)
	assert.Nil(t, nonExistentTask)

	emptyTask, exists := taskManager.GetTask("")
	assert.False(t, exists)
	assert.Nil(t, emptyTask)
}

func TestDefaultTaskManager_UpdateTask(t *testing.T) {
	tests := []struct {
		name        string
		newState    types.TaskState
		newMessage  *types.Message
		expectError bool
	}{
		{
			name:     "update to working state",
			newState: types.TaskStateWorking,
			newMessage: &types.Message{
				Kind:      "message",
				MessageID: "updated-msg-1",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Now working on the task",
					},
				},
			},
			expectError: false,
		},
		{
			name:     "update to completed state",
			newState: types.TaskStateCompleted,
			newMessage: &types.Message{
				Kind:      "message",
				MessageID: "updated-msg-2",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Task completed successfully",
					},
				},
			},
			expectError: false,
		},
		{
			name:     "update to failed state",
			newState: types.TaskStateFailed,
			newMessage: &types.Message{
				Kind:      "message",
				MessageID: "updated-msg-3",
				Role:      "assistant",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Task failed with error",
					},
				},
			},
			expectError: false,
		},
		{
			name:        "update with nil message",
			newState:    types.TaskStateWorking,
			newMessage:  nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			taskManager := server.NewDefaultTaskManager(logger, 20)

			originalMessage := &types.Message{
				Kind:      "message",
				MessageID: "original-msg",
				Role:      "user",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Original message",
					},
				},
			}

			task := taskManager.CreateTask("test-context", types.TaskStateSubmitted, originalMessage)
			originalTimestamp := task.Status.Timestamp

			err := taskManager.UpdateTask(task.ID, tt.newState, tt.newMessage)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				updatedTask, exists := taskManager.GetTask(task.ID)
				assert.True(t, exists)
				assert.Equal(t, tt.newState, updatedTask.Status.State)
				assert.Equal(t, tt.newMessage, updatedTask.Status.Message)

				if originalTimestamp != nil && updatedTask.Status.Timestamp != nil {
					originalTime, err := time.Parse(time.RFC3339Nano, *originalTimestamp)
					assert.NoError(t, err)
					updatedTime, err := time.Parse(time.RFC3339Nano, *updatedTask.Status.Timestamp)
					assert.NoError(t, err)
					assert.True(t, updatedTime.After(originalTime))
				}
			}
		})
	}
}

func TestDefaultTaskManager_UpdateNonExistentTask(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 20)

	message := &types.Message{
		Kind:      "message",
		MessageID: "test-msg",
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Update message",
			},
		},
	}

	err := taskManager.UpdateTask("non-existent-id", types.TaskStateCompleted, message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")
}

func TestDefaultTaskManager_CleanupCompletedTasks(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 20)

	submittedTask := taskManager.CreateTask("context-1", types.TaskStateSubmitted, nil)
	workingTask := taskManager.CreateTask("context-2", types.TaskStateWorking, nil)
	completedTask := taskManager.CreateTask("context-3", types.TaskStateCompleted, nil)
	failedTask := taskManager.CreateTask("context-4", types.TaskStateFailed, nil)

	_, exists := taskManager.GetTask(submittedTask.ID)
	assert.True(t, exists)
	_, exists = taskManager.GetTask(workingTask.ID)
	assert.True(t, exists)
	_, exists = taskManager.GetTask(completedTask.ID)
	assert.True(t, exists)
	_, exists = taskManager.GetTask(failedTask.ID)
	assert.True(t, exists)

	taskManager.CleanupCompletedTasks()

	_, exists = taskManager.GetTask(submittedTask.ID)
	assert.True(t, exists, "submitted task should remain")
	_, exists = taskManager.GetTask(workingTask.ID)
	assert.True(t, exists, "working task should remain")
	_, exists = taskManager.GetTask(completedTask.ID)
	assert.False(t, exists, "completed task should be cleaned up")
	_, exists = taskManager.GetTask(failedTask.ID)
	assert.False(t, exists, "failed task should be cleaned up")
}

func TestDefaultTaskManager_ConcurrentAccess(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 20)

	numGoroutines := 10
	tasksChan := make(chan *types.Task, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			message := &types.Message{
				Kind:      "message",
				MessageID: "concurrent-msg",
				Role:      "user",
			}
			task := taskManager.CreateTask("concurrent-context", types.TaskStateSubmitted, message)
			tasksChan <- task
		}(i)
	}

	createdTasks := make([]*types.Task, 0, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		task := <-tasksChan
		createdTasks = append(createdTasks, task)
	}

	taskIDs := make(map[string]bool)
	for _, task := range createdTasks {
		assert.NotEmpty(t, task.ID)
		assert.False(t, taskIDs[task.ID], "Task ID should be unique: %s", task.ID)
		taskIDs[task.ID] = true
	}

	assert.Len(t, taskIDs, numGoroutines, "All tasks should have unique IDs")
}

func TestNewDefaultTaskManager(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 20)

	assert.NotNil(t, taskManager)
}

func TestNewDefaultTaskManager_WithNilLogger(t *testing.T) {
	assert.NotPanics(t, func() {
		taskManager := server.NewDefaultTaskManager(nil, 20)
		assert.NotNil(t, taskManager)
	})
}

func TestDefaultTaskManager_ConversationContextPreservation(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 20)

	contextID := "test-conversation-context"

	firstMessage := &types.Message{
		Kind:      "message",
		MessageID: "msg-1",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Hello, what's the weather like?",
			},
		},
	}

	task1 := taskManager.CreateTask(contextID, types.TaskStateSubmitted, firstMessage)
	assert.NotNil(t, task1)
	assert.Equal(t, contextID, task1.ContextID)
	assert.Len(t, task1.History, 1)
	assert.Equal(t, *firstMessage, task1.History[0])

	assistantResponse1 := &types.Message{
		Kind:      "message",
		MessageID: "msg-response-1",
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "It's sunny today with a temperature of 72°F.",
			},
		},
	}

	err := taskManager.UpdateTask(task1.ID, types.TaskStateCompleted, assistantResponse1)
	assert.NoError(t, err)

	updatedTask1, exists := taskManager.GetTask(task1.ID)
	assert.True(t, exists)
	assert.Len(t, updatedTask1.History, 2)
	assert.Equal(t, *firstMessage, updatedTask1.History[0])
	assert.Equal(t, *assistantResponse1, updatedTask1.History[1])

	secondMessage := &types.Message{
		Kind:      "message",
		MessageID: "msg-2",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "What about tomorrow?",
			},
		},
	}

	task2 := taskManager.CreateTask(contextID, types.TaskStateSubmitted, secondMessage)
	assert.NotNil(t, task2)
	assert.Equal(t, contextID, task2.ContextID)
	assert.NotEqual(t, task1.ID, task2.ID)

	assert.Len(t, task2.History, 3)
	assert.Equal(t, *firstMessage, task2.History[0])
	assert.Equal(t, *assistantResponse1, task2.History[1])
	assert.Equal(t, *secondMessage, task2.History[2])

	assistantResponse2 := &types.Message{
		Kind:      "message",
		MessageID: "msg-response-2",
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Tomorrow will be partly cloudy with a high of 68°F.",
			},
		},
	}

	err = taskManager.UpdateTask(task2.ID, types.TaskStateCompleted, assistantResponse2)
	assert.NoError(t, err)

	updatedTask2, exists := taskManager.GetTask(task2.ID)
	assert.True(t, exists)
	assert.Len(t, updatedTask2.History, 4)
	assert.Equal(t, *firstMessage, updatedTask2.History[0])
	assert.Equal(t, *assistantResponse1, updatedTask2.History[1])
	assert.Equal(t, *secondMessage, updatedTask2.History[2])
	assert.Equal(t, *assistantResponse2, updatedTask2.History[3])

	thirdMessage := &types.Message{
		Kind:      "message",
		MessageID: "msg-3",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Should I bring an umbrella?",
			},
		},
	}

	task3 := taskManager.CreateTask(contextID, types.TaskStateSubmitted, thirdMessage)
	assert.NotNil(t, task3)
	assert.Equal(t, contextID, task3.ContextID)

	assert.Len(t, task3.History, 5)
	assert.Equal(t, *firstMessage, task3.History[0])
	assert.Equal(t, *assistantResponse1, task3.History[1])
	assert.Equal(t, *secondMessage, task3.History[2])
	assert.Equal(t, *assistantResponse2, task3.History[3])
	assert.Equal(t, *thirdMessage, task3.History[4])
}

func TestDefaultTaskManager_ConversationHistoryIsolation(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 20)

	contextID1 := "context-1"
	contextID2 := "context-2"

	message1 := &types.Message{
		Kind:      "message",
		MessageID: "msg-1",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Message for context 1",
			},
		},
	}

	message2 := &types.Message{
		Kind:      "message",
		MessageID: "msg-2",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Message for context 2",
			},
		},
	}

	task1 := taskManager.CreateTask(contextID1, types.TaskStateSubmitted, message1)
	task2 := taskManager.CreateTask(contextID2, types.TaskStateSubmitted, message2)

	assert.Len(t, task1.History, 1)
	assert.Equal(t, *message1, task1.History[0])

	assert.Len(t, task2.History, 1)
	assert.Equal(t, *message2, task2.History[0])

	response1 := &types.Message{
		Kind:      "message",
		MessageID: "response-1",
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Response to context 1",
			},
		},
	}

	err := taskManager.UpdateTask(task1.ID, types.TaskStateCompleted, response1)
	assert.NoError(t, err)

	message3 := &types.Message{
		Kind:      "message",
		MessageID: "msg-3",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Follow-up for context 1",
			},
		},
	}

	task3 := taskManager.CreateTask(contextID1, types.TaskStateSubmitted, message3)

	assert.Len(t, task3.History, 3)
	assert.Equal(t, *message1, task3.History[0])
	assert.Equal(t, *response1, task3.History[1])
	assert.Equal(t, *message3, task3.History[2])

	message4 := &types.Message{
		Kind:      "message",
		MessageID: "msg-4",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Follow-up for context 2",
			},
		},
	}

	task4 := taskManager.CreateTask(contextID2, types.TaskStateSubmitted, message4)

	assert.Len(t, task4.History, 2)
	assert.Equal(t, *message2, task4.History[0])
	assert.Equal(t, *message4, task4.History[1])
}

func TestDefaultTaskManager_GetConversationHistory(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 20)

	contextID := "test-context"

	history := taskManager.GetConversationHistory(contextID)
	assert.Empty(t, history)

	message := &types.Message{
		Kind:      "message",
		MessageID: "msg-1",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Test message",
			},
		},
	}

	task := taskManager.CreateTask(contextID, types.TaskStateSubmitted, message)

	response := &types.Message{
		Kind:      "message",
		MessageID: "response-1",
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Test response",
			},
		},
	}

	err := taskManager.UpdateTask(task.ID, types.TaskStateCompleted, response)
	assert.NoError(t, err)

	history = taskManager.GetConversationHistory(contextID)
	assert.Len(t, history, 2)
	assert.Equal(t, *message, history[0])
	assert.Equal(t, *response, history[1])

	history[0].Parts = []types.Part{
		map[string]interface{}{
			"kind": "text",
			"text": "Modified message",
		},
	}

	freshHistory := taskManager.GetConversationHistory(contextID)
	assert.Len(t, freshHistory, 2)
	assert.Equal(t, "Test message", freshHistory[0].Parts[0].(map[string]interface{})["text"])
}

func TestDefaultTaskManager_UpdateConversationHistory(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 20)

	contextID := "test-context"

	messages := []types.Message{
		{
			Kind:      "message",
			MessageID: "msg-1",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
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
				map[string]interface{}{
					"kind": "text",
					"text": "First response",
				},
			},
		},
	}

	taskManager.UpdateConversationHistory(contextID, messages)

	history := taskManager.GetConversationHistory(contextID)
	assert.Len(t, history, 2)
	assert.Equal(t, messages[0], history[0])
	assert.Equal(t, messages[1], history[1])

	messages[0].Parts = []types.Part{
		map[string]interface{}{
			"kind": "text",
			"text": "Modified message",
		},
	}

	freshHistory := taskManager.GetConversationHistory(contextID)
	assert.Equal(t, "First message", freshHistory[0].Parts[0].(map[string]interface{})["text"])
}

func TestDefaultTaskManager_ConversationHistoryLimit(t *testing.T) {
	logger := zap.NewNop()
	maxHistory := 3
	taskManager := server.NewDefaultTaskManager(logger, maxHistory)

	contextID := "test-context"

	messages := []types.Message{
		{Kind: "message", MessageID: "msg-1", Role: "user", Parts: []types.Part{map[string]interface{}{"kind": "text", "text": "Message 1"}}},
		{Kind: "message", MessageID: "msg-2", Role: "assistant", Parts: []types.Part{map[string]interface{}{"kind": "text", "text": "Response 1"}}},
		{Kind: "message", MessageID: "msg-3", Role: "user", Parts: []types.Part{map[string]interface{}{"kind": "text", "text": "Message 2"}}},
		{Kind: "message", MessageID: "msg-4", Role: "assistant", Parts: []types.Part{map[string]interface{}{"kind": "text", "text": "Response 2"}}},
		{Kind: "message", MessageID: "msg-5", Role: "user", Parts: []types.Part{map[string]interface{}{"kind": "text", "text": "Message 3"}}},
	}

	taskManager.UpdateConversationHistory(contextID, messages)

	history := taskManager.GetConversationHistory(contextID)
	assert.Len(t, history, maxHistory)

	assert.Equal(t, "msg-3", history[0].MessageID)
	assert.Equal(t, "msg-4", history[1].MessageID)
	assert.Equal(t, "msg-5", history[2].MessageID)
}

func TestDefaultTaskManager_ConversationHistoryLimitViaCreateTask(t *testing.T) {
	logger := zap.NewNop()
	maxHistory := 2
	taskManager := server.NewDefaultTaskManager(logger, maxHistory)

	contextID := "test-context"

	message1 := &types.Message{
		Kind:      "message",
		MessageID: "msg-1",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "First message",
			},
		},
	}
	task1 := taskManager.CreateTask(contextID, types.TaskStateSubmitted, message1)

	response1 := &types.Message{
		Kind:      "message",
		MessageID: "response-1",
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "First response",
			},
		},
	}
	err := taskManager.UpdateTask(task1.ID, types.TaskStateCompleted, response1)
	assert.NoError(t, err)

	message2 := &types.Message{
		Kind:      "message",
		MessageID: "msg-2",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Second message",
			},
		},
	}
	_ = taskManager.CreateTask(contextID, types.TaskStateSubmitted, message2)

	message3 := &types.Message{
		Kind:      "message",
		MessageID: "msg-3",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Third message",
			},
		},
	}
	task3 := taskManager.CreateTask(contextID, types.TaskStateSubmitted, message3)

	assert.LessOrEqual(t, len(task3.History), maxHistory)

	assert.Equal(t, "msg-3", task3.History[len(task3.History)-1].MessageID)
}

func TestDefaultTaskManager_ConversationHistoryLimitZeroDirectInstantiation(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 0)

	contextID := "test-context"
	messages := make([]types.Message, 5)
	for i := 0; i < 5; i++ {
		messages[i] = types.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("msg-%d", i),
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": fmt.Sprintf("Message %d", i),
				},
			},
		}
	}

	taskManager.UpdateConversationHistory(contextID, messages)
	history := taskManager.GetConversationHistory(contextID)

	assert.Len(t, history, 0)
}

// TestDefaultTaskManager_CancelTask_StateValidation tests the new cancellation state validation
func TestDefaultTaskManager_CancelTask_StateValidation(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 10)

	tests := []struct {
		name          string
		initialState  types.TaskState
		shouldSucceed bool
		errorMsg      string
	}{
		{
			name:          "can cancel submitted task",
			initialState:  types.TaskStateSubmitted,
			shouldSucceed: true,
		},
		{
			name:          "can cancel working task",
			initialState:  types.TaskStateWorking,
			shouldSucceed: true,
		},
		{
			name:          "can cancel input-required task",
			initialState:  types.TaskStateInputRequired,
			shouldSucceed: true,
		},
		{
			name:          "can cancel auth-required task",
			initialState:  types.TaskStateAuthRequired,
			shouldSucceed: true,
		},
		{
			name:          "can cancel unknown task",
			initialState:  types.TaskStateUnknown,
			shouldSucceed: true,
		},
		{
			name:          "cannot cancel completed task",
			initialState:  types.TaskStateCompleted,
			shouldSucceed: false,
			errorMsg:      "cannot be canceled: current state is completed",
		},
		{
			name:          "cannot cancel failed task",
			initialState:  types.TaskStateFailed,
			shouldSucceed: false,
			errorMsg:      "cannot be canceled: current state is failed",
		},
		{
			name:          "cannot cancel already canceled task",
			initialState:  types.TaskStateCanceled,
			shouldSucceed: false,
			errorMsg:      "cannot be canceled: current state is canceled",
		},
		{
			name:          "cannot cancel rejected task",
			initialState:  types.TaskStateRejected,
			shouldSucceed: false,
			errorMsg:      "cannot be canceled: current state is rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a task in the specified initial state
			task := taskManager.CreateTask("test-context", tt.initialState, &types.Message{
				Kind:      "message",
				MessageID: "test-msg",
				Role:      "user",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Test message",
					},
				},
			})

			// Try to cancel the task
			err := taskManager.CancelTask(task.ID)

			if tt.shouldSucceed {
				assert.NoError(t, err)
				
				// Verify task was actually canceled
				retrievedTask, exists := taskManager.GetTask(task.ID)
				assert.True(t, exists)
				assert.Equal(t, types.TaskStateCanceled, retrievedTask.Status.State)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				
				// Verify task state didn't change
				retrievedTask, exists := taskManager.GetTask(task.ID)
				assert.True(t, exists)
				assert.Equal(t, tt.initialState, retrievedTask.Status.State)
			}
		})
	}
}

// TestDefaultTaskManager_PauseTaskForInput tests the task pausing functionality
func TestDefaultTaskManager_PauseTaskForInput(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 10)

	t.Run("pause existing task successfully", func(t *testing.T) {
		// Create a working task
		task := taskManager.CreateTask("test-context", types.TaskStateWorking, &types.Message{
			Kind:      "message",
			MessageID: "initial-msg",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Initial message",
				},
			},
		})

		// Pause the task
		pauseMessage := &types.Message{
			Kind:      "message",
			MessageID: "pause-msg",
			Role:      "assistant",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Please provide more information",
				},
			},
		}

		err := taskManager.PauseTaskForInput(task.ID, pauseMessage)
		assert.NoError(t, err)

		// Verify task state changed to input-required
		retrievedTask, exists := taskManager.GetTask(task.ID)
		assert.True(t, exists)
		assert.Equal(t, types.TaskStateInputRequired, retrievedTask.Status.State)
		assert.Equal(t, pauseMessage, retrievedTask.Status.Message)

		// Verify message was added to history
		assert.Len(t, retrievedTask.History, 2) // initial + pause message
		assert.Equal(t, pauseMessage.MessageID, retrievedTask.History[1].MessageID)
	})

	t.Run("pause with nil message", func(t *testing.T) {
		task := taskManager.CreateTask("test-context-2", types.TaskStateWorking, nil)

		err := taskManager.PauseTaskForInput(task.ID, nil)
		assert.NoError(t, err)

		retrievedTask, exists := taskManager.GetTask(task.ID)
		assert.True(t, exists)
		assert.Equal(t, types.TaskStateInputRequired, retrievedTask.Status.State)
		assert.Nil(t, retrievedTask.Status.Message)
	})

	t.Run("pause non-existent task", func(t *testing.T) {
		err := taskManager.PauseTaskForInput("non-existent-id", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task not found")
	})
}

// TestDefaultTaskManager_ResumeTaskWithInput tests the task resumption functionality
func TestDefaultTaskManager_ResumeTaskWithInput(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 10)

	t.Run("resume paused task successfully", func(t *testing.T) {
		// Create and pause a task
		task := taskManager.CreateTask("test-context", types.TaskStateWorking, nil)
		err := taskManager.PauseTaskForInput(task.ID, nil)
		assert.NoError(t, err)

		// Resume the task
		resumeMessage := &types.Message{
			Kind:      "message",
			MessageID: "resume-msg",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Here is the additional information",
				},
			},
		}

		err = taskManager.ResumeTaskWithInput(task.ID, resumeMessage)
		assert.NoError(t, err)

		// Verify task state changed to working
		retrievedTask, exists := taskManager.GetTask(task.ID)
		assert.True(t, exists)
		assert.Equal(t, types.TaskStateWorking, retrievedTask.Status.State)
		assert.Equal(t, resumeMessage, retrievedTask.Status.Message)

		// Verify message was added to history
		assert.Len(t, retrievedTask.History, 1) // resume message
		assert.Equal(t, resumeMessage.MessageID, retrievedTask.History[0].MessageID)
	})

	t.Run("resume task not in input-required state", func(t *testing.T) {
		task := taskManager.CreateTask("test-context-2", types.TaskStateWorking, nil)

		err := taskManager.ResumeTaskWithInput(task.ID, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in input-required state")
		assert.Contains(t, err.Error(), "current state: working")
	})

	t.Run("resume non-existent task", func(t *testing.T) {
		err := taskManager.ResumeTaskWithInput("non-existent-id", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task not found")
	})
}

// TestDefaultTaskManager_IsTaskPaused tests the task pause check functionality
func TestDefaultTaskManager_IsTaskPaused(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 10)

	t.Run("check paused task", func(t *testing.T) {
		task := taskManager.CreateTask("test-context", types.TaskStateWorking, nil)
		
		// Initially not paused
		isPaused, err := taskManager.IsTaskPaused(task.ID)
		assert.NoError(t, err)
		assert.False(t, isPaused)

		// Pause the task
		err = taskManager.PauseTaskForInput(task.ID, nil)
		assert.NoError(t, err)

		// Now should be paused
		isPaused, err = taskManager.IsTaskPaused(task.ID)
		assert.NoError(t, err)
		assert.True(t, isPaused)
	})

	t.Run("check non-existent task", func(t *testing.T) {
		isPaused, err := taskManager.IsTaskPaused("non-existent-id")
		assert.Error(t, err)
		assert.False(t, isPaused)
		assert.Contains(t, err.Error(), "task not found")
	})
}

// TestDefaultTaskManager_PollTaskStatus_InputRequired tests polling behavior with input-required state
func TestDefaultTaskManager_PollTaskStatus_InputRequired(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 10)

	t.Run("polling returns when task reaches input-required state", func(t *testing.T) {
		task := taskManager.CreateTask("test-context", types.TaskStateWorking, nil)

		// Start polling in a goroutine
		resultChan := make(chan *types.Task, 1)
		errorChan := make(chan error, 1)

		go func() {
			result, err := taskManager.PollTaskStatus(task.ID, 10*time.Millisecond, 1*time.Second)
			if err != nil {
				errorChan <- err
			} else {
				resultChan <- result
			}
		}()

		// Give polling a moment to start
		time.Sleep(50 * time.Millisecond)

		// Pause the task (set to input-required)
		err := taskManager.PauseTaskForInput(task.ID, nil)
		assert.NoError(t, err)

		// Polling should complete
		select {
		case result := <-resultChan:
			assert.Equal(t, types.TaskStateInputRequired, result.Status.State)
		case err := <-errorChan:
			t.Fatalf("Polling failed with error: %v", err)
		case <-time.After(2 * time.Second):
			t.Fatal("Polling did not complete within timeout")
		}
	})
}

// TestDefaultTaskManager_InputRequiredWorkflow tests the complete input-required workflow
func TestDefaultTaskManager_InputRequiredWorkflow(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger, 10)

	// Create initial task
	initialMessage := &types.Message{
		Kind:      "message",
		MessageID: "initial-msg",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Process this request",
			},
		},
	}
	task := taskManager.CreateTask("test-context", types.TaskStateWorking, initialMessage)

	// 1. Pause task for input
	pauseMessage := &types.Message{
		Kind:      "message",
		MessageID: "pause-msg",
		Role:      "assistant",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "I need more information. What is your preference?",
			},
		},
	}
	err := taskManager.PauseTaskForInput(task.ID, pauseMessage)
	assert.NoError(t, err)

	// 2. Verify task is paused
	isPaused, err := taskManager.IsTaskPaused(task.ID)
	assert.NoError(t, err)
	assert.True(t, isPaused)

	// 3. Try to cancel paused task (should succeed)
	cancelErr := taskManager.CancelTask(task.ID)
	assert.NoError(t, cancelErr)

	// 4. Create another task for resumption test
	task2 := taskManager.CreateTask("test-context-2", types.TaskStateWorking, initialMessage)
	err = taskManager.PauseTaskForInput(task2.ID, pauseMessage)
	assert.NoError(t, err)

	// 5. Resume with user input
	resumeMessage := &types.Message{
		Kind:      "message",
		MessageID: "resume-msg",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "I prefer option A",
			},
		},
	}
	err = taskManager.ResumeTaskWithInput(task2.ID, resumeMessage)
	assert.NoError(t, err)

	// 6. Verify task is no longer paused
	isPaused, err = taskManager.IsTaskPaused(task2.ID)
	assert.NoError(t, err)
	assert.False(t, isPaused)

	// 7. Verify complete conversation history
	retrievedTask, exists := taskManager.GetTask(task2.ID)
	assert.True(t, exists)
	assert.Equal(t, types.TaskStateWorking, retrievedTask.Status.State)
	assert.Len(t, retrievedTask.History, 3) // initial + pause + resume messages
	
	// Verify message order
	assert.Equal(t, "initial-msg", retrievedTask.History[0].MessageID)
	assert.Equal(t, "pause-msg", retrievedTask.History[1].MessageID)
	assert.Equal(t, "resume-msg", retrievedTask.History[2].MessageID)
}
