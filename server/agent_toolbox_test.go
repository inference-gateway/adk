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

	expectedResult := "Input requested from user: Please provide more details about your request"
	if result != expectedResult {
		t.Errorf("Expected result '%s', got '%s'", expectedResult, result)
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
