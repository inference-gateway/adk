package server

import (
	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/server/skills"
	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

// SkillsAgentBuilder extends AgentBuilder to support built-in skills
type SkillsAgentBuilder interface {
	AgentBuilder
	// WithSkills enables built-in skills with the provided configuration
	WithSkills(skillsConfig *config.SkillsConfig) SkillsAgentBuilder
	// WithSkillsEnabled enables built-in skills with default configuration
	WithSkillsEnabled() SkillsAgentBuilder
	// GetSkillsForAgentCard returns skills information for AgentCard
	GetSkillsForAgentCard() []types.AgentSkill
	// BuildWithSkills creates an agent with skills and returns the enhanced toolbox
	BuildWithSkills() (*OpenAICompatibleAgentImpl, ToolBox, error)
}

// skillsAgentBuilder implements SkillsAgentBuilder
type skillsAgentBuilder struct {
	AgentBuilder
	logger        *zap.Logger
	skillsConfig  *config.SkillsConfig
	skillsRegistry *skills.SkillsRegistry
}

// NewSkillsAgentBuilder creates a new agent builder with skills support
func NewSkillsAgentBuilder(logger *zap.Logger) SkillsAgentBuilder {
	return &skillsAgentBuilder{
		AgentBuilder: NewAgentBuilder(logger),
		logger:       logger,
	}
}

// WithSkills enables built-in skills with the provided configuration
func (sab *skillsAgentBuilder) WithSkills(skillsConfig *config.SkillsConfig) SkillsAgentBuilder {
	sab.skillsConfig = skillsConfig
	if skillsConfig.Enabled {
		sab.skillsRegistry = skills.NewSkillsRegistry(sab.logger, skillsConfig)
	}
	return sab
}

// WithSkillsEnabled enables built-in skills with default configuration
func (sab *skillsAgentBuilder) WithSkillsEnabled() SkillsAgentBuilder {
	defaultConfig := config.GetDefaultSkillsConfig()
	defaultConfig.Enabled = true
	
	// Enable all skills with default settings
	defaultConfig.ReadSkill.Enabled = true
	defaultConfig.WriteSkill.Enabled = true
	defaultConfig.EditSkill.Enabled = true
	defaultConfig.MultiEdit.Enabled = true
	defaultConfig.WebSearch.Enabled = true
	defaultConfig.WebFetch.Enabled = true
	
	return sab.WithSkills(defaultConfig)
}

// GetSkillsForAgentCard returns skills information for AgentCard
func (sab *skillsAgentBuilder) GetSkillsForAgentCard() []types.AgentSkill {
	if sab.skillsRegistry == nil {
		return []types.AgentSkill{}
	}

	skillsInfo := sab.skillsRegistry.GetSkillsForAgentCard()
	agentSkills := make([]types.AgentSkill, 0, len(skillsInfo))

	for _, skillInfo := range skillsInfo {
		skill := types.AgentSkill{
			ID:          skillInfo["id"].(string),
			Name:        skillInfo["name"].(string),
			Description: skillInfo["description"].(string),
			Tags:        skillInfo["tags"].([]string),
		}

		if inputModes, ok := skillInfo["inputModes"].([]string); ok {
			skill.InputModes = inputModes
		}

		if outputModes, ok := skillInfo["outputModes"].([]string); ok {
			skill.OutputModes = outputModes
		}

		if examples, ok := skillInfo["examples"].([]string); ok {
			skill.Examples = examples
		}

		agentSkills = append(agentSkills, skill)
	}

	return agentSkills
}

// BuildWithSkills creates an agent with skills and returns the enhanced toolbox
func (sab *skillsAgentBuilder) BuildWithSkills() (*OpenAICompatibleAgentImpl, ToolBox, error) {
	var toolBox ToolBox

	// Create enhanced toolbox with skills if enabled
	if sab.skillsRegistry != nil {
		toolBox = NewSkillsToolBox(sab.skillsRegistry)
	} else {
		toolBox = NewDefaultToolBox()
	}

	// Set the toolbox in the agent builder
	sab.AgentBuilder = sab.WithToolBox(toolBox)

	// Build the agent
	agent, err := sab.AgentBuilder.Build()
	if err != nil {
		return nil, nil, err
	}

	return agent, toolBox, nil
}

// Override Build to include skills by default
func (sab *skillsAgentBuilder) Build() (*OpenAICompatibleAgentImpl, error) {
	agent, _, err := sab.BuildWithSkills()
	return agent, err
}

// Enhanced server builder functions

// CreateAgentWithSkills creates an agent with skills using the provided configuration
func CreateAgentWithSkills(logger *zap.Logger, agentConfig *config.AgentConfig, skillsConfig *config.SkillsConfig) (*OpenAICompatibleAgentImpl, ToolBox, []types.AgentSkill, error) {
	// Create LLM client
	llmClient, err := NewOpenAICompatibleLLMClient(agentConfig, logger)
	if err != nil {
		return nil, nil, nil, err
	}

	// Create skills agent builder
	skillsBuilder := NewSkillsAgentBuilder(logger)
	
	// Configure the base agent builder
	if impl, ok := skillsBuilder.(*skillsAgentBuilder); ok {
		impl.AgentBuilder = impl.AgentBuilder.WithConfig(agentConfig).WithLLMClient(llmClient)
	}

	// Add skills if enabled
	if skillsConfig.Enabled {
		skillsBuilder = skillsBuilder.WithSkills(skillsConfig)
	}

	// Build agent with skills
	agent, toolBox, err := skillsBuilder.BuildWithSkills()
	if err != nil {
		return nil, nil, nil, err
	}

	// Get skills for agent card
	agentSkills := skillsBuilder.GetSkillsForAgentCard()

	return agent, toolBox, agentSkills, nil
}

// CreateServerWithSkills creates an A2A server with skills support
func CreateServerWithSkills(cfg config.Config, logger *zap.Logger) (A2AServer, error) {
	// Create agent with skills
	agent, _, agentSkills, err := CreateAgentWithSkills(logger, &cfg.AgentConfig, &cfg.SkillsConfig)
	if err != nil {
		return nil, err
	}

	// Create agent card with skills
	agentCard := types.AgentCard{
		Name:            cfg.AgentName,
		Description:     cfg.AgentDescription,
		Version:         cfg.AgentVersion,
		URL:             cfg.AgentURL,
		ProtocolVersion: "0.3.0",
		Capabilities: types.AgentCapabilities{
			Streaming:              &cfg.CapabilitiesConfig.Streaming,
			PushNotifications:      &cfg.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: &cfg.CapabilitiesConfig.StateTransitionHistory,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills:             agentSkills,
	}

	// Build server with agent and agent card
	return NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithAgentCard(agentCard).
		WithDefaultTaskHandlers().
		Build()
}