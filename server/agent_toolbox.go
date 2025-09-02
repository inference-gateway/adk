package server

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/inference-gateway/sdk"
)

// ToolBox defines the interface for a collection of tools that can be used by OpenAI-compatible agents
type ToolBox interface {
	// GetTools returns all available tools in OpenAI function call format
	GetTools() []sdk.ChatCompletionTool

	// ExecuteTool executes a tool by name with the provided arguments
	// Returns the tool result as a string and any error that occurred
	ExecuteTool(ctx context.Context, toolName string, arguments map[string]any) (string, error)

	// GetToolNames returns a list of all available tool names
	GetToolNames() []string

	// HasTool checks if a tool with the given name exists
	HasTool(toolName string) bool
}

// Tool represents a single tool that can be executed
type Tool interface {
	// GetName returns the name of the tool
	GetName() string

	// GetDescription returns a description of what the tool does
	GetDescription() string

	// GetParameters returns the JSON schema for the tool parameters
	GetParameters() map[string]any

	// Execute runs the tool with the provided arguments
	Execute(ctx context.Context, arguments map[string]any) (string, error)
}

// DefaultToolBox is a default implementation of ToolBox
type DefaultToolBox struct {
	tools map[string]Tool
}

// NewToolBox creates a new empty DefaultToolBox
func NewToolBox() *DefaultToolBox {
	return &DefaultToolBox{
		tools: make(map[string]Tool),
	}
}

// NewDefaultToolBox creates a new DefaultToolBox with built-in tools
func NewDefaultToolBox() *DefaultToolBox {
	toolBox := NewToolBox()

	inputRequiredTool := NewBasicTool(
		"input_required",
		"Request additional input from the user when current information is insufficient to complete the task",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "The message to display to the user explaining what information is needed",
				},
			},
			"required": []string{"message"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			message := args["message"].(string)
			return fmt.Sprintf("Input requested from user: %s", message), nil
		},
	)
	toolBox.AddTool(inputRequiredTool)

	return toolBox
}

// AddTool adds a tool to the toolbox
func (tb *DefaultToolBox) AddTool(tool Tool) {
	tb.tools[tool.GetName()] = tool
}

// GetTools returns all available tools in OpenAI function call format
func (tb *DefaultToolBox) GetTools() []sdk.ChatCompletionTool {
	tools := make([]sdk.ChatCompletionTool, 0, len(tb.tools))

	for _, tool := range tb.tools {
		description := tool.GetDescription()
		parameters := tool.GetParameters()

		tools = append(tools, sdk.ChatCompletionTool{
			Type: sdk.Function,
			Function: sdk.FunctionObject{
				Name:        tool.GetName(),
				Description: &description,
				Parameters:  (*sdk.FunctionParameters)(&parameters),
			},
		})
	}

	return tools
}

// ExecuteTool executes a tool by name with the provided arguments
func (tb *DefaultToolBox) ExecuteTool(ctx context.Context, toolName string, arguments map[string]any) (string, error) {
	tool, exists := tb.tools[toolName]
	if !exists {
		return "", &ToolNotFoundError{ToolName: toolName}
	}

	return tool.Execute(ctx, arguments)
}

// GetToolNames returns a list of all available tool names
func (tb *DefaultToolBox) GetToolNames() []string {
	names := make([]string, 0, len(tb.tools))
	for name := range tb.tools {
		names = append(names, name)
	}
	return names
}

// HasTool checks if a tool with the given name exists
func (tb *DefaultToolBox) HasTool(toolName string) bool {
	_, exists := tb.tools[toolName]
	return exists
}

// ToolNotFoundError represents an error when a requested tool is not found
type ToolNotFoundError struct {
	ToolName string
}

func (e *ToolNotFoundError) Error() string {
	return "tool not found: " + e.ToolName
}

// BasicTool is a simple implementation of the Tool interface using function callbacks
type BasicTool struct {
	name        string
	description string
	parameters  map[string]any
	executor    func(ctx context.Context, arguments map[string]any) (string, error)
}

// NewBasicTool creates a new BasicTool
func NewBasicTool(
	name string,
	description string,
	parameters map[string]any,
	executor func(ctx context.Context, arguments map[string]any) (string, error),
) *BasicTool {
	return &BasicTool{
		name:        name,
		description: description,
		parameters:  parameters,
		executor:    executor,
	}
}

func (t *BasicTool) GetName() string {
	return t.name
}

func (t *BasicTool) GetDescription() string {
	return t.description
}

func (t *BasicTool) GetParameters() map[string]any {
	return t.parameters
}

func (t *BasicTool) Execute(ctx context.Context, arguments map[string]any) (string, error) {
	return t.executor(ctx, arguments)
}

// JSONTool creates a tool result that can be marshaled to JSON
func JSONTool(result any) (string, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
