package server

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/assert"

	otelapi "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	baggage "go.opentelemetry.io/otel/baggage"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
	trace "go.opentelemetry.io/otel/trace"

	sdk "github.com/inference-gateway/sdk"

	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

func TestNewDefaultToolBox_IncludesInputRequiredTool(t *testing.T) {
	toolBox := NewDefaultToolBox(nil)

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
	toolBox := NewDefaultToolBox(nil)
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

func TestNewDefaultToolBox_OnlyInputRequired(t *testing.T) {
	toolBox := NewDefaultToolBox(nil)

	if toolBox.HasTool("create_artifact") {
		t.Error("Expected create_artifact tool to not be in default toolbox")
	}

	if !toolBox.HasTool("input_required") {
		t.Error("Expected input_required tool to always be present")
	}

	toolNames := toolBox.GetToolNames()
	if len(toolNames) != 1 {
		t.Errorf("Expected only 1 tool (input_required) in default toolbox, got %d", len(toolNames))
	}
}

func TestDefaultToolBox_WithCreateArtifactAdded(t *testing.T) {
	toolBox := NewDefaultToolBox(&serverConfig.ToolBoxConfig{
		EnableCreateArtifact: true,
	})

	if !toolBox.HasTool("create_artifact") {
		t.Error("Expected create_artifact tool to be present after adding")
	}

	if !toolBox.HasTool("input_required") {
		t.Error("Expected input_required tool to always be present")
	}

	toolNames := toolBox.GetToolNames()
	if len(toolNames) != 2 {
		t.Errorf("Expected 2 tools with create_artifact added, got %d", len(toolNames))
	}
}

func TestNewDefaultToolBox_DefaultBehavior(t *testing.T) {
	toolBox := NewDefaultToolBox(nil)

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
	toolBox := NewDefaultToolBox(&serverConfig.ToolBoxConfig{
		EnableCreateArtifact: true,
	})

	tools := toolBox.GetTools()

	var foundTool *sdk.FunctionObject
	for _, tool := range tools {
		if tool.Function.Name == "create_artifact" {
			foundTool = &tool.Function
			break
		}
	}

	if foundTool == nil {
		t.Error("Expected to find create_artifact tool in GetTools() result")
		return
	}

	if foundTool.Description == nil || *foundTool.Description == "" {
		t.Error("Expected create_artifact tool to have a description")
	}

	if foundTool.Parameters == nil {
		t.Error("Expected create_artifact tool to have parameters")
		return
	}

	params := map[string]any(*foundTool.Parameters)
	properties, ok := params["properties"].(map[string]any)
	if !ok {
		t.Error("Expected parameters to have properties")
		return
	}

	if _, exists := properties["content"]; !exists {
		t.Error("Expected create_artifact tool to have content parameter")
	}

	if _, exists := properties["type"]; !exists {
		t.Error("Expected create_artifact tool to have type parameter")
	}

	if _, exists := properties["name"]; !exists {
		t.Error("Expected create_artifact tool to have name parameter")
	}

	if _, exists := properties["filename"]; !exists {
		t.Error("Expected create_artifact tool to have filename parameter")
	}

	required, ok := params["required"].([]string)
	if !ok {
		t.Error("Expected parameters to have required array")
		return
	}

	expectedRequired := []string{"content", "type", "filename"}
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
		"content":  "test content",
		"type":     "url",
		"filename": "test.txt",
	}

	result, err := executeCreateArtifact(ctx, args)

	if err == nil {
		t.Error("Expected error when task not found in context")
	}

	if result != "" {
		t.Errorf("Expected empty result on error, got: %s", result)
	}

	expectedError := "task not found in context"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestExecuteCreateArtifact_MissingContent(t *testing.T) {
	task := &types.Task{ID: "test-task"}
	artifactService := &ArtifactServiceImpl{
		storage: nil,
		logger:  nil,
	}

	ctx := context.WithValue(context.Background(), TaskContextKey, task)
	ctx = context.WithValue(ctx, ArtifactServiceContextKey, artifactService)

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

func TestExecuteCreateArtifact_MissingFilename(t *testing.T) {
	task := &types.Task{ID: "test-task"}
	artifactService := &ArtifactServiceImpl{
		storage: nil,
		logger:  nil,
	}

	ctx := context.WithValue(context.Background(), TaskContextKey, task)
	ctx = context.WithValue(ctx, ArtifactServiceContextKey, artifactService)

	args := map[string]any{
		"content": "test content",
		"type":    "url",
	}

	result, err := executeCreateArtifact(ctx, args)

	if err == nil {
		t.Error("Expected error when filename is missing")
	}

	if result != "" {
		t.Errorf("Expected empty result on error, got: %s", result)
	}

	expectedError := "filename is required and must be a non-empty string"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestExecuteCreateArtifact_InvalidType(t *testing.T) {
	task := &types.Task{ID: "test-task"}
	artifactService := &ArtifactServiceImpl{
		storage: nil,
		logger:  nil,
	}

	ctx := context.WithValue(context.Background(), TaskContextKey, task)
	ctx = context.WithValue(ctx, ArtifactServiceContextKey, artifactService)

	args := map[string]any{
		"content":  "test content",
		"type":     "invalid",
		"filename": "test.txt",
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

func TestExecuteCreateArtifact_MissingService(t *testing.T) {
	task := &types.Task{ID: "test-task"}

	ctx := context.WithValue(context.Background(), TaskContextKey, task)

	args := map[string]any{
		"content":  "test content",
		"type":     "url",
		"filename": "test.txt",
	}

	result, err := executeCreateArtifact(ctx, args)

	if err == nil {
		t.Error("Expected error when service is not configured")
	}

	if result != "" {
		t.Errorf("Expected empty result on error, got: %s", result)
	}

	expectedError := "artifact service not found in context - cannot create URL-based artifacts"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestDefaultToolBox_ExecuteToolSpan verifies every tool execution is wrapped
// in a tool.<name> span parented on the caller's span, so custom tools trace
// without their own instrumentation.
func TestDefaultToolBox_ExecuteToolSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prev := otelapi.GetTracerProvider()
	otelapi.SetTracerProvider(tp)
	t.Cleanup(func() { otelapi.SetTracerProvider(prev) })

	toolBox := NewDefaultToolBox(nil)
	toolBox.AddTool(NewBasicTool("navigate", "nav", map[string]any{"type": "object"},
		func(ctx context.Context, _ map[string]any) (string, error) {
			assert.True(t, trace.SpanFromContext(ctx).SpanContext().IsValid())
			return "ok", nil
		}))

	member, _ := baggage.NewMember(serverConfig.DefaultAttrSessionIDKey, "s1")
	bag, _ := baggage.New(member)
	ctx := baggage.ContextWithBaggage(context.Background(), bag)
	ctx, parent := tp.Tracer("test").Start(ctx, "task.process")
	_, err := toolBox.ExecuteTool(ctx, "navigate", nil)
	parent.End()
	assert.NoError(t, err)

	spans := exporter.GetSpans()
	if assert.Len(t, spans, 2) {
		assert.Equal(t, "tool.navigate", spans[0].Name)
		assert.Equal(t, parent.SpanContext().SpanID(), spans[0].Parent.SpanID())
		assert.Contains(t, spans[0].Attributes, attribute.String(serverConfig.DefaultAttrSessionIDKey, "s1"))
	}
}
