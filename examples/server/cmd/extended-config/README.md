# Extended Configuration Example

This example demonstrates how to extend the A2A server configuration with custom application-specific settings while maintaining full compatibility with the base A2A configuration system.

## Features Demonstrated

- **Configuration Extension**: Shows how to embed the base `config.Config` struct and add custom fields
- **Environment Variable Support**: All custom fields support environment variable configuration with defaults
- **Custom Validation**: Implements custom validation logic for extended configuration fields
- **Nested Configuration**: Demonstrates nested configuration structures with prefixed environment variables
- **Configuration Access**: Shows how to access both base and custom configuration in task handlers

## Custom Configuration Structure

The example defines a `CustomAppConfig` struct that:

```go
type CustomAppConfig struct {
    config.Config                    // Embed base A2A configuration
    DatabaseURL       string         `env:"DATABASE_URL"`
    RedisURL          string         `env:"REDIS_URL"`
    AppName           string         `env:"APP_NAME,default=ExtendedConfigExample"`
    MaxConnections    int            `env:"MAX_CONNECTIONS,default=100"`
    EnableRateLimiter bool           `env:"ENABLE_RATE_LIMITER,default=true"`
    FeatureFlags      CustomFeatureFlags `env:",prefix=FEATURE_"`
}
```

## Environment Variables

### Base A2A Configuration
All standard A2A environment variables are supported:
- `DEBUG=true` - Enable debug mode
- `SERVER_PORT=8080` - HTTP server port
- `AGENT_CLIENT_PROVIDER=openai` - LLM provider
- `AGENT_CLIENT_MODEL=gpt-4` - LLM model
- And all other base configuration options...

### Custom Application Configuration
- `DATABASE_URL` - PostgreSQL database connection URL
- `REDIS_URL` - Redis cache connection URL
- `APP_NAME` - Application name (default: "ExtendedConfigExample")
- `MAX_CONNECTIONS` - Maximum database connections (default: 100)
- `ENABLE_RATE_LIMITER` - Enable API rate limiting (default: true)

### Feature Flags (with FEATURE_ prefix)
- `FEATURE_ENABLE_NEW_UI=true` - Enable new UI experience
- `FEATURE_ENABLE_ADVANCED_AUTH=true` - Enable advanced authentication
- `FEATURE_MAX_FILE_SIZE=20971520` - Maximum file upload size in bytes
- `FEATURE_TEMP_DIRECTORY=/tmp/myapp` - Temporary directory

## Running the Example

### Basic Run
```bash
go run main.go
```

### With Custom Configuration
```bash
export DATABASE_URL="postgresql://localhost/myapp"
export REDIS_URL="redis://localhost:6379"
export APP_NAME="MyCustomApp"
export MAX_CONNECTIONS="50"
export FEATURE_ENABLE_NEW_UI="true"
export FEATURE_ENABLE_ADVANCED_AUTH="true"
export DEBUG="true"
export SERVER_PORT="9090"
go run main.go
```

### With Docker Environment File
```bash
# Create .env file
cat > .env << EOF
DATABASE_URL=postgresql://localhost/myapp
REDIS_URL=redis://localhost:6379
APP_NAME=MyCustomApp
MAX_CONNECTIONS=50
FEATURE_ENABLE_NEW_UI=true
DEBUG=true
SERVER_PORT=9090
EOF

# Load environment and run
set -a && source .env && set +a
go run main.go
```

## Key Implementation Details

### 1. Configuration Loading
```go
appConfig := &CustomAppConfig{}
if err := config.LoadExtended(ctx, appConfig); err != nil {
    log.Fatal("failed to load configuration:", err)
}
```

### 2. Custom Validation
```go
func (c *CustomAppConfig) Validate() error {
    if c.MaxConnections < 1 {
        return fmt.Errorf("MAX_CONNECTIONS must be at least 1")
    }
    return nil
}
```

### 3. Base Configuration Access
```go
func (c *CustomAppConfig) GetBaseConfig() *config.Config {
    return &c.Config
}
```

### 4. Using Configuration in Task Handlers
```go
func (h *CustomTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
    // Access custom configuration
    responseText := fmt.Sprintf("Hello from %s!", h.appConfig.AppName)
    
    if h.appConfig.FeatureFlags.EnableNewUI {
        responseText += " [New UI Enabled]"
    }
    
    // Use configuration for business logic
    // ...
}
```

## Configuration Patterns

### 1. Simple Embedding (Recommended)
```go
type MyConfig struct {
    config.Config
    MyField string `env:"MY_FIELD"`
}
```

### 2. Interface-based Approach
```go
type MyConfig struct {
    BaseConfig *config.Config
    MyField    string `env:"MY_FIELD"`
}

func (c *MyConfig) GetBaseConfig() *config.Config {
    return c.BaseConfig
}
```

### 3. Named Field Approach
```go
type MyConfig struct {
    A2AConfig config.Config
    MyField   string `env:"MY_FIELD"`
}

func (c *MyConfig) GetConfig() *config.Config {
    return &c.A2AConfig
}
```

## Testing Your Extended Configuration

The new configuration system provides testing utilities:

```go
func TestMyConfig(t *testing.T) {
    cfg := &MyConfig{}
    err := config.LoadExtendedWithDefaults(context.Background(), cfg)
    assert.NoError(t, err)
    
    // Test that base config is accessible
    baseConfig, err := config.ExtractBaseConfig(cfg)
    assert.NoError(t, err)
    assert.NotNil(t, baseConfig)
}
```

## Migration from Old Configuration

If you have existing configuration code, you can migrate gradually:

### Before (Old Way)
```go
cfg := config.Config{
    AgentName: "my-agent",
    Debug: true,
}
// Manual environment processing
envconfig.Process(ctx, &cfg)
```

### After (New Way)
```go
type MyConfig struct {
    config.Config
    CustomField string `env:"CUSTOM_FIELD"`
}

cfg := &MyConfig{}
config.LoadExtended(ctx, cfg)
```

The new approach provides better structure, validation, and maintainability while remaining fully compatible with existing A2A server builders.