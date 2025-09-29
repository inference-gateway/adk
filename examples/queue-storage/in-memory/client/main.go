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
	// Setup logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	// Get server URL from environment or use default
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	logger.Info("Starting A2A client for in-memory queue storage demo",
		zap.String("server_url", serverURL))

	// Create A2A client
	a2aClient := client.NewClientWithLogger(serverURL, logger)

	// Test server health
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := a2aClient.GetHealth(ctx)
	if err != nil {
		logger.Fatal("Failed to get server health", zap.Error(err))
	}

	logger.Info("Server health check passed", zap.String("status", health.Status))

	// Submit multiple tasks to demonstrate queue processing
	tasks := []string{
		"Process order #12345",
		"Generate monthly report for March 2024",
		"Send notification to user@example.com",
		"Backup database to cloud storage",
		"Clean up temporary files older than 30 days",
	}

	var submittedTasks []string

	for i, taskContent := range tasks {
		contextID := fmt.Sprintf("demo-context-%d", i+1)

		logger.Info("Submitting task to in-memory queue",
			zap.String("context_id", contextID),
			zap.String("content", taskContent))

		message := types.Message{
			ContextID: &contextID,
			Kind:      "request",
			MessageID: fmt.Sprintf("msg-%d", i+1),
			Role:      "user",
			Parts: []types.Part{
				types.TextPart{
					Kind: "text",
					Text: taskContent,
				},
			},
		}
		params := types.MessageSendParams{
			Message: message,
		}
		resp, err := a2aClient.SendTask(ctx, params)
		if err != nil {
			logger.Error("Failed to submit task",
				zap.Error(err),
				zap.String("content", taskContent))
			continue
		}

		// Parse the response to extract task
		var task types.Task
		resultBytes, ok := resp.Result.(json.RawMessage)
		if !ok {
			logger.Error("Failed to cast response to json.RawMessage",
				zap.String("content", taskContent))
			continue
		}
		if err := json.Unmarshal(resultBytes, &task); err != nil {
			logger.Error("Failed to parse task response",
				zap.Error(err),
				zap.String("content", taskContent))
			continue
		}

		submittedTasks = append(submittedTasks, task.ID)

		logger.Info("Task submitted successfully",
			zap.String("task_id", task.ID),
			zap.String("context_id", contextID))

		// Small delay between submissions to see queue behavior
		time.Sleep(1 * time.Second)
	}

	// Wait a bit for processing to complete
	logger.Info("Waiting for tasks to be processed by in-memory queue...")
	time.Sleep(5 * time.Second)

	// Check status of submitted tasks
	for _, taskID := range submittedTasks {
		// Create fresh context for each GetTask call
		taskCtx, taskCancel := context.WithTimeout(context.Background(), 5*time.Second)
		params := types.TaskQueryParams{
			ID: taskID,
		}
		resp, err := a2aClient.GetTask(taskCtx, params)
		taskCancel()
		if err != nil {
			logger.Error("Failed to get task status",
				zap.Error(err),
				zap.String("task_id", taskID))
			continue
		}

		// Parse the response directly as Task struct
		var task types.Task
		resultBytes, ok := resp.Result.(json.RawMessage)
		if !ok {
			logger.Error("Failed to cast response to json.RawMessage",
				zap.String("task_id", taskID))
			continue
		}
		if err := json.Unmarshal(resultBytes, &task); err != nil {
			logger.Error("Failed to parse task response",
				zap.Error(err),
				zap.String("task_id", taskID))
			continue
		}
		status := task.Status
		history := task.History

		logger.Info("Task status",
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID),
			zap.String("state", string(status.State)),
			zap.Int("history_length", len(history)))

		// Print the last message if available
		if len(history) > 0 {
			lastMessage := history[len(history)-1]
			// Extract text from the last message parts
			var content string
			for _, part := range lastMessage.Parts {
				if textPart, ok := part.(types.TextPart); ok {
					content = textPart.Text
					break
				}
			}
			contentPreview := content
			if len(content) > 100 {
				contentPreview = content[:100] + "..."
			}
			logger.Info("Task response",
				zap.String("task_id", task.ID),
				zap.String("role", lastMessage.Role),
				zap.String("content", contentPreview))
		}
	}

	// Demonstrate queue stats
	logger.Info("In-memory queue demo completed successfully",
		zap.Int("tasks_submitted", len(submittedTasks)))

	logger.Info("Demo highlights:")
	logger.Info("✓ Tasks queued and processed using in-memory storage")
	logger.Info("✓ No external dependencies required")
	logger.Info("✓ Fast processing with direct memory access")
	logger.Info("✓ Perfect for development and testing scenarios")
	logger.Info("⚠ Tasks will be lost if server restarts (memory-only)")
}
