<h1 align="center">Agent Development Kit (ADK)</h1>

<p align="center">
  <strong>Build powerful, interoperable AI agents with the Agent-to-Agent (A2A) protocol</strong>
</p>

> ⚠️ **Early Stage Warning**: This project is in its early stages of development. Breaking changes are expected as we iterate and improve the API. Please use pinned versions in production environments and be prepared to update your code when upgrading versions.

<p align="center">
  <!-- CI Status Badge -->
  <a href="https://github.com/inference-gateway/adk/actions/workflows/ci.yml?query=branch%3Amain">
    <img src="https://github.com/inference-gateway/adk/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI Status"/>
  </a>
  <!-- Version Badge -->
  <a href="https://github.com/inference-gateway/adk/releases">
    <img src="https://img.shields.io/github/v/release/inference-gateway/adk?color=blue&style=flat-square" alt="Version"/>
  </a>
  <!-- License Badge -->
  <a href="https://github.com/inference-gateway/adk/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/inference-gateway/adk?color=blue&style=flat-square" alt="License"/>
  </a>
  <!-- Go Version -->
  <img src="https://img.shields.io/github/go-mod/go-version/inference-gateway/adk?style=flat-square" alt="Go Version"/>
</p>

---

## Table of Contents

- [Overview](#overview)
  - [What is A2A?](#what-is-a2a)
- [🚀 Quick Start](#-quick-start)
  - [Installation](#installation)
  - [Basic Usage (Minimal Server)](#basic-usage-minimal-server)
  - [AI-Powered Server](#ai-powered-server)
  - [Health Check Example](#health-check-example)
  - [Examples](#examples)
- [✨ Key Features](#-key-features)
  - [Core Capabilities](#core-capabilities)
  - [Developer Experience](#developer-experience)
  - [Production Ready](#production-ready)
- [🛠️ Development](#️-development)
  - [Prerequisites](#prerequisites)
  - [Development Workflow](#development-workflow)
  - [Available Tasks](#available-tasks)
  - [Build-Time Agent Metadata](#build-time-agent-metadata)
- [📖 API Reference](#-api-reference)
  - [Core Components](#core-components)
    - [A2AServer](#a2aserver)
    - [A2AServerBuilder](#a2aserverbuilder)
    - [AgentBuilder](#agentbuilder)
    - [A2AClient](#a2aclient)
    - [Agent Health Monitoring](#agent-health-monitoring)
    - [LLM Client](#llm-client)
  - [Configuration](#configuration)
- [🔧 Advanced Usage](#-advanced-usage)
  - [Building Custom Agents with AgentBuilder](#building-custom-agents-with-agentbuilder)
  - [Custom Tools](#custom-tools)
  - [Custom Task Processing](#custom-task-processing)
  - [Push Notifications](#push-notifications)
  - [Agent Metadata](#agent-metadata)
  - [Environment Configuration](#environment-configuration)
- [🌐 A2A Ecosystem](#-a2a-ecosystem)
  - [Related Projects](#related-projects)
  - [A2A Agents](#a2a-agents)
- [📋 Requirements](#-requirements)
- [🐳 Docker Support](#-docker-support)
- [🧪 Testing](#-testing)
- [📄 License](#-license)
- [🤝 Contributing](#-contributing)
  - [Getting Started](#getting-started)
  - [Development Guidelines](#development-guidelines)
  - [Before Submitting](#before-submitting)
  - [Pull Request Process](#pull-request-process)
- [📞 Support](#-support)
  - [Issues & Questions](#issues--questions)
- [🔗 Resources](#-resources)
  - [Documentation](#documentation)

---

## Overview

The **A2A ADK (Agent Development Kit)** is a Go library that simplifies building [Agent-to-Agent (A2A) protocol](https://github.com/inference-gateway/schemas/tree/main/a2a) compatible agents. A2A enables seamless communication between AI agents, allowing them to collaborate, delegate tasks, and share capabilities across different systems and providers.

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
go get github.com/inference-gateway/adk
```

### Basic Usage (Minimal Server)

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/inference-gateway/adk/server"
    "go.uber.org/zap"
)

func main() {
    // Initialize logger
    logger, _ := zap.NewDevelopment()
    defer logger.Sync()

    // Create the simplest A2A server
    a2aServer := server.NewDefaultA2AServer(nil)

    // Start server
    ctx := context.Background()
    go func() {
        if err := a2aServer.Start(ctx); err != nil {
            log.Fatal("Server failed to start:", err)
        }
    }()

    logger.Info("Server running on port 8080")

    // Wait for shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down server...")
    a2aServer.Stop(ctx)
}
```

### AI-Powered Server

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/inference-gateway/adk/server"
    "github.com/inference-gateway/adk/server/config"
    "github.com/sethvargo/go-envconfig"
    "go.uber.org/zap"
)

func main() {
    // Initialize logger
    logger, _ := zap.NewDevelopment()
    defer logger.Sync()

    // Load configuration from environment
    cfg := config.Config{
        Port: "8080",
    }

    ctx := context.Background()
    if err := envconfig.Process(ctx, &cfg); err != nil {
        log.Fatal("Failed to load config:", err)
    }

    // Create toolbox with custom tools
    toolBox := server.NewDefaultToolBox()

    // Add a weather tool
    weatherTool := server.NewBasicTool(
        "get_weather",
        "Get weather information",
        map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{
                    "type": "string",
                    "description": "City name",
                },
            },
            "required": []string{"location"},
        },
        func(ctx context.Context, args map[string]interface{}) (string, error) {
            location := args["location"].(string)
            return fmt.Sprintf(`{"location": "%s", "temperature": "22°C"}`, location), nil
        },
    )
    toolBox.AddTool(weatherTool)

    // Create LLM client (requires AGENT_CLIENT_API_KEY environment variable)
    var a2aServer server.A2AServer
    var err error
    if cfg.AgentConfig != nil && cfg.AgentConfig.APIKey != "" {
        // Modern approach using AgentBuilder
        agent, err := server.NewAgentBuilder(logger).
            WithConfig(cfg.AgentConfig).
            WithToolBox(toolBox).
            Build()
        if err != nil {
            log.Fatal("Failed to create agent:", err)
        }
        
        a2aServer, err = server.NewA2AServerBuilder(cfg, logger).
            WithAgent(agent).
            WithAgentCardFromFile(".well-known/agent.json").
            Build()
        if err != nil {
            log.Fatal("Failed to create server:", err)
        }
    } else {
        // Mock mode without actual LLM
        agent, err := server.NewAgentBuilder(logger).
            WithToolBox(toolBox).
            Build()
        if err != nil {
            log.Fatal("Failed to create agent:", err)
        }
        
        a2aServer, err = server.NewA2AServerBuilder(cfg, logger).
            WithAgent(agent).
            WithAgentCardFromFile(".well-known/agent.json").
            Build()
        if err != nil {
            log.Fatal("Failed to create server:", err)
        }
    }

    // Start server
    go func() {
        if err := a2aServer.Start(ctx); err != nil {
            log.Fatal("Server failed to start:", err)
        }
    }()

    logger.Info("AI-powered A2A server running", zap.String("port", cfg.ServerConfig.Port))

    // Wait for shutdown signal
    select {}
}
```

### Health Check Example

Monitor the health status of A2A agents for service discovery and load balancing:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/inference-gateway/adk/client"
)

func main() {
    // Create client
    client := client.NewClient("http://localhost:8080")

    // Monitor agent health
    ctx := context.Background()
    
    // Single health check
    health, err := client.GetHealth(ctx)
    if err != nil {
        log.Printf("Health check failed: %v", err)
        return
    }
    
    fmt.Printf("Agent health: %s\n", health.Status)
    
    // Periodic health monitoring
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            health, err := client.GetHealth(ctx)
            if err != nil {
                log.Printf("Health check failed: %v", err)
                continue
            }
            
            switch health.Status {
            case "healthy":
                fmt.Printf("[%s] Agent is healthy\n", time.Now().Format("15:04:05"))
            case "degraded":
                fmt.Printf("[%s] Agent is degraded - some functionality may be limited\n", time.Now().Format("15:04:05"))
            case "unhealthy":
                fmt.Printf("[%s] Agent is unhealthy - may not be able to process requests\n", time.Now().Format("15:04:05"))
            default:
                fmt.Printf("[%s] Unknown health status: %s\n", time.Now().Format("15:04:05"), health.Status)
            }
        }
    }
}
```

### Examples

For complete working examples, see the [examples](./examples/) directory:

- **[Minimal Server](./examples/server/cmd/minimal/)** - Basic A2A server without AI capabilities
- **[AI-Powered Server](./examples/server/cmd/aipowered/)** - Full A2A server with LLM integration
- **[JSON AgentCard Server](./examples/server/cmd/json-agentcard/)** - A2A server with agent metadata loaded from JSON file
- **[Client Example](./examples/client/)** - A2A client implementation
- **[Paused Task Example](./examples/client/cmd/pausedtask/)** - Handle input-required task pausing
- **[Health Check Example](#health-check-example)** - Monitor agent health status

## ✨ Key Features

### Core Capabilities

- 🤖 **A2A Protocol Compliance**: Full implementation of the Agent-to-Agent communication standard
- 🔌 **Multi-Provider Support**: Works with OpenAI, Ollama, Groq, Cohere, and other LLM providers
- 🌊 **Real-time Streaming**: Stream responses as they're generated from language models
- 🔧 **Custom Tools**: Easy integration of custom tools and capabilities
- 🔐 **Secure Authentication**: Built-in OIDC/OAuth2 authentication support
- 📨 **Push Notifications**: Webhook notifications for real-time task state updates
- ⏸️ **Task Pausing**: Built-in support for input-required state pausing and resumption

### Developer Experience

- ⚙️ **Environment Configuration**: Simple setup through environment variables
- 📊 **Task Management**: Built-in task queuing, polling, and lifecycle management
- 📋 **Task Listing**: Listing with filtering and pagination (`tasks/list`)
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

- Go 1.24 or later
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

4. **Run tests**:
   ```bash
   task test
   ```

### Available Tasks

| Task                       | Description                                                |
| -------------------------- | ---------------------------------------------------------- |
| `task a2a:download-schema` | Download the latest A2A schema                             |
| `task a2a:generate-types`  | Generate Go types from A2A schema                          |
| `task generate:mocks`      | Generate all testing mocks                                 |
| `task lint`                | Run static analysis and linting                            |
| `task test`                | Run all tests                                              |
| `task tidy`                | Tidy Go modules                                            |
| `task clean`               | Clean up build artifacts                                   |

### Build-Time Agent Metadata

The ADK supports injecting agent metadata at build time using Go linker flags (LD flags). This makes agent information immutable and embedded in the binary, which is useful for production deployments.

#### Available LD Flags

The following build-time metadata variables can be set via LD flags:

- **`BuildAgentName`** - The agent's display name
- **`BuildAgentDescription`** - A description of the agent's capabilities  
- **`BuildAgentVersion`** - The agent's version number

#### Usage Examples

**Direct Go Build:**

```bash
# Build your application with custom LD flags
go build -ldflags="-X github.com/inference-gateway/adk/server.BuildAgentName='MyAgent' \
  -X github.com/inference-gateway/adk/server.BuildAgentDescription='My custom agent description' \
  -X github.com/inference-gateway/adk/server.BuildAgentVersion='1.2.3'" \
  -o bin/my-agent ./cmd/server/main.go
```

**Docker Build:**

```dockerfile
# Build with custom metadata in Docker
FROM golang:1.24-alpine AS builder

ARG AGENT_NAME="Production Agent"
ARG AGENT_DESCRIPTION="Production deployment agent with enhanced capabilities"  
ARG AGENT_VERSION="1.0.0"

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build \
    -ldflags="-X github.com/inference-gateway/adk/server.BuildAgentName='${AGENT_NAME}' \
              -X github.com/inference-gateway/adk/server.BuildAgentDescription='${AGENT_DESCRIPTION}' \
              -X github.com/inference-gateway/adk/server.BuildAgentVersion='${AGENT_VERSION}'" \
    -o bin/agent .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/bin/agent .
CMD ["./agent"]
```

## 📖 API Reference

### Core Components

#### A2AServer

The main server interface that handles A2A protocol communication.

```go
// Create a default A2A server
func NewDefaultA2AServer(cfg *config.Config) *A2AServerImpl

// Create a server with agent integration
func SimpleA2AServerWithAgent(cfg config.Config, logger *zap.Logger, agent OpenAICompatibleAgent, agentCard adk.AgentCard) (A2AServer, error)

// Create a server with custom components
func CustomA2AServer(cfg config.Config, logger *zap.Logger, taskHandler TaskHandler, processor TaskResultProcessor, agentCard adk.AgentCard) (A2AServer, error)

// Create a server with full customization
func NewA2AServer(cfg *config.Config, logger *zap.Logger, otel otel.OpenTelemetry) *A2AServerImpl
```

#### A2AServerBuilder

Build A2A servers with custom configurations using a fluent interface:

```go
// Basic server with agent
a2aServer, err := server.NewA2AServerBuilder(config, logger).
    WithAgent(agent).
    WithAgentCardFromFile(".well-known/agent.json").
    Build()
if err != nil {
    // handle error
}

// Server with custom task handler
a2aServer, err := server.NewA2AServerBuilder(config, logger).
    WithTaskHandler(customTaskHandler).
    WithTaskResultProcessor(customProcessor).
    WithAgentCardFromFile(".well-known/agent.json").
    Build()
if err != nil {
    // handle error
}

// Server with custom logger
a2aServer, err := server.NewA2AServerBuilder(config, logger).
    WithLogger(customLogger).
    WithAgent(agent).
    WithAgentCardFromFile(".well-known/agent.json").
    Build()
if err != nil {
    // handle error
}
```

#### AgentBuilder

Build OpenAI-compatible agents that live inside the A2A server using a fluent interface:

```go
// Basic agent with custom LLM
agent, err := server.NewAgentBuilder(logger).
    WithLLMClient(customLLMClient).
    WithToolBox(toolBox).
    Build()
if err != nil {
    // handle error
}

// Agent with system prompt
agent, err := server.NewAgentBuilder(logger).
    WithSystemPrompt("You are a helpful assistant").
    WithMaxChatCompletion(10).
    Build()
if err != nil {
    // handle error
}

// Use with A2A server builder
a2aServer, err := server.NewA2AServerBuilder(config, logger).
    WithAgent(agent).
    WithAgentCardFromFile(".well-known/agent.json").
    Build()
if err != nil {
    // handle error
}
```

#### A2AClient

The client interface for communicating with A2A servers:

```go
// Basic client creation
client := client.NewClient("http://localhost:8080")

// Client with custom logger
client := client.NewClientWithLogger("http://localhost:8080", logger)

// Client with custom configuration
config := &client.Config{
    BaseURL:    "http://localhost:8080",
    Timeout:    45 * time.Second,
    MaxRetries: 5,
}
client := client.NewClientWithConfig(config)

// Using the client
agentCard, err := client.GetAgentCard(ctx)
health, err := client.GetHealth(ctx)
response, err := client.SendTask(ctx, params)
err = client.SendTaskStreaming(ctx, params, eventChan)
```

#### Agent Health Monitoring

Monitor the health status of A2A agents to ensure they are operational:

```go
// Check agent health
health, err := client.GetHealth(ctx)
if err != nil {
    log.Printf("Health check failed: %v", err)
    return
}

// Process health status
switch health.Status {
case "healthy":
    log.Println("Agent is healthy")
case "degraded":
    log.Println("Agent is degraded - some functionality may be limited")
case "unhealthy":
    log.Println("Agent is unhealthy - may not be able to process requests")
default:
    log.Printf("Unknown health status: %s", health.Status)
}
```

**Health Status Values:**
- `healthy`: Agent is fully operational
- `degraded`: Agent is partially operational (some functionality may be limited)
- `unhealthy`: Agent is not operational or experiencing significant issues

**Use Cases:**
- Monitor agent availability in distributed systems
- Implement health checks for load balancers
- Detect and respond to agent failures
- Service discovery and routing decisions

### LLM Client

Create OpenAI-compatible LLM clients for agents:

```go
// Create LLM client with configuration
llmClient, err := server.NewOpenAICompatibleLLMClient(agentConfig, logger)

// Use with agent builder
agent, err := server.NewAgentBuilder(logger).
    WithLLMClient(llmClient).
    Build()
```

### Configuration

The configuration is managed through environment variables and the config package:

```go
type Config struct {
    AgentURL                      string              `env:"AGENT_URL,default=http://helloworld-agent:8080"`
    Debug                         bool                `env:"DEBUG,default=false"`
    Port                          string              `env:"PORT,default=8080"`
    StreamingStatusUpdateInterval time.Duration       `env:"STREAMING_STATUS_UPDATE_INTERVAL,default=1s"`
    AgentConfig                   *AgentConfig        `env:",prefix=AGENT_CLIENT_"`
    CapabilitiesConfig            *CapabilitiesConfig `env:",prefix=CAPABILITIES_"`
    TLSConfig                     *TLSConfig          `env:",prefix=TLS_"`
    AuthConfig                    *AuthConfig         `env:",prefix=AUTH_"`
    QueueConfig                   *QueueConfig        `env:",prefix=QUEUE_"`
    ServerConfig                  *ServerConfig       `env:",prefix=SERVER_"`
    TelemetryConfig               *TelemetryConfig    `env:",prefix=TELEMETRY_"`
}

type AgentConfig struct {
    Provider                    string            `env:"PROVIDER"`
    Model                       string            `env:"MODEL"`
    BaseURL                     string            `env:"BASE_URL"`
    APIKey                      string            `env:"API_KEY"`
    Timeout                     time.Duration     `env:"TIMEOUT,default=30s"`
    MaxRetries                  int               `env:"MAX_RETRIES,default=3"`
    MaxChatCompletionIterations int               `env:"MAX_CHAT_COMPLETION_ITERATIONS,default=10"`
    MaxTokens                   int               `env:"MAX_TOKENS,default=4096"`
    Temperature                 float64           `env:"TEMPERATURE,default=0.7"`
    SystemPrompt                string            `env:"SYSTEM_PROMPT"`
    QueueConfig                 QueueConfig       `env:",prefix=QUEUE_"`
}
```

## 🔧 Advanced Usage

### Building Custom Agents with AgentBuilder

The `AgentBuilder` provides a fluent interface for creating highly customized agents with specific configurations, LLM clients, and toolboxes.

#### Basic Agent Creation

```go
logger := zap.NewDevelopment()

// Create a simple agent with defaults
agent, err := server.SimpleAgent(logger)
if err != nil {
    log.Fatal("Failed to create agent:", err)
}

// Or use the builder pattern for more control
agent, err = server.NewAgentBuilder(logger).
    WithSystemPrompt("You are a helpful AI assistant specialized in customer support.").
    WithMaxChatCompletion(15).
    WithMaxConversationHistory(30).
    WithAgentCardFromFile(".well-known/agent.json").
    Build()
```

#### Agent with Custom Configuration

```go
cfg := &config.AgentConfig{
    Provider:                    "openai",
    Model:                       "gpt-4",
    APIKey:                      "your-api-key",
    MaxTokens:                   4096,
    Temperature:                 0.7,
    MaxChatCompletionIterations: 10,
    MaxConversationHistory:      20,
    SystemPrompt:                "You are a travel planning assistant.",
}

agent, err := server.NewAgentBuilder(logger).
    WithConfig(cfg).
    Build()
```

#### Agent with Custom LLM Client

```go
// Create a custom LLM client
llmClient, err := server.NewOpenAICompatibleLLMClient(cfg, logger)
if err != nil {
    log.Fatal("Failed to create LLM client:", err)
}

// Build agent with the custom client
agent, err := server.NewAgentBuilder(logger).
    WithLLMClient(llmClient).
    WithSystemPrompt("You are a coding assistant.").
    Build()
```

#### Fully Configured Agent

```go
// Create toolbox with custom tools
toolBox := server.NewDefaultToolBox()

// Add custom tools (see Custom Tools section below)
weatherTool := server.NewBasicTool(/* ... */)
toolBox.AddTool(weatherTool)

// Build a fully configured agent
agent, err := server.NewAgentBuilder(logger).
    WithConfig(cfg).
    WithLLMClient(llmClient).
    WithToolBox(toolBox).
    WithSystemPrompt("You are a comprehensive AI assistant with weather capabilities.").
    WithMaxChatCompletion(20).
    WithMaxConversationHistory(50).
    Build()
if err != nil {
    // handle error
}

// Use the agent in your server
a2aServer, err := server.NewA2AServerBuilder(serverCfg, logger).
    WithAgent(agent).
    WithAgentCardFromFile(".well-known/agent.json").
    Build()
if err != nil {
    // handle error
}
```

### Custom Tools

Create custom tools to extend your agent's capabilities:

```go
// Create a toolbox
toolBox := server.NewDefaultToolBox()

// Create a custom tool using NewBasicTool
weatherTool := server.NewBasicTool(
    "get_weather",
    "Get current weather for a location",
    map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{
                "type":        "string",
                "description": "The city and state, e.g. San Francisco, CA",
            },
        },
        "required": []string{"location"},
    },
    func(ctx context.Context, args map[string]interface{}) (string, error) {
        location := args["location"].(string)

        // Your weather API logic here
        result := getWeather(location)

        response, _ := json.Marshal(result)
        return string(response), nil
    },
)

// Add the tool to the toolbox
toolBox.AddTool(weatherTool)

// Set the toolbox on your agent
agent.SetToolBox(toolBox)
```

### Loading AgentCard from JSON File

Load agent metadata from static JSON files, making it possible to serve agent cards without requiring Go code changes. This approach improves readability and allows non-developers to manage agent configuration.

#### Environment Variable

Configure the JSON file path using:

```bash
# Set the path to your JSON AgentCard file
export AGENT_CARD_FILE_PATH="/path/to/your/.well-known/agent.json"
```

#### JSON AgentCard Structure

Create a JSON file following the A2A AgentCard specification:

```json
{
  "name": "Weather Assistant",
  "description": "A specialized AI agent that provides comprehensive weather information",
  "version": "2.1.0",
  "url": "https://weather-agent.example.com",
  "documentationUrl": "https://weather-agent.example.com/docs",
  "iconUrl": "https://weather-agent.example.com/icon.png",
  "capabilities": {
    "streaming": true,
    "pushNotifications": true,
    "stateTransitionHistory": false
  },
  "defaultInputModes": ["text"],
  "defaultOutputModes": ["text", "json"],
  "skills": [
    {
      "id": "current-weather",
      "name": "Current Weather",
      "description": "Get current weather conditions for any location",
      "tags": ["weather", "current", "conditions"],
      "inputModes": ["text"],
      "outputModes": ["text", "json"],
      "examples": [
        "What's the weather in New York?",
        "Current conditions in Tokyo"
      ]
    }
  ],
  "provider": {
    "organization": "Weather Corp",
    "url": "https://weathercorp.example.com"
  },
  "securitySchemes": {
    "apiKey": {
      "type": "apiKey",
      "in": "header",
      "name": "X-API-Key",
      "description": "API key for weather service access"
    }
  },
  "security": [{"apiKey": []}]
}
```

#### Using with Server Builder

Load JSON AgentCard using the server builder:

```go
// Automatically load during server creation (if AGENT_CARD_FILE_PATH is set)
a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
    WithAgent(agent).
    Build()
if err != nil {
    // handle error
}

// Or explicitly specify the file path
a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
    WithAgent(agent).
    WithAgentCardFromFile("./.well-known/agent.json").
    Build()
if err != nil {
    // handle error
}

// Or load after server creation
if err := a2aServer.LoadAgentCardFromFile("./.well-known/agent.json"); err != nil {
    log.Printf("Failed to load agent card: %v", err)
}
```

#### Complete Example

See the complete example at [`examples/server/cmd/json-agentcard/`](./examples/server/cmd/json-agentcard/main.go):

```bash
# Run with default agent card
cd examples/server/cmd/json-agentcard
go run main.go

# Run with custom agent card file
AGENT_CARD_FILE_PATH="./my-custom-card.json" go run main.go

# Run with AI capabilities
export AGENT_CLIENT_API_KEY="sk-..."
export AGENT_CARD_FILE_PATH="./.well-known/agent.json"
go run main.go
```

#### Benefits

- **Improved Readability**: Agent configuration in human-readable JSON format
- **No Code Changes**: Update agent metadata without recompiling
- **Version Control Friendly**: Easy to track changes in agent configuration
- **Team Collaboration**: Non-developers can manage agent metadata
- **Deployment Flexibility**: Different agent cards for different environments

### Task Pausing for User Input

Agents can pause tasks to request additional input from clients using the input-required state:

```go
// Server-side: Pause task for input (TaskManager interface)
err := taskManager.PauseTaskForInput(taskID, &adk.Message{
    Role: "assistant",
    Parts: []adk.Part{{
        Kind: "text",
        Text: "I need additional information. Please provide your preferences:",
    }},
})

// Client-side: Monitor and resume paused tasks
for {
    task, err := client.GetTask(ctx, taskID)
    if err != nil {
        log.Printf("Error getting task: %v", err)
        continue
    }
    
    switch task.Status.State {
    case adk.TaskStateInputRequired:
        // Display agent's request and get user input
        fmt.Printf("Agent: %s\n", getMessageText(task.Status.Message))
        userInput := getUserInput()
        
        // Resume task with user input (TaskID in message as per schema)
        _, err = client.SendTask(ctx, adk.MessageSendParams{
            Message: adk.Message{
                Role: "user",
                TaskID: &taskID,  // Resume existing task
                Parts: []adk.Part{{Kind: "text", Text: userInput}},
            },
        })
        if err != nil {
            log.Printf("Error resuming task: %v", err)
        }
    case adk.TaskStateCompleted, adk.TaskStateFailed:
        return task // Task finished
    }
    
    time.Sleep(2 * time.Second) // Poll interval
}
```

#### Tool Use and MCP Integration

Leverage standard tool use patterns and Model Context Protocol (MCP) with task pausing:

**Tool-Based Pausing:**
```go
// Define a tool that requires user input
inputTool := server.NewBasicTool(
    "request_user_input",
    "Request additional input from the user",
    map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "prompt": {"type": "string", "description": "Question for the user"},
        },
    },
    func(ctx context.Context, args map[string]interface{}) (string, error) {
        prompt := args["prompt"].(string)
        
        // Extract taskID from context (set by agent)
        taskID := ctx.Value("taskID").(string)
        
        // Pause the task and request input
        err := taskManager.PauseTaskForInput(taskID, &adk.Message{
            Role: "assistant",
            Parts: []adk.Part{{Kind: "text", Text: prompt}},
        })
        
        return "task_paused_for_input", err
    },
)
```

**MCP Tool Integration:**
```go
// MCP tools can seamlessly pause tasks for user confirmation
mcpConfirmTool := server.NewBasicTool(
    "mcp_confirm_action",
    "Request user confirmation via MCP protocol",
    map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "action": {"type": "string", "description": "Action requiring confirmation"},
            "details": {"type": "object", "description": "Action details"},
        },
    },
    func(ctx context.Context, args map[string]interface{}) (string, error) {
        // Use MCP to present structured confirmation request
        confirmationRequest := buildMCPConfirmation(args)
        
        taskID := ctx.Value("taskID").(string)
        err := taskManager.PauseTaskForInput(taskID, confirmationRequest)
        
        return "awaiting_mcp_confirmation", err
    },
)
```

This pattern enables agents to seamlessly integrate human-in-the-loop workflows while maintaining tool use standards and MCP compatibility.

### Custom Task Processing

Implement custom business logic for task completion:

```go
type CustomTaskProcessor struct{}

func (ctp *CustomTaskProcessor) ProcessToolResult(toolCallResult string) *adk.Message {
    // Parse the tool result
    var result map[string]interface{}
    json.Unmarshal([]byte(toolCallResult), &result)

    // Apply your business logic
    if shouldCompleteTask(result) {
        return &adk.Message{
            Role:    "assistant",
            Parts: []adk.Part{
                {
                    Kind:    "text",
                    Content: "Task completed successfully!",
                },
            },
        }
    }

    // Return nil to continue processing
    return nil
}

// Set the processor when building your server
a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
    WithTaskResultProcessor(&CustomTaskProcessor{}).
    WithAgentCardFromFile(".well-known/agent.json").
    Build()
if err != nil {
    // handle error
}
```

### Push Notifications

Configure webhook notifications to receive real-time updates when task states change:

```go
// Create an HTTP push notification sender
notificationSender := server.NewHTTPPushNotificationSender(logger)

// Create a task manager with push notification support
taskManager := server.NewDefaultTaskManagerWithNotifications(
    logger,
    100, // max conversation history
    notificationSender,
)

// Or set it on an existing task manager
taskManager.SetNotificationSender(notificationSender)

// Configure push notification webhooks for a task
config := adk.TaskPushNotificationConfig{
    TaskID: "task-123",
    PushNotificationConfig: adk.PushNotificationConfig{
        URL:   "https://your-app.com/webhooks/task-updates",
        Token: &token, // Optional Bearer token
        Authentication: &adk.PushNotificationAuthenticationInfo{
            Schemes:     []string{"bearer"},
            Credentials: &bearerToken,
        },
    },
}

// Set the configuration
_, err := taskManager.SetTaskPushNotificationConfig(config)
if err != nil {
    log.Printf("Failed to set push notification config: %v", err)
}
```

#### Webhook Payload

When a task state changes, your webhook will receive a POST request with this payload:

```json
{
  "type": "task_update",
  "taskId": "task-123",
  "state": "completed",
  "timestamp": "2025-06-16T10:30:00Z",
  "task": {
    "id": "task-123",
    "kind": "task",
    "status": {
      "state": "completed",
      "message": {
        "role": "assistant",
        "parts": [{"kind": "text", "text": "Task completed successfully"}]
      },
      "timestamp": "2025-06-16T10:30:00Z"
    },
    "contextId": "context-456",
    "history": [...]
  }
}
```

#### Authentication Options

Push notifications support multiple authentication schemes:

```go
// Bearer token authentication
config := adk.TaskPushNotificationConfig{
    PushNotificationConfig: adk.PushNotificationConfig{
        URL:   "https://your-app.com/webhook",
        Token: &bearerToken, // Simple token field
    },
}

// Advanced authentication with custom schemes
config := adk.TaskPushNotificationConfig{
    PushNotificationConfig: adk.PushNotificationConfig{
        URL: "https://your-app.com/webhook",
        Authentication: &adk.PushNotificationAuthenticationInfo{
            Schemes:     []string{"bearer", "basic"},
            Credentials: &credentials,
        },
    },
}
```

#### Managing Push Notification Configs

```go
// List all configs for a task
configs, err := taskManager.ListTaskPushNotificationConfigs(
    adk.ListTaskPushNotificationConfigParams{ID: "task-123"},
)

// Get a specific config
config, err := taskManager.GetTaskPushNotificationConfig(
    adk.GetTaskPushNotificationConfigParams{ID: "config-id"},
)

// Delete a config
err := taskManager.DeleteTaskPushNotificationConfig(
    adk.DeleteTaskPushNotificationConfigParams{ID: "config-id"},
)
```

### Agent Metadata

Agent metadata can be configured in two ways: at build-time via LD flags (recommended for production) or at runtime via configuration.

#### Build-Time Metadata (Recommended)

Agent metadata is embedded directly into the binary during compilation using Go linker flags. This approach ensures immutable agent information and is ideal for production deployments:

```bash
# Build your application with custom LD flags
go build -ldflags="-X github.com/inference-gateway/adk/server.BuildAgentName='Weather Assistant' \
  -X github.com/inference-gateway/adk/server.BuildAgentDescription='Specialized weather analysis agent' \
  -X github.com/inference-gateway/adk/server.BuildAgentVersion='2.0.0'" \
  -o bin/app .
```

#### Runtime Metadata Configuration

For development or when dynamic configuration is needed, you can override the build-time metadata through the server's setter methods:

```go
cfg := config.Config{
    Port: "8080",
    CapabilitiesConfig: &config.CapabilitiesConfig{
        Streaming:              true,
        PushNotifications:      true,
        StateTransitionHistory: false,
    },
}

// The server uses build-time metadata as defaults (server.BuildAgentName, etc.)
// but you can override them at runtime if needed
a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
    WithAgentCardFromFile(".well-known/agent.json").
    Build()
if err != nil {
    log.Fatal("Failed to create server:", err)
}

// Override build-time metadata for development
a2aServer.SetAgentName("Development Weather Assistant")
a2aServer.SetAgentDescription("Development version with debug features")
a2aServer.SetAgentVersion("dev-1.0.0")
```

**Note:** Build-time metadata takes precedence as defaults, but can be overridden at runtime using the setter methods (`SetAgentName`, `SetAgentDescription`, `SetAgentVersion`).

### Environment Configuration

Key environment variables for configuring your agent:

```bash
# Server configuration
PORT="8080"

# Agent metadata configuration (via LD flags only - see Build-Time Agent Metadata section)
# AGENT_NAME, AGENT_DESCRIPTION, AGENT_VERSION are set at build time via LD flags
AGENT_CARD_FILE_PATH="./.well-known/agent.json"    # Path to JSON AgentCard file (optional)

# LLM client configuration
AGENT_CLIENT_PROVIDER="openai"              # openai, anthropic, deepseek, ollama
AGENT_CLIENT_MODEL="gpt-4"                  # Model name
AGENT_CLIENT_API_KEY="your-api-key"         # Required for AI features
AGENT_CLIENT_BASE_URL="https://api.openai.com/v1"  # Custom endpoint
AGENT_CLIENT_MAX_TOKENS="4096"              # Max tokens for completion
AGENT_CLIENT_TEMPERATURE="0.7"              # Temperature for completion
AGENT_CLIENT_SYSTEM_PROMPT="You are a helpful assistant"

# Capabilities
CAPABILITIES_STREAMING="true"
CAPABILITIES_PUSH_NOTIFICATIONS="true"
CAPABILITIES_STATE_TRANSITION_HISTORY="false"

# Authentication (optional)
AUTH_ENABLE="false"
AUTH_ISSUER_URL="http://keycloak:8080/realms/inference-gateway-realm"
AUTH_CLIENT_ID="inference-gateway-client"
AUTH_CLIENT_SECRET="your-secret"

# TLS (optional)
SERVER_TLS_ENABLE="false"
SERVER_TLS_CERT_PATH="/path/to/cert.pem"
SERVER_TLS_KEY_PATH="/path/to/key.pem"
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

- **Go**: 1.24 or later
- **Dependencies**: See [go.mod](./go.mod) for full dependency list

## 🐳 Docker Support

Build and run your A2A agent application in a container. Here's an example Dockerfile for an application using the ADK:

```dockerfile
FROM golang:1.24-alpine AS builder

# Build arguments for agent metadata
ARG AGENT_NAME="My A2A Agent"
ARG AGENT_DESCRIPTION="A custom A2A agent built with the ADK"
ARG AGENT_VERSION="1.0.0"

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with custom agent metadata
RUN go build \
    -ldflags="-X github.com/inference-gateway/adk/server.BuildAgentName='${AGENT_NAME}' \
              -X github.com/inference-gateway/adk/server.BuildAgentDescription='${AGENT_DESCRIPTION}' \
              -X github.com/inference-gateway/adk/server.BuildAgentVersion='${AGENT_VERSION}'" \
    -o bin/agent .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/bin/agent .
CMD ["./agent"]
```

**Build with custom metadata:**

```bash
docker build \
  --build-arg AGENT_NAME="Weather Assistant" \
  --build-arg AGENT_DESCRIPTION="AI-powered weather forecasting agent" \
  --build-arg AGENT_VERSION="2.0.0" \
  -t my-a2a-agent .
```

## 🧪 Testing

The ADK follows table-driven testing patterns and provides comprehensive test coverage:

```go
func TestA2AServerEndpoints(t *testing.T) {
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
        {
            name:           "a2a endpoint",
            endpoint:       "/a2a",
            method:         "POST",
            expectedStatus: http.StatusOK,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Each test case has isolated mocks
            server := setupTestServer(t)
            defer server.Close()

            // Test implementation with table-driven approach
            resp := makeRequest(t, server, tt.method, tt.endpoint)
            assert.Equal(t, tt.expectedStatus, resp.StatusCode)
        })
    }
}
```

Run tests with:

```bash
task test
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
3. **Generate mocks** (if interfaces changed): `task generate:mocks`
4. **Run linting**: `task lint`
5. **All tests pass**: `task test`

### Pull Request Process

1. Update documentation for any new features
2. Add tests for new functionality
3. Ensure all CI checks pass
4. Request review from maintainers

For more details, see [CONTRIBUTING.md](./CONTRIBUTING.md).

## 📞 Support

### Issues & Questions

- **Bug Reports**: [GitHub Issues](https://github.com/inference-gateway/adk/issues)
- **Documentation**: [Official Docs](https://docs.inference-gateway.com)

## 🔗 Resources

### Documentation

- [A2A Protocol Specification](https://github.com/inference-gateway/schemas/tree/main/a2a)
- [API Documentation](https://docs.inference-gateway.com/a2a)

---

<p align="center">
  <strong>Built with ❤️ by the Inference Gateway team</strong>
</p>

<p align="center">
  <a href="https://github.com/inference-gateway">GitHub</a> •
  <a href="https://docs.inference-gateway.com">Documentation</a>
</p>
