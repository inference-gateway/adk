package server

import (
	"context"
	"testing"
)

func TestNewDefaultToolBox_CreatesEmptyToolBox(t *testing.T) {
	toolBox := NewDefaultToolBox()

	if toolBox == nil {
		t.Error("Expected NewDefaultToolBox to return a non-nil toolbox")
		return
	}

	toolNames := toolBox.GetToolNames()
	if len(toolNames) != 0 {
		t.Errorf("Expected default toolbox to have 0 tools, got %d", len(toolNames))
	}

	tools := toolBox.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected default toolbox to return 0 tools from GetTools(), got %d", len(tools))
	}

	if toolBox.HasTool("input_required") {
		t.Error("Expected default toolbox to not have input_required tool anymore")
	}
}

func TestDefaultToolBox_AddTool(t *testing.T) {
	toolBox := NewDefaultToolBox()
	
	// Add a test tool to verify the toolbox functionality
	testTool := NewBasicTool(
		"test_tool",
		"A test tool for verification",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			return "test result", nil
		},
	)
	
	toolBox.AddTool(testTool)
	
	tools := toolBox.GetTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool after adding test tool, got %d", len(tools))
	}
	
	if !toolBox.HasTool("test_tool") {
		t.Error("Expected toolbox to have test_tool after adding it")
	}
	
	result, err := toolBox.ExecuteTool(context.Background(), "test_tool", map[string]interface{}{})
	if err != nil {
		t.Errorf("Expected no error when executing test_tool, got: %v", err)
	}
	
	if result != "test result" {
		t.Errorf("Expected result 'test result', got '%s'", result)
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

	if toolBox.HasTool("nonexistent_tool") {
		t.Error("Expected empty toolbox to not have any tools")
	}

	testTool := NewBasicTool(
		"test_tool",
		"A test tool",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
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
