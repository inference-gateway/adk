package server

import (
	"testing"

	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
	zap "go.uber.org/zap"
)

func TestExtensibleStorage(t *testing.T) {
	logger := zap.NewNop()
	storage := NewInMemoryStorage(logger, 100)

	contextID1 := "context-1"
	contextID2 := "context-2"

	task1 := &types.Task{
		ID:        "task-1",
		ContextID: contextID1,
		Kind:      "task",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		History: []types.Message{
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
		},
	}

	task2 := &types.Task{
		ID:        "task-2",
		ContextID: contextID1,
		Kind:      "task",
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
		},
		History: []types.Message{
			{
				Kind:      "message",
				MessageID: "msg-2",
				Role:      "user",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Goodbye",
					},
				},
			},
		},
	}

	task3 := &types.Task{
		ID:        "task-3",
		ContextID: contextID2,
		Kind:      "task",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		History: []types.Message{},
	}

	t.Run("Store and retrieve tasks", func(t *testing.T) {
		err := storage.StoreTask(task1)
		if err != nil {
			t.Fatalf("Failed to store task1: %v", err)
		}

		err = storage.StoreTask(task2)
		if err != nil {
			t.Fatalf("Failed to store task2: %v", err)
		}

		err = storage.StoreTask(task3)
		if err != nil {
			t.Fatalf("Failed to store task3: %v", err)
		}

		retrievedTask, found := storage.GetTask("task-1")
		if !found {
			t.Fatal("Task-1 not found")
		}
		if retrievedTask.ID != "task-1" {
			t.Errorf("Expected task ID 'task-1', got '%s'", retrievedTask.ID)
		}

		retrievedTask, found = storage.GetTaskByContextAndID(contextID1, "task-1")
		if !found {
			t.Fatal("Task-1 not found by context and ID")
		}
		if retrievedTask.ContextID != contextID1 {
			t.Errorf("Expected context ID '%s', got '%s'", contextID1, retrievedTask.ContextID)
		}

		_, found = storage.GetTaskByContextAndID(contextID2, "task-1")
		if found {
			t.Error("Task-1 should not be found in context-2")
		}
	})

	t.Run("List tasks by context", func(t *testing.T) {
		tasks, err := storage.ListTasksByContext(contextID1, TaskFilter{})
		if err != nil {
			t.Fatalf("Failed to list tasks for context1: %v", err)
		}
		if len(tasks) != 2 {
			t.Errorf("Expected 2 tasks for context1, got %d", len(tasks))
		}

		workingState := types.TaskStateWorking
		tasks, err = storage.ListTasksByContext(contextID1, TaskFilter{
			State: &workingState,
		})
		if err != nil {
			t.Fatalf("Failed to list working tasks for context1: %v", err)
		}
		if len(tasks) != 1 {
			t.Errorf("Expected 1 working task for context1, got %d", len(tasks))
		}

		tasks, err = storage.ListTasksByContext(contextID2, TaskFilter{})
		if err != nil {
			t.Fatalf("Failed to list tasks for context2: %v", err)
		}
		if len(tasks) != 1 {
			t.Errorf("Expected 1 task for context2, got %d", len(tasks))
		}
	})

	t.Run("Context management", func(t *testing.T) {
		contexts := storage.GetContextsWithTasks()
		if len(contexts) != 2 {
			t.Errorf("Expected 2 contexts with tasks, got %d", len(contexts))
		}

		messages := []types.Message{
			{
				Kind:      "message",
				MessageID: "conv-msg-1",
				Role:      "user",
				Parts: []types.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Hello from conversation",
					},
				},
			},
		}
		storage.UpdateConversationHistory(contextID1, messages)

		allContexts := storage.GetContexts()
		found := false
		for _, ctx := range allContexts {
			if ctx == contextID1 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Context1 should be in the list of contexts with conversation history")
		}
	})

	t.Run("Storage statistics", func(t *testing.T) {
		stats := storage.GetStats()

		if stats.TotalTasks != 3 {
			t.Errorf("Expected 3 total tasks, got %d", stats.TotalTasks)
		}

		if stats.ContextsWithTasks != 2 {
			t.Errorf("Expected 2 contexts with tasks, got %d", stats.ContextsWithTasks)
		}

		if stats.TasksByState[string(types.TaskStateWorking)] != 2 {
			t.Errorf("Expected 2 working tasks, got %d", stats.TasksByState[string(types.TaskStateWorking)])
		}

		if stats.TasksByState[string(types.TaskStateCompleted)] != 1 {
			t.Errorf("Expected 1 completed task, got %d", stats.TasksByState[string(types.TaskStateCompleted)])
		}
	})

	t.Run("Delete context and tasks", func(t *testing.T) {
		err := storage.DeleteContextAndTasks(contextID2)
		if err != nil {
			t.Fatalf("Failed to delete context2 and tasks: %v", err)
		}

		_, found := storage.GetTask("task-3")
		if found {
			t.Error("Task-3 should have been deleted")
		}

		contexts := storage.GetContextsWithTasks()
		for _, ctx := range contexts {
			if ctx == contextID2 {
				t.Error("Context2 should not be in contexts with tasks")
			}
		}

		_, found = storage.GetTask("task-1")
		if !found {
			t.Error("Task-1 should still exist")
		}

		_, found = storage.GetTask("task-2")
		if !found {
			t.Error("Task-2 should still exist")
		}
	})
}

func TestStorageExtensibility(t *testing.T) {
	logger := zap.NewNop()

	t.Run("Storage interface compatibility", func(t *testing.T) {
		storage := NewInMemoryStorage(logger, 100)
		assert.NotNil(t, storage, "InMemoryStorage should implement Storage interface")

		_, err := NewDatabaseStorage(logger, "postgres://localhost:5432/test")
		if err == nil {
			t.Log("DatabaseStorage constructor works")
		}

		_, err = NewRedisStorage(logger, "redis://localhost:6379")
		if err == nil {
			t.Log("RedisStorage constructor works")
		}
	})

	t.Run("TaskFilter functionality", func(t *testing.T) {
		storage := NewInMemoryStorage(logger, 100)

		contextID := "test-context"
		workingState := types.TaskStateWorking
		completedState := types.TaskStateCompleted

		tasks := []*types.Task{
			{
				ID:        "task-w1",
				ContextID: contextID,
				Kind:      "task",
				Status:    types.TaskStatus{State: types.TaskStateWorking},
			},
			{
				ID:        "task-w2",
				ContextID: contextID,
				Kind:      "task",
				Status:    types.TaskStatus{State: types.TaskStateWorking},
			},
			{
				ID:        "task-c1",
				ContextID: contextID,
				Kind:      "task",
				Status:    types.TaskStatus{State: types.TaskStateCompleted},
			},
		}

		for _, task := range tasks {
			err := storage.StoreTask(task)
			assert.NoError(t, err)
		}

		workingTasks, err := storage.ListTasks(TaskFilter{
			State: &workingState,
		})
		if err != nil {
			t.Fatalf("Failed to list working tasks: %v", err)
		}
		if len(workingTasks) != 2 {
			t.Errorf("Expected 2 working tasks, got %d", len(workingTasks))
		}

		completedTasks, err := storage.ListTasks(TaskFilter{
			State: &completedState,
		})
		if err != nil {
			t.Fatalf("Failed to list completed tasks: %v", err)
		}
		if len(completedTasks) != 1 {
			t.Errorf("Expected 1 completed task, got %d", len(completedTasks))
		}

		contextTasks, err := storage.ListTasks(TaskFilter{
			ContextID: &contextID,
		})
		if err != nil {
			t.Fatalf("Failed to list tasks by context: %v", err)
		}
		if len(contextTasks) != 3 {
			t.Errorf("Expected 3 tasks for context, got %d", len(contextTasks))
		}

		paginatedTasks, err := storage.ListTasks(TaskFilter{
			Limit:  2,
			Offset: 0,
		})
		if err != nil {
			t.Fatalf("Failed to list paginated tasks: %v", err)
		}
		if len(paginatedTasks) != 2 {
			t.Errorf("Expected 2 tasks with limit=2, got %d", len(paginatedTasks))
		}
	})
}
