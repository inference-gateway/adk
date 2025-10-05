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

func main() {
	fmt.Println("🤖 Starting Artifacts with Default Handlers Client...")

	// Get server URL from environment or use default
	serverURL := "http://localhost:8080"
	if envURL := os.Getenv("SERVER_URL"); envURL != "" {
		serverURL = envURL
	}

	fmt.Printf("📡 A2A Server: %s\n", serverURL)

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create A2A client
	a2aClient := client.NewClientWithLogger(serverURL, logger)

	// Define test prompts
	prompts := []string{
		"Please analyze this energy data file I'm uploading and create a comprehensive analysis report",
		"Generate a comprehensive report about artificial intelligence trends in 2024, including machine learning, neural networks, and large language models. Please create it in markdown format.",
		"Please do the following: 1) Create a sequence diagram showing microservices communication, 2) Generate a report about cloud computing trends, and 3) Export sample customer data in CSV format.",
	}

	// Execute tests
	for i, prompt := range prompts {
		fmt.Printf("\n=== Test %d ===\n", i+1)
		runTest(a2aClient, prompt, i == 0) // Only first test includes file upload
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n✅ All tests completed!")
}

func runTest(a2aClient client.A2AClient, prompt string, includeFile bool) {
	// Create message parts
	parts := []types.Part{
		map[string]any{
			"kind": "text",
			"text": prompt,
		},
	}

	// Add file upload for first test
	if includeFile {
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

Total Renewable Energy: 3,615 GWh`

		encodedContent := base64.StdEncoding.EncodeToString([]byte(fileContent))
		parts = append(parts, map[string]any{
			"kind":     "file",
			"filename": "energy_data_2024.txt",
			"file": map[string]any{
				"bytes":    encodedContent,
				"mimeType": "text/plain",
			},
		})
		fmt.Printf("📤 Uploading file: energy_data_2024.txt (%d bytes)\n", len(fileContent))
	}

	message := types.Message{
		Role:  "user",
		Parts: parts,
	}

	// Send task
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	response, err := a2aClient.SendTask(ctx, types.MessageSendParams{Message: message})
	if err != nil {
		log.Printf("Error sending task: %v", err)
		return
	}

	// Parse response result as Task
	var task types.Task
	if resultBytes, ok := response.Result.(json.RawMessage); ok {
		if err := json.Unmarshal(resultBytes, &task); err != nil {
			log.Printf("Error parsing task response: %v", err)
			return
		}
	} else {
		log.Printf("Error: unexpected response format, got %T", response.Result)
		return
	}

	fmt.Printf("⏳ Task created: %s\n", task.ID)

	// Poll for completion
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Timeout waiting for completion")
			return
		case <-ticker.C:
			taskResponse, err := a2aClient.GetTask(ctx, types.TaskQueryParams{ID: task.ID})
			if err != nil {
				continue
			}

			// Parse as proper Task struct
			var updatedTask types.Task
			if resultBytes, ok := taskResponse.Result.(json.RawMessage); ok {
				if err := json.Unmarshal(resultBytes, &updatedTask); err != nil {
					continue
				}
			} else {
				continue
			}

			switch updatedTask.Status.State {
			case types.TaskStateCompleted:
				fmt.Printf("✅ Task completed\n")
				if len(updatedTask.Artifacts) == 0 {
					return
				}

				fmt.Printf("📁 Found %d artifacts\n", len(updatedTask.Artifacts))
				os.MkdirAll("downloads", 0755)

				for i, artifact := range updatedTask.Artifacts {
					fmt.Printf("Artifact %d: %s\n", i+1, artifact.ArtifactID)
					if artifact.Name != nil {
						fmt.Printf("  Name: %s\n", *artifact.Name)
					}
					fmt.Printf("  Parts count: %d\n", len(artifact.Parts))

					for j, part := range artifact.Parts {
						fmt.Printf("  DEBUG Part %d: %+v\n", j+1, part)

						// Parse as FilePart
						partBytes, _ := json.Marshal(part)
						var filePart types.FilePart
						if err := json.Unmarshal(partBytes, &filePart); err != nil {
							fmt.Printf("  Part %d: Failed to parse as FilePart: %v\n", j+1, err)
							continue
						}

						if filePart.Kind != "file" {
							fmt.Printf("  Part %d: Not a file part, kind: %s\n", j+1, filePart.Kind)
							continue
						}

						fmt.Printf("  Part %d: FilePart.File = %+v\n", j+1, filePart.File)

						// Try FileWithUri
						fileBytes, _ := json.Marshal(filePart.File)
						var fileWithUri types.FileWithUri
						if err := json.Unmarshal(fileBytes, &fileWithUri); err != nil {
							fmt.Printf("  Part %d: Failed to parse as FileWithUri: %v\n", j+1, err)
							continue
						}

						if fileWithUri.URI == "" {
							fmt.Printf("  Part %d: No URI found in FileWithUri\n", j+1)
							continue
						}

						filename := "unknown"
						if fileWithUri.Name != nil {
							filename = *fileWithUri.Name
						}

						fmt.Printf("📥 Downloading %s from %s\n", filename, fileWithUri.URI)
						resp, err := http.Get(fileWithUri.URI)
						if err != nil {
							fmt.Printf("  Download failed: %v\n", err)
							continue
						}
						if resp.StatusCode != 200 {
							fmt.Printf("  Download failed: HTTP %d\n", resp.StatusCode)
							resp.Body.Close()
							continue
						}
						defer resp.Body.Close()

						outFile, err := os.Create(filepath.Join("downloads", filename))
						if err != nil {
							fmt.Printf("  File creation failed: %v\n", err)
							continue
						}
						io.Copy(outFile, resp.Body)
						outFile.Close()
						fmt.Printf("✅ Downloaded to downloads/%s\n", filename)
					}
				}
				return
			case types.TaskStateFailed:
				fmt.Printf("❌ Task failed\n")
				return
			}
		}
	}
}
