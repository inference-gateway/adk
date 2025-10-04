package main

import (
	"context"
	"encoding/base64"
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

// Artifacts with Default Handlers and Filesystem Server Client Example
//
// This client demonstrates how to interact with an A2A server that uses
// default task handlers with automatic artifact extraction and a filesystem
// server for artifact storage. It shows:
// - Uploading files to the server
// - Triggering artifact-creating tools
// - Downloading generated artifacts
//
// To run: go run main.go
func main() {
	fmt.Println("🤖 Starting Artifacts with Default Handlers Client...")

	// Get server URLs from environment or use defaults
	serverURL := "http://localhost:8080"
	artifactsURL := "http://localhost:8081"
	if envURL := os.Getenv("SERVER_URL"); envURL != "" {
		serverURL = envURL
	}
	if envURL := os.Getenv("ARTIFACTS_URL"); envURL != "" {
		artifactsURL = envURL
	}

	fmt.Printf("📡 A2A Server: %s\n", serverURL)
	fmt.Printf("📁 Artifacts Server: %s\n", artifactsURL)

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create A2A client
	a2aClient := client.NewClientWithLogger(serverURL, logger)

	// Test 1: Upload a file and have it processed
	fmt.Println("\n=== Test 1: File Upload and Processing ===")
	testFileUpload(a2aClient, logger, artifactsURL)

	time.Sleep(2 * time.Second)

	// Test 2: Generate a report with artifacts
	fmt.Println("\n=== Test 2: Report Generation with Artifacts ===")
	testReportGeneration(a2aClient, logger, artifactsURL)

	time.Sleep(2 * time.Second)

	// Test 3: Create multiple artifacts in one request
	fmt.Println("\n=== Test 3: Multiple Artifact Creation ===")
	testMultipleArtifacts(a2aClient, logger, artifactsURL)

	fmt.Println("\n✅ All tests completed!")
	fmt.Println("\nFeatures Demonstrated:")
	fmt.Println("✓ File upload from client to server")
	fmt.Println("✓ File processing and analysis by the agent")
	fmt.Println("✓ Artifact creation using filesystem storage")
	fmt.Println("✓ Artifact download from filesystem server")
	fmt.Println("✓ Default handlers automatically extracting artifacts")
}

// testFileUpload demonstrates uploading a file to the server
func testFileUpload(a2aClient client.A2AClient, logger *zap.Logger, artifactsURL string) {
	fmt.Println("📤 Uploading a file to the server...")

	// Create a sample file content to upload
	fileContent := `Energy Data Report 2024
========================

Solar Energy Production:
- Q1: 250 GWh
- Q2: 320 GWh
- Q3: 380 GWh
- Q4: 290 GWh

Wind Energy Production:
- Q1: 180 GWh
- Q2: 220 GWh
- Q3: 195 GWh
- Q4: 210 GWh

Hydroelectric Production:
- Q1: 450 GWh
- Q2: 480 GWh
- Q3: 430 GWh
- Q4: 410 GWh

Total Renewable Energy: 3,615 GWh
Year-over-Year Growth: 12.5%`

	// Encode file content as base64
	encodedContent := base64.StdEncoding.EncodeToString([]byte(fileContent))

	// Create message with file upload
	message := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Please analyze this energy data file I'm uploading and create a comprehensive analysis report",
			},
			map[string]any{
				"kind":     "file",
				"filename": "energy_data_2024.txt",
				"file": map[string]any{
					"bytes":    encodedContent,
					"mimeType": "text/plain",
				},
			},
		},
	}

	fmt.Printf("   📊 File size: %d bytes\n", len(fileContent))
	fmt.Printf("   📄 Filename: energy_data_2024.txt\n")

	// Send and wait for completion
	task, err := sendMessageAndWait(a2aClient, message, logger)
	if err != nil {
		log.Printf("Error in file upload test: %v", err)
		return
	}

	// Display results and download artifacts
	displayTaskResult("File Upload Test", task, artifactsURL)
}

// testReportGeneration demonstrates report generation with artifacts
func testReportGeneration(a2aClient client.A2AClient, logger *zap.Logger, artifactsURL string) {
	fmt.Println("📝 Requesting report generation...")

	// Create message requesting report
	message := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Generate a comprehensive report about artificial intelligence trends in 2024, including machine learning, neural networks, and large language models. Please create it in markdown format.",
			},
		},
	}

	// Send and wait for completion
	task, err := sendMessageAndWait(a2aClient, message, logger)
	if err != nil {
		log.Printf("Error in report generation test: %v", err)
		return
	}

	// Display results and download artifacts
	displayTaskResult("Report Generation Test", task, artifactsURL)
}

// testMultipleArtifacts demonstrates creating multiple artifacts
func testMultipleArtifacts(a2aClient client.A2AClient, logger *zap.Logger, artifactsURL string) {
	fmt.Println("📦 Requesting multiple artifact creation...")

	// Create message requesting multiple artifacts
	message := types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Please do the following: 1) Create a sequence diagram showing microservices communication, 2) Generate a report about cloud computing trends, and 3) Export sample customer data in CSV format.",
			},
		},
	}

	// Send and wait for completion
	task, err := sendMessageAndWait(a2aClient, message, logger)
	if err != nil {
		log.Printf("Error in multiple artifacts test: %v", err)
		return
	}

	// Display results and download artifacts
	displayTaskResult("Multiple Artifacts Test", task, artifactsURL)
}

// sendMessageAndWait sends a message and waits for task completion
func sendMessageAndWait(a2aClient client.A2AClient, message types.Message, logger *zap.Logger) (*types.Task, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

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

	fmt.Printf("⏳ Task created: %s - Polling for completion...\n", taskID)

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

			default:
				// Continue polling
			}
		}
	}
}

// displayTaskResult shows the task result and downloads any artifacts
func displayTaskResult(testName string, task *types.Task, artifactsURL string) {
	fmt.Printf("\n📋 %s Results:\n", testName)

	// Show final response
	if task.Status.Message != nil {
		responseText := getMessageText(task.Status.Message)
		fmt.Printf("Response: %s\n", truncateText(responseText, 200))
	}

	// Process and download artifacts
	if len(task.Artifacts) > 0 {
		fmt.Printf("\n📁 Artifacts Found: %d\n", len(task.Artifacts))

		// Create downloads directory
		downloadsDir := "downloads"
		if err := os.MkdirAll(downloadsDir, 0755); err != nil {
			log.Printf("Failed to create downloads directory: %v", err)
			return
		}

		for i, artifact := range task.Artifacts {
			fmt.Printf("\nArtifact %d:\n", i+1)
			fmt.Printf("  ID: %s\n", artifact.ArtifactID)

			if artifact.Name != nil {
				fmt.Printf("  Name: %s\n", *artifact.Name)
			}

			if artifact.Description != nil {
				fmt.Printf("  Description: %s\n", *artifact.Description)
			}

			// Process each part of the artifact
			for _, part := range artifact.Parts {
				if partMap, ok := part.(map[string]any); ok {
					if kind, ok := partMap["kind"].(string); ok && kind == "file" {
						// Extract file information
						filename := "unknown"
						if fn, ok := partMap["filename"].(string); ok {
							filename = fn
						}

						// Check for URI (filesystem server artifact)
						if uri, ok := partMap["uri"].(string); ok {
							fmt.Printf("  📥 Downloading: %s\n", filename)
							fmt.Printf("     URL: %s\n", uri)

							// Download the artifact
							if err := downloadArtifact(uri, filename, downloadsDir); err != nil {
								fmt.Printf("     ❌ Download failed: %v\n", err)
							} else {
								savedPath := filepath.Join(downloadsDir, filename)
								fmt.Printf("     ✅ Saved to: %s\n", savedPath)

								// Show preview of downloaded content
								if content, err := os.ReadFile(savedPath); err == nil {
									preview := truncateText(string(content), 150)
									fmt.Printf("     Preview: %s\n", preview)
								}
							}
						} else if file, ok := partMap["file"].(map[string]any); ok {
							// Handle embedded file content
							if bytes, ok := file["bytes"].(string); ok {
								fmt.Printf("  💾 Saving embedded file: %s\n", filename)

								// Decode and save
								if decoded, err := base64.StdEncoding.DecodeString(bytes); err == nil {
									savedPath := filepath.Join(downloadsDir, filename)
									if err := os.WriteFile(savedPath, decoded, 0644); err == nil {
										fmt.Printf("     ✅ Saved to: %s\n", savedPath)
										preview := truncateText(string(decoded), 150)
										fmt.Printf("     Preview: %s\n", preview)
									}
								}
							}
						}
					}
				}
			}
		}

		fmt.Printf("\n🎉 Artifacts processed successfully!\n")
	} else {
		fmt.Printf("\n📁 No artifacts found in response\n")
	}
}

// downloadArtifact downloads an artifact from the given URI
func downloadArtifact(uri, filename, downloadsDir string) error {
	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make the request
	resp, err := httpClient.Get(uri)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// Create the output file
	outputPath := filepath.Join(downloadsDir, filename)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	// Copy the content
	_, err = io.Copy(outFile, resp.Body)
	return err
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

// truncateText truncates text to specified length
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
