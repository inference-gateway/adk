package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
)

// Config holds client configuration
type Config struct {
	Environment  string `env:"ENVIRONMENT,default=development"`
	ServerURL    string `env:"SERVER_URL,default=http://localhost:8080"`
	DownloadsDir string `env:"DOWNLOADS_DIR,default=downloads"`
}

func main() {
	ctx := context.Background()
	cfg := loadConfig(ctx)
	logger := initLogger(cfg.Environment)
	defer logger.Sync()

	logger.Info("client starting",
		zap.String("server_url", cfg.ServerURL),
		zap.String("downloads_dir", cfg.DownloadsDir))

	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	// Test prompts that should trigger autonomous artifact creation
	prompts := []struct {
		text     string
		expected string
	}{
		{
			text:     "Create a JSON report with sample user data including names, emails, and ages for 3 users",
			expected: "JSON report",
		},
		{
			text:     "Generate a CSV file with product inventory data (name, SKU, quantity, price) for 5 products",
			expected: "CSV file",
		},
		{
			text:     "Write a Python script that calculates fibonacci numbers recursively",
			expected: "Python script",
		},
	}

	for i, prompt := range prompts {
		fmt.Printf("\n--- Request %d ---\n", i+1)
		fmt.Printf("Sending: %s\n", prompt.text)
		fmt.Printf("Expected: %s\n\n", prompt.expected)

		if err := processPrompt(ctx, a2aClient, prompt.text, cfg.DownloadsDir, logger); err != nil {
			logger.Error("failed to process prompt", zap.Int("request", i+1), zap.Error(err))
			continue
		}

		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n--- All Requests Complete ---")
	fmt.Printf("Check the '%s' directory for downloaded artifacts\n", cfg.DownloadsDir)
}

// loadConfig loads configuration from environment variables
func loadConfig(ctx context.Context) Config {
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}
	return cfg
}

// initLogger initializes the logger based on environment
func initLogger(environment string) *zap.Logger {
	var logger *zap.Logger
	var err error

	if environment == "development" || environment == "dev" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}

	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}

	return logger
}

// processPrompt sends a prompt and handles the complete workflow
func processPrompt(ctx context.Context, a2aClient client.A2AClient, prompt, downloadsDir string, logger *zap.Logger) error {
	// Send task
	taskID, err := sendTask(ctx, a2aClient, prompt)
	if err != nil {
		return fmt.Errorf("send task: %w", err)
	}

	fmt.Printf("Task ID: %s\n", taskID)
	fmt.Print("Polling for result")

	// Poll for completion
	task, err := pollForCompletion(ctx, a2aClient, taskID)
	if err != nil {
		return fmt.Errorf("poll for completion: %w", err)
	}

	// Display response
	displayResponse(task)

	// Download artifacts if any
	if len(task.Artifacts) > 0 {
		return downloadArtifacts(ctx, a2aClient, task, downloadsDir, logger)
	}

	fmt.Println("\n⚠️  No artifacts were created (LLM may have chosen not to create an artifact)")
	return nil
}

// sendTask sends a message and returns the task ID
func sendTask(ctx context.Context, a2aClient client.A2AClient, prompt string) (string, error) {
	message := types.Message{
		Role: types.RoleUser,
		Parts: []types.Part{
			types.CreateTextPart(prompt),
		},
	}

	params := types.MessageSendParams{
		Message: message,
	}

	response, err := a2aClient.SendTask(ctx, params)
	if err != nil {
		return "", err
	}

	var taskResult struct {
		ID string `json:"id"`
	}

	resultBytes, ok := response.Result.(json.RawMessage)
	if !ok {
		return "", fmt.Errorf("failed to parse result as json.RawMessage")
	}

	if err := json.Unmarshal(resultBytes, &taskResult); err != nil {
		return "", fmt.Errorf("parse task ID: %w", err)
	}

	return taskResult.ID, nil
}

// pollForCompletion polls until task completes or fails
func pollForCompletion(ctx context.Context, a2aClient client.A2AClient, taskID string) (*types.Task, error) {
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for task completion")
		case <-ticker.C:
			fmt.Print(".")

			task, err := getTask(ctx, a2aClient, taskID)
			if err != nil {
				continue
			}

			switch task.Status.State {
			case types.TaskStateCompleted:
				fmt.Println("\n✓ Task completed!")
				return task, nil
			case types.TaskStateFailed:
				fmt.Println("\n✗ Task failed")
				if task.Status.Message != nil {
					responseJSON, _ := json.MarshalIndent(task.Status.Message, "", "  ")
					fmt.Printf("Error: %s\n", string(responseJSON))
				}
				return nil, fmt.Errorf("task failed")
			}
		}
	}
}

// getTask retrieves a task by ID
func getTask(ctx context.Context, a2aClient client.A2AClient, taskID string) (*types.Task, error) {
	taskResponse, err := a2aClient.GetTask(ctx, types.TaskQueryParams{ID: taskID})
	if err != nil {
		return nil, err
	}

	taskResultBytes, ok := taskResponse.Result.(json.RawMessage)
	if !ok {
		return nil, fmt.Errorf("failed to parse task result")
	}

	var task types.Task
	if err := json.Unmarshal(taskResultBytes, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}

	return &task, nil
}

// displayResponse prints the task response
func displayResponse(task *types.Task) {
	if task.Status.Message == nil {
		return
	}

	for _, part := range task.Status.Message.Parts {
		if part.Text != nil {
			fmt.Printf("\nResponse: %s\n", *part.Text)
		}
	}
}

// downloadArtifacts downloads all artifacts from a task
func downloadArtifacts(ctx context.Context, a2aClient client.A2AClient, task *types.Task, downloadsDir string, logger *zap.Logger) error {
	fmt.Printf("\n📎 Found %d artifact(s):\n", len(task.Artifacts))

	// Display artifact details
	for i, artifact := range task.Artifacts {
		fmt.Printf("\n🗂️  Artifact %d:\n", i+1)
		fmt.Printf("   ID: %s\n", artifact.ArtifactID)
		if artifact.Name != nil {
			fmt.Printf("   Name: %s\n", *artifact.Name)
		}
		if artifact.Description != nil {
			fmt.Printf("   Description: %s\n", *artifact.Description)
		}
	}

	// Use artifact helper to download
	helper := a2aClient.GetArtifactHelper()
	downloadConfig := &client.DownloadConfig{
		OutputDir:            downloadsDir,
		OverwriteExisting:    true,
		OrganizeByArtifactID: true,
	}

	fmt.Println("\n📥 Downloading artifacts...")
	results, err := helper.DownloadAllArtifacts(ctx, task, downloadConfig)
	if err != nil {
		return fmt.Errorf("download artifacts: %w", err)
	}

	// Display results
	for _, result := range results {
		if result.Error != nil {
			fmt.Printf("   ❌ Failed to download %s: %v\n", result.FileName, result.Error)
			logger.Error("artifact download failed",
				zap.String("filename", result.FileName),
				zap.Error(result.Error))
		} else {
			fmt.Printf("   ✅ Downloaded %s (%d bytes)\n      Saved to: %s\n",
				result.FileName, result.BytesWritten, result.FilePath)
			logger.Info("artifact downloaded successfully",
				zap.String("filename", result.FileName),
				zap.String("path", result.FilePath),
				zap.Int64("bytes", result.BytesWritten))
		}
	}

	return nil
}
