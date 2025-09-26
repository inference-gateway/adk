# AI-Powered A2A Example

This example demonstrates an A2A server integrated with an AI language model (OpenAI, Anthropic, etc.).

## What This Example Shows

- A2A server with AI agent integration
- Client sending various prompts to test AI responses
- Configuration for different LLM providers
- Docker Compose setup with environment variables

## Directory Structure

```
ai-powered/
├── client/
│   ├── main.go       # Client sending AI prompts
│   └── Dockerfile
├── server/
│   ├── main.go       # AI-powered A2A server
│   └── Dockerfile
├── docker-compose.yaml
└── README.md
```

## Prerequisites

You need an API key from one of the supported providers:
- OpenAI
- Anthropic
- Or use an Inference Gateway URL

## Running the Example

### Using Docker Compose (Recommended)

1. Set your API key:
```bash
export AGENT_CLIENT_API_KEY="your-api-key-here"
```

2. (Optional) Configure provider and model:
```bash
export AGENT_CLIENT_PROVIDER="openai"  # or "anthropic"
export AGENT_CLIENT_MODEL="gpt-4o-mini"  # or "claude-3-haiku"
```

3. Run the example:
```bash
docker-compose up --build
```

### Running Locally

#### Start the Server

```bash
cd server
export AGENT_CLIENT_API_KEY="your-api-key-here"
export AGENT_CLIENT_PROVIDER="openai"
export AGENT_CLIENT_MODEL="gpt-4o-mini"
go run main.go
```

#### Run the Client

```bash
cd client
go run main.go
```

## Configuration

### Server Environment Variables

- `PORT`: Server port (default: 8080)
- `AGENT_NAME`: Agent identifier (default: ai-powered-agent)
- `AGENT_CLIENT_BASE_URL`: LLM API endpoint (default: https://api.openai.com)
- `AGENT_CLIENT_PROVIDER`: LLM provider (openai, anthropic, etc.)
- `AGENT_CLIENT_MODEL`: Model to use (gpt-4o-mini, claude-3-haiku, etc.)
- `AGENT_CLIENT_API_KEY`: Your API key (required)
- `LOG_LEVEL`: Logging verbosity (default: debug)

### Using Inference Gateway

If you have an Inference Gateway deployed:

```bash
export INFERENCE_GATEWAY_URL="https://your-gateway.com"
export AGENT_CLIENT_API_KEY="your-gateway-key"
```

## Example Output

The client sends three different prompts and displays AI responses:

```
--- Request 1 ---
Sending: What is the capital of France?
Response: The capital of France is Paris.

--- Request 2 ---
Sending: Write a haiku about programming
Response: Code flows like water,
Bugs surface, then disappear—
Logic finds its way.

--- Request 3 ---
Sending: Explain quantum computing in simple terms
Response: Quantum computing uses quantum bits (qubits) that can be both 0 and 1
simultaneously, unlike classical bits. This allows quantum computers to
process many calculations at once...
```

## Understanding the Code

### Server (`server/main.go`)

The server creates an AI agent and processes messages:

```go
// Build AI agent
agent := server.NewAgentBuilder().
    WithProvider(provider).
    WithModel(model).
    WithBaseURL(baseURL).
    WithAPIKey(apiKey).
    Build()

// Build server with AI handler
a2aServer := server.NewA2AServerBuilder().
    WithAgent(agent).
    Build()
```

### Client (`client/main.go`)

The client sends various prompts to test the AI:

```go
messages := []types.Message{
    {Role: "user", Content: "What is the capital of France?"},
    {Role: "user", Content: "Write a haiku about programming"},
    // ...
}
```

## Next Steps

- Try the `streaming` example for real-time AI responses
- Check `travel-planner` for a complex AI agent scenario
- Explore `artifacts` for handling file generation with AI