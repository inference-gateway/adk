# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Essential Development Tasks

Use Taskfile.yml for all project operations:

```bash
# Core development workflow
task lint                    # Run golangci-lint for code quality checks
task lint:examples           # Run markdown-lint on all examples
task test                    # Run all tests with coverage
task tidy                    # Tidy all Go modules
task format                  # Format Go files and markdown

# A2A schema management (required when working on A2A protocol)
task a2a:download-schema     # Download latest A2A schema from upstream
task a2a:generate-types      # Generate Go types from A2A schema

# Mock generation for testing
task generate:mocks          # Generate all mocks using counterfeiter
task generate:mocks:clean    # Clean and regenerate all mocks

# Pre-commit setup
task precommit:install       # Install Git pre-commit hook (recommended)
```

### Individual Test Execution

```bash
# Run specific test files
go test -v ./server/...
go test -v ./client/...

# Run specific test functions
go test -run TestAgentBuilder ./server/
go test -run TestTaskHandler ./server/
```

## Architecture Overview

### Core Components

**A2A Server (`server/server.go`)**

- Main server implementing Agent-to-Agent protocol
- Handles HTTP endpoints for task submission and streaming
- Manages task lifecycle and state transitions
- Provides health monitoring and agent card endpoints

**A2A Server Builder (`server/server_builder.go`)**

- Fluent interface for server construction
- Configures task handlers, agents, storage backends
- Supports both default and custom implementations
- Enables modular server composition

**Task Management System**

- `TaskManager` (`server/task_manager.go`) - Core task orchestration
- `TaskHandler` - Interface for background/polling task processing
- `StreamableTaskHandler` - Interface for real-time streaming tasks
- `Storage` - Pluggable storage backends (memory/Redis)

**Agent System**

- `OpenAICompatibleAgent` (`server/agent.go`) - LLM integration layer
- `AgentBuilder` (`server/agent_builder.go`) - Agent configuration
- `ToolBox` (`server/agent_toolbox.go`) - Tool management for agents
- `CallbackConfig` (`server/callbacks.go`) - Lifecycle hooks for agents
- Support for custom tools and system prompts

**Client Interface (`client/`)**

- A2A client for communicating with other agents
- Task submission and streaming capabilities
- Health monitoring and agent discovery

### Key Architectural Patterns

**Builder Pattern**

- Used throughout for flexible component configuration
- `A2AServerBuilder`, `AgentBuilder` provide fluent interfaces
- Enables optional component composition

**Interface-Driven Design**

- All major components implement interfaces for testability
- Extensive mock generation using counterfeiter
- Enables dependency injection and testing isolation

**Dual Task Processing Models**

- Background/polling for asynchronous workflows
- Streaming for real-time interactive scenarios
- Both support input-required pausing and state management

**Storage Abstraction**

- Memory storage for development (default)
- Redis storage for production with horizontal scaling
- Configurable via `QUEUE_PROVIDER` environment variable

**Callback System**

- Lifecycle hooks for intercepting and modifying behavior
- Six callback types: BeforeAgent/AfterAgent, BeforeModel/AfterModel, BeforeTool/AfterTool
- Flow control: skip default behavior or modify outputs
- Use cases: guardrails, caching, logging, authorization, sanitization

### Callback Hooks

The ADK provides a comprehensive callback system for hooking into the agent execution lifecycle:

**Agent Lifecycle Callbacks**

- `BeforeAgent`: Called before agent execution starts
  - Use for: validation, guardrails, early returns
  - Return non-nil to skip agent execution
- `AfterAgent`: Called after agent execution completes
  - Use for: post-processing, logging, output modification
  - Return non-nil to replace agent output

**LLM Interaction Callbacks**

- `BeforeModel`: Called before each LLM call
  - Use for: request caching, guardrails, request modification
  - Return non-nil to skip LLM call
- `AfterModel`: Called after each LLM response
  - Use for: response modification, logging, sanitization
  - Return non-nil to replace LLM response

**Tool Execution Callbacks**

- `BeforeTool`: Called before each tool execution
  - Use for: authorization, caching, argument validation
  - Return non-nil to skip tool execution
- `AfterTool`: Called after each tool execution
  - Use for: result modification, logging, sanitization
  - Return non-nil to replace tool result

**Configuration Example**

```go
callbackConfig := &server.CallbackConfig{
    BeforeAgent: []server.BeforeAgentCallback{
        func(ctx context.Context, callbackCtx *server.CallbackContext) *types.Message {
            // Implement guardrails or validation
            return nil // proceed with execution
        },
    },
    BeforeModel: []server.BeforeModelCallback{
        func(ctx context.Context, callbackCtx *server.CallbackContext, req *server.LLMRequest) *server.LLMResponse {
            // Implement caching or request modification
            return nil // proceed with LLM call
        },
    },
    BeforeTool: []server.BeforeToolCallback{
        func(ctx context.Context, tool server.Tool, args map[string]any, toolCtx *server.ToolContext) map[string]any {
            // Implement authorization or caching
            return nil // proceed with tool execution
        },
    },
}

agent, err := server.NewAgentBuilder(logger).
    WithCallbacks(callbackConfig).
    Build()
```

See `examples/callbacks/` for a complete working example.

### Configuration System

Environment-based configuration with sensible defaults:

- Server settings (port, TLS, timeouts)
- Agent/LLM configuration (provider, model, API keys)
- Task management (retention, cleanup intervals)
- Storage backends (memory vs Redis)
- Telemetry and authentication (optional)

See `server/config/config.go` for complete configuration structure.

## Testing Strategy

### Testing Approach

- Table-driven tests throughout codebase
- Comprehensive mock generation for all interfaces
- Isolated test dependencies with dedicated mock servers
- Coverage requirements for new functionality

### Mock Management

Mocks are generated using counterfeiter and stored in `server/mocks/`:

- Run `task generate:mocks` after interface changes
- Each test case uses isolated mock instances
- Mock implementations support fluent test setup

### Test Organization

```
server/
├── *_test.go           # Unit tests alongside implementation
├── mocks/              # Generated mocks via counterfeiter
└── serverfakes/        # Legacy fakes (being migrated)
```

## Code Conventions

### Go Standards

- Use early returns to avoid deep nesting
- Prefer switch statements over if-else chains
- Implement interfaces for mockability
- Use lowercase log messages for consistency
- Strong typing with interface-based design

### A2A Protocol Integration

- Always download latest schema: `task a2a:download-schema`
- Regenerate types after schema updates: `task a2a:generate-types`
- Types are generated in `types/generated_types.go`
- Manual types in `types/types.go` for extensions

### Commit Workflow

1. Run `task lint` before committing
2. Run `task test` to ensure all tests pass
3. Install pre-commit hook: `task precommit:install`
4. Schema updates require regenerating types

## Examples and Documentation

The `examples/` directory contains complete working implementations:

- `minimal/` - Basic A2A server without AI capabilities
- `ai-powered/` - Full A2A server with LLM integration
- `streaming/` - Real-time streaming response handling
- `static-agent-card/` - JSON-based agent metadata management
- `default-handlers/` - Built-in task processing patterns
- `callbacks/` - Lifecycle hooks for guardrails, caching, and logging
- `input-required/` - Interactive conversation flow with input pausing
- `artifacts-*/` - Artifact creation and storage examples
- `queue-storage/` - Different queue backends for task management
- `tls-example/` - Secure HTTPS communication with TLS

Each example includes setup instructions and demonstrates specific ADK features.
