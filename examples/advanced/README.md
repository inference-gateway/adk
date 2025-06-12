# Advanced A2A Server Example

This example demonstrates an advanced A2A (Agent-to-Agent) server configuration with custom handlers, enhanced capabilities, and intelligent message processing.

## Features

- 🔧 **Custom Task Handler** - Intelligent message processing with context-aware responses
- 🎯 **Custom Result Processor** - Advanced tool result processing and completion logic
- 📋 **Custom Agent Provider** - Enhanced agent metadata with extended capabilities
- 🧠 **Smart Message Routing** - Content-based response generation
- 📈 **Enhanced Logging** - Development-mode logging with detailed output
- ⚡ **Performance Optimized** - Larger queue size and optimized timeouts
- 🔄 **State Management** - Enhanced state transition history

## Running the Example

```bash
cd examples/advanced
go run main.go
```

The server will start on `http://localhost:8080` with enhanced capabilities.

## Enhanced Features

### Custom Task Handler

The advanced example includes a custom task handler that:

- Analyzes message content for intelligent routing
- Provides context-aware responses based on keywords
- Includes metadata in responses for enhanced debugging
- Maintains detailed task history with custom processing information

### Smart Message Processing

- **Help Requests**: Detects help-related queries and provides assistance
- **Greetings**: Recognizes greetings and responds appropriately
- **Long Messages**: Handles detailed messages with comprehensive processing
- **Empty Messages**: Gracefully handles empty or malformed input

### Custom Agent Capabilities

- Extended input/output modes (JSON, Markdown, HTML)
- Enhanced skill definitions
- Custom versioning and branding
- Advanced capability flags

## Available Endpoints

- `GET /health` - Health check endpoint
- `GET /.well-known/agent.json` - **Enhanced** agent capabilities with custom skills
- `POST /a2a` - A2A protocol endpoint with **custom processing logic**

## Testing the Advanced Features

### Send a Help Request

```bash
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "id": "help-1",
    "params": {
      "message": {
        "kind": "message",
        "messageId": "msg-help",
        "role": "user",
        "parts": [
          {
            "kind": "text",
            "text": "I need help with A2A protocol"
          }
        ]
      }
    }
  }'
```

### Send a Greeting

```bash
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "id": "greeting-1",
    "params": {
      "message": {
        "kind": "message",
        "messageId": "msg-hello",
        "role": "user",
        "parts": [
          {
            "kind": "text",
            "text": "Hello there!"
          }
        ]
      }
    }
  }'
```

### Send a Complex Message

```bash
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "id": "complex-1",
    "params": {
      "message": {
        "kind": "message",
        "messageId": "msg-complex",
        "role": "user",
        "parts": [
          {
            "kind": "text",
            "text": "This is a very detailed message that contains a lot of information about what I want to accomplish with the A2A agent system and requires comprehensive processing with multiple steps and considerations."
          }
        ]
      }
    }
  }'
```

## Configuration Highlights

The advanced example uses enhanced configuration:

- **Queue Size**: 500 (5x larger than standard)
- **Cleanup Interval**: 15 seconds (2x faster)
- **Timeouts**: Extended for complex processing
- **LLM Integration**: Configured for OpenAI GPT-4
- **Enhanced Capabilities**: All advanced features enabled
- **Development Logging**: Detailed debugging output

## Custom Components

### CustomTaskHandler

- Intelligent content analysis
- Context-aware response generation
- Enhanced metadata tracking
- Performance metrics logging

### CustomTaskResultProcessor

- Advanced tool result processing
- Custom completion logic
- Enhanced result formatting
- Timestamp tracking

### CustomAgentInfoProvider

- Extended agent capabilities
- Custom skill definitions
- Enhanced metadata
- Professional branding

## Production Considerations

For production deployment, consider:

1. **Enable TLS** - Set `TLSConfig.Enable = true` with proper certificates
2. **Enable Authentication** - Configure OIDC authentication
3. **Environment Variables** - Use environment-based configuration
4. **Health Monitoring** - Implement proper health checks and metrics
5. **Error Handling** - Add comprehensive error handling and recovery
6. **Rate Limiting** - Implement request rate limiting
7. **Logging** - Switch to production logging configuration

## Next Steps

- Integrate with your LLM provider (OpenAI, Anthropic, etc.)
- Add custom tools and function calling
- Implement persistent task storage
- Add monitoring and observability
- Build custom business logic handlers
