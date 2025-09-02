package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"
)

func main() {
	fmt.Println("✈️  Starting Travel Planning A2A Agent...")

	// Step 1: Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Step 2: Load configuration from environment
	cfg := config.Config{
		AgentName:        "travel-planner-agent",
		AgentDescription: "An AI-powered travel planning agent that helps create personalized vacation itineraries by gathering preferences through interactive conversations",
		AgentVersion:     "1.0.0",
		QueueConfig: config.QueueConfig{
			CleanupInterval: 5 * time.Minute,
		},
		ServerConfig: config.ServerConfig{
			Port: "8080",
		},
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, &cfg); err != nil {
		logger.Fatal("failed to process environment config", zap.Error(err))
	}

	// Step 3: Create specialized travel planning toolbox
	toolBox := createTravelPlanningToolBox()

	// Step 4: Create LLM client for AI capabilities
	llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.AgentConfig, logger)
	if err != nil {
		logger.Fatal("failed to create LLM client", zap.Error(err))
	}

	// Step 5: Create AI agent with travel planning expertise
	agent, err := server.NewAgentBuilder(logger).
		WithConfig(&cfg.AgentConfig).
		WithSystemPrompt(getTravelPlanningSystemPrompt()).
		WithLLMClient(llmClient).
		WithToolBox(toolBox).
		WithMaxConversationHistory(50). // Keep long context for travel planning
		WithMaxChatCompletion(15).      // Allow multiple iterations for complex planning
		Build()
	if err != nil {
		logger.Fatal("failed to create travel planning agent", zap.Error(err))
	}

	// Step 6: Create agent card with travel planning capabilities
	agentCard := types.AgentCard{
		Name:        cfg.AgentName,
		Description: cfg.AgentDescription,
		URL:         fmt.Sprintf("http://localhost:%s", cfg.ServerConfig.Port),
		Version:     cfg.AgentVersion,
		Capabilities: types.AgentCapabilities{
			Streaming:              &cfg.CapabilitiesConfig.Streaming,
			PushNotifications:      &cfg.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: &cfg.CapabilitiesConfig.StateTransitionHistory,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []types.AgentSkill{
			{
				ID:          "vacation-planning",
				Name:        "vacation-planning",
				Description: "Create comprehensive vacation itineraries based on user preferences",
				Tags:        []string{"travel", "planning", "itinerary", "vacation"},
				Examples:    []string{"Plan a 7-day trip to Italy", "Create a budget itinerary for Tokyo"},
			},
			{
				ID:          "destination-research",
				Name:        "destination-research",
				Description: "Research and recommend travel destinations based on preferences",
				Tags:        []string{"travel", "research", "destinations", "recommendations"},
				Examples:    []string{"Find warm destinations for winter travel", "Suggest cultural destinations in Europe"},
			},
			{
				ID:          "budget-planning",
				Name:        "budget-planning",
				Description: "Estimate costs and create budget breakdowns for trips",
				Tags:        []string{"travel", "budget", "cost", "planning"},
				Examples:    []string{"Estimate costs for 2-week Europe trip", "Budget breakdown for family vacation"},
			},
		},
	}

	// Step 7: Build the A2A server
	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithAgent(agent).
		WithAgentCard(agentCard).
		WithDefaultTaskHandlers().
		Build()
	if err != nil {
		logger.Fatal("failed to build A2A server", zap.Error(err))
	}

	// Handle shutdown gracefully
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("shutting down travel planning agent server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("failed to shutdown server gracefully", zap.Error(err))
	}

	logger.Info("travel planning agent server stopped")
	fmt.Println("✈️  Travel Planning Agent stopped. Safe travels! 🌟")
}

func createTravelPlanningToolBox() server.ToolBox {
	toolBox := server.NewDefaultToolBox()

	// Add weather information tool
	weatherTool := server.NewBasicTool(
		"get_destination_weather",
		"Get current weather information and seasonal climate data for a travel destination",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"destination": map[string]any{
					"type":        "string",
					"description": "The destination city or location",
				},
				"travel_dates": map[string]any{
					"type":        "string",
					"description": "Optional travel dates to check seasonal weather",
				},
			},
			"required": []string{"destination"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			dest := args["destination"].(string)

			// Mock weather data - in production, this would call a real weather API
			// Convert to JSON string
			return fmt.Sprintf(`{"destination": "%s", "current_weather": {"temperature": "22°C (72°F)", "conditions": "Partly cloudy", "humidity": "65%%"}, "seasonal_info": {"best_time_to_visit": "April-October for mild weather", "peak_season": "June-August", "rainy_season": "November-March"}, "travel_tips": ["Pack layers for variable weather", "Umbrella recommended for spring visits", "Comfortable walking shoes essential"]}`, dest), nil
		},
	)
	toolBox.AddTool(weatherTool)

	// Add budget estimation tool
	budgetTool := server.NewBasicTool(
		"estimate_travel_budget",
		"Estimate budget breakdown for a trip including flights, accommodation, food, and activities",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"destination": map[string]any{
					"type":        "string",
					"description": "Travel destination",
				},
				"duration": map[string]any{
					"type":        "number",
					"description": "Trip duration in days",
				},
				"travelers": map[string]any{
					"type":        "number",
					"description": "Number of travelers",
				},
				"style": map[string]any{
					"type":        "string",
					"description": "Travel style: budget, mid-range, or luxury",
					"enum":        []string{"budget", "mid-range", "luxury"},
				},
			},
			"required": []string{"destination", "duration", "travelers", "style"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			dest := args["destination"].(string)
			duration := int(args["duration"].(float64))
			travelers := int(args["travelers"].(float64))
			style := args["style"].(string)

			// Mock budget calculation - in production, this would use real pricing data
			baseCostPerDay := map[string]int{
				"budget":    50,
				"mid-range": 150,
				"luxury":    400,
			}

			dailyCost := baseCostPerDay[style]
			totalCost := dailyCost * duration * travelers

			return fmt.Sprintf(`{"destination": "%s", "duration_days": %d, "travelers": %d, "style": "%s", "total_estimated": "$%d", "breakdown": {"accommodation": "$%d", "food": "$%d", "activities": "$%d", "transport": "$%d"}, "daily_average": "$%d per person per day", "notes": ["Estimates are approximate and vary by season", "Flight costs not included - add $200-2000 depending on distance", "Consider travel insurance ($50-200)", "Emergency fund recommended (10-20%% extra)"]}`,
				dest, duration, travelers, style, totalCost,
				totalCost*40/100, totalCost*30/100, totalCost*20/100, totalCost*10/100,
				dailyCost), nil
		},
	)
	toolBox.AddTool(budgetTool)

	// Add activity recommendation tool
	activitiesTool := server.NewBasicTool(
		"get_destination_activities",
		"Get recommended activities and attractions for a travel destination",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"destination": map[string]any{
					"type":        "string",
					"description": "Travel destination",
				},
				"interests": map[string]any{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "Travel interests (culture, adventure, food, nature, etc.)",
				},
				"duration": map[string]any{
					"type":        "number",
					"description": "Available days for activities",
				},
			},
			"required": []string{"destination"},
		},
		func(ctx context.Context, args map[string]any) (string, error) {
			dest := args["destination"].(string)

			// Mock activity recommendations - in production, this would use tourism APIs
			return fmt.Sprintf(`{"destination": "%s", "top_attractions": [{"name": "Historic Old Town", "type": "culture", "duration": "2-3 hours", "cost": "Free", "description": "Explore medieval streets and historic architecture"}, {"name": "Local Food Market", "type": "food", "duration": "1-2 hours", "cost": "$20-40", "description": "Sample regional specialties and local ingredients"}, {"name": "Scenic Hiking Trail", "type": "nature", "duration": "Half day", "cost": "Free", "description": "Beautiful nature walk with panoramic views"}], "hidden_gems": ["Local artisan workshop", "Secret rooftop bar with city views", "Traditional family restaurant off tourist path"], "seasonal_activities": {"spring": ["Garden tours", "Outdoor festivals"], "summer": ["Beach activities", "Open-air concerts"], "fall": ["Harvest festivals", "Wine tastings"], "winter": ["Christmas markets", "Museum visits"]}}`, dest), nil
		},
	)
	toolBox.AddTool(activitiesTool)

	return toolBox
}

func getTravelPlanningSystemPrompt() string {
	return `You are a knowledgeable and enthusiastic travel planning assistant specializing in creating personalized vacation itineraries. Your expertise includes:

🌍 CORE EXPERTISE:
- Destination research and recommendations
- Budget planning and cost estimation  
- Activity and attraction suggestions
- Cultural insights and local experiences
- Practical travel logistics
- Seasonal travel considerations

💬 CONVERSATION STYLE:
- Friendly, enthusiastic, and helpful
- Ask thoughtful questions to understand preferences
- Provide detailed, actionable advice
- Share insider tips and hidden gems
- Be culturally sensitive and inclusive

🔄 INTERACTIVE PLANNING PROCESS:
When users request vacation planning help, gather information systematically:

1. **Destination Preferences**: Where they want to go (or help them choose)
2. **Budget Range**: Their spending comfort level
3. **Trip Duration**: How long they can travel
4. **Travel Style**: Adventure, relaxation, culture, food, etc.
5. **Group Details**: Solo, couple, family, friends
6. **Special Interests**: Specific activities or experiences they want
7. **Practical Needs**: Accommodation preferences, dietary restrictions, accessibility

⏸️ SMART PAUSING:
Use the input_required tool when you need specific information to create a good itinerary:
- After initial greeting, pause to gather basic preferences
- When multiple destination options exist, pause for user choice
- For budget planning, pause to understand their comfort level
- When clarification is needed on group size or special requirements

🛠️ TOOL USAGE:
- Use get_destination_weather for climate and seasonal information
- Use estimate_travel_budget for cost breakdowns
- Use get_destination_activities for attractions and experiences
- Always provide context and reasoning for your recommendations

🎯 FINAL DELIVERABLE:
Create comprehensive, day-by-day itineraries including:
- Accommodation suggestions
- Daily activity schedules
- Budget breakdown
- Practical tips and recommendations
- Local cultural insights
- Transportation guidance

Remember: Great travel planning is about understanding the person, not just the place. Ask questions that help you create truly personalized experiences!`
}
