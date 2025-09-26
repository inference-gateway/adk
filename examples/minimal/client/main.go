package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
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
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Printf("Failed to sync logger: %v", err)
		}
	}()

	// Create client
	a2aClient := client.NewClientWithLogger(serverURL, logger)

	// Create a simple task
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create the message
	message := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Hello, this is a test message. Please respond with a greeting.",
			},
		},
	}

	fmt.Println("Sending message to server...")

	// Send the message using SendTask
	params := types.MessageSendParams{
		Message: message,
	}

	response, err := a2aClient.SendTask(ctx, params)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Extract task from response
	if response.Result == nil {
		log.Fatal("No result in response")
	}

	// Parse the task from the result
	taskBytes, err := json.Marshal(response.Result)
	if err != nil {
		log.Fatalf("Failed to marshal task: %v", err)
	}

	var task types.Task
	if err := json.Unmarshal(taskBytes, &task); err != nil {
		log.Fatalf("Failed to unmarshal task: %v", err)
	}

	fmt.Printf("Task created with ID: %s, initial state: %s\n", task.ID, task.Status.State)

	// Poll for task completion
	fmt.Println("Polling for task completion...")
	for range 10 {
		time.Sleep(500 * time.Millisecond)

		getParams := types.TaskQueryParams{
			ID: task.ID,
		}

		getResponse, err := a2aClient.GetTask(ctx, getParams)
		if err != nil {
			log.Printf("Failed to get task status: %v", err)
			continue
		}

		if getResponse.Result == nil {
			continue
		}

		// Parse the updated task
		updatedTaskBytes, err := json.Marshal(getResponse.Result)
		if err != nil {
			continue
		}

		var updatedTask types.Task
		if err := json.Unmarshal(updatedTaskBytes, &updatedTask); err != nil {
			continue
		}

		fmt.Printf("Task state: %s\n", updatedTask.Status.State)

		if updatedTask.Status.State == types.TaskStateCompleted {
			// Pretty print the completed task
			responseJSON, err := json.MarshalIndent(updatedTask, "", "  ")
			if err != nil {
				log.Fatalf("Failed to marshal response: %v", err)
			}

			fmt.Println("\nCompleted task:")
			fmt.Println(string(responseJSON))
			return
		}

		if updatedTask.Status.State == types.TaskStateFailed {
			fmt.Println("Task failed!")
			return
		}
	}

	fmt.Println("Task did not complete within timeout")
}
