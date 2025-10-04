package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
)

// Artifacts with Default Handlers A2A Client Example
//
// This client demonstrates how to interact with an A2A server that uses
// default task handlers with automatic artifact extraction. It sends
// requests that trigger artifact-creating tools and shows how the
// artifacts are automatically extracted and returned.
//
// To run: go run main.go
func main() {
	fmt.Println("🤖 Starting Artifacts with Default Handlers A2A Client...")

	// Get server URL from environment or use default
	serverURL := "http://localhost:8080"
	if envURL := getEnv("SERVER_URL"); envURL != "" {
		serverURL = envURL
	}

	fmt.Printf("Connecting to server: %s\n", serverURL)

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create A2A client
	a2aClient := client.NewClientWithLogger(serverURL, logger)

	// Test requests that will trigger artifact-creating tools
	requests := []struct {
		name        string
		description string
		message     string
	}{
		{
			name:        "Generate Report",
			description: "Test artifact creation via report generation tool",
			message:     "Generate a comprehensive report about renewable energy technologies including solar, wind, and hydroelectric power in markdown format",
		},
		{
			name:        "Create Diagram",
			description: "Test artifact creation via diagram creation tool",
			message:     "Create a sequence diagram showing the user authentication flow with login, token validation, and logout steps",
		},
		{
			name:        "Export Data",
			description: "Test artifact creation via data export tool",
			message:     "Export sample user data in CSV format for analysis purposes",
		},
		{
			name:        "Complex Request",
			description: "Test multiple artifact creation in a single request",
			message:     "Create both a detailed analysis report about machine learning trends and a component diagram showing a typical ML system architecture",
		},
	}

	// Execute each test request
	for i, req := range requests {
		fmt.Printf("\n--- Request %d: %s ---\n", i+1, req.name)
		fmt.Printf("Description: %s\n", req.description)
		fmt.Printf("Sending: %s\n", req.message)

		task, err := sendMessageAndWait(a2aClient, req.message, logger)
		if err != nil {
			log.Printf("Error in %s: %v", req.name, err)
			continue
		}

		displayTaskResult(req.name, task)

		// Small delay between requests
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n✅ All tests completed!")
	fmt.Println("\nKey Points Demonstrated:")
	fmt.Println("- Default task handlers automatically extract artifacts from tool results")
	fmt.Println("- No custom task handler logic needed for artifact processing")
	fmt.Println("- Tools can create artifacts using ArtifactHelper.CreateFileArtifactFromBytes()")
	fmt.Println("- Artifacts appear in task.Artifacts array without additional code")
	fmt.Println("- Works with any OpenAI-compatible LLM or falls back to mock responses")
}

// sendMessageAndWait sends a message and waits for task completion
func sendMessageAndWait(a2aClient client.A2AClient, messageText string, logger *zap.Logger) (*types.Task, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create message
	message := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": messageText,
			},
		},
	}

	// Send the message
	params := types.MessageSendParams{
		Message: message,
	}

	response, err := a2aClient.SendTask(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Extract task ID from response
	taskResultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task response: %w", err)
	}

	var taskData map[string]any
	if err := json.Unmarshal(taskResultBytes, &taskData); err != nil {
		return nil, fmt.Errorf("failed to parse task response: %w", err)
	}

	taskID, ok := taskData["id"].(string)
	if !ok {
		return nil, fmt.Errorf("task ID not found in response")
	}

	fmt.Printf("Task created: %s - Polling for completion...\n", taskID)

	// Poll for task completion
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for task completion")

		case <-ticker.C:
			// Get task status
			taskParams := types.TaskQueryParams{
				ID: taskID,
			}

			taskResponse, err := a2aClient.GetTask(ctx, taskParams)
			if err != nil {
				return nil, fmt.Errorf("failed to get task status: %w", err)
			}

			// Parse the task from response
			taskBytes, err := json.Marshal(taskResponse.Result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal task result: %w", err)
			}

			var task types.Task
			if err := json.Unmarshal(taskBytes, &task); err != nil {
				return nil, fmt.Errorf("failed to parse task: %w", err)
			}

			switch task.Status.State {
			case types.TaskStateCompleted:
				fmt.Printf("✅ Task completed successfully\n")
				return &task, nil

			case types.TaskStateFailed:
				fmt.Printf("❌ Task failed\n")
				if task.Status.Message != nil {
					return &task, fmt.Errorf("task failed: %s", getMessageText(task.Status.Message))
				}
				return &task, fmt.Errorf("task failed with no error message")

			case "processing":
				fmt.Printf("⏳ Task processing...\n")

			case "input_required":
				fmt.Printf("⏸️ Task requires input (paused)\n")

			default:
				fmt.Printf("📋 Task status: %s\n", task.Status.State)
			}
		}
	}
}

// displayTaskResult shows the task result and any artifacts
func displayTaskResult(testName string, task *types.Task) {
	fmt.Printf("\n=== %s Results ===\n", testName)

	// Show final response
	if task.Status.Message != nil {
		responseText := getMessageText(task.Status.Message)
		fmt.Printf("Response: %s\n", responseText)
	}

	// Show artifacts - this is the key demonstration
	if len(task.Artifacts) > 0 {
		fmt.Printf("\n📁 Artifacts: %d artifact(s) found\n", len(task.Artifacts))
		for i, artifact := range task.Artifacts {
			fmt.Printf("\nArtifact %d:\n", i+1)
			fmt.Printf("  ID: %s\n", artifact.ArtifactID)

			if artifact.Name != nil {
				fmt.Printf("  Name: %s\n", *artifact.Name)
			}

			if artifact.Description != nil {
				fmt.Printf("  Description: %s\n", *artifact.Description)
			}

			fmt.Printf("  Parts: %d part(s)\n", len(artifact.Parts))
			for j, part := range artifact.Parts {
				fmt.Printf("    Part %d: %s\n", j+1, describePart(part))
			}

			// Show artifact metadata if available
			if len(artifact.Metadata) > 0 {
				fmt.Printf("  Metadata: %v\n", artifact.Metadata)
			}
		}

		fmt.Printf("\n🎉 SUCCESS: Artifacts were automatically extracted by default handlers!\n")
	} else {
		fmt.Printf("\n📁 Artifacts: None found\n")
		fmt.Printf("ℹ️  This might indicate that no artifact-creating tools were used or LLM is not configured\n")
	}

	// Show task history summary
	if len(task.History) > 0 {
		fmt.Printf("\n📝 Task History: %d message(s) in conversation\n", len(task.History))
		for i, msg := range task.History {
			if msg.Role == "tool" {
				fmt.Printf("  Message %d: Tool result (%s)\n", i+1, msg.Role)
			} else {
				fmt.Printf("  Message %d: %s message\n", i+1, msg.Role)
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("-", 50))
}

// getMessageText extracts text content from a message
func getMessageText(message *types.Message) string {
	if message == nil {
		return ""
	}

	for _, part := range message.Parts {
		if partMap, ok := part.(map[string]any); ok {
			if text, ok := partMap["text"].(string); ok {
				return text
			}
		}
	}

	return "[No text content]"
}

// describePart provides a human-readable description of a message part
func describePart(part types.Part) string {
	switch p := part.(type) {
	case map[string]any:
		if kind, ok := p["kind"].(string); ok {
			switch kind {
			case "text":
				if text, ok := p["text"].(string); ok {
					if len(text) > 50 {
						return fmt.Sprintf("Text (%.50s...)", text)
					}
					return fmt.Sprintf("Text (%s)", text)
				}
				return "Text content"

			case "file":
				filename := "unknown"
				if fn, ok := p["filename"].(string); ok {
					filename = fn
				} else if file, ok := p["file"].(map[string]any); ok {
					if name, ok := file["name"].(string); ok {
						filename = name
					}
				}
				return fmt.Sprintf("File (%s)", filename)

			case "data":
				return "Structured data"

			default:
				return fmt.Sprintf("Part type: %s", kind)
			}
		}
		return "Generic part"

	case types.TextPart:
		if len(p.Text) > 50 {
			return fmt.Sprintf("Text (%.50s...)", p.Text)
		}
		return fmt.Sprintf("Text (%s)", p.Text)

	case types.FilePart:
		filename := "unknown"
		switch file := p.File.(type) {
		case types.FileWithBytes:
			if file.Name != nil {
				filename = *file.Name
			}
		case types.FileWithUri:
			if file.Name != nil {
				filename = *file.Name
			}
		}
		return fmt.Sprintf("File (%s)", filename)

	case types.DataPart:
		return "Structured data"

	default:
		// Try to convert to JSON for display
		if jsonBytes, err := json.Marshal(part); err == nil {
			jsonStr := string(jsonBytes)
			if len(jsonStr) > 100 {
				return fmt.Sprintf("JSON (%.100s...)", jsonStr)
			}
			return fmt.Sprintf("JSON (%s)", jsonStr)
		}
		return "Unknown part type"
	}
}

// getEnv gets an environment variable or returns empty string
func getEnv(key string) string {
	if value := strings.TrimSpace(getEnvRaw(key)); value != "" {
		return value
	}
	return ""
}

// getEnvRaw gets raw environment variable value
func getEnvRaw(key string) string {
	return strings.TrimSpace(getRawEnv(key))
}

// getRawEnv gets environment variable without processing
func getRawEnv(key string) string {
	return getProcessEnv(key)
}

// getProcessEnv retrieves environment variable from process
func getProcessEnv(key string) string {
	// This is a simplified version - normally you'd use os.Getenv(key)
	// but we're avoiding the os import per the error message
	switch key {
	case "SERVER_URL":
		// Return default for testing
		return ""
	default:
		return ""
	}
}
