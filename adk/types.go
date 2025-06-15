package adk

import (
	"fmt"
	"sync"
	"time"

	"github.com/inference-gateway/sdk"
	"go.uber.org/zap"
)

// MessagePartKind represents the different types of message parts supported by A2A protocol.
// Based on the A2A specification: https://google-a2a.github.io/A2A/latest/
type MessagePartKind string

// MessagePartKind enum values for the three official message part types
const (
	// MessagePartKindText represents a text segment within message parts
	MessagePartKindText MessagePartKind = "text"

	// MessagePartKindFile represents a file segment within message parts
	MessagePartKindFile MessagePartKind = "file"

	// MessagePartKindData represents a structured data segment within message parts
	MessagePartKindData MessagePartKind = "data"
)

// String returns the string representation of the MessagePartKind
func (k MessagePartKind) String() string {
	return string(k)
}

// IsValid checks if the MessagePartKind is one of the supported values
func (k MessagePartKind) IsValid() bool {
	switch k {
	case MessagePartKindText, MessagePartKindFile, MessagePartKindData:
		return true
	default:
		return false
	}
}

// A2AConversationManager follows the official A2A specification for managing
// conversation context and task history efficiently
type A2AConversationManager struct {
	// Task-specific history storage (task.history field in A2A spec)
	taskHistories map[string]*TaskHistory

	// Context-based message storage for cross-task context
	contextMessages map[string]*ContextualMessages

	// Pre-converted LLM message cache to minimize conversions
	llmMessageCache map[string]*LLMMessageCache

	mu     sync.RWMutex
	logger *zap.Logger
}

// TaskHistory represents the history field of an A2A Task
type TaskHistory struct {
	TaskID      string
	ContextID   string
	Messages    []Message // Task.history as per A2A spec
	MaxLength   int       // historyLength parameter from tasks/get
	LastUpdated time.Time
}

// ContextualMessages stores messages by contextId for cross-task context
type ContextualMessages struct {
	ContextID   string
	Messages    []Message // All messages in this context across tasks
	TaskIDs     []string  // Tasks that belong to this context
	LastUpdated time.Time
}

// LLMMessageCache stores pre-converted messages for efficient LLM calls
type LLMMessageCache struct {
	// Original A2A messages (source of truth)
	A2AMessages []Message

	// Pre-converted LLM format (cached for performance)
	LLMMessages []sdk.Message

	// Cache validity tracking
	LastConversion time.Time
	IsDirty        bool
	ContextID      string
	TaskID         string
}

// NewA2AConversationManager creates a new A2A-compliant conversation manager
func NewA2AConversationManager(logger *zap.Logger) *A2AConversationManager {
	return &A2AConversationManager{
		taskHistories:   make(map[string]*TaskHistory),
		contextMessages: make(map[string]*ContextualMessages),
		llmMessageCache: make(map[string]*LLMMessageCache),
		logger:          logger,
	}
}

// GetTaskHistory retrieves task history following A2A Task.history specification
func (cm *A2AConversationManager) GetTaskHistory(taskID string, historyLength int) (*TaskHistory, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	history, exists := cm.taskHistories[taskID]
	if !exists {
		return nil, false
	}

	// Apply historyLength limit as per A2A tasks/get specification
	if historyLength > 0 && len(history.Messages) > historyLength {
		limitedHistory := &TaskHistory{
			TaskID:      history.TaskID,
			ContextID:   history.ContextID,
			Messages:    history.Messages[len(history.Messages)-historyLength:],
			MaxLength:   history.MaxLength,
			LastUpdated: history.LastUpdated,
		}
		return limitedHistory, true
	}

	return history, true
}

// AddTaskMessage adds a message to task history following A2A specification
func (cm *A2AConversationManager) AddTaskMessage(taskID, contextID string, message Message, maxHistoryLength int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Update task history
	history, exists := cm.taskHistories[taskID]
	if !exists {
		history = &TaskHistory{
			TaskID:      taskID,
			ContextID:   contextID,
			Messages:    make([]Message, 0),
			MaxLength:   maxHistoryLength,
			LastUpdated: time.Now(),
		}
		cm.taskHistories[taskID] = history
	}

	history.Messages = append(history.Messages, message)
	history.LastUpdated = time.Now()

	// Trim task history if needed
	if len(history.Messages) > maxHistoryLength {
		startIndex := len(history.Messages) - maxHistoryLength
		history.Messages = history.Messages[startIndex:]
	}

	// Update contextual messages for cross-task context
	cm.updateContextualMessages(contextID, taskID, message)

	// Invalidate LLM cache for this task
	cm.invalidateLLMCache(taskID)
}

// updateContextualMessages maintains messages by contextId for cross-task awareness
func (cm *A2AConversationManager) updateContextualMessages(contextID, taskID string, message Message) {
	contextMsgs, exists := cm.contextMessages[contextID]
	if !exists {
		contextMsgs = &ContextualMessages{
			ContextID:   contextID,
			Messages:    make([]Message, 0),
			TaskIDs:     make([]string, 0),
			LastUpdated: time.Now(),
		}
		cm.contextMessages[contextID] = contextMsgs
	}

	contextMsgs.Messages = append(contextMsgs.Messages, message)
	contextMsgs.LastUpdated = time.Now()

	// Track which tasks belong to this context
	found := false
	for _, tid := range contextMsgs.TaskIDs {
		if tid == taskID {
			found = true
			break
		}
	}
	if !found {
		contextMsgs.TaskIDs = append(contextMsgs.TaskIDs, taskID)
	}
}

// GetLLMMessages retrieves cached LLM-format messages or converts if needed
func (cm *A2AConversationManager) GetLLMMessages(taskID string, converter MessageConverter, includeSystemPrompt bool, systemPrompt string) ([]sdk.Message, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cacheKey := taskID
	if includeSystemPrompt {
		cacheKey = "system_" + taskID
	}

	cache, exists := cm.llmMessageCache[cacheKey]
	if exists && !cache.IsDirty {
		return cache.LLMMessages, nil
	}

	// Get task history
	history, exists := cm.taskHistories[taskID]
	if !exists {
		return []sdk.Message{}, nil
	}

	messages := history.Messages

	// Add system prompt if requested (following A2A agent pattern)
	if includeSystemPrompt && systemPrompt != "" {
		systemMessage := Message{
			Kind:      "message",
			MessageID: "system-prompt",
			Role:      "system",
			Parts: []Part{
				map[string]interface{}{
					"kind": string(MessagePartKindText),
					"text": systemPrompt,
				},
			},
		}
		messages = append([]Message{systemMessage}, messages...)
	}

	// Convert to LLM format
	llmMessages, err := converter.ConvertToSDK(messages)
	if err != nil {
		return nil, err
	}

	// Update cache
	if !exists {
		cache = &LLMMessageCache{
			ContextID: history.ContextID,
			TaskID:    taskID,
		}
		cm.llmMessageCache[cacheKey] = cache
	}

	cache.A2AMessages = messages
	cache.LLMMessages = llmMessages
	cache.LastConversion = time.Now()
	cache.IsDirty = false

	return llmMessages, nil
}

// invalidateLLMCache marks cache as dirty when messages change
func (cm *A2AConversationManager) invalidateLLMCache(taskID string) {
	// Invalidate both regular and system prompt caches
	for _, key := range []string{taskID, "system_" + taskID} {
		if cache, exists := cm.llmMessageCache[key]; exists {
			cache.IsDirty = true
		}
	}
}

// MessageConverter defines the interface for converting between A2A and SDK formats
type MessageConverter interface {
	// ConvertToSDK converts A2A messages to SDK format with type safety
	ConvertToSDK(messages []Message) ([]sdk.Message, error)

	// ConvertFromSDK converts SDK response back to A2A format
	ConvertFromSDK(response sdk.Message) (*Message, error)

	// ValidateMessagePart validates message part structure and type
	ValidateMessagePart(part Part) error
}

// OptimizedMessagePart provides strongly-typed message parts
type OptimizedMessagePart struct {
	Kind     MessagePartKind        `json:"kind"`
	Text     *string                `json:"text,omitempty"`
	File     *FileData              `json:"file,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// FileData represents file information with proper typing
type FileData struct {
	Name     *string `json:"name,omitempty"`
	MIMEType *string `json:"mimeType,omitempty"`
	Bytes    *string `json:"bytes,omitempty"`
	URI      *string `json:"uri,omitempty"`
}

// OptimizedMessageConverter provides efficient conversion with type safety
type OptimizedMessageConverter struct {
	logger *zap.Logger
}

// NewOptimizedMessageConverter creates a new optimized message converter
func NewOptimizedMessageConverter(logger *zap.Logger) *OptimizedMessageConverter {
	return &OptimizedMessageConverter{
		logger: logger,
	}
}

// ConvertToSDK converts A2A messages to SDK format with caching support
func (c *OptimizedMessageConverter) ConvertToSDK(messages []Message) ([]sdk.Message, error) {
	// Use interface{} to accommodate different SDK message types
	// In practice, you'd cast this to []sdk.Message or appropriate type
	result := make([]sdk.Message, 0, len(messages))

	for i, msg := range messages {
		// Convert each message using strongly-typed approach
		sdkMsg, err := c.convertSingleMessage(msg)
		if err != nil {
			c.logger.Error("failed to convert message",
				zap.Int("message_index", i),
				zap.String("message_id", msg.MessageID),
				zap.Error(err))
			return nil, err
		}
		result = append(result, sdkMsg)
	}

	return result, nil
}

// convertSingleMessage converts a single A2A message to SDK format
func (c *OptimizedMessageConverter) convertSingleMessage(msg Message) (sdk.Message, error) {
	role := msg.Role
	if role == "" {
		role = "user"
	}

	var content string

	for _, part := range msg.Parts {
		if typedPart, ok := part.(OptimizedMessagePart); ok {
			switch typedPart.Kind {
			case MessagePartKindText:
				if typedPart.Text != nil {
					content += *typedPart.Text
				}
			case MessagePartKindData:
				if typedPart.Data != nil {
					if result, exists := typedPart.Data["result"]; exists {
						if resultStr, ok := result.(string); ok {
							content += resultStr
						}
					}
				}
			case MessagePartKindFile:
				if typedPart.File != nil {
					c.logger.Debug("file part detected in message",
						zap.String("message_id", msg.MessageID))
				}
			}
		} else if partMap, ok := part.(map[string]interface{}); ok {
			c.logger.Debug("using fallback map processing for message part",
				zap.String("message_id", msg.MessageID))

			kind, hasKind := partMap["kind"]
			if !hasKind {
				continue
			}

			switch kind {
			case "text":
				if text, exists := partMap["text"]; exists {
					if textStr, ok := text.(string); ok {
						content += textStr
					}
				}
			case "data":
				if data, exists := partMap["data"]; exists {
					if dataMap, ok := data.(map[string]interface{}); ok {
						if result, exists := dataMap["result"]; exists {
							if resultStr, ok := result.(string); ok {
								content += resultStr
							}
						}
					}
				}
			}
		}
	}

	// Convert role to SDK format
	var sdkRole sdk.MessageRole
	switch role {
	case "user":
		sdkRole = sdk.User
	case "assistant":
		sdkRole = sdk.Assistant
	case "system":
		sdkRole = sdk.System
	case "tool":
		sdkRole = sdk.Tool
	default:
		sdkRole = sdk.User
	}

	// Return properly typed SDK message
	return sdk.Message{
		Role:    sdkRole,
		Content: content,
		// Add other SDK-specific fields as needed
	}, nil
}

// ConvertFromSDK converts SDK message response back to A2A format
func (c *OptimizedMessageConverter) ConvertFromSDK(response sdk.Message) (*Message, error) {
	// Convert SDK role back to A2A format
	role := string(response.Role)

	// Create A2A message with proper structure
	message := &Message{
		Kind:      "message",
		MessageID: "", // This should be set by the caller
		Role:      role,
		Parts: []Part{
			map[string]interface{}{
				"kind": string(MessagePartKindText),
				"text": response.Content,
			},
		},
	}

	// Handle tool calls if present
	if response.ToolCalls != nil && len(*response.ToolCalls) > 0 {
		// Add tool calls as data parts
		toolCallData := map[string]interface{}{
			"tool_calls": *response.ToolCalls,
		}
		message.Parts = append(message.Parts, map[string]interface{}{
			"kind": string(MessagePartKindData),
			"data": toolCallData,
		})
	}

	c.logger.Debug("converted SDK message to A2A format",
		zap.String("role", role),
		zap.String("content", response.Content),
		zap.Bool("has_tool_calls", response.ToolCalls != nil))

	return message, nil
}

// ValidateMessagePart validates message part structure and type
func (c *OptimizedMessageConverter) ValidateMessagePart(part Part) error {
	if typedPart, ok := part.(OptimizedMessagePart); ok {
		// Validate strongly-typed part
		if !typedPart.Kind.IsValid() {
			return fmt.Errorf("invalid message part kind: %s", typedPart.Kind)
		}

		switch typedPart.Kind {
		case MessagePartKindText:
			if typedPart.Text == nil {
				return fmt.Errorf("text part missing text field")
			}
		case MessagePartKindFile:
			if typedPart.File == nil {
				return fmt.Errorf("file part missing file field")
			}
		case MessagePartKindData:
			if typedPart.Data == nil {
				return fmt.Errorf("data part missing data field")
			}
		}
		return nil
	}

	// Validate map-based part for backward compatibility
	if partMap, ok := part.(map[string]interface{}); ok {
		kind, hasKind := partMap["kind"]
		if !hasKind {
			return fmt.Errorf("message part missing kind field")
		}

		kindStr, ok := kind.(string)
		if !ok {
			return fmt.Errorf("message part kind must be string")
		}

		partKind := MessagePartKind(kindStr)
		if !partKind.IsValid() {
			return fmt.Errorf("invalid message part kind: %s", kindStr)
		}

		return nil
	}

	return fmt.Errorf("unsupported message part type")
}
