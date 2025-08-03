package config

import (
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Config holds all application configuration
type Config struct {
	AgentName                     string              // Build-time metadata, not configurable via environment
	AgentDescription              string              // Build-time metadata, not configurable via environment
	AgentVersion                  string              // Build-time metadata, not configurable via environment
	AgentURL                      string              `env:"AGENT_URL"`
	AgentCardFilePath             string              `env:"AGENT_CARD_FILE_PATH" description:"Path to JSON file containing static agent card definition"`
	Debug                         bool                `env:"DEBUG,default=false"`
	Timezone                      string              `env:"TIMEZONE,default=UTC" description:"Timezone for timestamps (e.g., UTC, America/New_York, Europe/London)"`
	StreamingStatusUpdateInterval time.Duration       `env:"STREAMING_STATUS_UPDATE_INTERVAL,default=1s"`
	AgentConfig                   AgentConfig         `env:",prefix=AGENT_CLIENT_"`
	CapabilitiesConfig            CapabilitiesConfig  `env:",prefix=CAPABILITIES_"`
	AuthConfig                    AuthConfig          `env:",prefix=AUTH_"`
	QueueConfig                   QueueConfig         `env:",prefix=QUEUE_"`
	TaskRetentionConfig           TaskRetentionConfig `env:",prefix=TASK_RETENTION_"`
	ServerConfig                  ServerConfig        `env:",prefix=SERVER_"`
	TelemetryConfig               TelemetryConfig     `env:",prefix=TELEMETRY_"`
}

// AgentConfig holds agent-specific configuration
type AgentConfig struct {
	Provider                    string            `env:"PROVIDER" description:"LLM provider name"`
	Model                       string            `env:"MODEL" description:"LLM model name"`
	BaseURL                     string            `env:"BASE_URL" description:"Base URL for the LLM provider API"`
	APIKey                      string            `env:"API_KEY" description:"API key for authentication"`
	Timeout                     time.Duration     `env:"TIMEOUT,default=30s" description:"Client timeout for requests"`
	MaxRetries                  int               `env:"MAX_RETRIES,default=3" description:"Maximum number of retries"`
	MaxChatCompletionIterations int               `env:"MAX_CHAT_COMPLETION_ITERATIONS,default=10" description:"Maximum chat completion iterations"`
	CustomHeaders               map[string]string `env:"CUSTOM_HEADERS" description:"Custom headers to include in requests"`
	TLSConfig                   ClientTLSConfig   `env:",prefix=TLS_" description:"TLS configuration for client"`
	ProxyURL                    string            `env:"PROXY_URL" description:"Proxy URL for requests"`
	UserAgent                   string            `env:"USER_AGENT,default=a2a-agent/1.0" description:"User agent string"`
	MaxTokens                   int               `env:"MAX_TOKENS,default=4096" description:"Maximum tokens for completion"`
	Temperature                 float64           `env:"TEMPERATURE,default=0.7" description:"Temperature for completion"`
	TopP                        float64           `env:"TOP_P,default=1.0" description:"Top-p for completion"`
	FrequencyPenalty            float64           `env:"FREQUENCY_PENALTY,default=0.0" description:"Frequency penalty for completion"`
	PresencePenalty             float64           `env:"PRESENCE_PENALTY,default=0.0" description:"Presence penalty for completion"`
	SystemPrompt                string            `env:"SYSTEM_PROMPT,default=You are a helpful AI assistant processing an A2A (Agent-to-Agent) task. Please provide helpful and accurate responses." description:"System prompt for LLM interactions"`
	MaxConversationHistory      int               `env:"MAX_CONVERSATION_HISTORY,default=20" description:"Maximum number of messages to keep in conversation history per context"`
}

// ClientTLSConfig holds TLS configuration for LLM client
type ClientTLSConfig struct {
	InsecureSkipVerify bool   `env:"INSECURE_SKIP_VERIFY,default=false" description:"Skip TLS certificate verification"`
	CertFile           string `env:"CERT_FILE" description:"Client certificate file"`
	KeyFile            string `env:"KEY_FILE" description:"Client private key file"`
	CAFile             string `env:"CA_FILE" description:"Certificate authority file"`
}

// CapabilitiesConfig defines agent capabilities
type CapabilitiesConfig struct {
	Streaming              bool `env:"STREAMING,default=true" description:"Enable streaming support"`
	PushNotifications      bool `env:"PUSH_NOTIFICATIONS,default=true" description:"Enable push notifications"`
	StateTransitionHistory bool `env:"STATE_TRANSITION_HISTORY,default=false" description:"Enable state transition history"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enable   bool   `env:"ENABLE,default=false"`
	CertPath string `env:"CERT_PATH" description:"TLS certificate path"`
	KeyPath  string `env:"KEY_PATH" description:"TLS key path"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enable       bool   `env:"ENABLE,default=false"`
	IssuerURL    string `env:"ISSUER_URL,default=http://keycloak:8080/realms/inference-gateway-realm"`
	ClientID     string `env:"CLIENT_ID,default=inference-gateway-client"`
	ClientSecret string `env:"CLIENT_SECRET"`
}

// QueueConfig holds task queue configuration
type QueueConfig struct {
	MaxSize         int           `env:"MAX_SIZE,default=100"`
	CleanupInterval time.Duration `env:"CLEANUP_INTERVAL,default=30s"`
}

// TaskRetentionConfig defines how many completed and failed tasks to retain
type TaskRetentionConfig struct {
	MaxCompletedTasks int           `env:"MAX_COMPLETED_TASKS,default=100" description:"Maximum number of completed tasks to retain (0 = unlimited)"`
	MaxFailedTasks    int           `env:"MAX_FAILED_TASKS,default=50" description:"Maximum number of failed tasks to retain (0 = unlimited)"`
	CleanupInterval   time.Duration `env:"CLEANUP_INTERVAL,default=5m" description:"How often to run cleanup (0 = manual cleanup only)"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port                  string        `env:"PORT,default=8080" description:"HTTP server port"`
	ReadTimeout           time.Duration `env:"READ_TIMEOUT,default=120s" description:"HTTP server read timeout"`
	WriteTimeout          time.Duration `env:"WRITE_TIMEOUT,default=120s" description:"HTTP server write timeout"`
	IdleTimeout           time.Duration `env:"IDLE_TIMEOUT,default=120s" description:"HTTP server idle timeout"`
	DisableHealthcheckLog bool          `env:"DISABLE_HEALTHCHECK_LOG,default=true" description:"Disable logging for health check requests"`
	TLSConfig             TLSConfig     `env:",prefix=TLS_"`
}

// MetricsConfig holds metrics server configuration
type MetricsConfig struct {
	Port         string        `env:"PORT,default=9090" description:"Metrics server port"`
	Host         string        `env:"HOST,default=" description:"Metrics server host (empty for all interfaces)"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT,default=30s" description:"Metrics server read timeout"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT,default=30s" description:"Metrics server write timeout"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT,default=60s" description:"Metrics server idle timeout"`
}

// TelemetryConfig holds telemetry configuration
type TelemetryConfig struct {
	Enable        bool          `env:"ENABLE,default=false" description:"Enable telemetry collection"`
	MetricsConfig MetricsConfig `env:",prefix=METRICS_"`
}

// Load loads configuration from environment variables, merging with the provided base config.
func Load(ctx context.Context, baseConfig *Config) (*Config, error) {
	return LoadWithLookuper(ctx, baseConfig, envconfig.OsLookuper())
}

// LoadWithLookuper creates and loads configuration using a custom lookuper and merges with user config
func LoadWithLookuper(ctx context.Context, baseConfig *Config, lookuper envconfig.Lookuper) (*Config, error) {
	var cfg Config

	if baseConfig != nil {
		cfg = *baseConfig
	}

	err := envconfig.ProcessWith(ctx, &envconfig.Config{
		Target:   &cfg,
		Lookuper: lookuper,
	})
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// NewWithDefaults creates a new config with defaults applied from struct tags.
func NewWithDefaults(ctx context.Context, baseConfig *Config) (*Config, error) {
	return LoadWithLookuper(ctx, baseConfig, &emptyLookuper{})
}

// emptyLookuper ensures that only default values from struct tags are used
type emptyLookuper struct{}

func (e *emptyLookuper) Lookup(key string) (string, bool) {
	return "", false
}

// Validate validates the configuration and applies corrections for invalid values
func (c *Config) Validate() error {
	if c.AgentConfig.MaxChatCompletionIterations < 1 {
		c.AgentConfig.MaxChatCompletionIterations = 1
	}

	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("invalid timezone '%s': %w", c.Timezone, err)
	}

	return nil
}

// GetTimezone returns the timezone location for timestamps
func (c *Config) GetTimezone() (*time.Location, error) {
	return time.LoadLocation(c.Timezone)
}

// GetCurrentTime returns the current time in the configured timezone
func (c *Config) GetCurrentTime() (time.Time, error) {
	loc, err := c.GetTimezone()
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().In(loc), nil
}
