package skills

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Tool represents a single tool that can be executed (copied to avoid import cycle)
type Tool interface {
	GetName() string
	GetDescription() string
	GetParameters() map[string]any
	Execute(ctx context.Context, arguments map[string]any) (string, error)
}

// SkillsRegistry manages the registration and execution of built-in skills
type SkillsRegistry struct {
	logger     *zap.Logger
	config     *SkillsConfig
	security   *SecurityValidator
	skills     map[string]Skill
	rateLimits map[string]*RateLimiter
	mutex      sync.RWMutex
}

// Skill represents a built-in skill that can be executed
type Skill interface {
	Tool
	IsEnabled() bool
	GetRateLimit() int
	RequiresApproval() bool
}

// NewSkillsRegistry creates a new skills registry
func NewSkillsRegistry(logger *zap.Logger, config *SkillsConfig) *SkillsRegistry {
	registry := &SkillsRegistry{
		logger:     logger,
		config:     config,
		security:   NewSecurityValidator(&config.Safety),
		skills:     make(map[string]Skill),
		rateLimits: make(map[string]*RateLimiter),
		mutex:      sync.RWMutex{},
	}

	// Register built-in skills if enabled
	if config.Enabled {
		registry.registerBuiltinSkills()
	}

	return registry
}

// registerBuiltinSkills registers all built-in skills based on configuration
func (sr *SkillsRegistry) registerBuiltinSkills() {
	// Register Read skill
	if sr.config.ReadSkill.Enabled {
		readSkill := NewReadSkill(sr.logger, &sr.config.ReadSkill, sr.security)
		sr.RegisterSkill(readSkill)
		sr.logger.Info("registered Read skill")
	}

	// Register Write skill
	if sr.config.WriteSkill.Enabled {
		writeSkill := NewWriteSkill(sr.logger, &sr.config.WriteSkill, sr.security)
		sr.RegisterSkill(writeSkill)
		sr.logger.Info("registered Write skill")
	}

	// Register Edit skill
	if sr.config.EditSkill.Enabled {
		editSkill := NewEditSkill(sr.logger, &sr.config.EditSkill, sr.security)
		sr.RegisterSkill(editSkill)
		sr.logger.Info("registered Edit skill")
	}

	// Register MultiEdit skill
	if sr.config.MultiEdit.Enabled {
		multiEditSkill := NewMultiEditSkill(sr.logger, &sr.config.MultiEdit, sr.security)
		sr.RegisterSkill(multiEditSkill)
		sr.logger.Info("registered MultiEdit skill")
	}

	// Register WebSearch skill
	if sr.config.WebSearch.Enabled {
		webSearchSkill := NewWebSearchSkill(sr.logger, &sr.config.WebSearch, sr.security)
		sr.RegisterSkill(webSearchSkill)
		sr.logger.Info("registered WebSearch skill")
	}

	// Register WebFetch skill
	if sr.config.WebFetch.Enabled {
		webFetchSkill := NewWebFetchSkill(sr.logger, &sr.config.WebFetch, sr.security)
		sr.RegisterSkill(webFetchSkill)
		sr.logger.Info("registered WebFetch skill")
	}
}

// RegisterSkill registers a skill in the registry
func (sr *SkillsRegistry) RegisterSkill(skill Skill) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	name := skill.GetName()
	sr.skills[name] = skill
	
	// Initialize rate limiter for this skill
	if skill.GetRateLimit() > 0 {
		sr.rateLimits[name] = NewRateLimiter(skill.GetRateLimit())
	}
}

// GetSkills returns all registered skills as Tools for the ToolBox
func (sr *SkillsRegistry) GetSkills() []Tool {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()

	tools := make([]Tool, 0, len(sr.skills))
	for _, skill := range sr.skills {
		if skill.IsEnabled() {
			tools = append(tools, skill)
		}
	}
	return tools
}

// GetSkillsForAgentCard returns skills formatted for AgentCard
func (sr *SkillsRegistry) GetSkillsForAgentCard() []map[string]interface{} {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()

	skills := make([]map[string]interface{}, 0, len(sr.skills))
	for _, skill := range sr.skills {
		if skill.IsEnabled() {
			skillInfo := map[string]interface{}{
				"id":          skill.GetName(),
				"name":        skill.GetName(),
				"description": skill.GetDescription(),
				"tags":        []string{"built-in", "file-ops", "web-ops"},
			}
			
			// Add input/output modes if this skill supports them
			skillInfo["inputModes"] = []string{"text"}
			skillInfo["outputModes"] = []string{"text", "json"}
			
			// Add examples if available
			if examples := sr.getSkillExamples(skill.GetName()); len(examples) > 0 {
				skillInfo["examples"] = examples
			}
			
			skills = append(skills, skillInfo)
		}
	}
	return skills
}

// getSkillExamples returns usage examples for a skill
func (sr *SkillsRegistry) getSkillExamples(skillName string) []string {
	examples := map[string][]string{
		"read": {
			"Read the contents of config.yaml",
			"Read lines 10-20 from server.log",
		},
		"write": {
			"Write configuration data to config.json",
			"Create a new document with specified content",
		},
		"edit": {
			"Replace 'old text' with 'new text' in file.txt",
			"Update configuration value in settings file",
		},
		"multi_edit": {
			"Apply multiple text replacements to a file",
			"Batch update multiple configuration values",
		},
		"web_search": {
			"Search for 'golang best practices'",
			"Find recent articles about 'microservices architecture'",
		},
		"web_fetch": {
			"Fetch content from https://api.example.com/data",
			"Download documentation from a public URL",
		},
	}
	return examples[skillName]
}

// ExecuteSkill executes a skill with rate limiting and security checks
func (sr *SkillsRegistry) ExecuteSkill(ctx context.Context, skillName string, arguments map[string]any) (string, error) {
	sr.mutex.RLock()
	skill, exists := sr.skills[skillName]
	sr.mutex.RUnlock()

	if !exists {
		return "", fmt.Errorf("skill not found: %s", skillName)
	}

	if !skill.IsEnabled() {
		return "", fmt.Errorf("skill disabled: %s", skillName)
	}

	// Apply rate limiting
	if rateLimiter, exists := sr.rateLimits[skillName]; exists {
		if !rateLimiter.Allow(skillName) {
			return "", fmt.Errorf("rate limit exceeded for skill: %s", skillName)
		}
	}

	// Create timeout context based on security configuration
	timeoutCtx, cancel := sr.security.TimeoutContext(ctx)
	defer cancel()

	// Execute the skill
	return skill.Execute(timeoutCtx, arguments)
}

// HasSkill checks if a skill is registered
func (sr *SkillsRegistry) HasSkill(skillName string) bool {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()
	
	skill, exists := sr.skills[skillName]
	return exists && skill.IsEnabled()
}

// GetSkillNames returns names of all enabled skills
func (sr *SkillsRegistry) GetSkillNames() []string {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()

	names := make([]string, 0, len(sr.skills))
	for name, skill := range sr.skills {
		if skill.IsEnabled() {
			names = append(names, name)
		}
	}
	return names
}

