# A2A Paused Task Streaming Example

This example demonstrates how to handle A2A tasks that require user input during streaming conversations. It shows how to:

- Start a streaming conversation with an agent
- Handle tasks that pause for input (`input-required` state)
- Provide user input to resume the task with continued streaming
- Display real-time streaming responses
- Show conversation history throughout the process

## Features

- ✅ **Real-time streaming**: Shows live response chunks as they arrive
- ⏸️ **Pause handling**: Detects when tasks need user input
- 🔄 **Resume with streaming**: Continues streaming after providing input
- 📝 **Conversation history**: Displays message history for context
- 🚀 **Interactive flow**: User can provide multiple inputs as needed

## Usage

### Environment Variables

Set these environment variables to configure the example:

```bash
export A2A_SERVER_URL="http://localhost:8080"    # A2A server URL
export STREAMING_TIMEOUT="2m"                     # Max time for streaming operations
```

### Running the Example

Start the example travelplanner A2A server first:
```bash
cd examples/server/cmd/travelplanner
go run main.go
```

Then run the client example:
```bash
cd examples/client/cmd/pausedtask-streaming
go run main.go
```

### Example Flow

1. **Start**: The example sends an initial message requesting vacation planning help
2. **Streaming**: Agent responds with streaming chunks asking for preferences
3. **Pause**: Task pauses when agent needs specific input (destination, budget, etc.)
4. **Input**: User provides requested information
5. **Resume**: Task resumes with streaming response based on input
6. **Repeat**: Process continues until planning is complete

### Sample Output

```
🚀 Starting paused task streaming example...
📝 Initial request: I need help planning a vacation. Please ask me questions to understand my preferences and then create a detailed itinerary.

📡 Processing streaming events...

⚡ [Event 1] Task working (Task: abc-123)
💬 I'd be happy to help you plan a vacation! Let me ask you some questions to create the perfect itinerary...

⏸️ [Event 15] Task paused - input required (Task: abc-123)

⏸️ Task paused for input!
📋 Task ID: abc-123
💭 Agent says: What destination are you interested in? Also, what's your approximate budget and how many days would you like to travel?

📝 Recent conversation:
--------------------------------------------------
👤 User: I need help planning a vacation...
🤖 Assistant: I'd be happy to help you plan a vacation! Let me ask you some questions...
--------------------------------------------------

💬 Please provide your input (or 'quit' to exit): I want to visit Japan for 7 days with a budget of $3000

🔄 Resuming task with streaming...
💬 Great choice! Japan is wonderful. Based on your 7-day timeframe and $3000 budget, I'll create a detailed itinerary...

✅ Task completed!
```

## Key Implementation Details

### Streaming Event Handling

The example processes different types of streaming events:

- **Status updates**: Task state changes (working, paused, completed)
- **Message chunks**: Real-time response content
- **Tool execution**: When agent uses tools
- **Error handling**: Failed operations

### Pause Detection

Tasks pause when they reach `input-required` state:

```go
case "input-required":
    *taskPaused = true
    fmt.Printf("⏸️  Task paused - input required\n")
```

### Resume with Streaming

When resuming, the example continues streaming:

```go
err = a2aClient.SendTaskStreaming(resumeCtx, resumeParams, eventChan)
```

This ensures the user sees real-time responses even after providing input.

### Conversation Context

The example shows recent conversation history when paused, helping users understand what information the agent needs.

## Requirements

- A2A server with streaming support
- Agent that can request user input (uses `input_required` tool)
- Network connectivity to the A2A server

## Related Examples

- **[pausedtask](../pausedtask/)**: Non-streaming paused task handling
- **[streaming](../streaming/)**: Basic streaming without pauses
- **[async](../async/)**: Polling-based async task handling
