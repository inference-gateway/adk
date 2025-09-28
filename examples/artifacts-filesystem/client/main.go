package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
)

func main() {
	// Get server URLs from environment or use defaults
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	artifactsURL := os.Getenv("ARTIFACTS_URL")
	if artifactsURL == "" {
		artifactsURL = "http://localhost:8081"
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	fmt.Printf("🔗 Connecting to A2A server: %s\n", serverURL)
	fmt.Printf("📁 Artifacts server: %s\n", artifactsURL)

	// Create client
	a2aClient := client.NewClientWithLogger(serverURL, logger)

	// Create a task that will generate an artifact
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create the message requesting an analysis report
	message := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Please create a detailed analysis report about renewable energy trends in 2024. Include charts and recommendations.",
			},
		},
	}

	fmt.Println("📝 Sending message to create analysis report...")

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

	fmt.Printf("⏳ Task created with ID: %s, initial state: %s\n", task.ID, task.Status.State)

	// Poll for task completion
	fmt.Println("🔍 Polling for task completion...")
	var completedTask types.Task

	for range 20 {
		time.Sleep(1 * time.Second)

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

		if err := json.Unmarshal(updatedTaskBytes, &completedTask); err != nil {
			continue
		}

		fmt.Printf("📊 Task state: %s\n", completedTask.Status.State)

		if completedTask.Status.State == types.TaskStateCompleted {
			fmt.Println("✅ Task completed successfully!")
			break
		}

		if completedTask.Status.State == types.TaskStateFailed {
			fmt.Println("❌ Task failed!")
			return
		}
	}

	if completedTask.Status.State != types.TaskStateCompleted {
		fmt.Println("⏰ Task did not complete within timeout")
		return
	}

	// Display task response
	if completedTask.Status.Message != nil {
		fmt.Println("\n📄 Response from server:")
		for _, part := range completedTask.Status.Message.Parts {
			if partMap, ok := part.(map[string]any); ok {
				if text, ok := partMap["text"].(string); ok {
					fmt.Printf("   %s\n", text)
				}
			}
		}
	}

	// Process artifacts
	if len(completedTask.Artifacts) > 0 {
		fmt.Printf("\n📎 Found %d artifact(s):\n", len(completedTask.Artifacts))

		for i, artifact := range completedTask.Artifacts {
			fmt.Printf("\n🗂️  Artifact %d:\n", i+1)
			fmt.Printf("   ID: %s\n", artifact.ArtifactID)
			if artifact.Name != nil {
				fmt.Printf("   Name: %s\n", *artifact.Name)
			}
			if artifact.Description != nil {
				fmt.Printf("   Description: %s\n", *artifact.Description)
			}

			// Process artifact parts to find downloadable files
			for _, part := range artifact.Parts {
				if partMap, ok := part.(map[string]any); ok {
					if kind, ok := partMap["kind"].(string); ok && kind == "file" {
						filename, hasFilename := partMap["filename"].(string)
						uri, hasURI := partMap["uri"].(string)

						if hasFilename && hasURI {
							fmt.Printf("   📥 File: %s\n", filename)
							fmt.Printf("   🔗 URI: %s\n", uri)

							// Download the artifact
							if err := downloadArtifact(uri, filename); err != nil {
								fmt.Printf("   ❌ Failed to download: %v\n", err)
							} else {
								fmt.Printf("   ✅ Downloaded successfully to: %s\n", filename)
							}
						}
					}
				}
			}
		}
	} else {
		fmt.Println("\n📎 No artifacts found in response")
	}

	fmt.Println("\n🎉 Client example completed!")
}

// downloadArtifact downloads an artifact from the given URI and saves it to the specified filename
func downloadArtifact(uri, filename string) error {
	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make the request
	resp, err := httpClient.Get(uri)
	if err != nil {
		return fmt.Errorf("failed to download artifact: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// Create the downloads directory if it doesn't exist
	downloadsDir := "downloads"
	if err := os.MkdirAll(downloadsDir, 0755); err != nil {
		return fmt.Errorf("failed to create downloads directory: %w", err)
	}

	// Create the output file
	outputPath := filepath.Join(downloadsDir, filename)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Copy the content
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}
