package config

import (
	"context"
	"fmt"
	"strings"
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
	ArtifactsConfig               ArtifactsConfig     `env:",prefix=ARTIFACTS_"`
	OTelConfig                    OTelConfig          // Standard OpenTelemetry SDK env vars (OTEL_*), read without a prefix
}

// AgentConfig holds agent-specific configuration
type AgentConfig struct {
	AgentName                   string            `env:"NAME" description:"Name of the agent for identification in callbacks and logging"`
	Provider                    string            `env:"PROVIDER" description:"LLM provider name"`
	Model                       string            `env:"MODEL" description:"LLM model name"`
	BaseURL                     string            `env:"BASE_URL" description:"Base URL for the LLM provider API"`
	APIKey                      string            `env:"API_KEY" description:"API key for authentication"`
	Timeout                     time.Duration     `env:"TIMEOUT,default=30s" description:"Client timeout for requests"`
	MaxRetries                  int               `env:"MAX_RETRIES,default=3" description:"Maximum number of retries"`
	MaxChatCompletionIterations int               `env:"MAX_CHAT_COMPLETION_ITERATIONS,default=50" description:"Maximum chat completion iterations"`
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
	ToolBoxConfig               ToolBoxConfig     `env:",prefix=TOOLS_" description:"Tool configuration for agents"`
	EnableUsageMetadata         bool              `env:"ENABLE_USAGE_METADATA,default=true" description:"Enable usage metadata (token counts and execution stats) in task responses"`
}

// ToolBoxConfig defines configuration options for creating a DefaultToolBox
type ToolBoxConfig struct {
	EnableCreateArtifact bool `env:"CREATE_ARTIFACT,default=false" description:"Enable create_artifact tool for autonomous artifact creation"`
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
	CleanupInterval time.Duration     `env:"CLEANUP_INTERVAL,default=120s"`
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
	TraceConfig   TraceConfig   `env:",prefix=TRACE_"`
	LogConfig     LogConfig     `env:",prefix=LOG_"`
	// AttrSessionIDKey is the span-attribute and baggage-member key used for the
	// session id. Defaults to the OTel semantic-convention key `session.id`.
	AttrSessionIDKey string `env:"ATTR_SESSION_ID_KEY,default=session.id" description:"Span attribute and baggage member key for the session id"`
	// AttrToolCallIDKey is the span-attribute and baggage-member key used for the
	// tool call id. Defaults to the OTel semantic-convention key `gen_ai.tool.call.id`.
	AttrToolCallIDKey string `env:"ATTR_TOOL_CALL_ID_KEY,default=gen_ai.tool.call.id" description:"Span attribute and baggage member key for the tool call id"`
}

// Default attribute/baggage keys following OTel semantic conventions. These
// mirror the `default=` struct tags above so direct struct construction (e.g. in
// tests) still resolves sensible keys.
const (
	DefaultAttrSessionIDKey  = "session.id"
	DefaultAttrToolCallIDKey = "gen_ai.tool.call.id"
)

// SessionIDKey returns the configured session-id attribute/baggage key, falling
// back to the default when unset (e.g. a config built directly, not loaded).
func (t TelemetryConfig) SessionIDKey() string {
	if t.AttrSessionIDKey != "" {
		return t.AttrSessionIDKey
	}
	return DefaultAttrSessionIDKey
}

// ToolCallIDKey returns the configured tool-call-id attribute/baggage key,
// falling back to the default when unset.
func (t TelemetryConfig) ToolCallIDKey() string {
	if t.AttrToolCallIDKey != "" {
		return t.AttrToolCallIDKey
	}
	return DefaultAttrToolCallIDKey
}

// OTelConfig holds the standard OpenTelemetry SDK environment variables. These
// follow the OTel specification naming and, when set, take precedence over the
// deprecated TELEMETRY_* aliases. They are read without a prefix so the standard
// names apply when the ADK config is loaded at the process root.
type OTelConfig struct {
	MetricsExporter      string `env:"OTEL_METRICS_EXPORTER" description:"Metrics exporter to use: prometheus, otlp, or none"`
	TracesExporter       string `env:"OTEL_TRACES_EXPORTER" description:"Traces exporter to use: otlp or none"`
	ExporterOTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" description:"OTLP endpoint base URL shared by traces and metrics"`
	ExporterOTLPProtocol string `env:"OTEL_EXPORTER_OTLP_PROTOCOL" description:"OTLP transport protocol: http/protobuf or grpc"`
	PrometheusHost       string `env:"OTEL_EXPORTER_PROMETHEUS_HOST" description:"Host for the Prometheus pull endpoint"`
	PrometheusPort       string `env:"OTEL_EXPORTER_PROMETHEUS_PORT" description:"Port for the Prometheus pull endpoint"`
}

// Telemetry exporter selection values, following the OpenTelemetry specification.
const (
	MetricsExporterPrometheus = "prometheus"
	ExporterOTLP              = "otlp"
	ExporterNone              = "none"

	OTLPProtocolHTTP = "http/protobuf"
	OTLPProtocolGRPC = "grpc"
)

// ResolvedTelemetry captures the effective telemetry exporter selection after
// merging the standard OTEL_* variables with the deprecated TELEMETRY_* aliases.
// Standard OTEL_* values win when set; otherwise the legacy TELEMETRY_* values
// (and their defaults) are used so existing deployments keep working.
type ResolvedTelemetry struct {
	MetricsExporter string
	TracesExporter  string
	OTLPEndpoint    string
	OTLPProtocol    string
	PrometheusHost  string
	PrometheusPort  string
}

// ResolveTelemetry computes the effective exporter selection, preferring the
// standard OTEL_* env vars and falling back to the deprecated TELEMETRY_* ones.
func (c *Config) ResolveTelemetry() ResolvedTelemetry {
	r := ResolvedTelemetry{
		MetricsExporter: strings.ToLower(strings.TrimSpace(c.OTelConfig.MetricsExporter)),
		TracesExporter:  strings.ToLower(strings.TrimSpace(c.OTelConfig.TracesExporter)),
		OTLPEndpoint:    strings.TrimSpace(c.OTelConfig.ExporterOTLPEndpoint),
		OTLPProtocol:    strings.ToLower(strings.TrimSpace(c.OTelConfig.ExporterOTLPProtocol)),
		PrometheusHost:  c.OTelConfig.PrometheusHost,
		PrometheusPort:  c.OTelConfig.PrometheusPort,
	}

	// Metrics exporter defaults to the legacy pull-based Prometheus behaviour.
	if r.MetricsExporter == "" {
		r.MetricsExporter = MetricsExporterPrometheus
	}

	// Traces exporter falls back to the deprecated TELEMETRY_TRACE_ENABLE flag.
	if r.TracesExporter == "" {
		if c.TelemetryConfig.TraceConfig.Enable {
			r.TracesExporter = ExporterOTLP
		} else {
			r.TracesExporter = ExporterNone
		}
	}

	// OTLP endpoint falls back to the deprecated TELEMETRY_TRACE_ENDPOINT.
	if r.OTLPEndpoint == "" {
		r.OTLPEndpoint = c.TelemetryConfig.TraceConfig.Endpoint
	}

	// OTLP protocol defaults to HTTP/protobuf per the OTel specification.
	if r.OTLPProtocol == "" {
		r.OTLPProtocol = OTLPProtocolHTTP
	}

	// Prometheus host/port fall back to the deprecated TELEMETRY_METRICS_* values.
	if r.PrometheusHost == "" {
		r.PrometheusHost = c.TelemetryConfig.MetricsConfig.Host
	}
	if r.PrometheusPort == "" {
		r.PrometheusPort = c.TelemetryConfig.MetricsConfig.Port
	}

	return r
}

// TraceConfig holds OTLP trace exporter configuration
type TraceConfig struct {
	Enable   bool              `env:"ENABLE,default=false" description:"Enable OTLP trace export"`
	Endpoint string            `env:"ENDPOINT,default=http://localhost:4318" description:"OTLP trace endpoint URL"`
	Headers  map[string]string `env:"HEADERS" description:"Custom headers for OTLP trace export"`
}

// LogConfig holds OTLP log exporter configuration.
// Reserved for future use - the OTLP log exporter is not yet wired.
type LogConfig struct {
	Enable   bool              `env:"ENABLE,default=false" description:"Enable OTLP log export (reserved, not yet wired)"`
	Endpoint string            `env:"ENDPOINT,default=http://localhost:4318" description:"OTLP log endpoint URL (reserved, not yet wired)"`
	Headers  map[string]string `env:"HEADERS" description:"Custom headers for OTLP log export (reserved, not yet wired)"`
}

// ArtifactsConfig holds artifacts server configuration
type ArtifactsConfig struct {
	Enable          bool                    `env:"ENABLE,default=false" description:"Enable artifacts server"`
	ServerConfig    ArtifactsServerConfig   `env:",prefix=SERVER_" description:"HTTP server configuration for artifacts server"`
	StorageConfig   ArtifactsStorageConfig  `env:",prefix=STORAGE_" description:"Storage configuration for artifacts"`
	RetentionConfig ArtifactRetentionConfig `env:",prefix=RETENTION_" description:"Artifact retention and cleanup configuration"`
}

// ArtifactsServerConfig holds artifacts HTTP server configuration
type ArtifactsServerConfig struct {
	Host         string        `env:"HOST,default=localhost" description:"Artifacts server host"`
	Port         string        `env:"PORT,default=8081" description:"Artifacts server port"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT,default=30s" description:"Artifacts server read timeout"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT,default=30s" description:"Artifacts server write timeout"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT,default=60s" description:"Artifacts server idle timeout"`
	TLSConfig    TLSConfig     `env:",prefix=TLS_" description:"TLS configuration for artifacts server"`
}

// ArtifactsStorageConfig holds storage configuration for artifacts
type ArtifactsStorageConfig struct {
	Provider    string            `env:"PROVIDER,default=filesystem" description:"Storage provider (filesystem, minio, s3, gcs)"`
	BasePath    string            `env:"BASE_PATH,default=./artifacts" description:"Base path for filesystem storage"`
	BaseURL     string            `env:"BASE_URL" description:"Base URL for accessing artifacts (e.g., https://api.example.com). If not set, will be auto-generated from server config"`
	Endpoint    string            `env:"ENDPOINT" description:"Storage endpoint URL (for MinIO, S3, etc.)"`
	AccessKey   string            `env:"ACCESS_KEY" description:"Storage access key"`
	SecretKey   string            `env:"SECRET_KEY" description:"Storage secret key"`
	BucketName  string            `env:"BUCKET_NAME,default=artifacts" description:"Storage bucket name"`
	Region      string            `env:"REGION,default=us-east-1" description:"Storage region"`
	UseSSL      bool              `env:"USE_SSL,default=true" description:"Use SSL for storage connections"`
	Credentials map[string]string `env:"CREDENTIALS" description:"Additional provider-specific credentials"`
}

// ArtifactRetentionConfig defines artifact cleanup policies
type ArtifactRetentionConfig struct {
	MaxArtifacts    int           `env:"MAX_ARTIFACTS,default=5" description:"Maximum artifacts to retain per task (0 = unlimited)"`
	MaxAge          time.Duration `env:"MAX_AGE,default=168h" description:"Maximum age for artifacts (0 = no age limit)"`
	CleanupInterval time.Duration `env:"CLEANUP_INTERVAL,default=24h" description:"How often to run cleanup (0 = manual cleanup only)"`
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
