package server_test

import (
	"fmt"
	"testing"

	server "github.com/inference-gateway/adk/server"
	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	zap "go.uber.org/zap"
)

func TestInMemoryStorage_ConversationHistory(t *testing.T) {
	logger := zap.NewNop()
	storage := server.NewInMemoryStorage(logger, 10)

	contextID := "test-context"

	t.Run("empty conversation history", func(t *testing.T) {
		history := storage.GetConversationHistory(contextID)
		assert.Empty(t, history)
	})

	t.Run("add single message", func(t *testing.T) {
		message := types.Message{
			Kind:      "message",
			MessageID: "msg-1",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hello",
				},
			},
		}

		err := storage.AddMessageToConversation(contextID, message)
		require.NoError(t, err)

		history := storage.GetConversationHistory(contextID)
		assert.Len(t, history, 1)
		assert.Equal(t, message.MessageID, history[0].MessageID)
		assert.Equal(t, message.Role, history[0].Role)
	})

	t.Run("prevent duplicate messages", func(t *testing.T) {
		message := types.Message{
			Kind:      "message",
			MessageID: "msg-1",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hello again",
				},
			},
		}

		err := storage.AddMessageToConversation(contextID, message)
		require.NoError(t, err)

		history := storage.GetConversationHistory(contextID)
		assert.Len(t, history, 1, "Should still have only 1 message, duplicate should be prevented")
		assert.Equal(t, "Hello", history[0].Parts[0].(map[string]interface{})["text"], "Original message should be preserved")
	})

	t.Run("add different messages", func(t *testing.T) {
		message2 := types.Message{
			Kind:      "message",
			MessageID: "msg-2",
			Role:      "assistant",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hi there!",
				},
			},
		}

		err := storage.AddMessageToConversation(contextID, message2)
		require.NoError(t, err)

		history := storage.GetConversationHistory(contextID)
		assert.Len(t, history, 2)
		assert.Equal(t, "msg-1", history[0].MessageID)
		assert.Equal(t, "msg-2", history[1].MessageID)
	})
}

func TestInMemoryStorage_ConversationHistoryTrimming(t *testing.T) {
	logger := zap.NewNop()
	maxHistory := 3
	storage := server.NewInMemoryStorage(logger, maxHistory)

	contextID := "test-context"

	for i := 1; i <= 5; i++ {
		message := types.Message{
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

		err := storage.AddMessageToConversation(contextID, message)
		require.NoError(t, err)
	}

	history := storage.GetConversationHistory(contextID)
	assert.Len(t, history, maxHistory, "History should be trimmed to max length")

	assert.Equal(t, "msg-3", history[0].MessageID)
	assert.Equal(t, "msg-4", history[1].MessageID)
	assert.Equal(t, "msg-5", history[2].MessageID)
}

func TestInMemoryStorage_UpdateConversationHistory(t *testing.T) {
	logger := zap.NewNop()
	storage := server.NewInMemoryStorage(logger, 10)

	contextID := "test-context"

	messages := []types.Message{
		{
			Kind:      "message",
			MessageID: "msg-1",
			Role:      "user",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hello",
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
					"text": "Hi there!",
				},
			},
		},
	}

	storage.UpdateConversationHistory(contextID, messages)

	history := storage.GetConversationHistory(contextID)
	assert.Len(t, history, 2)
	assert.Equal(t, "msg-1", history[0].MessageID)
	assert.Equal(t, "msg-2", history[1].MessageID)
}

func TestInMemoryStorage_TaskManagement(t *testing.T) {
	logger := zap.NewNop()
	storage := server.NewInMemoryStorage(logger, 10)

	task := &types.Task{
		ID:   "task-1",
		Kind: "task",
		Status: types.TaskStatus{
			State: types.TaskStateSubmitted,
		},
		ContextID: "context-1",
		History:   []types.Message{},
	}

	t.Run("store and retrieve task", func(t *testing.T) {
		err := storage.StoreTask(task)
		require.NoError(t, err)

		retrievedTask, exists := storage.GetTask("task-1")
		assert.True(t, exists)
		assert.Equal(t, task.ID, retrievedTask.ID)
		assert.Equal(t, task.Status.State, retrievedTask.Status.State)
	})

	t.Run("update task", func(t *testing.T) {
		task.Status.State = types.TaskStateCompleted
		err := storage.UpdateTask(task)
		require.NoError(t, err)

		retrievedTask, exists := storage.GetTask("task-1")
		assert.True(t, exists)
		assert.Equal(t, types.TaskStateCompleted, retrievedTask.Status.State)
	})

	t.Run("list tasks", func(t *testing.T) {
		filter := server.TaskFilter{
			Limit:  10,
			Offset: 0,
		}

		tasks, err := storage.ListTasks(filter)
		require.NoError(t, err)
		assert.Len(t, tasks, 1)
		assert.Equal(t, "task-1", tasks[0].ID)
	})

	t.Run("delete task", func(t *testing.T) {
		err := storage.DeleteTask("task-1")
		require.NoError(t, err)

		_, exists := storage.GetTask("task-1")
		assert.False(t, exists)
	})
}

func TestInMemoryStorage_ContextManagement(t *testing.T) {
	logger := zap.NewNop()
	storage := server.NewInMemoryStorage(logger, 10)

	contextID1 := "context-1"
	contextID2 := "context-2"

	message1 := types.Message{
		Kind:      "message",
		MessageID: "msg-1",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Hello from context 1",
			},
		},
	}

	message2 := types.Message{
		Kind:      "message",
		MessageID: "msg-2",
		Role:      "user",
		Parts: []types.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "Hello from context 2",
			},
		},
	}

	err := storage.AddMessageToConversation(contextID1, message1)
	require.NoError(t, err)

	err = storage.AddMessageToConversation(contextID2, message2)
	require.NoError(t, err)

	t.Run("get contexts", func(t *testing.T) {
		contexts := storage.GetContexts()
		assert.Len(t, contexts, 2)
		assert.Contains(t, contexts, contextID1)
		assert.Contains(t, contexts, contextID2)
	})

	t.Run("delete context", func(t *testing.T) {
		err := storage.DeleteContext(contextID1)
		require.NoError(t, err)

		history := storage.GetConversationHistory(contextID1)
		assert.Empty(t, history)

		history = storage.GetConversationHistory(contextID2)
		assert.Len(t, history, 1)
	})
}

func TestInMemoryStorage_CleanupOperations(t *testing.T) {
	logger := zap.NewNop()
	storage := server.NewInMemoryStorage(logger, 10)

	completedTask := &types.Task{
		ID:   "completed-task",
		Kind: "task",
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
		},
		ContextID: "context-1",
	}

	workingTask := &types.Task{
		ID:   "working-task",
		Kind: "task",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		ContextID: "context-2",
	}

	failedTask := &types.Task{
		ID:   "failed-task",
		Kind: "task",
		Status: types.TaskStatus{
			State: types.TaskStateFailed,
		},
		ContextID: "context-3",
	}

	err := storage.StoreTask(completedTask)
	require.NoError(t, err)

	err = storage.StoreTask(workingTask)
	require.NoError(t, err)

	err = storage.StoreTask(failedTask)
	require.NoError(t, err)

	t.Run("cleanup completed tasks", func(t *testing.T) {
		cleanedCount := storage.CleanupCompletedTasks()
		assert.Equal(t, 2, cleanedCount)

		_, exists := storage.GetTask("working-task")
		assert.True(t, exists)

		_, exists = storage.GetTask("completed-task")
		assert.False(t, exists)

		_, exists = storage.GetTask("failed-task")
		assert.False(t, exists)
	})
}
