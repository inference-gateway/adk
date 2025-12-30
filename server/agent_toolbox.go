package server

import (
	"context"
	"encoding/json"
	"fmt"

	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
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

	// GetTool retrieves a tool by name, returning the tool and a boolean indicating if it was found
	GetTool(toolName string) (Tool, bool)
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
// The config parameter determines which tools are enabled
func NewDefaultToolBox(cfg *config.ToolBoxConfig) *DefaultToolBox {
	toolBox := NewToolBox()

	inputRequiredTool := NewBasicTool(
		"input_required",
		"REQUIRED: Use this tool when you need additional information from the user to provide a complete and accurate response. Call this instead of making assumptions or providing incomplete answers. Examples: missing location for weather, unclear requirements, ambiguous requests, or when more context would significantly improve the response quality.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "Clear, specific message explaining exactly what additional information you need from the user to complete their request. Be specific about what's missing and why it's needed.",
				},
			},
			"required": []string{"message"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			// NOTE: This handler is never executed in practice.
			// The agent intercepts input_required tool calls before execution
			// (see agent.go:186-204 and agent_streamable.go:325-354) and extracts
			// the 'message' argument directly from the tool call to create an
			// input_required response. This handler exists only to satisfy the
			// tool registration interface requirements.
			return "", nil
		},
	)
	toolBox.AddTool(inputRequiredTool)

	if cfg != nil && cfg.EnableCreateArtifact {
		createArtifactTool := NewBasicTool(
			"create_artifact",
			"Create an artifact file and make it available via downloadable URL. Use this tool to save important content, outputs, or generated files that the user might want to access or download. The artifact will be stored on the filesystem and made available through a URL.",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content": map[string]any{
						"type":        "string",
						"description": "The text content to save as an artifact file",
					},
					"type": map[string]any{
						"type":        "string",
						"description": "Must be 'url' - indicates the artifact will be available as a downloadable URL",
						"enum":        []string{"url"},
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Optional name for the artifact (will be auto-generated if not provided)",
					},
					"filename": map[string]any{
						"type":        "string",
						"description": "Filename with extension (e.g., 'report.json', 'data.csv', 'script.js')",
					},
				},
				"required": []string{"content", "type", "filename"},
			},
			func(ctx context.Context, args map[string]any) (string, error) {
				return executeCreateArtifact(ctx, args)
			},
		)
		toolBox.AddTool(createArtifactTool)
	}

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

// GetTool retrieves a tool by name, returning the tool and a boolean indicating if it was found
func (tb *DefaultToolBox) GetTool(toolName string) (Tool, bool) {
	tool, exists := tb.tools[toolName]
	return tool, exists
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

// executeCreateArtifact implements the create_artifact tool functionality
func executeCreateArtifact(ctx context.Context, args map[string]any) (string, error) {
	task, ok := ctx.Value(TaskContextKey).(*types.Task)
	if !ok {
		return "", fmt.Errorf("task not found in context")
	}

	artifactService, ok := ctx.Value(ArtifactServiceContextKey).(ArtifactService)
	if !ok || artifactService == nil {
		return "", fmt.Errorf("artifact service not found in context - cannot create URL-based artifacts")
	}

	content, ok := args["content"].(string)
	if !ok || content == "" {
		return "", fmt.Errorf("content is required and must be a non-empty string")
	}

	artifactType, ok := args["type"].(string)
	if !ok || artifactType != "url" {
		return "", fmt.Errorf("type must be 'url'")
	}

	filename, ok := args["filename"].(string)
	if !ok || filename == "" {
		return "", fmt.Errorf("filename is required and must be a non-empty string")
	}

	name, _ := args["name"].(string)
	if name == "" {
		name = "Generated Content"
	}

	data := []byte(content)
	mimeType := artifactService.GetMimeTypeFromExtension(filename)
	artifact, err := artifactService.CreateFileArtifact(
		name,
		fmt.Sprintf("Artifact created by create_artifact tool: %s", name),
		filename,
		data,
		mimeType,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create artifact: %w", err)
	}

	artifactService.AddArtifactToTask(task, artifact)

	if len(artifact.Parts) > 0 && artifact.Parts[0].File != nil {
		filePart := artifact.Parts[0].File
		if filePart.FileWithURI != nil {
			return JSONTool(map[string]any{
				"success":     true,
				"message":     fmt.Sprintf("Artifact '%s' created successfully", name),
				"artifact_id": artifact.ArtifactID,
				"url":         *filePart.FileWithURI,
				"filename":    filename,
			})
		}
	}

	return JSONTool(map[string]any{
		"success":     true,
		"message":     fmt.Sprintf("Artifact '%s' created successfully", name),
		"artifact_id": artifact.ArtifactID,
		"filename":    filename,
	})
}
