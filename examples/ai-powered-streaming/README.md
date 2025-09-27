# AI-Powered Streaming A2A Example

## Purpose

This example demonstrates the **combination of AI/LLM integration with real-time streaming capabilities**. It shows how to build an A2A server that can process natural language requests using AI models while streaming responses in real-time for an interactive, chat-like experience.

**Key Benefits:**

- **Real-time AI Interaction**: Stream AI responses as they're generated for immediate user feedback
- **Tool Integration**: AI agent has access to tools (weather, time) for improved capabilities
- **Dual Processing**: Supports both streaming and background task processing

This example is ideal for building conversational AI agents, chatbots, or any application requiring real-time AI responses.

## Overview

The ai-powered-streaming example shows:

- **AI Streaming Handler**: Custom `AIStreamingTaskHandler` implementation with AI integration
- **Real-time Processing**: Character-by-character streaming of AI responses
- **Tool Integration**: Weather and time tools available to the AI agent
- **LLM Provider Support**: Compatible with OpenAI, Anthropic, and other providers
- **Event-driven Architecture**: Uses cloud events for streaming communication

## Key Features

### AI Streaming Architecture

The server implements both `TaskHandler` and `StreamableTaskHandler` interfaces using the `AIStreamingTaskHandler` type:

```go
type AIStreamingTaskHandler struct {
    logger *zap.Logger
    agent  server.OpenAICompatibleAgent
}

// Background processing
func (h *AIStreamingTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error)

// Real-time streaming
func (h *AIStreamingTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan server.StreamEvent, error)
```

### AI Agent with Tools

The AI agent includes practical tools:

- **Weather Tool**: Get current weather for any location
- **Time Tool**: Get current date and time
- **Streaming Support**: Real-time response generation

### Server Configuration

```go
// Build server with AI streaming capabilities
server.NewA2AServerBuilder(cfg.A2A, logger).
    WithBackgroundTaskHandler(taskHandler).
    WithStreamingTaskHandler(taskHandler).
    WithAgentCard(agentCard).
    Build()
```

## Requirements

- **LLM Provider**: This example requires an AI provider (OpenAI, Anthropic, etc.)
- **API Key**: Valid API key for your chosen provider
- **Go 1.25+**: For building and running the example
- **Streaming Enabled**: Server must have streaming capabilities enabled

## Configuration

Configure via environment variables:

### Required Configuration

- `A2A_AGENT_CLIENT_PROVIDER`: LLM provider (`openai`, `anthropic`)
- `A2A_AGENT_CLIENT_MODEL`: Model name (e.g., `gpt-4`, `claude-3-haiku-20240307`)

### Optional Configuration

- `ENVIRONMENT`: Runtime environment (default: `development`)
- `A2A_SERVER_PORT`: Server port (default: `8080`)
- `A2A_DEBUG`: Enable debug logging (default: `false`)
- `A2A_CAPABILITIES_STREAMING`: Enable streaming (default: `true`)
- `A2A_AGENT_CLIENT_BASE_URL`: Custom LLM endpoint URL (optional)

### Example Configuration

**IMPORTANT**: You must set both `A2A_AGENT_CLIENT_PROVIDER` and `A2A_AGENT_CLIENT_MODEL` environment variables for the AI agent to function. The server will fail to start if these are not configured.

For OpenAI:

```bash
export A2A_AGENT_CLIENT_PROVIDER=openai
export A2A_AGENT_CLIENT_MODEL=gpt-4
```

For Anthropic:

```bash
export A2A_AGENT_CLIENT_PROVIDER=anthropic
export A2A_AGENT_CLIENT_MODEL=claude-3-haiku-20240307
```

For Docker Compose usage, create a `.env` file in the example directory:

```bash
# .env file for docker-compose
A2A_AGENT_CLIENT_PROVIDER=openai
A2A_AGENT_CLIENT_MODEL=gpt-4
```

## Running the Example

### 1. Set Environment Variables

Create a `.env` file or export variables:

```bash
# Required
export A2A_AGENT_CLIENT_PROVIDER=openai
export A2A_AGENT_CLIENT_MODEL=gpt-4

# Optional
export A2A_SERVER_PORT=8080
export A2A_DEBUG=true
```

### 2. Start the Server

```bash
cd server
go run main.go
```

Expected output:

```
🤖⚡ Starting AI-Powered Streaming A2A Server...
2024/01/15 10:30:00 INFO configuration loaded
2024/01/15 10:30:00 INFO ✅ AI agent created with streaming capabilities
2024/01/15 10:30:00 INFO ✅ server created with AI streaming capabilities
2024/01/15 10:30:00 INFO 🌐 server running on port 8080
```

### 3. Run the Client

In another terminal:

```bash
cd client
go run main.go
```

## Example Interactions

The client demonstrates three types of interactions:

### 1. AI Streaming with Tool Usage

**Request**: "Can you get the weather for New York and then explain what activities would be good for that weather?"

**Response**: Streams AI thinking process, tool calls, and recommendations in real-time.

### 2. AI Streaming Conversation

**Request**: "Tell me an interesting story about artificial intelligence in the future."

**Response**: Streams creative storytelling with natural AI narrative flow.

### 3. Regular AI Task

**Request**: "What's the current time and how can I improve my productivity?"

**Response**: Background processing with AI response and tool usage.

## Understanding the Code

### Server Architecture

**AI Streaming Handler** (`server/main.go`):

- Implements both background and streaming interfaces
- Uses AI agent for intelligent responses
- Handles real-time event streaming
- Integrates with tools for enhanced capabilities

**Configuration** (`server/config/config.go`):

- Standard A2A configuration pattern
- Environment variable support with `A2A_` prefix

### Client Architecture

**Streaming Client** (`client/main.go`):

- Demonstrates multiple interaction patterns
- Real-time delta processing
- Event handling and display
- Comparison of streaming vs. background processing

### AI Integration Features

1. **LLM Client**: OpenAI-compatible client for various providers
2. **Tool Integration**: Weather and time tools for enhanced capabilities
3. **Streaming Support**: Real-time response generation with proper event handling
4. **Error Handling**: Graceful degradation and error reporting

## Benefits of AI Streaming

1. **Immediate Feedback**: Users see responses as they're generated
2. **Interactive Experience**: Chat-like interface with real-time updates
3. **Tool Integration**: AI can use tools and stream the process
4. **Scalable Architecture**: Supports both streaming and background processing
5. **Production Ready**: Proper error handling and configuration management

## Comparison with Other Examples

| Example                  | AI Integration | Streaming | Use Case                          |
| ------------------------ | -------------- | --------- | --------------------------------- |
| **ai-powered**           | ✅ Full        | ❌ No     | Background AI processing          |
| **streaming**            | ❌ Mock only   | ✅ Yes    | Real-time responses (no AI)       |
| **ai-powered-streaming** | ✅ Full        | ✅ Yes    | Real-time AI responses with tools |

## File Structure

```
ai-powered-streaming/
├── README.md                    # This file
├── server/
│   ├── config/
│   │   └── config.go           # Configuration with A2A prefix
│   └── main.go                 # AI streaming server implementation
└── client/
    └── main.go                 # Streaming client with AI demos
```

## Next Steps

- Try different AI models and providers
- Experiment with custom tools and capabilities
- Integrate with webhooks for push notifications
- Scale with multiple AI agents and load balancing
