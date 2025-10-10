package config

import (
	"github.com/caarlos0/env/v6"
	serverConfig "github.com/inference-gateway/adk/server/config"
	"go.uber.org/zap"
)

// Config holds the complete server configuration
type Config struct {
	Environment string              `env:"ENVIRONMENT,default=development"`
	A2A         serverConfig.Config `env:",prefix=A2A_"`
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		logger, _ := zap.NewDevelopment()
		logger.Fatal("failed to parse config", zap.Error(err))
	}
	return cfg
}