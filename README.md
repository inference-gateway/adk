<h1 align="center">Application Development Kit (ADK) for A2A-compatible Agents</h1>

<p align="center">
  <strong>Build powerful, interoperable AI agents with the Agent-to-Agent (A2A) protocol</strong>
</p>

<p align="center">
  <!-- CI Status Badge -->
  <a href="https://github.com/inference-gateway/a2a/actions/workflows/ci.yml?query=branch%3Amain">
    <img src="https://github.com/inference-gateway/a2a/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI Status"/>
  </a>
  <!-- Version Badge -->
  <a href="https://github.com/inference-gateway/a2a/releases">
    <img src="https://img.shields.io/github/v/release/inference-gateway/a2a?color=blue&style=flat-square" alt="Version"/>
  </a>
  <!-- License Badge -->
  <a href="https://github.com/inference-gateway/a2a/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/inference-gateway/a2a?color=blue&style=flat-square" alt="License"/>
  </a>
  <!-- Go Version -->
  <img src="https://img.shields.io/github/go-mod/go-version/inference-gateway/a2a?style=flat-square" alt="Go Version"/>
</p>

---

## Overview

The **A2A ADK (Application Development Kit)** is a Go library that simplifies building [Agent-to-Agent (A2A) protocol](https://github.com/inference-gateway/schemas/tree/main/a2a) compatible agents. A2A enables seamless communication between AI agents, allowing them to collaborate, delegate tasks, and share capabilities across different systems and providers.

### What is A2A?

Agent-to-Agent (A2A) is a standardized protocol that enables AI agents to:

- **Communicate** with each other using a unified JSON-RPC interface
- **Delegate tasks** to specialized agents with specific capabilities
- **Stream responses** in real-time for better user experience
- **Authenticate** securely using OIDC/OAuth2
- **Discover capabilities** through standardized agent cards

## 🚀 Quick Start

### Installation

```bash
go get github.com/inference-gateway/a2a
```

### Basic Usage

```go
package main

import (
    "log"

    "github.com/inference-gateway/a2a/adk"
    "github.com/inference-gateway/sdk"
    "go.uber.org/zap"
)

func main() {
    // Initialize logger
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // Configure your agent
    cfg := adk.Config{
        Port: 8080,
        AgentConfig: adk.AgentConfig{
            Name:        "my-agent",
            Description: "A helpful AI agent",
            Version:     "1.0.0",
        },
    }

    // Create SDK client (supports OpenAI, Ollama, Groq, etc.)
    client := sdk.NewClient("your-api-key", "https://api.openai.com/v1")

    // Initialize tools handler
    toolsHandler := adk.NewToolsHandler()
    // Register your custom tools here

    // Create A2A agent
    agent := adk.NewA2AAgent(cfg, logger, client, toolsHandler)

    // Setup and start server
    router := agent.SetupRouter(nil)
    log.Fatal(router.Run(":8080"))
}
```

### Examples

For complete working examples, see the [examples](./examples/) directory:

- **[Server Example](./examples/server/)** - Complete A2A server implementation
- **[Client Example](./examples/client/)** - A2A client implementation (coming soon)

## ✨ Key Features

### Core Capabilities

- 🤖 **A2A Protocol Compliance**: Full implementation of the Agent-to-Agent communication standard
- 🔌 **Multi-Provider Support**: Works with OpenAI, Ollama, Groq, Cohere, and other LLM providers
- 🌊 **Real-time Streaming**: Stream responses as they're generated from language models
- 🔧 **Custom Tools**: Easy integration of custom tools and capabilities
- 🔐 **Secure Authentication**: Built-in OIDC/OAuth2 authentication support

### Developer Experience

- ⚙️ **Environment Configuration**: Simple setup through environment variables
- 📊 **Task Management**: Built-in task queuing, polling, and lifecycle management
- 🏗️ **Extensible Architecture**: Pluggable components for custom business logic
- 📚 **Type-Safe**: Generated types from A2A schema for compile-time safety
- 🧪 **Well Tested**: Comprehensive test coverage with table-driven tests

### Production Ready

- 🌿 **Lightweight**: Optimized binary size
- 🛡️ **Production Hardened**: Configurable timeouts, TLS support, and error handling
- 🐳 **Containerized**: OCI compliant and works with Docker and Docker Compose
- ☸️ **Kubernetes Native**: Ready for cloud-native deployments
- 📊 **Observability**: OpenTelemetry integration for monitoring and tracing

## 🛠️ Development

### Prerequisites

- Go 1.24.3 or later
- [Task](https://taskfile.dev/) for build automation
- [golangci-lint](https://golangci-lint.run/) for linting

### Development Workflow

1. **Download latest A2A schema**:

   ```bash
   task a2a:download-schema
   ```

2. **Generate types from schema**:

   ```bash
   task a2a:generate-types
   ```

3. **Run linting**:

   ```bash
   task lint
   ```

4. **Build the application**:

   ```bash
   task build
   ```

5. **Run tests**:
   ```bash
   task test
   ```

### Available Tasks

| Task                       | Description                       |
| -------------------------- | --------------------------------- |
| `task a2a:download-schema` | Download the latest A2A schema    |
| `task a2a:generate-types`  | Generate Go types from A2A schema |
| `task lint`                | Run static analysis and linting   |
| `task build`               | Build the application binary      |
| `task test`                | Run all tests                     |
| `task tidy`                | Tidy Go modules                   |

## 📖 API Reference

### Core Components

#### A2AAgent

The main agent implementation that handles A2A protocol communication.

```go
type A2AAgent struct {
    // Configuration and dependencies
}

// Create a new agent
func NewA2AAgent(cfg Config, logger *zap.Logger, client sdk.Client, toolsHandler *ToolsHandler) *A2AAgent

// Set custom task result processor
func (agent *A2AAgent) SetTaskResultProcessor(processor TaskResultProcessor)

// Set custom agent info provider
func (agent *A2AAgent) SetAgentInfoProvider(provider AgentInfoProvider)

// Setup HTTP router with A2A endpoints
func (agent *A2AAgent) SetupRouter(oidcAuthenticator OIDCAuthenticator) *gin.Engine
```

#### ToolsHandler

Manages custom tools and capabilities for your agent.

```go
type ToolsHandler struct {
    // Tool definitions and handlers
}

// Create a new tools handler
func NewToolsHandler() *ToolsHandler

// Register a custom tool
func (th *ToolsHandler) RegisterTool(toolDef sdk.ChatCompletionTool, handler ToolCallHandler)

// Get all registered tool definitions
func (th *ToolsHandler) GetAllToolDefinitions() []sdk.ChatCompletionTool
```

### Configuration

```go
type Config struct {
    Port         int                    `env:"PORT,default=8080"`
    AgentConfig  AgentConfig           `env:",prefix=AGENT_"`
    AuthConfig   AuthConfig            `env:",prefix=AUTH_"`
    QueueConfig  QueueConfig           `env:",prefix=QUEUE_"`
}

type AgentConfig struct {
    Name         string `env:"NAME,required"`
    Description  string `env:"DESCRIPTION,required"`
    Version      string `env:"VERSION,default=1.0.0"`
}
```

## 🔧 Advanced Usage

### Custom Tools

Create custom tools to extend your agent's capabilities:

```go
// Define your tool
toolDef := sdk.ChatCompletionTool{
    Type: "function",
    Function: &sdk.FunctionDefinition{
        Name:        "get_weather",
        Description: "Get current weather for a location",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{
                    "type":        "string",
                    "description": "The city and state, e.g. San Francisco, CA",
                },
            },
            "required": []string{"location"},
        },
    },
}

// Implement the tool handler
weatherHandler := func(ctx context.Context, arguments string) (string, error) {
    var params struct {
        Location string `json:"location"`
    }

    if err := json.Unmarshal([]byte(arguments), &params); err != nil {
        return "", err
    }

    // Your weather API logic here
    result := getWeather(params.Location)

    response, _ := json.Marshal(result)
    return string(response), nil
}

// Register the tool
toolsHandler.RegisterTool(toolDef, weatherHandler)
```

### Custom Task Processing

Implement custom business logic for task completion:

```go
type CustomTaskProcessor struct{}

func (ctp *CustomTaskProcessor) ProcessToolResult(toolCallResult string) *a2a.Message {
    // Parse the tool result
    var result map[string]interface{}
    json.Unmarshal([]byte(toolCallResult), &result)

    // Apply your business logic
    if shouldCompleteTask(result) {
        return &a2a.Message{
            Role:    "assistant",
            Content: "Task completed successfully!",
        }
    }

    // Return nil to continue processing
    return nil
}

// Set the processor
agent.SetTaskResultProcessor(&CustomTaskProcessor{})
```

### Agent Metadata

Customize your agent's capabilities and metadata:

```go
type CustomAgentInfo struct{}

func (cai *CustomAgentInfo) GetAgentCard(baseConfig Config) a2a.AgentCard {
    return a2a.AgentCard{
        Name:        baseConfig.AgentConfig.Name,
        Description: baseConfig.AgentConfig.Description,
        Version:     baseConfig.AgentConfig.Version,
        Capabilities: a2a.AgentCapabilities{
            Streaming:         true,
            TaskManagement:    true,
            PushNotifications: false,
        },
        // Add custom metadata
        Metadata: map[string]interface{}{
            "specialization": "weather-analysis",
            "supported_regions": []string{"US", "EU", "APAC"},
        },
    }
}

// Set the provider
agent.SetAgentInfoProvider(&CustomAgentInfo{})
```

## 🌐 A2A Ecosystem

This ADK is part of the broader Inference Gateway ecosystem:

### Related Projects

- **[Inference Gateway](https://github.com/inference-gateway/inference-gateway)** - Unified API gateway for AI providers
- **[Go SDK](https://github.com/inference-gateway/go-sdk)** - Go client library for Inference Gateway
- **[TypeScript SDK](https://github.com/inference-gateway/typescript-sdk)** - TypeScript/JavaScript client library
- **[Python SDK](https://github.com/inference-gateway/python-sdk)** - Python client library
- **[Rust SDK](https://github.com/inference-gateway/rust-sdk)** - Rust client library

### A2A Agents

- **[Awesome A2A](https://github.com/inference-gateway/awesome-a2a)** - Curated list of A2A-compatible agents
- **[Google Calendar Agent](https://github.com/inference-gateway/google-calendar-agent)** - Google Calendar integration agent

## 📋 Requirements

- **Go**: 1.24.3 or later
- **Dependencies**: See [go.mod](./go.mod) for full dependency list

## 🐳 Docker Support

Build and run your agent in a container:

```dockerfile
FROM golang:1.24.3-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o bin/agent .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/bin/agent .
CMD ["./agent"]
```

## 🧪 Testing

The ADK follows table-driven testing patterns and provides utilities for testing A2A agents:

```go
func TestAgentEndpoints(t *testing.T) {
    tests := []struct {
        name           string
        endpoint       string
        method         string
        expectedStatus int
    }{
        {
            name:           "health check",
            endpoint:       "/health",
            method:         "GET",
            expectedStatus: http.StatusOK,
        },
        {
            name:           "agent info",
            endpoint:       "/.well-known/agent.json",
            method:         "GET",
            expectedStatus: http.StatusOK,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## 📄 License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.

## 🤝 Contributing

We welcome contributions! Here's how you can help:

### Getting Started

1. **Fork the repository**
2. **Clone your fork**:
   ```bash
   git clone https://github.com/your-username/a2a.git
   cd a2a
   ```
3. **Create a feature branch**:
   ```bash
   git checkout -b feature/amazing-feature
   ```

### Development Guidelines

- Follow the established code style and conventions
- Write table-driven tests for new functionality
- Use early returns to simplify logic and avoid deep nesting
- Prefer switch statements over if-else chains
- Ensure type safety with proper interfaces
- Use lowercase log messages for consistency

### Before Submitting

1. **Download latest schema**: `task a2a:download-schema`
2. **Generate types**: `task a2a:generate-types`
3. **Run linting**: `task lint`
4. **Build successfully**: `task build`
5. **All tests pass**: `task test`

### Pull Request Process

1. Update documentation for any new features
2. Add tests for new functionality
3. Ensure all CI checks pass
4. Request review from maintainers

For more details, see [CONTRIBUTING.md](./CONTRIBUTING.md).

## 📞 Support

### Issues & Questions

- **Bug Reports**: [GitHub Issues](https://github.com/inference-gateway/a2a/issues)
- **Documentation**: [Official Docs](https://docs.inference-gateway.com)

### Community

- **Discord**: Join our [Discord community](https://discord.gg/inference-gateway)
- **Twitter**: Follow [@InferenceGW](https://twitter.com/InferenceGW) for updates

## 🗺️ Roadmap

- [ ] **Enhanced Tool System**: More built-in tools and better tool chaining
- [ ] **Agent Discovery**: Automatic discovery and registration of agents
- [ ] **Monitoring Dashboard**: Built-in monitoring and analytics
- [ ] **Multi-language SDKs**: Additional language support beyond Go
- [ ] **Performance Optimization**: Further reduce resource consumption
- [ ] **Advanced Authentication**: Support for more auth providers

## 🔗 Resources

### Documentation

- [A2A Protocol Specification](https://github.com/inference-gateway/schemas/tree/main/a2a)
- [API Documentation](https://docs.inference-gateway.com/a2a)
- [Examples Repository](https://github.com/inference-gateway/examples)

---

<p align="center">
  <strong>Built with ❤️ by the Inference Gateway team</strong>
</p>

<p align="center">
  <a href="https://github.com/inference-gateway">GitHub</a> •
  <a href="https://docs.inference-gateway.com">Documentation</a> •
</p>
