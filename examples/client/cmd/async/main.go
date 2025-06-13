package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/inference-gateway/a2a/adk"
	"github.com/inference-gateway/a2a/adk/client"
	"go.uber.org/zap"
)

const (
	defaultServerURL = "http://localhost:8080/a2a"
	pollInterval     = 2 * time.Second
	maxPollTimeout   = 30 * time.Second
)

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Get server URL from environment or use default
	serverURL := os.Getenv("A2A_SERVER_URL")
	if serverURL == "" {
		serverURL = defaultServerURL
	}

	logger.Info("starting simple a2a async polling example",
		zap.String("server_url", serverURL))

	// Create A2A client
	a2aClient := client.NewClient(serverURL)
	ctx := context.Background()

	// Submit task using A2A ADK
	logger.Info("submitting task to agent")

	msgParams := adk.MessageSendParams{
		Message: adk.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("msg-%d", time.Now().Unix()),
			Role:      "user",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Explain the benefits of renewable energy in 3 key points",
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

	// Parse task from response - handle interface{} type
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

	// Start async polling in a goroutine
	resultChan := make(chan *adk.Task, 1)
	errorChan := make(chan error, 1)

	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		pollCount := 0
		startTime := time.Now()

		logger.Info("starting background polling",
			zap.String("task_id", task.ID),
			zap.Duration("poll_interval", pollInterval))

		for {
			select {
			case <-ctx.Done():
				errorChan <- ctx.Err()
				return

			case <-ticker.C:
				pollCount++
				elapsed := time.Since(startTime)

				// Get task status using A2A client
				taskResp, err := a2aClient.GetTask(ctx, adk.TaskQueryParams{
					ID: task.ID,
				})
				if err != nil {
					logger.Error("failed to get task status",
						zap.Error(err),
						zap.String("task_id", task.ID),
						zap.Int("poll_count", pollCount))
					continue // Continue polling on error
				}

				// Parse updated task - handle interface{} type
				var updatedTask adk.Task
				taskResultBytes, ok := taskResp.Result.(json.RawMessage)
				if !ok {
					logger.Error("unexpected task response result type",
						zap.String("task_id", task.ID))
					continue
				}
				if err := json.Unmarshal(taskResultBytes, &updatedTask); err != nil {
					logger.Error("failed to parse task response",
						zap.Error(err),
						zap.String("task_id", task.ID))
					continue
				}

				logger.Info("poll status update",
					zap.String("task_id", task.ID),
					zap.String("state", string(updatedTask.Status.State)),
					zap.Int("poll_count", pollCount),
					zap.Duration("elapsed", elapsed))

				// Check if task is complete
				switch updatedTask.Status.State {
				case adk.TaskStateCompleted:
					logger.Info("task completed successfully",
						zap.String("task_id", task.ID),
						zap.Duration("total_time", elapsed),
						zap.Int("total_polls", pollCount))
					resultChan <- &updatedTask
					return

				case adk.TaskStateFailed:
					errorMsg := "unknown error"
					if updatedTask.Status.Message != nil {
						// Extract text from error message
						for _, part := range updatedTask.Status.Message.Parts {
							if partMap, ok := part.(map[string]interface{}); ok {
								if textContent, exists := partMap["text"]; exists {
									if textStr, ok := textContent.(string); ok {
										errorMsg = textStr
									}
								}
							}
						}
					}
					errorChan <- fmt.Errorf("task failed: %s", errorMsg)
					return

				case adk.TaskStateCanceled:
					errorChan <- fmt.Errorf("task was canceled")
					return

				case adk.TaskStateSubmitted, adk.TaskStateWorking:
					// Continue polling
					continue

				default:
					logger.Warn("task in unexpected state",
						zap.String("task_id", task.ID),
						zap.String("state", string(updatedTask.Status.State)))
					continue
				}
			}
		}
	}()

	// Wait for result or timeout
	select {
	case completedTask := <-resultChan:
		logger.Info("=== Task Completed ===")
		logger.Info("task details",
			zap.String("task_id", completedTask.ID),
			zap.String("context_id", completedTask.ContextID),
			zap.String("final_state", string(completedTask.Status.State)))

		// Display final response if available
		if len(completedTask.History) > 0 {
			lastMessage := completedTask.History[len(completedTask.History)-1]
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
					logger.Info("=== Final Assistant Response ===")
					fmt.Println(responseText)
				}
			}
		}

		logger.Info("async example completed successfully")

	case err := <-errorChan:
		logger.Fatal("polling failed", zap.Error(err))

	case <-time.After(maxPollTimeout):
		logger.Fatal("task polling timed out", zap.Duration("timeout", maxPollTimeout))
	}
}

func boolPtr(b bool) *bool {
	return &b
}
