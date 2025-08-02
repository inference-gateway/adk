package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	adk "github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

// MockTaskManager manages tasks and their state transitions for the mock server
type MockTaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*adk.Task
	ctx   context.Context
}

// NewMockTaskManager creates a new mock task manager
func NewMockTaskManager(ctx context.Context) *MockTaskManager {
	return &MockTaskManager{
		tasks: make(map[string]*adk.Task),
		ctx:   ctx,
	}
}

// CreateTask creates a new task and starts its state progression
func (m *MockTaskManager) CreateTask(message adk.Message) *adk.Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	contextID := uuid.New().String()
	taskID := uuid.New().String()

	task := &adk.Task{
		ID:        taskID,
		ContextID: contextID,
		Kind:      "task",
		Status: adk.TaskStatus{
			State: adk.TaskStateSubmitted,
		},
		History:   []adk.Message{message},
		Artifacts: []adk.Artifact{},
	}

	m.tasks[taskID] = task

	// Start the task state progression in a goroutine
	go m.progressTaskState(taskID)

	return task
}

// GetTask retrieves a task by ID
func (m *MockTaskManager) GetTask(taskID string) (*adk.Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	return task, exists
}

// ResumeTask resumes a paused task with user input
func (m *MockTaskManager) ResumeTask(taskID string, userMessage adk.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status.State != adk.TaskStateInputRequired {
		return fmt.Errorf("task %s is not in input-required state", taskID)
	}

	// Add user input to history
	task.History = append(task.History, userMessage)
	task.Status.State = adk.TaskStateWorking
	task.Status.Message = nil // Clear the input request message

	// Continue the task progression
	go m.progressTaskStateFromWorking(taskID)

	return nil
}

// CancelTask cancels a task
func (m *MockTaskManager) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Status.State = adk.TaskStateCanceled

	return nil
}

// progressTaskState simulates task state progression with input-required pause
func (m *MockTaskManager) progressTaskState(taskID string) {
	// submitted -> working (after 1 second)
	time.Sleep(1 * time.Second)
	m.updateTaskState(taskID, adk.TaskStateWorking, nil)

	// working -> input-required (after 2 seconds, requesting audience info)
	time.Sleep(2 * time.Second)
	inputRequestMessage := &adk.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("input-request-%d", time.Now().Unix()),
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "I'd be happy to help you create a presentation outline about climate change! To create the most effective outline, I need to know: What is the specific audience for this presentation? (e.g., students, business executives, policymakers, general public, etc.)",
			},
		},
	}
	m.updateTaskState(taskID, adk.TaskStateInputRequired, inputRequestMessage)
}

// progressTaskStateFromWorking continues task progression after user input
func (m *MockTaskManager) progressTaskStateFromWorking(taskID string) {
	// working -> completed (after 1 second with final response)
	time.Sleep(1 * time.Second)

	m.mu.Lock()
	task, exists := m.tasks[taskID]
	if !exists {
		m.mu.Unlock()
		return
	}

	// Get the user's audience input from the last message
	var audienceInfo string = "general audience"
	if len(task.History) > 1 {
		lastMsg := task.History[len(task.History)-1]
		for _, part := range lastMsg.Parts {
			if partMap, ok := part.(map[string]interface{}); ok {
				if textContent, exists := partMap["text"]; exists {
					if textStr, ok := textContent.(string); ok {
						audienceInfo = textStr
					}
				}
			}
		}
	}

	// Create comprehensive final response based on audience
	finalResponse := m.generateOutlineResponse(audienceInfo)

	completionMessage := adk.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("completion-%d", time.Now().Unix()),
		Role:      "assistant",
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": finalResponse,
			},
		},
	}

	task.History = append(task.History, completionMessage)
	task.Status.State = adk.TaskStateCompleted
	task.Status.Message = nil

	m.mu.Unlock()
}

// generateOutlineResponse creates a tailored response based on audience
func (m *MockTaskManager) generateOutlineResponse(audienceInfo string) string {
	return fmt.Sprintf(`Perfect! Based on your audience ("%s"), here's a comprehensive presentation outline about climate change:

**Climate Change Presentation Outline**

**I. Introduction (5 minutes)**
   - What is climate change?
   - Why it matters to %s
   - Presentation roadmap

**II. The Science Behind Climate Change (10 minutes)**
   - Greenhouse effect basics
   - Human activities contributing to climate change
   - Key evidence and data trends

**III. Current and Future Impacts (10 minutes)**
   - Environmental impacts (rising temperatures, sea levels, extreme weather)
   - Economic consequences
   - Social and health effects
   - Specific relevance to %s

**IV. Solutions and Actions (10 minutes)**
   - Mitigation strategies (reducing emissions)
   - Adaptation approaches (preparing for changes)
   - Role of %s in climate action
   - Technology and innovation opportunities

**V. Call to Action (5 minutes)**
   - What %s can do immediately
   - Long-term commitments and goals
   - Resources for further engagement

**VI. Q&A (10 minutes)**
   - Address audience questions and concerns

**Supporting Materials Needed:**
- Current climate data and graphs
- Local/regional impact examples
- Success stories relevant to %s
- Actionable next steps handout

This outline can be adjusted based on your specific time constraints and the depth of information your %s audience requires. Would you like me to elaborate on any particular section?`,
		audienceInfo, audienceInfo, audienceInfo, audienceInfo, audienceInfo, audienceInfo, audienceInfo)
}

// updateTaskState updates a task's state and optional message
func (m *MockTaskManager) updateTaskState(taskID string, state adk.TaskState, message *adk.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return
	}

	task.Status.State = state
	task.Status.Message = message

	if message != nil {
		task.History = append(task.History, *message)
	}
}

// MockServer represents the mock A2A server
type MockServer struct {
	taskManager *MockTaskManager
	logger      *zap.Logger
	agentCard   *adk.AgentCard
}

// NewMockServer creates a new mock server
func NewMockServer(ctx context.Context, logger *zap.Logger) *MockServer {
	streaming := false
	pushNotifications := false
	stateTransitionHistory := true

	agentCard := &adk.AgentCard{
		Name:               "climate-presentation-agent",
		Description:        "A mock A2A agent that demonstrates input-required task states by helping create climate change presentations",
		Version:            "1.0.0",
		URL:                "http://localhost:8080",
		ProtocolVersion:    "1.0.0",
		Skills:             []adk.AgentSkill{},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Capabilities: adk.AgentCapabilities{
			Streaming:              &streaming,
			PushNotifications:      &pushNotifications,
			StateTransitionHistory: &stateTransitionHistory,
		},
	}

	return &MockServer{
		taskManager: NewMockTaskManager(ctx),
		logger:      logger,
		agentCard:   agentCard,
	}
}

// SetupRoutes configures the HTTP routes for the mock server
func (s *MockServer) SetupRoutes() *gin.Engine {
	router := gin.Default()

	// Agent info endpoint
	router.GET("/.well-known/agent.json", s.handleAgentInfo)

	// Health check endpoint
	router.GET("/health", s.handleHealth)

	// Main A2A endpoint
	router.POST("/a2a", s.handleA2ARequest)

	return router
}

// handleAgentInfo returns the agent card
func (s *MockServer) handleAgentInfo(c *gin.Context) {
	s.logger.Info("agent card requested")
	c.JSON(200, s.agentCard)
}

// handleHealth returns server health status
func (s *MockServer) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   s.agentCard.Version,
	})
}

// A2ARequest represents an A2A JSON-RPC request
type A2ARequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      interface{} `json:"id"`
}

// A2AResponse represents an A2A JSON-RPC response
type A2AResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// handleA2ARequest handles A2A protocol requests
func (s *MockServer) handleA2ARequest(c *gin.Context) {
	var req A2ARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("failed to parse A2A request", zap.Error(err))
		c.JSON(400, A2AResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32700, "message": "Parse error"},
			ID:      req.ID,
		})
		return
	}

	s.logger.Info("received a2a request", zap.String("method", req.Method), zap.Any("id", req.ID))

	switch req.Method {
	case "message/send":
		s.handleMessageSend(c, req)
	case "tasks/get":
		s.handleTasksGet(c, req)
	case "tasks/cancel":
		s.handleTasksCancel(c, req)
	default:
		c.JSON(200, A2AResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32601, "message": "Method not found"},
			ID:      req.ID,
		})
	}
}

// handleMessageSend handles message/send requests
func (s *MockServer) handleMessageSend(c *gin.Context, req A2ARequest) {
	// Parse message send params
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		c.JSON(200, A2AResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32602, "message": "Invalid params"},
			ID:      req.ID,
		})
		return
	}

	var params adk.MessageSendParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to unmarshal message send params", zap.Error(err))
		c.JSON(200, A2AResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32602, "message": "Invalid params"},
			ID:      req.ID,
		})
		return
	}

	// Check if this is resuming an existing task
	if params.Message.TaskID != nil {
		// Resume existing task with user input
		taskID := *params.Message.TaskID
		err := s.taskManager.ResumeTask(taskID, params.Message)
		if err != nil {
			s.logger.Error("failed to resume task", zap.Error(err), zap.String("task_id", taskID))
			c.JSON(200, A2AResponse{
				JSONRPC: "2.0",
				Error:   map[string]interface{}{"code": -32603, "message": err.Error()},
				ID:      req.ID,
			})
			return
		}

		s.logger.Info("task resumed successfully", zap.String("task_id", taskID))

		// Return success response for task resume
		c.JSON(200, A2AResponse{
			JSONRPC: "2.0",
			Result:  map[string]interface{}{"status": "resumed", "task_id": taskID},
			ID:      req.ID,
		})
		return
	}

	// Create new task
	task := s.taskManager.CreateTask(params.Message)
	s.logger.Info("new task created", zap.String("task_id", task.ID), zap.String("context_id", task.ContextID))

	c.JSON(200, A2AResponse{
		JSONRPC: "2.0",
		Result:  task,
		ID:      req.ID,
	})
}

// handleTasksGet handles tasks/get requests
func (s *MockServer) handleTasksGet(c *gin.Context, req A2ARequest) {
	// Parse task query params
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		c.JSON(200, A2AResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32602, "message": "Invalid params"},
			ID:      req.ID,
		})
		return
	}

	var params adk.TaskQueryParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to unmarshal task query params", zap.Error(err))
		c.JSON(200, A2AResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32602, "message": "Invalid params"},
			ID:      req.ID,
		})
		return
	}

	// Get task
	task, exists := s.taskManager.GetTask(params.ID)
	if !exists {
		c.JSON(200, A2AResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32603, "message": "Task not found"},
			ID:      req.ID,
		})
		return
	}

	s.logger.Info("task retrieved", zap.String("task_id", task.ID), zap.String("state", string(task.Status.State)))

	c.JSON(200, A2AResponse{
		JSONRPC: "2.0",
		Result:  task,
		ID:      req.ID,
	})
}

// handleTasksCancel handles tasks/cancel requests
func (s *MockServer) handleTasksCancel(c *gin.Context, req A2ARequest) {
	// Parse task ID params
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		c.JSON(200, A2AResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32602, "message": "Invalid params"},
			ID:      req.ID,
		})
		return
	}

	var params adk.TaskIdParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to unmarshal task ID params", zap.Error(err))
		c.JSON(200, A2AResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32602, "message": "Invalid params"},
			ID:      req.ID,
		})
		return
	}

	// Cancel task
	err = s.taskManager.CancelTask(params.ID)
	if err != nil {
		s.logger.Error("failed to cancel task", zap.Error(err), zap.String("task_id", params.ID))
		c.JSON(200, A2AResponse{
			JSONRPC: "2.0",
			Error:   map[string]interface{}{"code": -32603, "message": err.Error()},
			ID:      req.ID,
		})
		return
	}

	s.logger.Info("task canceled successfully", zap.String("task_id", params.ID))

	c.JSON(200, A2AResponse{
		JSONRPC: "2.0",
		Result:  map[string]interface{}{"status": "canceled", "task_id": params.ID},
		ID:      req.ID,
	})
}

func main() {
	fmt.Println("🧪 Starting Mock A2A Server with Input-Required State Simulation...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Create mock server
	ctx := context.Background()
	server := NewMockServer(ctx, logger)

	// Setup routes
	router := server.SetupRoutes()

	logger.Info("✅ mock server created with input-required state simulation")
	logger.Info("🤖 agent metadata",
		zap.String("name", server.agentCard.Name),
		zap.String("description", server.agentCard.Description),
		zap.String("version", server.agentCard.Version))

	fmt.Println("\n🎯 Test the mock server:")
	fmt.Println("📋 Agent info: http://localhost:8080/.well-known/agent.json")
	fmt.Println("💚 Health check: http://localhost:8080/health")
	fmt.Println("📡 A2A endpoint: http://localhost:8080/a2a")
	fmt.Println("\n📘 This server simulates the following task flow:")
	fmt.Println("  1. submitted → working (1s)")
	fmt.Println("  2. working → input-required (2s) - asks for audience info")
	fmt.Println("  3. [PAUSED] - waits for user input")
	fmt.Println("  4. input-required → working (when user provides input)")
	fmt.Println("  5. working → completed (1s) - provides tailored outline")

	logger.Info("🌐 server running", zap.String("port", "8080"))
	fmt.Printf("\n🚀 Ready! Run the pausedtask client example to test the input-required flow.\n\n")

	if err := router.Run(":8080"); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}
