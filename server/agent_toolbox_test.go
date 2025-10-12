package server

import (
	"context"
	"testing"

	sdk "github.com/inference-gateway/sdk"
)

func TestNewDefaultToolBox_IncludesInputRequiredTool(t *testing.T) {
	toolBox := NewDefaultToolBox()

	if !toolBox.HasTool("input_required") {
		t.Error("Expected default toolbox to include 'input_required' tool")
	}

	toolNames := toolBox.GetToolNames()
	found := false
	for _, name := range toolNames {
		if name == "input_required" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'input_required' to be in tool names list")
	}

	result, err := toolBox.ExecuteTool(context.Background(), "input_required", map[string]any{
		"message": "Please provide more details about your request",
	})
	if err != nil {
		t.Errorf("Expected no error when executing input_required tool, got: %v", err)
	}

	expectedResult := ""
	if result != expectedResult {
		t.Errorf("Expected empty result (no-op handler), got '%s'", result)
	}
}

func TestNewDefaultToolBox_GetTools(t *testing.T) {
	toolBox := NewDefaultToolBox()
	tools := toolBox.GetTools()

	if len(tools) == 0 {
		t.Error("Expected at least one tool in default toolbox")
	}

	var inputRequiredTool *sdk.FunctionObject
	for _, tool := range tools {
		if tool.Function.Name == "input_required" {
			inputRequiredTool = &tool.Function
			break
		}
	}

	if inputRequiredTool == nil {
		t.Error("Expected to find input_required tool in GetTools() result")
		return
	}

	if inputRequiredTool.Description == nil || *inputRequiredTool.Description == "" {
		t.Error("Expected input_required tool to have a description")
	}

	if inputRequiredTool.Parameters == nil {
		t.Error("Expected input_required tool to have parameters")
	}
}

func TestNewToolBox_CreatesEmptyToolBox(t *testing.T) {
	toolBox := NewToolBox()

	if toolBox == nil {
		t.Error("Expected NewToolBox to return a non-nil toolbox")
		return
	}

	toolNames := toolBox.GetToolNames()
	if len(toolNames) != 0 {
		t.Errorf("Expected empty toolbox to have 0 tools, got %d", len(toolNames))
	}

	tools := toolBox.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected empty toolbox to return 0 tools from GetTools(), got %d", len(tools))
	}

	if toolBox.HasTool("input_required") {
		t.Error("Expected empty toolbox to not have any tools, including input_required")
	}

	testTool := NewBasicTool(
		"test_tool",
		"A test tool",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			return "test result", nil
		},
	)

	toolBox.AddTool(testTool)

	if !toolBox.HasTool("test_tool") {
		t.Error("Expected to be able to add tool to empty toolbox")
	}

	if len(toolBox.GetToolNames()) != 1 {
		t.Errorf("Expected toolbox to have 1 tool after adding, got %d", len(toolBox.GetToolNames()))
	}
}

func TestNewDefaultToolBoxWithCreateArtifact_Disabled(t *testing.T) {
	toolBox := NewDefaultToolBoxWithCreateArtifact(false)

	// Should only have input_required tool
	if toolBox.HasTool("create_artifact") {
		t.Error("Expected create_artifact tool to be disabled when CreateArtifact is false")
	}

	if !toolBox.HasTool("input_required") {
		t.Error("Expected input_required tool to always be present")
	}

	toolNames := toolBox.GetToolNames()
	if len(toolNames) != 1 {
		t.Errorf("Expected only 1 tool (input_required) when create_artifact is disabled, got %d", len(toolNames))
	}
}

func TestNewDefaultToolBoxWithCreateArtifact_Enabled(t *testing.T) {
	toolBox := NewDefaultToolBoxWithCreateArtifact(true)

	// Should have both tools
	if !toolBox.HasTool("create_artifact") {
		t.Error("Expected create_artifact tool to be enabled when CreateArtifact is true")
	}

	if !toolBox.HasTool("input_required") {
		t.Error("Expected input_required tool to always be present")
	}

	toolNames := toolBox.GetToolNames()
	if len(toolNames) != 2 {
		t.Errorf("Expected 2 tools when create_artifact is enabled, got %d", len(toolNames))
	}
}

func TestNewDefaultToolBox_DefaultBehavior(t *testing.T) {
	toolBox := NewDefaultToolBox()

	// Should only have input_required tool by default
	if toolBox.HasTool("create_artifact") {
		t.Error("Expected create_artifact tool to be disabled by default")
	}

	if !toolBox.HasTool("input_required") {
		t.Error("Expected input_required tool to always be present")
	}

	toolNames := toolBox.GetToolNames()
	if len(toolNames) != 1 {
		t.Errorf("Expected only 1 tool (input_required) by default, got %d", len(toolNames))
	}
}

func TestCreateArtifactTool_GetTools(t *testing.T) {
	toolBox := NewDefaultToolBoxWithCreateArtifact(true)
	tools := toolBox.GetTools()

	var createArtifactTool *sdk.FunctionObject
	for _, tool := range tools {
		if tool.Function.Name == "create_artifact" {
			createArtifactTool = &tool.Function
			break
		}
	}

	if createArtifactTool == nil {
		t.Error("Expected to find create_artifact tool in GetTools() result")
		return
	}

	if createArtifactTool.Description == nil || *createArtifactTool.Description == "" {
		t.Error("Expected create_artifact tool to have a description")
	}

	if createArtifactTool.Parameters == nil {
		t.Error("Expected create_artifact tool to have parameters")
		return
	}

	// Check parameters structure
	params := map[string]any(*createArtifactTool.Parameters)
	properties, ok := params["properties"].(map[string]any)
	if !ok {
		t.Error("Expected parameters to have properties")
		return
	}

	// Check required content parameter
	if _, exists := properties["content"]; !exists {
		t.Error("Expected create_artifact tool to have content parameter")
	}

	// Check required type parameter
	if _, exists := properties["type"]; !exists {
		t.Error("Expected create_artifact tool to have type parameter")
	}

	// Check optional parameters
	if _, exists := properties["name"]; !exists {
		t.Error("Expected create_artifact tool to have name parameter")
	}

	if _, exists := properties["filename"]; !exists {
		t.Error("Expected create_artifact tool to have filename parameter")
	}

	// Check required fields
	required, ok := params["required"].([]string)
	if !ok {
		t.Error("Expected parameters to have required array")
		return
	}

	expectedRequired := []string{"content", "type"}
	for _, req := range expectedRequired {
		found := false
		for _, actual := range required {
			if actual == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected %s to be in required parameters", req)
		}
	}
}

func TestExecuteCreateArtifact_MissingContext(t *testing.T) {
	ctx := context.Background()
	args := map[string]any{
		"content": "test content",
		"type":    "url",
	}

	result, err := executeCreateArtifact(ctx, args)

	if err == nil {
		t.Error("Expected error when task manager not found in context")
	}

	if result != "" {
		t.Errorf("Expected empty result on error, got: %s", result)
	}

	expectedError := "task manager not found in context"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestExecuteCreateArtifact_MissingContent(t *testing.T) {
	// Create mock task manager and artifact helper
	taskManager := &DefaultTaskManager{}
	artifactHelper := NewArtifactHelper()

	ctx := context.WithValue(context.Background(), "taskManager", taskManager)
	ctx = context.WithValue(ctx, "artifactHelper", artifactHelper)

	args := map[string]any{
		"type": "url",
	}

	result, err := executeCreateArtifact(ctx, args)

	if err == nil {
		t.Error("Expected error when content is missing")
	}

	if result != "" {
		t.Errorf("Expected empty result on error, got: %s", result)
	}

	expectedError := "content is required and must be a non-empty string"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestExecuteCreateArtifact_InvalidType(t *testing.T) {
	// Create mock task manager and artifact helper
	taskManager := &DefaultTaskManager{}
	artifactHelper := NewArtifactHelper()

	ctx := context.WithValue(context.Background(), "taskManager", taskManager)
	ctx = context.WithValue(ctx, "artifactHelper", artifactHelper)

	args := map[string]any{
		"content": "test content",
		"type":    "invalid",
	}

	result, err := executeCreateArtifact(ctx, args)

	if err == nil {
		t.Error("Expected error when type is invalid")
	}

	if result != "" {
		t.Errorf("Expected empty result on error, got: %s", result)
	}

	expectedError := "type must be 'url'"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestDetectFilename(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "JSON object",
			content:  `{"key": "value", "number": 123}`,
			expected: "content.json",
		},
		{
			name:     "JSON array",
			content:  `[{"key": "value"}, {"key": "value2"}]`,
			expected: "content.json",
		},
		{
			name:     "HTML document",
			content:  `<!DOCTYPE html><html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`,
			expected: "content.html",
		},
		{
			name:     "HTML with tag",
			content:  `<html><body>Hello World</body></html>`,
			expected: "content.html",
		},
		{
			name:     "XML document",
			content:  `<?xml version="1.0" encoding="UTF-8"?><root><item>test</item></root>`,
			expected: "content.xml",
		},
		{
			name:     "XML with namespace",
			content:  `<root xmlns="http://example.com"><item>test</item></root>`,
			expected: "content.xml",
		},
		{
			name:     "CSS styles",
			content:  `.class { color: red; margin: 10px; }`,
			expected: "content.css",
		},
		{
			name:     "JavaScript function",
			content:  `function test() { return "hello"; }`,
			expected: "content.js",
		},
		{
			name:     "JavaScript const",
			content:  `const greeting = "hello world";`,
			expected: "content.js",
		},
		{
			name:     "JavaScript arrow function",
			content:  `const test = () => { return "hello"; };`,
			expected: "content.js",
		},
		{
			name:     "Markdown with headers",
			content:  `# Main Title\n## Subtitle\nSome **bold** text`,
			expected: "content.md",
		},
		{
			name:     "Markdown with code blocks",
			content:  "Some text\n```javascript\nconsole.log('hello');\n```",
			expected: "content.md",
		},
		{
			name:     "CSV data",
			content:  `name,age,city\nJohn,25,New York\nJane,30,San Francisco`,
			expected: "content.csv",
		},
		{
			name:     "Plain text",
			content:  `This is just plain text without any special formatting.`,
			expected: "content.txt",
		},
		{
			name:     "Invalid JSON",
			content:  `{"key": "value"`,
			expected: "content.txt",
		},
		{
			name:     "Empty content",
			content:  ``,
			expected: "content.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectFilename(tt.content)
			if result != tt.expected {
				t.Errorf("detectFilename() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultToolBox_BackwardCompatibility(t *testing.T) {
	// Test that the existing NewDefaultToolBox function still works
	// and behaves the same as before (only input_required tool)
	defaultToolBox := NewDefaultToolBox()
	disabledToolBox := NewDefaultToolBoxWithCreateArtifact(false)

	// Both should have the same number of tools
	defaultNames := defaultToolBox.GetToolNames()
	disabledNames := disabledToolBox.GetToolNames()

	if len(defaultNames) != len(disabledNames) {
		t.Errorf("Expected same behavior: default toolbox has %d tools, disabled has %d", len(defaultNames), len(disabledNames))
	}

	// Both should have input_required tool
	if !defaultToolBox.HasTool("input_required") || !disabledToolBox.HasTool("input_required") {
		t.Error("Expected both toolboxes to have input_required tool")
	}

	// Neither should have create_artifact tool
	if defaultToolBox.HasTool("create_artifact") || disabledToolBox.HasTool("create_artifact") {
		t.Error("Expected neither toolbox to have create_artifact tool when disabled")
	}
}
