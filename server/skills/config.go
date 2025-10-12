package skills

import (
	"github.com/inference-gateway/adk/server/config"
)

// Re-export configuration types for convenience
type SkillsConfig = config.SkillsConfig
type SafetyConfig = config.SafetyConfig
type SkillConfig = config.SkillConfig
type WriteSkillConfig = config.WriteSkillConfig
type EditSkillConfig = config.EditSkillConfig
type MultiEditConfig = config.MultiEditConfig
type WebSearchConfig = config.WebSearchConfig
type WebFetchConfig = config.WebFetchConfig

// GetDefaultSkillsConfig returns a default skills configuration
func GetDefaultSkillsConfig() *SkillsConfig {
	return config.GetDefaultSkillsConfig()
}
