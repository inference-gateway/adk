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

	// Prepare different types of messages to test AI capabilities with tools
	prompts := []string{
		"What's the weather in London?",
		"What time is it?",
		"Can you check the weather in Paris and tell me the current time?",
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

		// Extract task ID from response
		var taskResult struct {
			ID string `json:"id"`
		}
		resultBytes, ok := response.Result.(json.RawMessage)
		if !ok {
			log.Printf("Failed to parse result as json.RawMessage")
			continue
		}
		if err := json.Unmarshal(resultBytes, &taskResult); err != nil {
			log.Printf("Failed to parse task ID: %v", err)
			continue
		}

		fmt.Printf("Task ID: %s\n", taskResult.ID)
		fmt.Print("Polling for result")

		// Poll for task completion
		for {
			time.Sleep(500 * time.Millisecond)
			fmt.Print(".")

			taskResponse, err := a2aClient.GetTask(ctx, types.TaskQueryParams{
				ID: taskResult.ID,
			})
			if err != nil {
				log.Printf("\nFailed to get task status: %v", err)
				break
			}

			var task types.Task
			taskResultBytes, ok := taskResponse.Result.(json.RawMessage)
			if !ok {
				log.Printf("\nFailed to parse task result as json.RawMessage")
				break
			}
			if err := json.Unmarshal(taskResultBytes, &task); err != nil {
				log.Printf("\nFailed to parse task: %v", err)
				break
			}

			// Check if task is completed
			if task.Status.State == types.TaskStateCompleted {
				fmt.Println("\n✓ Task completed!")

				// Display the response
				if task.Status.Message != nil {
					for _, part := range task.Status.Message.Parts {
						if partMap, ok := part.(map[string]any); ok {
							if text, ok := partMap["text"].(string); ok {
								fmt.Printf("\nResponse: %s\n", text)
							}
						}
					}
				}
				break
			} else if task.Status.State == types.TaskStateFailed {
				fmt.Println("\n✗ Task failed")
				if task.Status.Message != nil {
					responseJSON, _ := json.MarshalIndent(task.Status.Message, "", "  ")
					fmt.Printf("Error: %s\n", string(responseJSON))
				}
				break
			}
		}
	}
}
