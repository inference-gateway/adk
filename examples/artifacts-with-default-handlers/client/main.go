package main

import (
	"context"
	"encoding/base64"
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
	Environment string `env:"ENVIRONMENT,default=development"`
	ServerURL   string `env:"SERVER_URL,default=http://localhost:8080"`
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
		zap.String("example", "artifacts with default handlers"))

	// Create A2A client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	// Define test prompts
	prompts := []string{
		"Please analyze this energy data file I'm uploading and create a comprehensive analysis report",
		"Generate a comprehensive report about artificial intelligence trends in 2024, including machine learning, neural networks, and large language models. Please create it in markdown format.",
		"Please do the following: 1) Create a sequence diagram showing microservices communication, 2) Generate a report about cloud computing trends, and 3) Export sample customer data in CSV format.",
	}

	// Execute tests
	for i, prompt := range prompts {
		logger.Info("running test", zap.Int("test_number", i+1))
		runTest(a2aClient, logger, prompt, i == 0) // Only first test includes file upload
		time.Sleep(2 * time.Second)
	}

	logger.Info("all tests completed")
}

func runTest(a2aClient client.A2AClient, logger *zap.Logger, prompt string, includeFile bool) {
	// Create message parts
	parts := []types.Part{
		types.TextPart{
			Kind: "text",
			Text: prompt,
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
		filename := "energy_data_2024.txt"
		mimeType := "text/plain"
		parts = append(parts, types.FilePart{
			Kind: "file",
			File: map[string]any{
				"bytes":    encodedContent,
				"mimeType": mimeType,
				"name":     filename,
			},
		})
		logger.Info("uploading file", zap.String("filename", "energy_data_2024.txt"), zap.Int("size_bytes", len(fileContent)))
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
		logger.Error("error sending task", zap.Error(err))
		return
	}

	// Parse response result as Task
	var task types.Task
	if resultBytes, ok := response.Result.(json.RawMessage); ok {
		if err := json.Unmarshal(resultBytes, &task); err != nil {
			logger.Error("error parsing task response", zap.Error(err))
			return
		}
	} else {
		logger.Error("unexpected response format", zap.String("type", fmt.Sprintf("%T", response.Result)))
		return
	}

	logger.Info("task created", zap.String("task_id", task.ID))

	// Poll for completion
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Warn("timeout waiting for completion")
			return
		case <-ticker.C:
			taskResponse, err := a2aClient.GetTask(ctx, types.TaskQueryParams{ID: task.ID})
			if err != nil {
				logger.Debug("error getting task status", zap.Error(err))
				continue
			}

			// Parse as proper Task struct
			var updatedTask types.Task
			if resultBytes, ok := taskResponse.Result.(json.RawMessage); ok {
				if err := json.Unmarshal(resultBytes, &updatedTask); err != nil {
					logger.Debug("error parsing task", zap.Error(err))
					continue
				}
			} else {
				continue
			}

			switch updatedTask.Status.State {
			case types.TaskStateCompleted:
				logger.Info("task completed")
				if len(updatedTask.Artifacts) == 0 {
					return
				}

				logger.Info("found artifacts", zap.Int("count", len(updatedTask.Artifacts)))

				helper := a2aClient.GetArtifactHelper()
				downloadConfig := &client.DownloadConfig{
					OutputDir:            "downloads",
					OverwriteExisting:    true,
					OrganizeByArtifactID: true,
				}

				results, err := helper.DownloadAllArtifacts(ctx, &updatedTask, downloadConfig)
				if err != nil {
					logger.Error("download error", zap.Error(err))
					return
				}

				for _, result := range results {
					if result.Error != nil {
						logger.Error("failed to download artifact",
							zap.String("filename", result.FileName),
							zap.Error(result.Error))
					} else {
						logger.Info("downloaded artifact",
							zap.String("filename", result.FileName),
							zap.Int64("bytes", result.BytesWritten),
							zap.String("path", result.FilePath))
						fmt.Printf("Downloaded %s to %s\n", result.FileName, result.FilePath)
					}
				}
				return
			case types.TaskStateFailed:
				logger.Error("task failed")
				return
			}
		}
	}
}
