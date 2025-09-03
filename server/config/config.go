package config

import (
	"context"
	"fmt"
	"reflect"
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
	Provider        string            `env:"PROVIDER,default=memory" description:"Message broker provider (memory, redis, sqs, pubsub)"`
	URL             string            `env:"URL" description:"Connection URL for the message broker"`
	MaxSize         int               `env:"MAX_SIZE,default=100"`
	CleanupInterval time.Duration     `env:"CLEANUP_INTERVAL,default=30s"`
	Credentials     map[string]string `env:"CREDENTIALS" description:"Broker-specific credentials"`
	Options         map[string]string `env:"OPTIONS" description:"Broker-specific configuration options"`
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

// ExtendableConfig provides a pattern for clients to extend A2A server configuration
// with their custom configuration structs.
//
// Example:
//   type MyConfig struct {
//     config.Config         // Embed the base A2A config
//     MyCustomField string `env:"MY_CUSTOM_FIELD"`
//   }
//
//   cfg, err := config.LoadExtended(ctx, &MyConfig{})
type ExtendableConfig interface {
	// GetBaseConfig returns the embedded base configuration
	GetBaseConfig() *Config
	// Validate allows custom validation of the extended configuration
	Validate() error
}

// Configurable interface for structs that embed Config
type Configurable interface {
	GetConfig() *Config
}

// LoadExtended loads configuration with support for extended/custom configuration structs.
// This function allows clients to define their own configuration structs that embed
// the base Config struct and add additional fields.
//
// The target must be a pointer to a struct that embeds Config either directly or
// provides a way to access it via GetBaseConfig() or GetConfig() methods.
//
// Example usage:
//   type MyAppConfig struct {
//     config.Config
//     DatabaseURL string `env:"DATABASE_URL"`
//     RedisURL    string `env:"REDIS_URL"`
//   }
//
//   cfg, err := config.LoadExtended(ctx, &MyAppConfig{})
func LoadExtended(ctx context.Context, target any) error {
	return LoadExtendedWithLookuper(ctx, target, envconfig.OsLookuper())
}

// LoadExtendedWithLookuper loads configuration with custom lookuper and support for extended configuration structs
func LoadExtendedWithLookuper(ctx context.Context, target any, lookuper envconfig.Lookuper) error {
	if target == nil {
		return fmt.Errorf("target cannot be nil")
	}

	// Process the extended configuration struct with environment variables
	err := envconfig.ProcessWith(ctx, &envconfig.Config{
		Target:   target,
		Lookuper: lookuper,
	})
	if err != nil {
		return fmt.Errorf("failed to process environment configuration: %w", err)
	}

	// Find and validate the base Config
	baseConfig, err := ExtractBaseConfig(target)
	if err != nil {
		return fmt.Errorf("failed to extract base config: %w", err)
	}

	if err := baseConfig.Validate(); err != nil {
		return fmt.Errorf("base config validation failed: %w", err)
	}

	// Validate extended configuration if it implements the interface
	if validator, ok := target.(ExtendableConfig); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("extended config validation failed: %w", err)
		}
	}

	return nil
}

// LoadExtendedWithDefaults creates extended configuration with defaults applied from struct tags
func LoadExtendedWithDefaults(ctx context.Context, target any) error {
	return LoadExtendedWithLookuper(ctx, target, &emptyLookuper{})
}

// MergeConfigs merges a base Config with an extended configuration struct.
// This is useful for combining programmatic configuration with environment-based configuration.
func MergeConfigs(ctx context.Context, base *Config, target any) error {
	if base == nil {
		return LoadExtended(ctx, target)
	}

	// First, set the base config in the target
	if err := setBaseConfig(target, base); err != nil {
		return fmt.Errorf("failed to set base config: %w", err)
	}

	return nil
}

// MergeConfigsWithEnvironment merges a base Config with an extended configuration struct and applies environment variables.
// This is useful for combining programmatic configuration with environment-based configuration.
//
// Note: Due to the behavior of the underlying envconfig library, environment variables will only override 
// fields that are zero-valued in the base config. Non-zero values in the base config will not be overridden
// by environment variables. For most use cases, use LoadExtended directly instead of this function.
func MergeConfigsWithEnvironment(ctx context.Context, base *Config, target any) error {
	if base == nil {
		return LoadExtended(ctx, target)
	}

	// First, set the base config in the target
	if err := setBaseConfig(target, base); err != nil {
		return fmt.Errorf("failed to set base config: %w", err)
	}

	// Then load environment variables on top
	// Note: Environment variables will only override zero-valued fields
	return LoadExtended(ctx, target)
}

// ExtractBaseConfig extracts the base Config from various target types
func ExtractBaseConfig(target any) (*Config, error) {
	if target == nil {
		return nil, fmt.Errorf("target is nil")
	}

	val := reflect.ValueOf(target)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("target must be a struct or pointer to struct")
	}

	// Method 1: Check if it implements ExtendableConfig
	if extendable, ok := target.(ExtendableConfig); ok {
		return extendable.GetBaseConfig(), nil
	}

	// Method 2: Check if it implements Configurable
	if configurable, ok := target.(Configurable); ok {
		return configurable.GetConfig(), nil
	}

	// Method 3: Look for embedded Config field
	configField := val.FieldByName("Config")
	if configField.IsValid() && configField.Type() == reflect.TypeOf(Config{}) {
		return configField.Addr().Interface().(*Config), nil
	}

	// Method 4: Look for any field of type Config
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)
		
		if field.Type() == reflect.TypeOf(Config{}) {
			return field.Addr().Interface().(*Config), nil
		}

		// Check for pointer to Config
		if field.Type() == reflect.TypeOf((*Config)(nil)) && !field.IsNil() {
			return field.Interface().(*Config), nil
		}

		// Check embedded structs recursively
		if fieldType.Anonymous && field.Kind() == reflect.Struct {
			if config, err := ExtractBaseConfig(field.Addr().Interface()); err == nil {
				return config, nil
			}
		}
	}

	return nil, fmt.Errorf("no Config field found in target struct")
}

// setBaseConfig sets the base Config in the target struct
func setBaseConfig(target any, base *Config) error {
	if target == nil || base == nil {
		return fmt.Errorf("target and base cannot be nil")
	}

	val := reflect.ValueOf(target)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a struct or pointer to struct")
	}

	// Look for Config field and set it
	configField := val.FieldByName("Config")
	if configField.IsValid() && configField.CanSet() && configField.Type() == reflect.TypeOf(Config{}) {
		configField.Set(reflect.ValueOf(*base))
		return nil
	}

	// Look for any field of type Config
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		
		if field.Type() == reflect.TypeOf(Config{}) && field.CanSet() {
			field.Set(reflect.ValueOf(*base))
			return nil
		}

		// Check for pointer to Config
		if field.Type() == reflect.TypeOf((*Config)(nil)) && field.CanSet() {
			field.Set(reflect.ValueOf(base))
			return nil
		}
	}

	return fmt.Errorf("no settable Config field found in target struct")
}
