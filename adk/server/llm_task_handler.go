package server

import (
	"context"
	"fmt"

	adk "github.com/inference-gateway/a2a/adk"
	config "github.com/inference-gateway/a2a/adk/server/config"
	zap "go.uber.org/zap"
)

// LLMTaskHandler implements the TaskHandler interface using an LLM client
type LLMTaskHandler struct {
	logger       *zap.Logger
	llmClient    LLMClient
	systemPrompt string
}

// NewLLMTaskHandler creates a new LLM-powered task handler
func NewLLMTaskHandler(logger *zap.Logger, llmClient LLMClient) *LLMTaskHandler {
	return &LLMTaskHandler{
		logger:       logger,
		llmClient:    llmClient,
		systemPrompt: "You are a helpful AI assistant processing an A2A (Agent-to-Agent) task. Please provide helpful and accurate responses.",
	}
}

// NewLLMTaskHandlerWithConfig creates a new LLM-powered task handler with configuration
func NewLLMTaskHandlerWithConfig(logger *zap.Logger, llmClient LLMClient, config *config.LLMProviderClientConfig) *LLMTaskHandler {
	systemPrompt := "You are a helpful AI assistant processing an A2A (Agent-to-Agent) task. Please provide helpful and accurate responses."
	if config != nil && config.SystemPrompt != "" {
		systemPrompt = config.SystemPrompt
	}

	return &LLMTaskHandler{
		logger:       logger,
		llmClient:    llmClient,
		systemPrompt: systemPrompt,
	}
}

// HandleTask processes a task using the LLM client
func (th *LLMTaskHandler) HandleTask(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error) {
	var messageKind string
	if message != nil {
		messageKind = message.Kind
	} else {
		messageKind = "nil"
	}

	th.logger.Info("processing task with llm handler",
		zap.String("task_id", task.ID),
		zap.String("message_kind", messageKind))

	if th.llmClient == nil {
		th.logger.Error("llm client not configured")
		return th.handleError(task, "LLM client not configured")
	}

	messages := th.prepareMessages(task, message)

	response, err := th.llmClient.CreateChatCompletion(ctx, messages)
	if err != nil {
		th.logger.Error("failed to process with llm", zap.Error(err))
		return th.handleError(task, fmt.Sprintf("LLM processing failed: %v", err))
	}

	return th.updateTaskWithResponse(task, message, response), nil
}

// HandleTaskStream processes a task using streaming LLM responses
func (th *LLMTaskHandler) HandleTaskStream(ctx context.Context, task *adk.Task, message *adk.Message) (<-chan *adk.Task, <-chan error) {
	taskChan := make(chan *adk.Task)
	errorChan := make(chan error, 1)

	go func() {
		defer close(taskChan)
		defer close(errorChan)

		th.logger.Info("processing task with streaming llm handler",
			zap.String("task_id", task.ID))

		if th.llmClient == nil {
			errorChan <- fmt.Errorf("llm client not configured")
			return
		}

		messages := th.prepareMessages(task, message)

		messageChan, errChan := th.llmClient.CreateStreamingChatCompletion(ctx, messages)

		var fullResponse string
		for {
			select {
			case streamMessage, ok := <-messageChan:
				if !ok {
					finalTask := th.updateTaskWithText(task, message, fullResponse)
					finalTask.Status.State = adk.TaskStateCompleted
					taskChan <- finalTask
					return
				}

				text := th.extractTextFromMessage(streamMessage)
				fullResponse += text

				intermediateTask := th.updateTaskWithText(task, message, fullResponse)
				intermediateTask.Status.State = adk.TaskStateWorking
				taskChan <- intermediateTask

			case err := <-errChan:
				if err != nil {
					th.logger.Error("streaming llm processing failed", zap.Error(err))
					errorChan <- err
					return
				}

			case <-ctx.Done():
				errorChan <- ctx.Err()
				return
			}
		}
	}()

	return taskChan, errorChan
}

// prepareMessages converts A2A task context to LLM messages
func (th *LLMTaskHandler) prepareMessages(task *adk.Task, message *adk.Message) []adk.Message {
	var messages []adk.Message

	systemMessage := &adk.Message{
		Kind:      "message",
		MessageID: "system-" + task.ID,
		Role:      "system",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": th.buildSystemPrompt(task),
			},
		},
	}
	messages = append(messages, *systemMessage)

	if task.History != nil {
		for _, historyMessage := range task.History {
			if historyMessage.Role == "user" || historyMessage.Role == "assistant" {
				messages = append(messages, historyMessage)
			}
		}
	}

	if message != nil {
		messages = append(messages, *message)
	}

	return messages
}

// buildSystemPrompt creates a system prompt based on task context
func (th *LLMTaskHandler) buildSystemPrompt(task *adk.Task) string {
	prompt := th.systemPrompt

	// Add task-specific context if available in metadata
	if task.Metadata != nil {
		if description, exists := task.Metadata["description"]; exists {
			if descStr, ok := description.(string); ok && descStr != "" {
				prompt += fmt.Sprintf("\n\nTask Description: %s", descStr)
			}
		}

		if _, exists := task.Metadata["context"]; exists {
			prompt += "\n\nTask Context: Please consider the provided context when responding."
		}
	}

	prompt += "\n\nPlease provide a helpful and accurate response to the user's message."

	return prompt
}

// updateTaskWithResponse updates the task with the LLM response
func (th *LLMTaskHandler) updateTaskWithResponse(task *adk.Task, userMessage *adk.Message, response *adk.Message) *adk.Task {
	if task.History == nil {
		task.History = []adk.Message{}
	}

	if userMessage != nil {
		task.History = append(task.History, *userMessage)
	}

	task.History = append(task.History, *response)

	task.Status.State = adk.TaskStateCompleted

	th.logger.Info("task processed with llm response",
		zap.String("task_id", task.ID),
		zap.Int("history_length", len(task.History)))

	return task
}

// updateTaskWithText updates the task with text content
func (th *LLMTaskHandler) updateTaskWithText(task *adk.Task, userMessage *adk.Message, text string) *adk.Task {
	response := &adk.Message{
		Kind:      "message",
		MessageID: "llm-response-" + task.ID,
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": text,
			},
		},
	}

	return th.updateTaskWithResponse(task, userMessage, response)
}

// extractTextFromMessage extracts text content from a message
func (th *LLMTaskHandler) extractTextFromMessage(message *adk.Message) string {
	var text string
	for _, part := range message.Parts {
		if partMap, ok := part.(map[string]interface{}); ok {
			if textContent, exists := partMap["text"]; exists {
				if textStr, ok := textContent.(string); ok {
					text += textStr
				}
			}
		}
	}
	return text
}

// handleError creates an error response task
func (th *LLMTaskHandler) handleError(task *adk.Task, errorMsg string) (*adk.Task, error) {
	task.Status.State = adk.TaskStateFailed

	errorMessage := &adk.Message{
		Kind:      "message",
		MessageID: "error-" + task.ID,
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": fmt.Sprintf("Error: %s", errorMsg),
			},
		},
	}

	if task.History == nil {
		task.History = []adk.Message{}
	}
	task.History = append(task.History, *errorMessage)

	return task, fmt.Errorf("%s", errorMsg)
}

// SetSystemPrompt allows customizing the system prompt
func (th *LLMTaskHandler) SetSystemPrompt(prompt string) {
	th.systemPrompt = prompt
}

// GetSystemPrompt returns the current system prompt
func (th *LLMTaskHandler) GetSystemPrompt() string {
	return th.systemPrompt
}
