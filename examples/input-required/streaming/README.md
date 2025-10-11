# Input-Required Streaming Example

This example demonstrates the input-required flow with real-time streaming, where agents can pause mid-stream to request additional information from users.

## What This Example Shows

- **Streaming input-required flow**: Real-time streaming that can pause for user input
- **Live interaction**: See responses appear character-by-character
- **Mid-stream pausing**: Streams can pause to request additional information
- **Event-driven architecture**: Handle different types of streaming events
- **Conversation continuity**: Maintain context across streaming sessions

## Running the Example

### Using Docker Compose (Recommended)

Start the server and inference gateway:

```bash
docker compose up --build
```

This will start:

- **Server**: A2A streaming server with input-required capabilities
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

### Streaming Flow

1. **Client sends message** via streaming endpoint
2. **Server processes** and starts streaming response
3. **Real-time deltas** are sent as the response is generated
4. **Stream pauses** if input is required
5. **User provides input** to continue
6. **Stream resumes** with additional context
7. **Stream completes** when task is finished

### Events

The streaming client handles multiple event types:

- **`EventDelta`**: Real-time text chunks
- **`EventInputRequired`**: Input required from user
- **`EventTaskStatusChanged`**: Task state updates
- **`EventIterationCompleted`**: Processing iteration complete
- **`EventStreamFailed`**: Stream error

## Configuration

### Enable AI

**Option 1: Using .env file (Recommended)**

1. Copy the example environment file:

   ```bash
   cp .env.example .env
   ```

2. Edit `.env` and configure your API key and agent settings:

   ```bash
   # Add your API key
   OPENAI_API_KEY=your-openai-api-key-here
   DEEPSEEK_API_KEY=your-deepseek-api-key-here

   # Configure the agent
   A2A_AGENT_CLIENT_PROVIDER=openai
   A2A_AGENT_CLIENT_MODEL=gpt-4o-mini
   ```

3. The docker-compose.yaml will automatically load these values

**Option 2: Edit docker-compose.yaml directly**

Uncomment these lines in `docker-compose.yaml`:

```yaml
environment:
  - A2A_AGENT_CLIENT_PROVIDER=openai
  - A2A_AGENT_CLIENT_MODEL=gpt-4o-mini
  - A2A_AGENT_CLIENT_BASE_URL=http://inference-gateway:8080/v1
```

And add your API key:

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
| `A2A_CAPABILITIES_STREAMING` | Streaming support | `true`  |

## Example Interactions

### Weather Query with Streaming

```
💬 Your message: What's the weather?
📤 Sending: What's the weather?
📥 Streaming response: I'd be happy to help you with the weather!
❓ Input Required: Could you please specify which location you'd like the weather for?
💬 Your response: New York
📤 Sending follow-up: New York
📥 Continued streaming: Let me check the weather for you... The weather is sunny and 72°F! (This is a demo response)
✅ Conversation complete!
```

### Real-time Calculation

```
💬 Your message: Calculate something
📤 Sending: Calculate something
📥 Streaming response: I can help you with calculations!
❓ Input Required: Could you please provide the specific numbers or equation you'd like me to calculate?
💬 Your response: 25 * 8
📤 Sending follow-up: 25 * 8
📥 Continued streaming: Let me work on that calculation... Based on your calculation request, I can help you with that math problem! (This is a demo response)
✅ Stream completed!
```

### Simple Greeting (No Input Required)

```
💬 Your message: Hello
📤 Sending: Hello
📥 Streaming response: Hello! I'm an assistant that demonstrates the input-required flow with streaming. Try asking me about the weather or a calculation to see how I request additional information!
✅ Response complete!
```

## Understanding the Code

### Streaming Server Implementation

The server handles streaming with input-required pausing:

```go
func (h *StreamingInputRequiredTaskHandler) HandleStreamingTask(ctx context.Context, task *types.Task, message *types.Message) (<-chan cloudevents.Event, error) {
    outputChan := make(chan cloudevents.Event, 100)

    go func() {
        defer close(outputChan)

        if h.agent != nil {
            // AI agent handles streaming automatically
            h.processWithAgentStreaming(ctx, task, message, outputChan)
        } else {
            // Manual demonstration with streaming
            h.processWithoutAgentStreaming(ctx, task, message, outputChan)
        }
    }()

    return outputChan, nil
}
```

### Streaming Events

The server sends different types of events:

```go
// Real-time text streaming
func (h *StreamingInputRequiredTaskHandler) sendStreamingText(outputChan chan<- cloudevents.Event, taskID, text string) {
    // Split text into chunks for realistic streaming
    words := strings.Fields(text)
    for _, word := range words {
        deltaMessage := &types.Message{Kind: "message", ...}
        event := types.NewDeltaEvent(deltaMessage)
        outputChan <- event
        time.Sleep(50 * time.Millisecond) // Simulate typing
    }
}

// Input required event
func (h *StreamingInputRequiredTaskHandler) sendInputRequiredEvent(outputChan chan<- cloudevents.Event, taskID, message string) {
    inputMessage := &types.Message{Kind: "input_required", ...}
    event := types.NewMessageEvent(types.EventInputRequired, inputMessage.MessageID, inputMessage)
    outputChan <- event
}
```

### Streaming Client Implementation

The client handles real-time events and input collection:

```go
func demonstrateStreamingInputRequiredFlow(a2aClient *client.A2AClient, initialMessage string, logger *zap.Logger) error {
    // Start streaming
    eventChan, err := a2aClient.SendMessageStreaming(ctx, params)

    // Process events in real-time
    for event := range eventChan {
        switch event.Type() {
        case types.EventDelta:
            // Display text as it streams
            var msg types.Message
            if err := event.DataAs(&msg); err == nil {
                text := extractMessageText(&msg)
                fmt.Print(text) // Real-time display
            }

        case types.EventInputRequired:
            // Handle input required
            var msg types.Message
            if err := event.DataAs(&msg); err == nil {
                inputRequiredMessage = extractMessageText(&msg)
                // Get user input and continue streaming
            }
        }
    }
}
```

## Advanced Features

### Mid-Stream Pausing

Unlike traditional input-required flows, streaming can pause mid-response:

```
📥 Streaming response: I can help you with the weather in
❓ Input Required: Could you please specify which city?
💬 Your response: Boston
📥 Continued streaming: Boston. The current weather is sunny and 68°F!
```

### Event Handling

The client demonstrates handling all streaming events:

- **Text deltas**: Display real-time text
- **Status updates**: Show processing state
- **Input requests**: Pause for user input
- **Error handling**: Graceful error recovery

### Context Preservation

Streaming maintains conversation context:

```go
// Continue streaming with same context
followUpParams := types.MessageStreamParams{
    ContextID: currentContextID, // Preserve context
    Message:   followUpMessage,
}

continuedEventChan, err := a2aClient.SendMessageStreaming(ctx, followUpParams)
```

## Troubleshooting

### Streaming Not Working

- Verify `A2A_CAPABILITIES_STREAMING=true` is set
- Check that server implements `StreamableTaskHandler`
- Ensure client properly handles streaming events

### Input-Required Not Pausing Stream

- Check that `EventInputRequired` events are being sent
- Verify client handles the event type correctly
- Review server logic for input-required detection

### Context Lost Between Streams

- Ensure same `ContextID` is used for follow-up messages
- Check that task history is maintained
- Verify conversation state persistence

### Real-time Display Issues

- Check for buffer flushing in output display
- Verify proper event ordering
- Review timing of delta events

## Performance Considerations

### Streaming Efficiency

- **Event buffering**: Balance real-time display with performance
- **Network optimization**: Consider event batching for high-volume streams
- **Memory management**: Properly close channels and clean up resources

### User Experience

- **Typing simulation**: Add realistic delays between text chunks
- **Clear indicators**: Show when waiting for input vs. streaming
- **Error recovery**: Handle network interruptions gracefully

## Next Steps

- Try the **non-streaming** version for comparison
- Explore **ai-powered-streaming** for advanced AI integration
- Check the main **input-required** README for comprehensive documentation
- Review **streaming** example for basic streaming concepts
