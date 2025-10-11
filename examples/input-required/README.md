# Input-Required Flow Examples

This example demonstrates how A2A agents can pause task execution to request additional information from users when needed. The input-required flow is essential for creating conversational agents that can handle ambiguous or incomplete requests gracefully.

## Table of Contents

- [What This Example Shows](#what-this-example-shows)
- [Directory Structure](#directory-structure)
- [How Input-Required Works](#how-input-required-works)
- [Available Examples](#available-examples)
- [Running the Examples](#running-the-examples)
- [Example Interactions](#example-interactions)
- [Configuration](#configuration)
- [Understanding the Code](#understanding-the-code)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

## What This Example Shows

- **Input-Required Flow**: How agents pause tasks to request missing information
- **Non-Streaming Mode**: Traditional request-response with input pausing
- **Streaming Mode**: Real-time streaming that can pause for user input
- **Task State Management**: How tasks transition between states during input-required flows
- **Built-in Tool Usage**: Leveraging the built-in `input_required` tool
- **Conversation Continuity**: Maintaining context across multiple user interactions

## Directory Structure

```
input-required/
├── README.md                          # This file
├── non-streaming/                     # Traditional request-response mode
│   ├── server/
│   │   ├── main.go                   # Server with input-required handling
│   │   ├── config/
│   │   │   └── config.go             # Configuration
│   │   └── go.mod                    # Dependencies
│   ├── client/
│   │   ├── main.go                   # Interactive client
│   │   └── go.mod                    # Dependencies
│   └── docker-compose.yaml           # Complete setup
└── streaming/                         # Real-time streaming mode
    ├── server/
    │   ├── main.go                   # Streaming server with input-required
    │   ├── config/
    │   │   └── config.go             # Configuration
    │   └── go.mod                    # Dependencies
    ├── client/
    │   ├── main.go                   # Streaming client
    │   └── go.mod                    # Dependencies
    └── docker-compose.yaml           # Complete setup
```

## How Input-Required Works

### The Flow

1. **User sends a message** to the agent
2. **Agent analyzes the request** and determines if additional information is needed
3. **Agent calls `input_required` tool** if information is missing
4. **Task pauses** with state `TaskStateInputRequired`
5. **User provides additional information**
6. **Agent continues processing** with the complete information
7. **Task completes** successfully

### Built-in Tool

The ADK includes a built-in `input_required` tool that:

- Takes a `message` parameter explaining what information is needed
- Automatically pauses the task execution
- Sets the task state to `TaskStateInputRequired`
- Allows the conversation to continue when user provides input

### Task States

During input-required flow, tasks transition through these states:

- `TaskStateSubmitted` → Initial state when task is created
- `TaskStateWorking` → Agent is processing the request
- `TaskStateInputRequired` → Agent needs user input to continue
- `TaskStateWorking` → Agent resumes processing with new input
- `TaskStateCompleted` → Task finishes successfully

## Available Examples

### Non-Streaming (`non-streaming/`)

Traditional request-response mode where:

- Client sends a message and waits for response
- Server processes and may request input
- Client handles input-required state and continues conversation
- Final response is returned when complete

**Use cases:**

- Form-based applications
- Batch processing workflows
- Traditional chat interfaces

### Streaming (`streaming/`)

Real-time streaming mode where:

- Server streams response chunks as they're generated
- Stream can pause mid-response to request input
- Client displays real-time updates and handles input requests
- Stream continues after user provides additional information

**Use cases:**

- Real-time chat applications
- Interactive code generation
- Live content creation

## Running the Examples

### Prerequisites

- Docker and Docker Compose installed
- (Optional) Go 1.21+ for local development
- (Optional) API keys for OpenAI, Anthropic, etc. for AI-powered responses

### Using Docker Compose (Recommended)

#### Non-Streaming Example

```bash
cd examples/input-required/non-streaming
docker-compose up --build
```

#### Streaming Example

```bash
cd examples/input-required/streaming
docker-compose up --build
```

### Running Locally

#### Start the Server

```bash
# Non-streaming
cd examples/input-required/non-streaming/server
go run main.go

# Streaming
cd examples/input-required/streaming/server
go run main.go
```

#### Run the Client

```bash
# Non-streaming
cd examples/input-required/non-streaming/client
go run main.go

# Streaming
cd examples/input-required/streaming/client
go run main.go
```

### With AI Integration

To enable AI-powered responses, add your API keys:

1. Copy the environment variables in `docker-compose.yaml`
2. Uncomment and set your provider and API key:

```yaml
environment:
  - A2A_AGENT_CLIENT_PROVIDER=openai
  - A2A_AGENT_CLIENT_MODEL=gpt-4o-mini
  - A2A_AGENT_CLIENT_BASE_URL=http://inference-gateway:8080/v1
```

3. Set your API key in the inference-gateway service:

```yaml
inference-gateway:
  environment:
    - OPENAI_API_KEY=${OPENAI_API_KEY}
```

## Example Interactions

### Weather Query (Missing Location)

```
User: "What's the weather?"
Agent: "I'd be happy to help you with the weather! Could you please specify which location you'd like the weather for?"
User: "New York"
Agent: "The weather in New York is currently sunny and 72°F!"
```

### Calculation Request (Missing Numbers)

```
User: "Calculate something for me"
Agent: "I can help you with calculations! Could you please provide the specific numbers or equation you'd like me to calculate?"
User: "What's 15 * 23?"
Agent: "15 * 23 = 345"
```

### Unclear Request

```
User: "Help me"
Agent: "I'd be happy to help! Could you please provide more details about what you'd like me to do? For example, you could ask about the weather or request a calculation."
User: "I need help with my homework"
Agent: "I'd be glad to help with your homework! What subject are you working on and what specific problem do you need help with?"
```

## Configuration

### Server Configuration

Both examples use environment variables with the `A2A_` prefix:

| Variable                     | Description                         | Default                                     |
| ---------------------------- | ----------------------------------- | ------------------------------------------- |
| `A2A_SERVER_PORT`            | Server port                         | `8080`                                      |
| `A2A_DEBUG`                  | Enable debug logging                | `false`                                     |
| `A2A_CAPABILITIES_STREAMING` | Enable streaming                    | `true` (streaming), `false` (non-streaming) |
| `A2A_AGENT_CLIENT_PROVIDER`  | AI provider (`openai`, `anthropic`) | _(none)_                                    |
| `A2A_AGENT_CLIENT_MODEL`     | AI model name                       | _(required if provider set)_                |

### AI Configuration

When AI is enabled, the agent uses the built-in `input_required` tool automatically:

- **System Prompt**: Configured to use `input_required` when information is missing
- **Tool Detection**: Automatically detects when requests need clarification
- **Smart Prompting**: Asks specific questions about missing information

## Understanding the Code

### Non-Streaming Server (`non-streaming/server/main.go`)

Key components:

```go
// Custom task handler with input-required logic
type InputRequiredTaskHandler struct {
    logger *zap.Logger
    agent  server.OpenAICompatibleAgent
}

// Processes tasks and demonstrates input-required flow
func (h *InputRequiredTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
    // With AI agent: agent automatically uses input_required tool
    // Without AI: manual logic demonstrates the flow
}
```

### Streaming Server (`streaming/server/main.go`)

Key components:

```go
// Streaming task handler with real-time input-required
type StreamingInputRequiredTaskHandler struct {
    logger *zap.Logger
    agent  server.OpenAICompatibleAgent
}

// Processes streaming tasks with input-required pausing
func (h *StreamingInputRequiredTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error) {
    // Streams events including deltas, input-required, and status updates
}
```

### Client Logic

Both clients handle the input-required flow:

1. **Send initial message**
2. **Monitor task status**
3. **Detect input-required state**
4. **Prompt user for additional input**
5. **Continue conversation with new context**
6. **Handle completion or further input requests**

### Built-in Tool Integration

When AI is enabled, agents automatically use the `input_required` tool:

```go
// Agent configuration with system prompt
agent, err := server.NewAgentBuilder(logger).
    WithSystemPrompt(`Use the input_required tool when you need additional information...`).
    WithDefaultToolBox(). // Includes input_required tool
    Build()
```

## Troubleshooting

### Common Issues

#### Input-Required Not Working

- **Check tool availability**: Ensure `WithDefaultToolBox()` is called when building the agent
- **Verify system prompt**: Make sure the agent is instructed to use `input_required` tool
- **Check task state**: Verify task reaches `TaskStateInputRequired` state

#### Streaming Issues

- **Enable streaming capability**: Set `A2A_CAPABILITIES_STREAMING=true`
- **Check event handling**: Ensure client properly handles `EventInputRequired` events
- **Verify context continuity**: Use same `ContextID` for follow-up messages

#### AI Integration Problems

- **API keys**: Verify API keys are set correctly in environment
- **Model availability**: Check that the specified model is available
- **Network connectivity**: Ensure inference gateway is accessible

### Debugging with A2A Debugger

```bash
# List tasks and their states
docker compose run --rm a2a-debugger tasks list --include-history

# Monitor specific task
docker compose run --rm a2a-debugger tasks get --id <task-id>
```

### Logs Analysis

Enable debug logging to see detailed flow:

```bash
# Add to environment
A2A_DEBUG=true

# Check logs
docker compose logs input-required-server
```

## Next Steps

### Explore Related Examples

- **`ai-powered/`** - Learn AI integration basics
- **`streaming/`** - Understand streaming fundamentals
- **`default-handlers/`** - Use built-in task handlers

### Advanced Scenarios

- **Multi-step input collection**: Request multiple pieces of information
- **Conditional input requirements**: Different requirements based on context
- **Input validation**: Validate user input before continuing
- **Timeout handling**: Handle cases where users don't respond

### Production Considerations

- **Error handling**: Robust error handling for input-required flows
- **State persistence**: Save task state during input-required pauses
- **User experience**: Clear messaging about what information is needed
- **Security**: Validate and sanitize user input

---

For more information about the A2A protocol and framework, see the main [README](../../README.md) or refer to the [official documentation](https://google.github.io/adk-docs/).
