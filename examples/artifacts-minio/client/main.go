package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
)

// Config holds client configuration
type Config struct {
	ServerURL    string `env:"SERVER_URL,default=http://localhost:8080" description:"A2A server URL"`
	ArtifactsURL string `env:"ARTIFACTS_URL,default=http://localhost:8081" description:"Artifacts server URL"`
	DownloadsDir string `env:"DOWNLOADS_DIR,default=downloads" description:"Directory to save downloaded artifacts"`
}

func main() {
	// Load configuration using go-envconfig
	ctx := context.Background()
	var cfg Config

	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	fmt.Printf("🔗 Connecting to A2A server: %s\n", cfg.ServerURL)
	fmt.Printf("📁 Artifacts server: %s\n", cfg.ArtifactsURL)
	fmt.Printf("☁️  Using MinIO cloud storage backend\n")

	// Create client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	// Create a task that will generate an artifact
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create message with file upload (demonstrating client artifact upload)
	message := createMessageWithFileUpload()

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
		fmt.Printf("\n📎 Found %d artifact(s) stored in MinIO:\n", len(completedTask.Artifacts))

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
							if err := downloadArtifact(uri, filename, cfg.DownloadsDir); err != nil {
								fmt.Printf("   ❌ Failed to download: %v\n", err)
							} else {
								fmt.Printf("   ✅ Downloaded successfully to: %s\n", filepath.Join(cfg.DownloadsDir, filename))
							}
						}
					}
				}
			}
		}
	} else {
		fmt.Println("\n📎 No artifacts found in response")
	}

	fmt.Println("\n🎉 MinIO artifacts client example completed!")
}

// downloadArtifact downloads an artifact from the given URI and saves it to the specified filename
func downloadArtifact(uri, filename, downloadsDir string) error {
	// Log the download source to demonstrate direct MinIO access
	fmt.Printf("   🌐 Downloading from: %s\n", uri)

	// Determine download source for demonstration
	if containsMinIOEndpoint(uri) {
		fmt.Printf("   ⚡ Direct download from MinIO storage (bypassing artifacts server)\n")
	} else {
		fmt.Printf("   🔄 Download via artifacts server proxy\n")
	}

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

// createMessageWithFileUpload creates a message that includes the dummy data file upload
func createMessageWithFileUpload() types.Message {
	// Path to the dummy data file
	filePath := "uploads/data.txt"

	fmt.Printf("📤 Uploading data file: %s\n", filePath)

	// Read the dummy file content
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("❌ Failed to read data file: %v\n", err)
		fmt.Println("Using default text message instead...")
		return types.Message{
			Role: "user",
			Parts: []types.Part{
				map[string]any{
					"kind": "text",
					"text": "Please create a detailed analysis report about renewable energy trends in 2024. Include charts and recommendations. This will be stored in MinIO cloud storage.",
				},
			},
		}
	}

	// Get filename and MIME type
	filename := filepath.Base(filePath)
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "text/plain"
	}

	// Encode file as base64
	encodedContent := base64.StdEncoding.EncodeToString(fileContent)

	fmt.Printf("   📊 File size: %d bytes\n", len(fileContent))
	fmt.Printf("   🏷️  MIME type: %s\n", mimeType)

	// Create message with both text and file parts
	return types.Message{
		Role: "user",
		Parts: []types.Part{
			map[string]any{
				"kind": "text",
				"text": "Please analyze the uploaded energy data and create a comprehensive report with insights and recommendations based on the provided statistics. The report will be stored in MinIO cloud storage for scalable access.",
			},
			map[string]any{
				"kind": "file",
				"file": map[string]any{
					"bytes":    encodedContent,
					"mimeType": mimeType,
				},
				"filename": filename,
			},
		},
	}
}

// containsMinIOEndpoint checks if the URI contains a MinIO endpoint (port 9000)
// to determine if we're downloading directly from MinIO vs artifacts server
func containsMinIOEndpoint(uri string) bool {
	return strings.Contains(uri, ":9000") || strings.Contains(uri, "localhost:9000")
}
