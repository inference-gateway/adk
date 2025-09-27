package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/inference-gateway/adk/client"
	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

func main() {
	// Get server URL from environment or use default
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create client
	a2aClient := client.NewClientWithLogger(serverURL, logger)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Prepare different types of messages to test default handlers
	prompts := []string{
		"Hello, how are you?",
		"What's the weather like?",
		"Can you help me with something?",
	}

	for i, prompt := range prompts {
		fmt.Printf("\n--- Request %d ---\n", i+1)
		fmt.Printf("Sending: %s\n", prompt)

		// Create message with proper structure
		message := types.Message{
			Role: "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": prompt,
				},
			},
		}

		// Send the message
		params := types.MessageSendParams{
			Message: message,
		}

		response, err := a2aClient.SendTask(ctx, params)
		if err != nil {
			log.Printf("Failed to send message %d: %v", i+1, err)
			continue
		}

		// Extract task ID from response for polling
		taskResultBytes, err := json.Marshal(response.Result)
		if err != nil {
			log.Printf("Failed to marshal task response: %v", err)
			continue
		}

		var taskData map[string]any
		if err := json.Unmarshal(taskResultBytes, &taskData); err != nil {
			log.Printf("Failed to parse task response: %v", err)
			continue
		}

		taskID, ok := taskData["id"].(string)
		if !ok {
			log.Printf("Task ID not found in response")
			continue
		}

		fmt.Printf("Task ID: %s - Polling for completion...\n", taskID)

		// Poll for task completion
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		completed := false
		for !completed {
			select {
			case <-ctx.Done():
				fmt.Printf("Context cancelled while polling task %s\n", taskID)
				return
			case <-ticker.C:
				taskResp, err := a2aClient.GetTask(ctx, types.TaskQueryParams{ID: taskID})
				if err != nil {
					log.Printf("Failed to get task status: %v", err)
					completed = true
					break
				}

				taskRespBytes, err := json.Marshal(taskResp.Result)
				if err != nil {
					log.Printf("Failed to marshal task response: %v", err)
					completed = true
					break
				}

				var task types.Task
				if err := json.Unmarshal(taskRespBytes, &task); err != nil {
					log.Printf("Failed to parse task: %v", err)
					completed = true
					break
				}

				fmt.Printf("Task %s status: %s\n", taskID, task.Status.State)

				// Check if task is in a final state
				switch task.Status.State {
				case types.TaskStateCompleted:
					fmt.Printf("Task completed successfully!\n")
					if task.Status.Message != nil {
						messageJSON, _ := json.MarshalIndent(task.Status.Message, "", "  ")
						fmt.Printf("Final Response:\n%s\n", string(messageJSON))
					}
					completed = true
				case types.TaskStateInputRequired:
					fmt.Printf("Task requires input - ending polling\n")
					if task.Status.Message != nil {
						messageJSON, _ := json.MarshalIndent(task.Status.Message, "", "  ")
						fmt.Printf("Partial Response:\n%s\n", string(messageJSON))
					}
					completed = true
				case types.TaskStateFailed, types.TaskStateCanceled, types.TaskStateRejected:
					fmt.Printf("Task ended with state: %s\n", task.Status.State)
					if task.Status.Message != nil {
						messageJSON, _ := json.MarshalIndent(task.Status.Message, "", "  ")
						fmt.Printf("Response:\n%s\n", string(messageJSON))
					}
					completed = true
				}
			}
		}
	}
}
