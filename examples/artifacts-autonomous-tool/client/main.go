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

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
)

// Config holds client configuration
type Config struct {
	Environment  string `env:"ENVIRONMENT,default=development"`
	ServerURL    string `env:"SERVER_URL,default=http://localhost:8080"`
	ArtifactsURL string `env:"ARTIFACTS_URL,default=http://localhost:8081"`
	DownloadsDir string `env:"DOWNLOADS_DIR,default=downloads"`
}

// downloadArtifact downloads an artifact from the given URL
func downloadArtifact(url, filename, downloadDir string, logger *zap.Logger) error {
	// Create downloads directory if it doesn't exist
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create downloads directory: %w", err)
	}

	// Download the artifact
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download artifact: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Create the file
	filepath := filepath.Join(downloadDir, filename)
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write the content
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	logger.Info("artifact downloaded successfully",
		zap.String("filename", filename),
		zap.String("path", filepath))

	return nil
}

func main() {
	// Load configuration
	ctx := context.Background()
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize logger based on environment
	var logger *zap.Logger
	var err error
	if cfg.Environment == "development" || cfg.Environment == "dev" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	logger.Info("client starting",
		zap.String("server_url", cfg.ServerURL),
		zap.String("artifacts_url", cfg.ArtifactsURL))

	// Create client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Test prompts that should trigger artifact creation
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

		// Create message
		message := types.Message{
			Role: "user",
			Parts: []types.Part{
				types.TextPart{
					Kind: "text",
					Text: prompt.text,
				},
			},
		}

		// Send the message
		params := types.MessageSendParams{
			Message: message,
		}

		response, err := a2aClient.SendTask(ctx, params)
		if err != nil {
			logger.Error("failed to send message", zap.Int("message_number", i+1), zap.Error(err))
			continue
		}

		// Extract task ID from response
		var taskResult struct {
			ID string `json:"id"`
		}
		resultBytes, ok := response.Result.(json.RawMessage)
		if !ok {
			logger.Error("failed to parse result as json.RawMessage")
			continue
		}
		if err := json.Unmarshal(resultBytes, &taskResult); err != nil {
			logger.Error("failed to parse task ID", zap.Error(err))
			continue
		}

		fmt.Printf("Task ID: %s\n", taskResult.ID)
		fmt.Print("Polling for result")

		// Poll for task completion
		var task types.Task
		for {
			time.Sleep(1 * time.Second)
			fmt.Print(".")

			taskResponse, err := a2aClient.GetTask(ctx, types.TaskQueryParams{
				ID: taskResult.ID,
			})
			if err != nil {
				logger.Error("failed to get task status", zap.Error(err))
				fmt.Println()
				break
			}

			taskResultBytes, ok := taskResponse.Result.(json.RawMessage)
			if !ok {
				logger.Error("failed to parse task result as json.RawMessage")
				fmt.Println()
				break
			}
			if err := json.Unmarshal(taskResultBytes, &task); err != nil {
				logger.Error("failed to parse task", zap.Error(err))
				fmt.Println()
				break
			}

			// Check if task is completed
			if task.Status.State == types.TaskStateCompleted {
				fmt.Println("\n✓ Task completed!")

				// Display the response
				if task.Status.Message != nil {
					for _, part := range task.Status.Message.Parts {
						if textPart, ok := part.(types.TextPart); ok {
							fmt.Printf("\nResponse: %s\n", textPart.Text)
						}
					}
				}

				// Check for artifacts
				if len(task.Artifacts) > 0 {
					fmt.Printf("\n📎 Found %d artifact(s):\n", len(task.Artifacts))

					for _, artifact := range task.Artifacts {
						name := "Unknown"
						if artifact.Name != nil {
							name = *artifact.Name
						}
						fmt.Printf("  - %s (ID: %s)\n", name, artifact.ArtifactID)

						// Download each artifact
						for _, part := range artifact.Parts {
							if filePart, ok := part.(types.FilePart); ok {
								if fileWithURI, ok := filePart.File.(types.FileWithUri); ok {
									filename := "artifact"
									if fileWithURI.Name != nil {
										filename = *fileWithURI.Name
									}

									fmt.Printf("    Downloading: %s from %s\n", filename, fileWithURI.URI)

									if err := downloadArtifact(fileWithURI.URI, filename, cfg.DownloadsDir, logger); err != nil {
										logger.Error("failed to download artifact",
											zap.String("filename", filename),
											zap.Error(err))
									} else {
										fmt.Printf("    ✓ Saved to: %s/%s\n", cfg.DownloadsDir, filename)
									}
								}
							}
						}
					}
				} else {
					fmt.Println("\n⚠️  No artifacts were created (LLM may have chosen not to create an artifact)")
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

		// Small delay between requests
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n--- All Requests Complete ---")
	fmt.Printf("Check the '%s' directory for downloaded artifacts\n", cfg.DownloadsDir)
}
