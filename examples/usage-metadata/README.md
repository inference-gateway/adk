# Usage Metadata Example

This example demonstrates how to track and retrieve **token usage** and **execution metrics** from A2A task responses using the `Task.Metadata` field.

## Table of Contents

- [What This Example Shows](#what-this-example-shows)
- [Features](#features)
- [Directory Structure](#directory-structure)
- [Running the Example](#running-the-example)
- [Understanding Usage Metadata](#understanding-usage-metadata)
- [Metadata Structure](#metadata-structure)
- [Configuration](#configuration)
- [Use Cases](#use-cases)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

## What This Example Shows

- Automatic token usage tracking from LLM responses
- Execution statistics collection (iterations, messages, tool calls)
- Metadata population in completed task responses
- Configuration options for enabling/disabling usage tracking
- How to access usage data in both background and streaming tasks

## Features

The usage metadata feature provides:

- **Token Usage**: Track LLM token consumption
  - `prompt_tokens`: Tokens used in the prompt
  - `completion_tokens`: Tokens generated in the response
  - `total_tokens`: Total tokens consumed
- **Execution Statistics**: Monitor agent behavior
  - `iterations`: Number of agent execution loops
  - `messages`: Total messages processed
  - `tool_calls`: Number of tool invocations
  - `failed_tools`: Failed tool execution count

## Directory Structure

```
usage-metadata/
├── server/
│   ├── main.go         # A2A server with usage tracking
│   └── config/
│       └── config.go   # Configuration structure
├── client/
│   └── main.go         # Client demonstrating metadata access
├── docker-compose.yaml # Docker setup with Inference Gateway
├── .env.example        # Environment configuration template
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

# Configure agent
A2A_AGENT_CLIENT_PROVIDER=openai
A2A_AGENT_CLIENT_MODEL=gpt-4o-mini

# Usage metadata is enabled by default
A2A_AGENT_CLIENT_ENABLE_USAGE_METADATA=true
```

3. Run the example:

```bash
docker-compose up --build
```

This will:

1. Start the Inference Gateway with your configured providers
2. Start the A2A server with usage metadata tracking enabled
3. Run the client to submit tasks and display usage statistics

### Running Locally

#### Start the Server

```bash
cd server
export A2A_AGENT_CLIENT_PROVIDER=openai
export A2A_AGENT_CLIENT_MODEL=gpt-4o-mini
export A2A_AGENT_CLIENT_BASE_URL=http://localhost:8080/v1
export A2A_AGENT_CLIENT_ENABLE_USAGE_METADATA=true
go run main.go
```

#### Run the Client

```bash
cd client
export A2A_SERVER_URL=http://localhost:8080
go run main.go
```

## Understanding Usage Metadata

Usage metadata is automatically collected during task execution and populated in the `Task.Metadata` field when the task completes. The ADK's `UsageTracker` component:

1. **Tracks Token Usage**: Captures token counts from each LLM response
2. **Monitors Execution**: Counts iterations, messages, and tool calls
3. **Aggregates Metrics**: Combines data from multiple LLM calls in a single task
4. **Populates Metadata**: Injects metrics into the task before returning results

### How It Works

**For Background Tasks:**

```go
handler := server.NewDefaultBackgroundTaskHandler(logger, agent)
// Usage metadata is automatically tracked and populated in Task.Metadata
```

**For Streaming Tasks:**

```go
handler := server.NewDefaultStreamingTaskHandler(logger, agent)
// Usage metadata is tracked during streaming and available after completion
```

**In Your Code:**

The `UsageTracker` is automatically injected into the agent execution context and collects metrics transparently. No additional code is required in your task handlers.

## Metadata Structure

The metadata is returned in the `Task.Metadata` field with the following structure:

```json
{
  "usage": {
    "prompt_tokens": 156,
    "completion_tokens": 89,
    "total_tokens": 245
  },
  "execution_stats": {
    "iterations": 2,
    "messages": 4,
    "tool_calls": 1,
    "failed_tools": 0
  }
}
```

### Field Descriptions

**usage** (only present when LLM calls were made):

- `prompt_tokens` (int64): Total tokens used in prompts across all LLM calls
- `completion_tokens` (int64): Total tokens generated in responses
- `total_tokens` (int64): Sum of prompt and completion tokens

**execution_stats** (always present):

- `iterations` (int): Number of agent execution loops
- `messages` (int): Total messages processed during execution
- `tool_calls` (int): Number of tools invoked by the agent
- `failed_tools` (int): Number of tool executions that failed

## Configuration

Usage metadata tracking is controlled by the `AGENT_CLIENT_ENABLE_USAGE_METADATA` environment variable:

| Environment Variable                     | Description                    | Default |
| ---------------------------------------- | ------------------------------ | ------- |
| `A2A_AGENT_CLIENT_ENABLE_USAGE_METADATA` | Enable usage metadata tracking | `true`  |

### Disabling Usage Metadata

To disable usage tracking:

```bash
export A2A_AGENT_CLIENT_ENABLE_USAGE_METADATA=false
```

Or in your `.env` file:

```bash
A2A_AGENT_CLIENT_ENABLE_USAGE_METADATA=false
```

When disabled, the `Task.Metadata` field will not include usage information.

## Use Cases

### Cost Monitoring

Track token usage to calculate LLM API costs:

```go
usage := task.Metadata["usage"].(map[string]any)
totalTokens := usage["total_tokens"].(int64)
cost := calculateCost(totalTokens, model)
```

### Performance Analysis

Monitor agent efficiency:

```go
stats := task.Metadata["execution_stats"].(map[string]any)
iterations := stats["iterations"].(int)
if iterations > threshold {
    log.Println("Task required many iterations, consider optimization")
}
```

### Resource Management

Set limits based on usage:

```go
usage := task.Metadata["usage"].(map[string]any)
if usage["total_tokens"].(int64) > maxTokens {
    return errors.New("token limit exceeded")
}
```

### Debugging

Analyze agent behavior:

```go
stats := task.Metadata["execution_stats"].(map[string]any)
log.Printf("Task completed in %d iterations with %d tool calls",
    stats["iterations"], stats["tool_calls"])
```

## Troubleshooting

### Metadata Field is Nil

- **Cause**: Usage metadata may be disabled or no metrics were collected
- **Solution**: Ensure `A2A_AGENT_CLIENT_ENABLE_USAGE_METADATA=true` and the task completed successfully

### Usage Field is Missing

- **Cause**: The `usage` field only appears when LLM calls were made
- **Solution**: Check that your agent is actually calling the LLM client and receiving responses with usage data

### Execution Stats Are Zero

- **Cause**: Task may have failed or been cancelled before execution
- **Solution**: Check `task.Status.State` and error logs

### Token Counts Don't Match Expectations

- **Cause**: Different LLM providers may calculate tokens differently
- **Solution**: Token counts reflect what the LLM provider reports in their API responses

## Next Steps

- Try the `ai-powered` example to see usage metadata with tool execution
- Check the `streaming` example for real-time usage tracking
- Explore the `callbacks` example to intercept and log usage data in real-time
- Review the integration tests in `server/usage_metadata_integration_test.go` for more examples
