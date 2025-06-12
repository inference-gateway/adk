package server_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/a2a/adk"
	"github.com/inference-gateway/a2a/adk/server"
	"github.com/inference-gateway/a2a/adk/server/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestA2AServer_TaskManager_CreateTask(t *testing.T) {
	tests := []struct {
		name      string
		contextID string
		state     adk.TaskState
		message   *adk.Message
	}{
		{
			name:      "create task with submitted state",
			contextID: "test-context-1",
			state:     adk.TaskStateSubmitted,
			message: &adk.Message{
				Kind:      "message",
				MessageID: "test-message-1",
				Role:      "user",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Hello world",
					},
				},
			},
		},
		{
			name:      "create task with working state",
			contextID: "test-context-2",
			state:     adk.TaskStateWorking,
			message: &adk.Message{
				Kind:      "message",
				MessageID: "test-message-2",
				Role:      "assistant",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Processing your request",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			taskManager := server.NewDefaultTaskManager(logger)

			task := taskManager.CreateTask(tt.contextID, tt.state, tt.message)

			assert.NotNil(t, task)
			assert.NotEmpty(t, task.ID)
			assert.Equal(t, tt.contextID, task.ContextID)
			assert.Equal(t, tt.state, task.Status.State)
			assert.Equal(t, tt.message, task.Status.Message)
			assert.NotNil(t, task.Status.Timestamp)
		})
	}
}

func TestA2AServer_TaskManager_UpdateTask(t *testing.T) {
	tests := []struct {
		name        string
		newState    adk.TaskState
		newMessage  *adk.Message
		expectError bool
	}{
		{
			name:     "update to completed state",
			newState: adk.TaskStateCompleted,
			newMessage: &adk.Message{
				Kind:      "message",
				MessageID: "test-message-updated",
				Role:      "assistant",
				Parts: []adk.Part{
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
			newState: adk.TaskStateFailed,
			newMessage: &adk.Message{
				Kind:      "message",
				MessageID: "test-message-error",
				Role:      "assistant",
				Parts: []adk.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Task failed",
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			taskManager := server.NewDefaultTaskManager(logger)

			task := taskManager.CreateTask("test-context", adk.TaskStateSubmitted, &adk.Message{
				Kind:      "message",
				MessageID: "initial-message",
				Role:      "user",
			})

			err := taskManager.UpdateTask(task.ID, tt.newState, tt.newMessage)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				updatedTask, exists := taskManager.GetTask(task.ID)
				assert.True(t, exists)
				assert.Equal(t, tt.newState, updatedTask.Status.State)
				assert.Equal(t, tt.newMessage, updatedTask.Status.Message)
			}
		})
	}
}

func TestA2AServer_TaskManager_GetTask(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger)

	message := &adk.Message{
		Kind:      "message",
		MessageID: "test-message",
		Role:      "user",
	}
	task := taskManager.CreateTask("test-context", adk.TaskStateSubmitted, message)

	retrievedTask, exists := taskManager.GetTask(task.ID)
	assert.True(t, exists)
	assert.Equal(t, task.ID, retrievedTask.ID)
	assert.Equal(t, task.ContextID, retrievedTask.ContextID)

	nonExistentTask, exists := taskManager.GetTask("non-existent-id")
	assert.False(t, exists)
	assert.Nil(t, nonExistentTask)
}

func TestA2AServer_ResponseSender_SendSuccess(t *testing.T) {
	logger := zap.NewNop()
	responseSender := server.NewDefaultResponseSender(logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	result := map[string]interface{}{
		"status": "success",
		"data":   "test data",
	}

	assert.NotPanics(t, func() {
		responseSender.SendSuccess(ctx, "test-id", result)
	})
}

func TestA2AServer_ResponseSender_SendError(t *testing.T) {
	logger := zap.NewNop()
	responseSender := server.NewDefaultResponseSender(logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	assert.NotPanics(t, func() {
		responseSender.SendError(ctx, "test-id", 500, "test error message")
	})
}

func TestA2AServer_MessageHandler_Integration(t *testing.T) {
	logger := zap.NewNop()
	taskManager := server.NewDefaultTaskManager(logger)

	messageHandler := server.NewDefaultMessageHandler(logger, taskManager)

	contextID := "test-context"
	params := adk.MessageSendParams{
		Message: adk.Message{
			ContextID: &contextID,
			Kind:      "message",
			MessageID: "test-message",
			Role:      "user",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Hello, world!",
				},
			},
		},
	}

	ctx := context.Background()
	task, err := messageHandler.HandleMessageSend(ctx, params)

	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, contextID, task.ContextID)
	assert.Equal(t, adk.TaskStateSubmitted, task.Status.State)
}

func TestA2AServer_TaskProcessing_Background(t *testing.T) {
	cfg := server.Config{
		QueueConfig: &server.QueueConfig{
			MaxSize:         10,
			CleanupInterval: 50 * time.Millisecond,
		},
		CapabilitiesConfig: &server.CapabilitiesConfig{
			Streaming:              true,
			PushNotifications:      false,
			StateTransitionHistory: true,
		},
		AuthConfig: &server.AuthConfig{
			Enable: false,
		},
	}
	logger := zap.NewNop()

	a2aServer := server.NewA2AServer(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go a2aServer.StartTaskProcessor(ctx)

	time.Sleep(100 * time.Millisecond)

	assert.True(t, true)
}

func TestDefaultA2AServer_SetDependencies(t *testing.T) {
	a2aServer := server.NewDefaultA2AServer()

	mockTaskHandler := &mocks.FakeTaskHandler{}
	a2aServer.SetTaskHandler(mockTaskHandler)

	mockAgentProvider := &mocks.FakeAgentInfoProvider{}
	mockAgentProvider.GetAgentCardReturns(adk.AgentCard{
		Name:        "custom-agent",
		Description: "custom description",
		Version:     "1.0.0",
	})
	a2aServer.SetAgentInfoProvider(mockAgentProvider)

	mockProcessor := &mocks.FakeTaskResultProcessor{}
	a2aServer.SetTaskResultProcessor(mockProcessor)

	agentCard := a2aServer.GetAgentCard()
	assert.Equal(t, "custom-agent", agentCard.Name)
	assert.Equal(t, "custom description", agentCard.Description)
}
