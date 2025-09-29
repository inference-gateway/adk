package config

import (
	serverConfig "github.com/inference-gateway/adk/server/config"
)

// Config holds the configuration for the in-memory queue storage example server
type Config struct {
	// Environment determines runtime environment (development, production, etc.)
	Environment string `env:"ENVIRONMENT,default=development"`

	// A2A contains all A2A server configuration
	// This is prefixed with A2A_ in environment variables
	A2A serverConfig.Config `env:",prefix=A2A_"`
}
