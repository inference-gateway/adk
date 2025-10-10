package server

import (
	"context"

	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

// CallbackContext provides context information to callback functions during execution
type CallbackContext struct {
	// AgentName is the name of the agent being executed
	AgentName string

	// InvocationID uniquely identifies this execution invocation
	InvocationID string

	// TaskID is the ID of the current task being processed
	TaskID string

	// ContextID is the conversation context ID
	ContextID string

	// State provides access to session state that can be read and modified
	State map[string]any

	// Logger provides access to the logger for callback implementations
	Logger *zap.Logger
}

// ToolContext provides context information to tool-related callback functions
type ToolContext struct {
	// AgentName is the name of the agent executing the tool
	AgentName string

	// InvocationID uniquely identifies this execution invocation
	InvocationID string

	// TaskID is the ID of the current task being processed
	TaskID string

	// ContextID is the conversation context ID
	ContextID string

	// State provides access to session state that can be read and modified
	State map[string]any

	// Logger provides access to the logger for callback implementations
	Logger *zap.Logger
}

// LLMRequest represents a request to be sent to the LLM
type LLMRequest struct {
	// Contents are the conversation messages being sent to the LLM
	Contents []types.Message

	// Config contains LLM configuration like system instruction, temperature, etc.
	Config *LLMConfig
}

// LLMConfig contains configuration for LLM requests
type LLMConfig struct {
	// SystemInstruction is the system prompt/instruction for the LLM
	SystemInstruction *types.Message

	// Temperature controls randomness in LLM responses (0.0-2.0)
	Temperature *float64

	// MaxTokens is the maximum number of tokens to generate
	MaxTokens *int
}

// LLMResponse represents a response from the LLM
type LLMResponse struct {
	// Content is the main response content from the LLM
	Content *types.Message
}

// Agent Lifecycle Callbacks

// BeforeAgentCallback is called immediately before the agent's main execution logic starts
// Return nil to allow normal execution, or return types.Message to skip agent execution and use its result as the final response.
//
// It's purpose is for setting up resources or state required for a specific agent run,
// performing validation checks on the session state (not implemented yet) before execution starts,
// adding additional logging points for agent activity or modifying the agent context before the core logic uses it.
type BeforeAgentCallback func(ctx context.Context, callbackContext *CallbackContext) *types.Message

// AfterAgentCallback is called immediately after the agent's execution completes
//
// It does not run if the agent was skipped due to BeforeAgentCallback returning a types.Message
// It's purpose can be for clean up tasks, post-execution validation, logging or modifying the final agent output.
//
// Return nil to use the original output, or return types.Message to replace the agent's output
type AfterAgentCallback func(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message

// LLM Interaction Callbacks

// BeforeModelCallback is called just before sending a request to the LLM
//
// It's purpose is to allow inspection and modification of the request going to the LLM.
// E.g. adding dynamic instructions, guardrails, modifying the model config or implementing request-level caching.
//
// Return nil to allow the request to proceed, or return LLMResponse to skip the LLM call.
// The returned LLMResponse is used directly as if it came from the model making it a powerful option for implementing guardrails or caching.
type BeforeModelCallback func(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse

// AfterModelCallback is called just after receiving a response from the LLM, before it's processed further by the invoking agent.
//
// It's purpose is to allow inspection or modification of the raw LLM response. E.g. log model outputs, reformat responses, censor sensitive information,
// parsing structured data or handling specific error codes.
//
// It's not clear in the official ADK docs whether the after callback is skipped if the BeforeModelCallback returns an LLMResponse.
// To avoid confusion the AfterModelCallback will always be invoked regardless if the LLMResponse is real or modified by the BeforeModelCallback.
//
// Return nil to use the original response, or return LLMResponse to replace the LLM response
type AfterModelCallback func(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse

// Tool Execution Callbacks

// BeforeToolCallback is called just before a tool's execution
//
// It's purpose is to allow inspection and modification of the tool arguments, perform authz checks, logging or implementing tool-level caching.
//
// Return nil to allow the tool to execute, or return map[string]interface{} to skip tool execution. The returned map is used directly as the result of the tool call.
// Making it useful for either caching or overriding the tool behaviour completely.
type BeforeToolCallback func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{}

// AfterToolCallback is called just after a tool's execution completes successfully.
//
// It's purpose is to allow inspection or modification of the tool's result before it's sent back to the LLM.
// Useful for logging, post-processing, or in the future saving parts of the result to the session state (not implemented yet).
//
// Return nil to use the original result, or return map[string]interface{} to replace the tool result
type AfterToolCallback func(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{}

// CallbackConfig holds all callback configurations for an agent
type CallbackConfig struct {
	// Agent lifecycle callbacks
	BeforeAgent []BeforeAgentCallback
	AfterAgent  []AfterAgentCallback

	// LLM interaction callbacks
	BeforeModel []BeforeModelCallback
	AfterModel  []AfterModelCallback

	// Tool execution callbacks
	BeforeTool []BeforeToolCallback
	AfterTool  []AfterToolCallback
}

// CallbackExecutor handles the execution of callbacks with proper flow control
type CallbackExecutor interface {
	// ExecuteBeforeAgent executes the before agent callback if configured
	ExecuteBeforeAgent(ctx context.Context, callbackContext *CallbackContext) *types.Message

	// ExecuteAfterAgent executes the after agent callback if configured
	ExecuteAfterAgent(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message

	// ExecuteBeforeModel executes the before model callback if configured
	ExecuteBeforeModel(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse

	// ExecuteAfterModel executes the after model callback if configured
	ExecuteAfterModel(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse

	// ExecuteBeforeTool executes the before tool callback if configured
	ExecuteBeforeTool(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{}

	// ExecuteAfterTool executes the after tool callback if configured
	ExecuteAfterTool(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{}
}

// DefaultCallbackExecutor implements CallbackExecutor with proper error handling and logging
type DefaultCallbackExecutor struct {
	config *CallbackConfig
	logger *zap.Logger
}

// NewCallbackExecutor creates a new callback executor with the given configuration
func NewCallbackExecutor(config *CallbackConfig, logger *zap.Logger) CallbackExecutor {
	return &DefaultCallbackExecutor{
		config: config,
		logger: logger,
	}
}

// ExecuteBeforeAgent executes all before agent callbacks if configured
// Returns the result of the first callback that returns a non-nil value (flow control)
// If all callbacks return nil, execution continues normally
func (ce *DefaultCallbackExecutor) ExecuteBeforeAgent(ctx context.Context, callbackContext *CallbackContext) *types.Message {
	if ce.config == nil || len(ce.config.BeforeAgent) == 0 {
		return nil
	}

	// Execute callbacks in sequence, respecting flow control
	for i, callback := range ce.config.BeforeAgent {
		var result *types.Message
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic and continue with next callback or normal execution
					ce.logger.Error("panic in BeforeAgent callback", zap.Int("callback_index", i), zap.Any("panic", r))
				}
			}()

			result = callback(ctx, callbackContext)
		}()

		if result != nil {
			// First non-nil result stops execution and returns the override
			ce.logger.Debug("BeforeAgent callback returned override, skipping remaining callbacks", zap.Int("callback_index", i))
			return result
		}
	}

	return nil
}

// ExecuteAfterAgent executes all after agent callbacks if configured
// Chains the callbacks - each callback receives the output of the previous callback
// Returns the final modified output, or nil if no callbacks modify the output
func (ce *DefaultCallbackExecutor) ExecuteAfterAgent(ctx context.Context, callbackContext *CallbackContext, agentOutput *types.Message) *types.Message {
	if ce.config == nil || len(ce.config.AfterAgent) == 0 {
		return nil
	}

	currentOutput := agentOutput
	var finalResult *types.Message

	// Execute callbacks in sequence, chaining their outputs
	for i, callback := range ce.config.AfterAgent {
		var result *types.Message
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic and continue with current output
					ce.logger.Error("panic in AfterAgent callback", zap.Int("callback_index", i), zap.Any("panic", r))
				}
			}()

			result = callback(ctx, callbackContext, currentOutput)
		}()

		if result != nil {
			// Use the modified output for the next callback
			currentOutput = result
			finalResult = result
			ce.logger.Debug("AfterAgent callback modified output", zap.Int("callback_index", i))
		}
	}

	return finalResult
}

// ExecuteBeforeModel executes all before model callbacks if configured
// Returns the result of the first callback that returns a non-nil value (flow control)
// If all callbacks return nil, execution continues normally with LLM call
func (ce *DefaultCallbackExecutor) ExecuteBeforeModel(ctx context.Context, callbackContext *CallbackContext, llmRequest *LLMRequest) *LLMResponse {
	if ce.config == nil || len(ce.config.BeforeModel) == 0 {
		return nil
	}

	// Execute callbacks in sequence, respecting flow control
	for i, callback := range ce.config.BeforeModel {
		var result *LLMResponse
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic and continue with next callback or LLM call
					ce.logger.Error("panic in BeforeModel callback", zap.Int("callback_index", i), zap.Any("panic", r))
				}
			}()

			result = callback(ctx, callbackContext, llmRequest)
		}()

		if result != nil {
			// First non-nil result stops execution and returns the override
			ce.logger.Debug("BeforeModel callback returned override, skipping LLM call and remaining callbacks", zap.Int("callback_index", i))
			return result
		}
	}

	return nil
}

// ExecuteAfterModel executes all after model callbacks if configured
// Chains the callbacks - each callback receives the response from the previous callback
// Returns the final modified response, or nil if no callbacks modify the response
func (ce *DefaultCallbackExecutor) ExecuteAfterModel(ctx context.Context, callbackContext *CallbackContext, llmResponse *LLMResponse) *LLMResponse {
	if ce.config == nil || len(ce.config.AfterModel) == 0 {
		return nil
	}

	currentResponse := llmResponse
	var finalResult *LLMResponse

	// Execute callbacks in sequence, chaining their outputs
	for i, callback := range ce.config.AfterModel {
		var result *LLMResponse
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic and continue with current response
					ce.logger.Error("panic in AfterModel callback", zap.Int("callback_index", i), zap.Any("panic", r))
				}
			}()

			result = callback(ctx, callbackContext, currentResponse)
		}()

		if result != nil {
			// Use the modified response for the next callback
			currentResponse = result
			finalResult = result
			ce.logger.Debug("AfterModel callback modified response", zap.Int("callback_index", i))
		}
	}

	return finalResult
}

// ExecuteBeforeTool executes all before tool callbacks if configured
// Returns the result of the first callback that returns a non-nil value (flow control)
// If all callbacks return nil, execution continues normally with tool call
func (ce *DefaultCallbackExecutor) ExecuteBeforeTool(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext) map[string]interface{} {
	if ce.config == nil || len(ce.config.BeforeTool) == 0 {
		return nil
	}

	// Execute callbacks in sequence, respecting flow control
	for i, callback := range ce.config.BeforeTool {
		var result map[string]interface{}
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic and continue with next callback or tool execution
					ce.logger.Error("panic in BeforeTool callback", zap.Int("callback_index", i), zap.Any("panic", r))
				}
			}()

			result = callback(ctx, tool, args, toolContext)
		}()

		if result != nil {
			// First non-nil result stops execution and returns the override
			ce.logger.Debug("BeforeTool callback returned override, skipping tool execution and remaining callbacks", zap.Int("callback_index", i))
			return result
		}
	}

	return nil
}

// ExecuteAfterTool executes all after tool callbacks if configured
// Chains the callbacks - each callback receives the result from the previous callback
// Returns the final modified result, or nil if no callbacks modify the result
func (ce *DefaultCallbackExecutor) ExecuteAfterTool(ctx context.Context, tool Tool, args map[string]interface{}, toolContext *ToolContext, toolResult map[string]interface{}) map[string]interface{} {
	if ce.config == nil || len(ce.config.AfterTool) == 0 {
		return nil
	}

	currentResult := toolResult
	var finalResult map[string]interface{}

	// Execute callbacks in sequence, chaining their outputs
	for i, callback := range ce.config.AfterTool {
		var result map[string]interface{}
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic and continue with current result
					ce.logger.Error("panic in AfterTool callback", zap.Int("callback_index", i), zap.Any("panic", r))
				}
			}()

			result = callback(ctx, tool, args, toolContext, currentResult)
		}()

		if result != nil {
			// Use the modified result for the next callback
			currentResult = result
			finalResult = result
			ce.logger.Debug("AfterTool callback modified result", zap.Int("callback_index", i))
		}
	}

	return finalResult
}
