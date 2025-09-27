# A2A Server with Static Agent Card Example

## Purpose

This example demonstrates the **`WithAgentCardFromFile()`** function, which allows you to load agent configuration from a JSON file instead of hardcoding it in Go code using `WithAgentCard()`.

**Key Benefits:**

- **Configuration as Code**: Agent metadata stored in version-controlled JSON files
- **Environment-specific Configs**: Different agent cards for dev/staging/production
- **No Recompilation**: Change agent description, skills, or capabilities without rebuilding
- **Runtime Overrides**: Dynamically override specific fields (like URLs) at startup

This demonstrates best practices for externalizing agent configuration in production deployments.

## Overview

The static agent card example shows:

- **JSON-based Configuration**: Agent metadata (name, description, capabilities, skills) defined in `agent-card.json`
- **WithAgentCardFromFile()**: Loading agent card configuration from a file
- **Runtime Overrides**: Dynamically overriding specific fields (like URL) at startup
- **Environment Variables**: Configuring the agent card file path via `A2A_AGENT_CARD_FILE`

## Key Features

### Agent Card Structure

The `agent-card.json` file contains the complete agent definition:

```json
{
  "name": "static-card-agent",
  "description": "A demonstration agent that loads its configuration from a static JSON file",
  "version": "0.1.0",
  "protocol_version": "0.3.0",
  "capabilities": {
    "streaming": false,
    "push_notifications": false,
    "state_transition_history": false
  },
  "skills": [
    {
      "name": "echo",
      "description": "Echo back user messages with a friendly response"
    }
  ]
}
```

### WithAgentCardFromFile Usage

```go
// Load agent card from file with runtime overrides
a2aServer, err := server.NewA2AServerBuilder(cfg.A2A.Config, logger).
    WithBackgroundTaskHandler(taskHandler).
    WithAgentCardFromFile(cfg.A2A.AgentCardFile, map[string]any{
        "url": fmt.Sprintf("http://localhost:%s", cfg.A2A.Config.ServerConfig.Port),
    }).
    Build()
```

## Configuration

Configure via environment variables:

- `ENVIRONMENT`: Runtime environment (default: development)
- `A2A_AGENT_CARD_FILE`: Path to agent card JSON file (default: agent-card.json)
- `A2A_SERVER_PORT`: Server port (default: 8080)
- `A2A_DEBUG`: Enable debug logging (default: false)

## Running the Example

### 1. Start the Server

```bash
cd examples/static-agent-card/server
go run main.go
```

With custom agent card file:

```bash
A2A_AGENT_CARD_FILE="./my-custom-card.json" go run main.go
```

### 2. Run the Client

In another terminal:

```bash
cd examples/static-agent-card/client
go run main.go
```

## Example Interaction

The client will demonstrate:

1. **Default greeting** when sending an empty message
2. **Help information** when sending "help"
3. **Echo responses** with context about the static configuration
4. **Skills demonstration** showing capabilities loaded from JSON

## Benefits of Static Agent Cards

1. **Separation of Concerns**: Agent metadata is separate from business logic
2. **Easy Updates**: Change agent description/skills without recompiling
3. **Environment-specific**: Different agent cards for dev/staging/production
4. **Version Control**: Track agent configuration changes in Git
5. **Dynamic Overrides**: Runtime field overrides (like URLs, ports)

## File Structure

```
examples/static-agent-card/
├── agent-card.json          # Agent configuration
├── server/
│   ├── config/
│   │   └── config.go        # Configuration with agent card file path
│   └── main.go              # Server implementation
├── client/
│   └── main.go              # Client demonstration
└── README.md                # This file
```
