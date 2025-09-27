package config

import (
	serverConfig "github.com/inference-gateway/adk/server/config"
)

type Config struct {
	Environment string    `env:"ENVIRONMENT,default=development"`
	A2A         A2AConfig `env:",prefix=A2A_"`
}

type A2AConfig struct {
	serverConfig.Config
	AgentCardFile string `env:"AGENT_CARD_FILE,default=agent-card.json"`
}
