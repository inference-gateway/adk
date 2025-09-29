package config

import (
	serverConfig "github.com/inference-gateway/adk/server/config"
)

// Config represents the complete configuration for the TLS example server
type Config struct {
	Environment string              `env:"ENVIRONMENT,default=development"`
	A2A         serverConfig.Config `env:",prefix=A2A_"`
}
