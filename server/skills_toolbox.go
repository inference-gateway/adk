package server

import (
	"context"

	"github.com/inference-gateway/adk/server/skills"
	sdk "github.com/inference-gateway/sdk"
)

// SkillsToolBox wraps the skills registry to provide ToolBox interface
type SkillsToolBox struct {
	registry *skills.SkillsRegistry
	base     ToolBox
}

// NewSkillsToolBox creates a new ToolBox with skills support
func NewSkillsToolBox(registry *skills.SkillsRegistry) *SkillsToolBox {
	return &SkillsToolBox{
		registry: registry,
		base:     NewDefaultToolBox(),
	}
}

// GetTools returns all tools including skills
func (stb *SkillsToolBox) GetTools() []sdk.ChatCompletionTool {
	// Get base tools
	tools := stb.base.GetTools()

	// Add skills tools
	for _, skill := range stb.registry.GetSkills() {
		description := skill.GetDescription()
		parameters := skill.GetParameters()

		tools = append(tools, sdk.ChatCompletionTool{
			Type: sdk.Function,
			Function: sdk.FunctionObject{
				Name:        skill.GetName(),
				Description: &description,
				Parameters:  (*sdk.FunctionParameters)(&parameters),
			},
		})
	}

	return tools
}

// ExecuteTool executes a tool, delegating to skills registry if it's a skill
func (stb *SkillsToolBox) ExecuteTool(ctx context.Context, toolName string, arguments map[string]any) (string, error) {
	// Check if it's a skill first
	if stb.registry.HasSkill(toolName) {
		return stb.registry.ExecuteSkill(ctx, toolName, arguments)
	}

	// Fall back to base toolbox
	return stb.base.ExecuteTool(ctx, toolName, arguments)
}

// GetToolNames returns names of all tools including skills
func (stb *SkillsToolBox) GetToolNames() []string {
	names := stb.base.GetToolNames()
	names = append(names, stb.registry.GetSkillNames()...)
	return names
}

// HasTool checks if a tool exists (including skills)
func (stb *SkillsToolBox) HasTool(toolName string) bool {
	return stb.base.HasTool(toolName) || stb.registry.HasSkill(toolName)
}
