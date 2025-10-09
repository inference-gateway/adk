# AI-Powered A2A Example

This example demonstrates an A2A server with AI/LLM integration and built-in tools for weather and time queries.

## Table of Contents

- [What This Example Shows](#what-this-example-shows)
- [Directory Structure](#directory-structure)
- [Running the Example](#running-the-example)
- [Server Configuration](#server-configuration)
- [Supported Providers](#supported-providers)
- [Built-in Tools](#built-in-tools)
- [Understanding the Code](#understanding-the-code)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

## What This Example Shows

- A2A server with AI agent integration using multiple LLM providers
- Built-in tools: weather lookup and current time
- Local Inference Gateway for provider abstraction
- Environment-based configuration following production patterns

## Directory Structure

```
ai-powered/
├── client/
│   └── main.go         # A2A client sending AI prompts
├── server/
│   ├── main.go         # AI-powered A2A server with tools
│   └── config/
│       └── config.go   # Configuration
├── docker-compose.yaml # Includes Inference Gateway, uses ../Dockerfile.server and ../Dockerfile.client
├── .env.example        # All provider API keys
└── README.md
```

## Running the Example

### Using Docker Compose (Recommended)

1. Copy environment variables:

```bash
cp .env.example .env
```

2. Edit `.env` and add your API key for at least one provider:

```bash
# Choose one or more providers
OPENAI_API_KEY=your_openai_api_key_here
ANTHROPIC_API_KEY=your_anthropic_api_key_here
DEEPSEEK_API_KEY=your_deepseek_api_key_here
# ... other providers

# Configure agent
A2A_AGENT_CLIENT_PROVIDER=openai
A2A_AGENT_CLIENT_MODEL=gpt-4o-mini
```

3. Run the example:

```bash
docker-compose up --build
```

This will:

1. Start the Inference Gateway with your configured providers
2. Start the AI-powered A2A server with weather and time tools
3. Run the client to test AI responses with tool usage

### Running Locally

#### Start the Server

```bash
cd server
export A2A_AGENT_CLIENT_PROVIDER=openai
export A2A_AGENT_CLIENT_MODEL=gpt-4o-mini
export A2A_AGENT_CLIENT_BASE_URL=https://api.openai.com
go run main.go
```

#### Run the Client

```bash
cd client
go run main.go
```

## Server Configuration

The server uses environment variables with the `A2A_` prefix for consistency with production agents:

| Environment Variable         | Description                              | Default                      |
| ---------------------------- | ---------------------------------------- | ---------------------------- |
| `ENVIRONMENT`                | Runtime environment                      | `development`                |
| `A2A_AGENT_NAME`             | Agent name (set via build-time LD flags) | `ai-powered-agent`           |
| `A2A_AGENT_DESCRIPTION`      | Agent description (build-time)           | AI-powered server with tools |
| `A2A_AGENT_VERSION`          | Agent version (build-time)               | `0.1.0`                      |
| `A2A_SERVER_PORT`            | Server port                              | `8080`                       |
| `A2A_DEBUG`                  | Enable debug logging                     | `false`                      |
| `A2A_CAPABILITIES_STREAMING` | Enable streaming support                 | `true`                       |
| `A2A_AGENT_CLIENT_BASE_URL`  | LLM API endpoint                         | Via Inference Gateway        |
| `A2A_AGENT_CLIENT_PROVIDER`  | LLM provider (openai, anthropic, etc.)   | Required                     |
| `A2A_AGENT_CLIENT_MODEL`     | Model name                               | Required                     |

## Supported Providers

Via the included Inference Gateway:

- OpenAI (GPT models)
- Anthropic (Claude models)
- DeepSeek
- Google (Gemini)
- Cloudflare Workers AI
- Cohere
- Mistral

## Built-in Tools

The server includes two sample tools:

- **Weather**: Get current weather for any location
- **Time**: Get current date and time

Example AI interaction:

```
User: "What's the weather in Tokyo and what time is it?"
AI: Uses both tools to provide current weather and time information
```

## Understanding the Code

### Server (`server/main.go`)

Creates an AI agent with tools and processes messages:

```go
// Create AI agent with LLM client and tools
agent, err := server.NewAgentBuilder(logger).
    WithConfig(&cfg.A2A.AgentConfig).
    WithLLMClient(llmClient).
    WithSystemPrompt("You are a helpful AI assistant with access to weather and time tools.").
    WithToolBox(toolBox).
    Build()

// Custom task handler for AI processing
taskHandler := NewAITaskHandler(logger)
taskHandler.SetAgent(agent)
```

### Configuration (`server/config/config.go`)

Follows the same pattern as production agents:

```go
type Config struct {
    Environment string              `env:"ENVIRONMENT,default=development"`
    A2A         serverConfig.Config `env:",prefix=A2A_"`
}
```

### Build-Time Metadata

Agent metadata is injected via LD flags at build time instead of being hardcoded:

```dockerfile
RUN go build -ldflags="-X github.com/inference-gateway/adk/server.BuildAgentName=${AGENT_NAME}" -o server .
```

## Troubleshooting

### Troubleshooting with A2A Debugger

```bash
# List tasks and debug the A2A server
docker compose run --rm a2a-debugger tasks list
```

## Next Steps

- Try the `minimal` example for basic A2A concepts
- Check the `streaming` example for real-time AI responses
- Explore other examples for different AI agent patterns
