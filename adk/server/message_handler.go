package server

import (
	"context"

	uuid "github.com/google/uuid"
	adk "github.com/inference-gateway/a2a/adk"
	zap "go.uber.org/zap"
)

// MessageHandler defines how to handle different types of A2A messages
type MessageHandler interface {
	// HandleMessageSend processes message/send requests
	HandleMessageSend(ctx context.Context, params adk.MessageSendParams) (*adk.Task, error)

	// HandleMessageStream processes message/stream requests (for streaming responses)
	HandleMessageStream(ctx context.Context, params adk.MessageSendParams) error
}

// DefaultMessageHandler implements the MessageHandler interface
type DefaultMessageHandler struct {
	logger      *zap.Logger
	taskManager TaskManager
}

// NewDefaultMessageHandler creates a new default message handler
func NewDefaultMessageHandler(logger *zap.Logger, taskManager TaskManager) *DefaultMessageHandler {
	return &DefaultMessageHandler{
		logger:      logger,
		taskManager: taskManager,
	}
}

// HandleMessageSend processes message/send requests
func (mh *DefaultMessageHandler) HandleMessageSend(ctx context.Context, params adk.MessageSendParams) (*adk.Task, error) {
	if len(params.Message.Parts) == 0 {
		return nil, NewEmptyMessagePartsError()
	}

	contextID := params.Message.ContextID
	if contextID == nil {
		newContextID := uuid.New().String()
		contextID = &newContextID
	}

	task := mh.taskManager.CreateTask(*contextID, adk.TaskStateSubmitted, &params.Message)

	mh.logger.Info("message send handled", zap.String("task_id", task.ID))
	return task, nil
}

// HandleMessageStream processes message/stream requests (for streaming responses)
func (mh *DefaultMessageHandler) HandleMessageStream(ctx context.Context, params adk.MessageSendParams) error {
	// TODO: Implement streaming support
	return NewStreamingNotImplementedError()
}
