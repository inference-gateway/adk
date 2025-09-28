package config

// Config represents the complete configuration for the TLS example client
type Config struct {
	Environment string `env:"ENVIRONMENT,default=development"`
	ServerURL   string `env:"A2A_SERVER_URL,default=https://localhost:8443"`
}