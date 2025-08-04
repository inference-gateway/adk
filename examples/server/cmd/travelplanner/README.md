# Travel Planning A2A Agent

A specialized A2A (Agent-to-Agent) server that demonstrates domain expertise in travel planning. This agent is designed to work perfectly with the paused task streaming client example, providing intelligent vacation planning through interactive conversations.

## Features

✈️ **Travel Planning Expertise**
- Personalized vacation itinerary creation
- Destination research and recommendations  
- Budget planning and cost estimation
- Activity and attraction suggestions
- Cultural insights and local experiences

🤖 **AI-Powered Intelligence**
- LLM-driven conversation and planning
- Smart pausing to gather user preferences
- Tool calling for weather, budget, and activity data
- Long conversation context for complex planning

🔄 **Interactive Planning Process**
- Intelligently pauses to gather preferences
- Asks clarifying questions about destinations, budget, interests
- Provides detailed, actionable travel advice
- Creates comprehensive day-by-day itineraries

## Perfect Match for Streaming Client

This server is specifically designed to complement the `pausedtask-streaming` client example:

- **Client Request**: "I need help planning a vacation..."
- **Server Response**: Asks clarifying questions about preferences
- **Pause Points**: Destination choice, budget range, travel style, group size
- **Final Output**: Complete travel itinerary with budget breakdown

## Available Tools

### 🌤️ Weather Information
- `get_destination_weather`: Current weather and seasonal climate data
- Provides best travel times, weather patterns, packing tips

### 💰 Budget Estimation  
- `estimate_travel_budget`: Cost breakdown for trips
- Calculates accommodation, food, activities, transport costs
- Supports budget, mid-range, and luxury travel styles

### 🎯 Activity Recommendations
- `get_destination_activities`: Attractions and experiences
- Suggests activities based on interests and trip duration
- Includes cultural sites, food experiences, outdoor activities

## Agent Skills

The agent card advertises three main capabilities:

1. **Vacation Planning**: Complete itinerary creation based on preferences
2. **Destination Research**: Recommendations based on climate, activities, budget
3. **Budget Planning**: Cost estimates and financial planning for trips

## Usage

### Environment Setup

```bash
# Required: Configure your LLM provider
export AGENT_CLIENT_PROVIDER=deepseek  # or openai, anthropic, etc.
export AGENT_CLIENT_MODEL=deepseek-chat
export AGENT_CLIENT_BASE_URL=http://localhost:8081/v1
export AGENT_CLIENT_API_KEY=your-api-key

# Optional: Server configuration
export SERVER_PORT=8080
```

### Start the Agent

```bash
cd examples/server/cmd/travelplanner
go run main.go
```

### Test with Client Examples

This server works best with the paused task streaming client:

```bash
# In another terminal
cd examples/client/cmd/pausedtask-streaming
go run main.go
```

The client will send: "I need help planning a vacation..." and the server will intelligently guide the user through the planning process.

## Example Conversation Flow

1. **Initial Request**: Client asks for vacation planning help
2. **Destination Gathering**: Agent asks about preferred destinations or helps choose
3. **Preference Collection**: Budget range, trip duration, travel style, group size
4. **Tool Usage**: Agent calls weather, budget, and activity tools
5. **Itinerary Creation**: Comprehensive day-by-day plan with costs
6. **Refinement**: Additional questions if needed for a perfect itinerary

## Architecture

```
Travel Planning Agent
├── System Prompt (Travel expertise instructions)
├── Specialized Tools
│   ├── Weather API simulation
│   ├── Budget calculator
│   └── Activity recommender
├── Smart Pausing Logic
│   ├── Input gathering
│   ├── Preference clarification
│   └── Decision points
└── A2A Protocol
    ├── Streaming support
    ├── Input-required states
    └── Conversation history
```

## Production Considerations

### Real Data Integration
- Replace mock weather data with actual weather APIs
- Integrate real tourism and activity databases
- Connect to actual travel booking systems
- Add real-time pricing data

### Enhanced Features
- Multi-language support for international travelers
- Accessibility information for travelers with disabilities
- Real-time flight and accommodation booking
- Group travel coordination features
- Travel document and visa requirement checking

### Scaling
- Add caching for weather and activity data
- Implement rate limiting for external APIs
- Add monitoring and analytics
- Support multiple concurrent planning sessions

## Testing the Agent

### Manual Testing
```bash
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send", 
    "params": {
      "message": {
        "kind": "message",
        "messageId": "travel-test",
        "role": "user",
        "parts": [{"kind": "text", "text": "Help me plan a vacation to Japan"}]
      }
    },
    "id": 1
  }'
```

### Agent Information
```bash
curl http://localhost:8080/.well-known/agent.json | jq .
```

## Related Examples

- **Client Side**: `examples/client/cmd/pausedtask-streaming` - Perfect client for this server
- **Server Side**: `examples/server/cmd/pausedtask` - Generic paused task server
- **Simple Streaming**: `examples/client/cmd/streaming` - Basic streaming without pauses

This travel planning agent demonstrates how to create domain-specific A2A agents that provide real value through specialized knowledge and interactive conversations.
