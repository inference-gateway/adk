package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/inference-gateway/adk/client"
	adk "github.com/inference-gateway/adk/types"
	"github.com/sethvargo/go-envconfig"
)

// Config represents the application configuration
type Config struct {
	ServerURL      string        `env:"A2A_SERVER_URL,default=http://localhost:8080"`
	PollInterval   time.Duration `env:"POLL_INTERVAL,default=2s"`
	MaxPollTimeout time.Duration `env:"MAX_POLL_TIMEOUT,default=120s"`
}

func main() {
	// Load configuration from environment variables
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	var config Config
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatalf("failed to process configuration: %v", err)
	}

	fmt.Printf("🚀 Starting A2A Paused Task Example\n")
	fmt.Printf("📡 Server: %s\n", config.ServerURL)
	fmt.Printf("⏱️  Poll interval: %v\n\n", config.PollInterval)

	// Create A2A client
	a2aClient := client.NewClient(config.ServerURL)

	// Setup signal handling for graceful interruption
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	var currentTaskID string
	
	go func() {
		<-sigChan
		fmt.Printf("\n\n🛑 Interrupted! Showing conversation history...\n")
		if currentTaskID != "" {
			showConversationHistory(ctx, a2aClient, currentTaskID)
		}
		cancel()
		os.Exit(0)
	}()

	// Check agent capabilities first
	fmt.Printf("🔍 Checking agent capabilities...\n")
	agentCard, err := a2aClient.GetAgentCard(ctx)
	if err != nil {
		log.Fatalf("failed to get agent card: %v", err)
	}

	fmt.Printf("✅ Connected to agent: %s v%s\n", agentCard.Name, agentCard.Version)
	fmt.Printf("📝 Description: %s\n\n", agentCard.Description)

	// Submit initial task - one that might require user input
	fmt.Printf("📨 Submitting task that requires user input...\n")

	msgParams := adk.MessageSendParams{
		Message: adk.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("msg-%d", time.Now().Unix()),
			Role:      "user",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "I need to write a presentation about climate change. Can you help me create an outline? But first, what is the specific audience for this presentation?",
				},
			},
		},
		Configuration: &adk.MessageSendConfiguration{
			Blocking:            boolPtr(false),
			AcceptedOutputModes: []string{"text"},
		},
	}

	// Send the task using A2A client
	resp, err := a2aClient.SendTask(ctx, msgParams)
	if err != nil {
		log.Fatalf("failed to send task: %v", err)
	}

	// Parse task from response
	var task adk.Task
	resultBytes, ok := resp.Result.(json.RawMessage)
	if !ok {
		log.Fatal("unexpected response result type")
	}
	if err := json.Unmarshal(resultBytes, &task); err != nil {
		log.Fatalf("failed to parse task response: %v", err)
	}

	currentTaskID = task.ID
	fmt.Printf("✅ Task submitted: %s\n", task.ID)
	fmt.Printf("📊 Initial state: %s\n\n", task.Status.State)

	// Monitor task with special handling for input-required state
	if err := monitorTaskWithInputHandling(ctx, a2aClient, task.ID, config); err != nil {
		log.Fatalf("task monitoring failed: %v", err)
	}

	fmt.Printf("🎉 Task completed successfully!\n")
}

// monitorTaskWithInputHandling polls task status and handles input-required state
func monitorTaskWithInputHandling(ctx context.Context, a2aClient client.A2AClient, taskID string, config Config) error {
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(config.MaxPollTimeout)
	defer timeoutTimer.Stop()

	startTime := time.Now()
	fmt.Printf("🔄 Monitoring task progress...\n")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeoutTimer.C:
			return fmt.Errorf("task monitoring timed out after %v", config.MaxPollTimeout)

		case <-ticker.C:
			// Get current task status
			taskResp, err := a2aClient.GetTask(ctx, adk.TaskQueryParams{ID: taskID})
			if err != nil {
				fmt.Printf("⚠️  Failed to get task status: %v\n", err)
				continue
			}

			// Parse updated task
			var task adk.Task
			taskResultBytes, ok := taskResp.Result.(json.RawMessage)
			if !ok {
				fmt.Printf("⚠️  Unexpected task response format\n")
				continue
			}
			if err := json.Unmarshal(taskResultBytes, &task); err != nil {
				fmt.Printf("⚠️  Failed to parse task response: %v\n", err)
				continue
			}

			fmt.Printf("📊 Task state: %s (elapsed: %v)\n", task.Status.State, time.Since(startTime).Round(time.Second))

			// Handle different task states
			switch task.Status.State {
			case adk.TaskStateCompleted:
				fmt.Printf("✅ Task completed successfully!\n\n")
				displayTaskResponse(&task)
				fmt.Printf("\n📜 Complete Conversation History:\n")
				showConversationHistory(ctx, a2aClient, taskID)
				return nil

			case adk.TaskStateFailed:
				errorMsg := extractErrorMessage(&task)
				return fmt.Errorf("task failed: %s", errorMsg)

			case adk.TaskStateCanceled:
				return fmt.Errorf("task was canceled")

			case adk.TaskStateRejected:
				return fmt.Errorf("task was rejected")

			case adk.TaskStateInputRequired:
				fmt.Printf("\n⏸️  Task paused - agent needs input!\n")
				
				// Show recent conversation for context
				showRecentConversation(&task, 3)

				// Display any message from the agent asking for input
				if task.Status.Message != nil {
					fmt.Printf("\n🤖 Agent's question:\n")
					fmt.Printf("%s\n", strings.Repeat("-", 40))
					displayMessage(task.Status.Message)
					fmt.Printf("%s\n", strings.Repeat("-", 40))
				}

				// Get user input
				userInput, err := getUserInput()
				if err != nil {
					return fmt.Errorf("failed to get user input: %w", err)
				}

				if userInput == "" {
					fmt.Printf("🚫 Canceling task...\n")
					_, cancelErr := a2aClient.CancelTask(ctx, adk.TaskIdParams{ID: taskID})
					if cancelErr != nil {
						fmt.Printf("⚠️  Failed to cancel task: %v\n", cancelErr)
					}
					return fmt.Errorf("task canceled by user")
				}

				// Resume task with user input
				fmt.Printf("▶️  Resuming task with your input...\n\n")
				resumeParams := adk.MessageSendParams{
					Message: adk.Message{
						Kind:      "message",
						MessageID: fmt.Sprintf("resume-msg-%d", time.Now().Unix()),
						Role:      "user",
						Parts: []adk.Part{
							map[string]interface{}{
								"kind": "text",
								"text": userInput,
							},
						},
						TaskID: &taskID,
					},
					Configuration: &adk.MessageSendConfiguration{
						Blocking:            boolPtr(false),
						AcceptedOutputModes: []string{"text"},
					},
				}

				_, err = a2aClient.SendTask(ctx, resumeParams)
				if err != nil {
					return fmt.Errorf("failed to resume task with input: %w", err)
				}

			case adk.TaskStateSubmitted, adk.TaskStateWorking, adk.TaskStateAuthRequired:
				// Continue polling for these states
				continue

			default:
				fmt.Printf("⚠️  Unexpected task state: %s\n", task.Status.State)
				continue
			}
		}
	}
}

// showConversationHistory displays the full conversation history (used on interrupt)
func showConversationHistory(ctx context.Context, a2aClient client.A2AClient, taskID string) {
	taskResp, err := a2aClient.GetTask(ctx, adk.TaskQueryParams{ID: taskID})
	if err != nil {
		fmt.Printf("❌ Failed to get task for history: %v\n", err)
		return
	}

	var task adk.Task
	taskResultBytes, ok := taskResp.Result.(json.RawMessage)
	if !ok {
		fmt.Printf("❌ Failed to parse task response\n")
		return
	}
	if err := json.Unmarshal(taskResultBytes, &task); err != nil {
		fmt.Printf("❌ Failed to unmarshal task: %v\n", err)
		return
	}

	fmt.Printf("\n📜 Conversation History for Task %s:\n", taskID)
	fmt.Printf("%s\n", strings.Repeat("=", 60))
	
	if len(task.History) == 0 {
		fmt.Printf("(No conversation history available)\n")
		return
	}

	for i, msg := range task.History {
		role := "👤 User"
		if msg.Role == "assistant" {
			role = "🤖 Assistant"
		}
		
		fmt.Printf("\n[%d] %s:\n", i+1, role)
		fmt.Printf("%s\n", strings.Repeat("-", 30))
		
		textContent := extractTextFromMessage(&msg)
		if textContent != "" {
			fmt.Printf("%s\n", textContent)
		} else {
			fmt.Printf("(No text content)\n")
		}
	}
	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
}

// showRecentConversation displays the last N messages from the conversation
func showRecentConversation(task *adk.Task, count int) {
	if len(task.History) == 0 {
		return
	}

	fmt.Printf("\n📝 Recent conversation:\n")
	
	start := len(task.History) - count
	if start < 0 {
		start = 0
	}

	for i := start; i < len(task.History); i++ {
		msg := task.History[i]
		role := "👤"
		if msg.Role == "assistant" {
			role = "🤖"
		}
		
		textContent := extractTextFromMessage(&msg)
		if textContent != "" {
			// Truncate long messages for context
			if len(textContent) > 100 {
				textContent = textContent[:97] + "..."
			}
			fmt.Printf("  %s %s\n", role, textContent)
		}
	}
}

// getUserInput prompts the user for input when task is paused
func getUserInput() (string, error) {
	fmt.Print("\n💬 Your response (or press Enter to cancel): ")

	reader := bufio.NewReader(os.Stdin)
	input, _, err := reader.ReadLine()
	if err != nil {
		return "", fmt.Errorf("failed to read user input: %w", err)
	}

	return strings.TrimSpace(string(input)), nil
}

// extractTextFromMessage extracts text content from a message
func extractTextFromMessage(message *adk.Message) string {
	var textContent string
	for _, part := range message.Parts {
		if partMap, ok := part.(map[string]interface{}); ok {
			if text, exists := partMap["text"]; exists {
				if textStr, ok := text.(string); ok {
					textContent += textStr
				}
			}
		}
	}
	return textContent
}

// displayMessage extracts and displays text content from a message
func displayMessage(message *adk.Message) {
	if message == nil {
		return
	}

	for _, part := range message.Parts {
		if partMap, ok := part.(map[string]interface{}); ok {
			if textContent, exists := partMap["text"]; exists {
				if textStr, ok := textContent.(string); ok {
					fmt.Println(textStr)
				}
			}
		}
	}
}

// displayTaskResponse shows the final task response
func displayTaskResponse(task *adk.Task) {
	fmt.Printf("🎯 Task completed: %s\n", task.ID)

	// Display final response if available
	if len(task.History) > 0 {
		lastMessage := task.History[len(task.History)-1]
		if lastMessage.Role == "assistant" {
			responseText := extractTextFromMessage(&lastMessage)
			if responseText != "" {
				fmt.Printf("\n🤖 Final response:\n")
				fmt.Printf("%s\n", strings.Repeat("-", 40))
				fmt.Printf("%s\n", responseText)
				fmt.Printf("%s\n", strings.Repeat("-", 40))
			}
		}
	}
}

// extractErrorMessage extracts error message from a failed task
func extractErrorMessage(task *adk.Task) string {
	errorMsg := "unknown error"
	if task.Status.Message != nil {
		errorMsg = extractTextFromMessage(task.Status.Message)
	}
	return errorMsg
}

func boolPtr(b bool) *bool {
	return &b
}
