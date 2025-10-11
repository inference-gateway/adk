# Input-Required Non-Streaming Example

This example demonstrates the input-required flow in traditional request-response mode, where tasks can pause to request additional information from users.

## What This Example Shows

- **Non-streaming input-required flow**: Traditional request-response with task pausing
- **Interactive conversation**: Multi-turn conversations with context preservation
- **Task state management**: How tasks transition through input-required states
- **Built-in tool usage**: Leveraging the `input_required` tool (with AI)
- **Manual demonstration**: Shows the flow without AI for learning purposes

## Running the Example

### Using Docker Compose (Recommended)

Start the server and inference gateway:

```bash
docker compose up --build
```

This will start:

- **Server**: A2A server with input-required capabilities
- **Inference Gateway**: For AI capabilities (optional)

To run the interactive client (in a separate terminal):

```bash
docker compose run --rm --build client
```

### Running Locally

#### Start the Server

```bash
cd server
go run main.go
```

#### Run the Client

```bash
cd client
go run main.go
```

## How It Works

### Without AI (Demo Mode)

The server demonstrates input-required logic manually:

- **Weather queries**: Requests location if not provided
- **Calculations**: Requests numbers if not provided
- **Unclear requests**: Asks for clarification

### With AI (AI Mode)

When AI is configured, the agent uses the built-in `input_required` tool:

- **Automatic detection**: AI determines when information is missing
- **Intelligent prompting**: AI asks specific, contextual questions
- **Natural conversation**: More fluid and natural interactions

## Configuration

### Enable AI

Uncomment these lines in `docker-compose.yaml`:

```yaml
environment:
  - A2A_AGENT_CLIENT_PROVIDER=openai
  - A2A_AGENT_CLIENT_MODEL=gpt-4o-mini
  - A2A_AGENT_CLIENT_BASE_URL=http://inference-gateway:8080/v1
```

And add your API key to the inference-gateway service:

```yaml
inference-gateway:
  environment:
    - OPENAI_API_KEY=${OPENAI_API_KEY}
```

### Environment Variables

| Variable                     | Description       | Default |
| ---------------------------- | ----------------- | ------- |
| `A2A_SERVER_PORT`            | Server port       | `8080`  |
| `A2A_DEBUG`                  | Debug logging     | `true`  |
| `A2A_CAPABILITIES_STREAMING` | Streaming support | `false` |

## Example Interactions

### Weather Query

```
💬 Your message: What's the weather?
📤 Sending: What's the weather?
🆔 Task ID: task-abc123
📊 Task Status: input_required
❓ Input Required: I'd be happy to help you with the weather! Could you please specify which location you'd like the weather for?
💬 Your response: New York
📤 Sending follow-up: New York
🔄 Continuing with Task ID: task-def456
📊 Task Status: completed
✅ Response: The weather is sunny and 72°F! (This is a demo response - no real weather data is fetched)
```

### Calculation Request

```
💬 Your message: Calculate something
📤 Sending: Calculate something
🆔 Task ID: task-xyz789
📊 Task Status: input_required
❓ Input Required: I can help you with calculations! Could you please provide the specific numbers or equation you'd like me to calculate?
💬 Your response: 15 * 23
📤 Sending follow-up: 15 * 23
🔄 Continuing with Task ID: task-uvw012
📊 Task Status: completed
✅ Response: Based on your calculation request, I can help you with that math problem! (This is a demo response)
```

## Understanding the Code

### Server Implementation

The server demonstrates both AI and manual approaches:

```go
// With AI agent
func (h *InputRequiredTaskHandler) processWithAgent(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
    // Agent automatically uses input_required tool when needed
    eventChan, err := h.agent.RunWithStream(toolCtx, messages)
    // Handle events including input_required
}

// Without AI (demo mode)
func (h *InputRequiredTaskHandler) processWithoutAgent(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
    // Manual logic to demonstrate input-required flow
    switch messageContent {
    case "weather":
        if !hasLocation {
            // Create input_required message
            inputMessage := &types.Message{Kind: "input_required", ...}
            task.Status.State = types.TaskStateInputRequired
        }
    }
}
```

### Client Implementation

The client handles the complete input-required flow:

```go
func demonstrateInputRequiredFlow(a2aClient client.A2AClient, initialMessage string, logger *zap.Logger) error {
    // Create message from user input
    message := types.Message{
        Role:      "user",
        Parts:     []types.Part{types.NewTextPart(initialMessage)},
    }

    // Send initial message
    params := types.MessageSendParams{Message: message}
    response, err := a2aClient.SendTask(ctx, params)

    // Extract task ID from response
    var taskResult struct {
        ID string `json:"id"`
    }
    json.Unmarshal(response.Result.(json.RawMessage), &taskResult)
    taskID := taskResult.ID

    // Manual polling loop
    for {
        time.Sleep(500 * time.Millisecond)

        // Get task status
        taskResponse, err := a2aClient.GetTask(ctx, types.TaskQueryParams{ID: taskID})

        var currentTask types.Task
        json.Unmarshal(taskResponse.Result.(json.RawMessage), &currentTask)

        switch currentTask.Status.State {
        case types.TaskStateInputRequired:
            // Get user input and send follow-up
            followUpResponse, err := a2aClient.SendTask(ctx, followUpParams)
            json.Unmarshal(followUpResponse.Result.(json.RawMessage), &taskResult)
            taskID = taskResult.ID

        case types.TaskStateCompleted:
            // Display final response
            return nil
        }
    }
}
```

## Troubleshooting

### Task Stuck in Working State

- Check server logs for processing errors
- Verify agent configuration if using AI
- Ensure proper message format

### Input-Required Not Triggered

- Verify the request triggers the condition (e.g., "weather" without location)
- Check if AI is configured and working properly
- Review system prompt for AI agent

### Context Not Preserved

- Ensure using the same `ContextID` for follow-up messages
- Check that task history is properly maintained

## Next Steps

- Try the **streaming** version for real-time interactions
- Explore the **ai-powered** example for advanced AI integration
- Check the main **input-required** README for comprehensive documentation
