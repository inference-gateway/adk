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
	logger *zap.Logger
}

// NewDefaultMessageHandler creates a new default message handler
func NewDefaultMessageHandler(logger *zap.Logger) *DefaultMessageHandler {
	return &DefaultMessageHandler{
		logger: logger,
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

	task := &adk.Task{
		ID: uuid.New().String(),
		Status: adk.TaskStatus{
			State: adk.TaskStateSubmitted,
		},
		ContextID: *contextID,
	}

	mh.logger.Info("message send handled", zap.String("task_id", task.ID))
	return task, nil
}

// HandleMessageStream processes message/stream requests (for streaming responses)
func (mh *DefaultMessageHandler) HandleMessageStream(ctx context.Context, params adk.MessageSendParams) error {
	// TODO: Implement streaming support
	return NewStreamingNotImplementedError()
}
