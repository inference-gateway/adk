package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/inference-gateway/adk/client"
	adk "github.com/inference-gateway/adk/types"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
)

// Config represents the application configuration
type Config struct {
	ServerURL      string        `env:"A2A_SERVER_URL,default=http://localhost:8080"`
	PollInterval   time.Duration `env:"POLL_INTERVAL,default=2s"`
	MaxPollTimeout time.Duration `env:"MAX_POLL_TIMEOUT,default=120s"`
}

func main() {
	// Load configuration from environment variables
	ctx := context.Background()
	var config Config
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatal("failed to process configuration", zap.Error(err))
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("starting A2A paused task (input-required) example",
		zap.String("server_url", config.ServerURL),
		zap.Duration("poll_interval", config.PollInterval),
		zap.Duration("max_poll_timeout", config.MaxPollTimeout))

	// Create A2A client
	a2aClient := client.NewClientWithLogger(config.ServerURL, logger)

	// Check agent capabilities first
	logger.Info("checking agent capabilities")
	agentCard, err := a2aClient.GetAgentCard(ctx)
	if err != nil {
		logger.Fatal("failed to get agent card", zap.Error(err))
	}

	logger.Info("agent card retrieved",
		zap.String("agent_name", agentCard.Name),
		zap.String("agent_version", agentCard.Version),
		zap.String("agent_description", agentCard.Description))

	// Submit initial task - one that might require user input
	logger.Info("submitting task that may require user input")

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
		logger.Fatal("failed to send task", zap.Error(err))
	}

	// Parse task from response
	var task adk.Task
	resultBytes, ok := resp.Result.(json.RawMessage)
	if !ok {
		logger.Fatal("unexpected response result type")
	}
	if err := json.Unmarshal(resultBytes, &task); err != nil {
		logger.Fatal("failed to parse task response", zap.Error(err))
	}

	logger.Info("task submitted successfully",
		zap.String("task_id", task.ID),
		zap.String("state", string(task.Status.State)))

	// Monitor task with special handling for input-required state
	if err := monitorTaskWithInputHandling(ctx, a2aClient, task.ID, config, logger); err != nil {
		logger.Fatal("task monitoring failed", zap.Error(err))
	}

	logger.Info("paused task example completed successfully")
}

// monitorTaskWithInputHandling polls task status and handles input-required state
func monitorTaskWithInputHandling(ctx context.Context, a2aClient client.A2AClient, taskID string, config Config, logger *zap.Logger) error {
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(config.MaxPollTimeout)
	defer timeoutTimer.Stop()

	pollCount := 0
	startTime := time.Now()

	logger.Info("starting task monitoring with input-required handling",
		zap.String("task_id", taskID),
		zap.Duration("poll_interval", config.PollInterval))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeoutTimer.C:
			return fmt.Errorf("task monitoring timed out after %v", config.MaxPollTimeout)

		case <-ticker.C:
			pollCount++
			elapsed := time.Since(startTime)

			// Get current task status
			taskResp, err := a2aClient.GetTask(ctx, adk.TaskQueryParams{
				ID: taskID,
			})
			if err != nil {
				logger.Error("failed to get task status",
					zap.Error(err),
					zap.String("task_id", taskID),
					zap.Int("poll_count", pollCount))
				continue // Continue polling on error
			}

			// Parse updated task
			var updatedTask adk.Task
			taskResultBytes, ok := taskResp.Result.(json.RawMessage)
			if !ok {
				logger.Error("unexpected task response result type",
					zap.String("task_id", taskID))
				continue
			}
			if err := json.Unmarshal(taskResultBytes, &updatedTask); err != nil {
				logger.Error("failed to parse task response",
					zap.Error(err),
					zap.String("task_id", taskID))
				continue
			}

			logger.Info("poll status update",
				zap.String("task_id", taskID),
				zap.String("state", string(updatedTask.Status.State)),
				zap.Int("poll_count", pollCount),
				zap.Duration("elapsed", elapsed))

			// Handle different task states
			switch updatedTask.Status.State {
			case adk.TaskStateCompleted:
				logger.Info("task completed successfully",
					zap.String("task_id", taskID),
					zap.Duration("total_time", elapsed),
					zap.Int("total_polls", pollCount))

				// Display final response
				displayTaskResponse(&updatedTask, logger)
				return nil

			case adk.TaskStateFailed:
				errorMsg := extractErrorMessage(&updatedTask)
				return fmt.Errorf("task failed: %s", errorMsg)

			case adk.TaskStateCanceled:
				return fmt.Errorf("task was canceled")

			case adk.TaskStateRejected:
				return fmt.Errorf("task was rejected")

			case adk.TaskStateInputRequired:
				// Task is paused waiting for user input - this is the key feature we're demonstrating
				logger.Info("=== TASK PAUSED FOR INPUT ===",
					zap.String("task_id", taskID))

				// Debug: log the task status to see what we're getting
				logger.Debug("task status details",
					zap.String("task_id", taskID),
					zap.Bool("has_status_message", updatedTask.Status.Message != nil))

				// Display any message from the agent asking for input
				if updatedTask.Status.Message != nil {
					fmt.Println("\n" + strings.Repeat("=", 50))
					fmt.Println("🤖 AGENT NEEDS INPUT:")
					fmt.Println(strings.Repeat("=", 50))
					displayMessage(updatedTask.Status.Message)
					fmt.Println(strings.Repeat("=", 50))
				} else {
					// Debug: if no message, show that we didn't get one
					fmt.Println("\n⚠️  Task is paused for input but no message was provided by the agent.")
					logger.Warn("task paused for input but no status message available",
						zap.String("task_id", taskID))
				}

				// Get user input
				userInput, err := getUserInput()
				if err != nil {
					return fmt.Errorf("failed to get user input: %w", err)
				}

				if userInput == "" {
					logger.Info("user chose to cancel, canceling task")
					_, cancelErr := a2aClient.CancelTask(ctx, adk.TaskIdParams{ID: taskID})
					if cancelErr != nil {
						logger.Error("failed to cancel task", zap.Error(cancelErr))
					}
					return fmt.Errorf("task canceled by user")
				}

				// Resume task with user input by sending a new message
				logger.Info("resuming task with user input",
					zap.String("task_id", taskID),
					zap.String("user_input", userInput))

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
						TaskID: &taskID, // Resume the existing task

					},
					Configuration: &adk.MessageSendConfiguration{
						Blocking:            boolPtr(false),
						AcceptedOutputModes: []string{"text"},
					},
				}

				// Send the resume message
				_, err = a2aClient.SendTask(ctx, resumeParams)
				if err != nil {
					return fmt.Errorf("failed to resume task with input: %w", err)
				}

				logger.Info("task resumed successfully, continuing monitoring")
				// Continue polling to monitor the resumed task

			case adk.TaskStateSubmitted, adk.TaskStateWorking, adk.TaskStateAuthRequired:
				// Continue polling for these states
				continue

			case adk.TaskStateUnknown:
				logger.Warn("task in unknown state, continuing to poll",
					zap.String("task_id", taskID))
				continue

			default:
				logger.Warn("unexpected task state",
					zap.String("task_id", taskID),
					zap.String("state", string(updatedTask.Status.State)))
				continue
			}
		}
	}
}

// getUserInput prompts the user for input when task is paused
func getUserInput() (string, error) {
	fmt.Print("\n💬 Please provide your input (or press Enter to cancel): ")

	reader := bufio.NewReader(os.Stdin)
	input, _, err := reader.ReadLine()
	if err != nil {
		return "", fmt.Errorf("failed to read user input: %w", err)
	}

	return strings.TrimSpace(string(input)), nil
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
func displayTaskResponse(task *adk.Task, logger *zap.Logger) {
	logger.Info("=== Task Completed ===")
	logger.Info("task details",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID),
		zap.String("final_state", string(task.Status.State)))

	// Display final response if available
	if len(task.History) > 0 {
		lastMessage := task.History[len(task.History)-1]
		if lastMessage.Role == "assistant" {
			// Extract text from final response
			var responseText string
			for _, part := range lastMessage.Parts {
				if partMap, ok := part.(map[string]interface{}); ok {
					if textContent, exists := partMap["text"]; exists {
						if textStr, ok := textContent.(string); ok {
							responseText += textStr
						}
					}
				}
			}
			if responseText != "" {
				fmt.Println("\n" + strings.Repeat("=", 50))
				fmt.Println("🤖 FINAL ASSISTANT RESPONSE:")
				fmt.Println(strings.Repeat("=", 50))
				fmt.Println(responseText)
				fmt.Println(strings.Repeat("=", 50))
			}
		}
	}
}

// extractErrorMessage extracts error message from a failed task
func extractErrorMessage(task *adk.Task) string {
	errorMsg := "unknown error"
	if task.Status.Message != nil {
		// Extract text from error message
		for _, part := range task.Status.Message.Parts {
			if partMap, ok := part.(map[string]interface{}); ok {
				if textContent, exists := partMap["text"]; exists {
					if textStr, ok := textContent.(string); ok {
						errorMsg = textStr
					}
				}
			}
		}
	}
	return errorMsg
}

func boolPtr(b bool) *bool {
	return &b
}
