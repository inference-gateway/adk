package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/inference-gateway/adk/client"
	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

func main() {
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create A2A client (assumes server is running on localhost:8080)
	a2aClient := client.NewClientWithLogger("http://localhost:8080", logger)

	ctx := context.Background()

	// Test 1: Send a task and retrieve artifacts
	fmt.Println("=== Artifact Demo: Client-Side Artifact Extraction ===\n")

	// Send a message to create artifacts
	message := types.Message{
		Kind:      "message",
		MessageID: uuid.New().String(),
		Role:      "user",
		Parts: []types.Part{
			types.TextPart{
				Kind: "text",
				Text: "Please analyze this sample request: 'I need help understanding A2A protocol artifacts and how they work in practice with real examples.'",
			},
		},
	}

	params := types.MessageSendParams{
		Message: message,
	}

	fmt.Println("Sending task to create artifacts...")
	response, err := a2aClient.SendTask(ctx, params)
	if err != nil {
		log.Fatalf("Failed to send task: %v", err)
	}

	// Extract task from response
	artifactHelper := a2aClient.GetArtifactHelper()
	task, err := artifactHelper.ExtractTaskFromResponse(response)
	if err != nil {
		log.Fatalf("Failed to extract task from response: %v", err)
	}

	fmt.Printf("Task ID: %s\n", task.ID)
	fmt.Printf("Task State: %s\n", task.Status.State)

	// Wait for task completion by polling
	fmt.Println("\nWaiting for task completion...")
	task = waitForTaskCompletion(ctx, a2aClient, task.ID)

	// Extract and analyze artifacts
	fmt.Printf("\nTask completed! Analyzing %d artifacts:\n\n", artifactHelper.GetArtifactCount(task))

	// Demonstrate various artifact extraction methods
	demonstrateArtifactExtraction(artifactHelper, task)

	fmt.Println("\n=== Demo completed successfully! ===")
}

// waitForTaskCompletion polls the task until it's completed
func waitForTaskCompletion(ctx context.Context, client client.A2AClient, taskID string) *types.Task {
	artifactHelper := client.GetArtifactHelper()

	for {
		params := types.TaskQueryParams{
			ID: taskID,
		}

		response, err := client.GetTask(ctx, params)
		if err != nil {
			log.Fatalf("Failed to get task: %v", err)
		}

		task, err := artifactHelper.ExtractTaskFromResponse(response)
		if err != nil {
			log.Fatalf("Failed to extract task: %v", err)
		}

		if task.Status.State == types.TaskStateCompleted || 
		   task.Status.State == types.TaskStateFailed {
			return task
		}

		fmt.Printf("Task state: %s, waiting...\n", task.Status.State)
		// In a real application, you'd want to add appropriate delays
		// time.Sleep(1 * time.Second)
		break // For demo purposes, assume it completes quickly
	}

	// For demo purposes, return the task as-is
	params := types.TaskQueryParams{ID: taskID}
	response, _ := client.GetTask(ctx, params)
	task, _ := artifactHelper.ExtractTaskFromResponse(response)
	return task
}

// demonstrateArtifactExtraction shows different ways to extract and work with artifacts
func demonstrateArtifactExtraction(helper *client.ArtifactHelper, task *types.Task) {
	// 1. Basic artifact information
	fmt.Printf("📊 Artifact Summary:\n")
	summary := helper.GetArtifactSummary(task)
	for partType, count := range summary {
		fmt.Printf("  - %s parts: %d\n", partType, count)
	}
	fmt.Println()

	// 2. Extract all artifacts
	artifacts := helper.ExtractArtifactsFromTask(task)
	for i, artifact := range artifacts {
		fmt.Printf("📄 Artifact %d:\n", i+1)
		fmt.Printf("  ID: %s\n", artifact.ArtifactID)
		
		if artifact.Name != nil {
			fmt.Printf("  Name: %s\n", *artifact.Name)
		}
		
		if artifact.Description != nil {
			fmt.Printf("  Description: %s\n", *artifact.Description)
		}
		
		fmt.Printf("  Parts: %d\n", len(artifact.Parts))
		fmt.Println()
	}

	// 3. Extract text content from text artifacts
	textArtifacts := helper.GetTextArtifacts(task)
	if len(textArtifacts) > 0 {
		fmt.Printf("📝 Text Artifacts (%d found):\n", len(textArtifacts))
		for i, artifact := range textArtifacts {
			texts := helper.ExtractTextFromArtifact(&artifact)
			fmt.Printf("  Text Artifact %d:\n", i+1)
			for j, text := range texts {
				fmt.Printf("    Text Part %d: %s\n", j+1, truncateText(text, 100))
			}
		}
		fmt.Println()
	}

	// 4. Extract file data from file artifacts
	fileArtifacts := helper.GetFileArtifacts(task)
	if len(fileArtifacts) > 0 {
		fmt.Printf("📁 File Artifacts (%d found):\n", len(fileArtifacts))
		for i, artifact := range fileArtifacts {
			files, err := helper.ExtractFileDataFromArtifact(&artifact)
			if err != nil {
				fmt.Printf("  Error extracting files from artifact %d: %v\n", i+1, err)
				continue
			}

			fmt.Printf("  File Artifact %d:\n", i+1)
			for j, file := range files {
				fmt.Printf("    File %d:\n", j+1)
				fmt.Printf("      Name: %s\n", file.GetFileName())
				fmt.Printf("      MIME Type: %s\n", file.GetMIMEType())
				
				if file.IsDataFile() {
					fmt.Printf("      Size: %d bytes\n", len(file.Data))
					fmt.Printf("      Content Preview: %s\n", truncateText(string(file.Data), 50))
				} else if file.IsURIFile() {
					fmt.Printf("      URI: %s\n", *file.URI)
				}
			}
		}
		fmt.Println()
	}

	// 5. Extract structured data from data artifacts
	dataArtifacts := helper.GetDataArtifacts(task)
	if len(dataArtifacts) > 0 {
		fmt.Printf("🗂️  Data Artifacts (%d found):\n", len(dataArtifacts))
		for i, artifact := range dataArtifacts {
			dataList := helper.ExtractDataFromArtifact(&artifact)
			fmt.Printf("  Data Artifact %d:\n", i+1)
			for j, data := range dataList {
				fmt.Printf("    Data Part %d:\n", j+1)
				jsonData, err := json.MarshalIndent(data, "      ", "  ")
				if err != nil {
					fmt.Printf("      Error marshaling data: %v\n", err)
				} else {
					fmt.Printf("      %s\n", string(jsonData))
				}
			}
		}
		fmt.Println()
	}

	// 6. Search artifacts by name
	fmt.Printf("🔍 Artifact Search Examples:\n")
	analysisArtifacts := helper.FilterArtifactsByName(task, "analysis")
	fmt.Printf("  Artifacts containing 'analysis': %d found\n", len(analysisArtifacts))
	
	requestArtifacts := helper.FilterArtifactsByName(task, "request")
	fmt.Printf("  Artifacts containing 'request': %d found\n", len(requestArtifacts))
	fmt.Println()

	// 7. Demonstrate getting specific artifacts by ID
	if len(artifacts) > 0 {
		firstArtifactID := artifacts[0].ArtifactID
		foundArtifact, exists := helper.GetArtifactByID(task, firstArtifactID)
		if exists {
			fmt.Printf("✅ Successfully retrieved artifact by ID: %s\n", foundArtifactID)
			if foundArtifact.Name != nil {
				fmt.Printf("   Name: %s\n", *foundArtifact.Name)
			}
		}
		fmt.Println()
	}
}

// truncateText truncates text to a specified length for display
func truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}