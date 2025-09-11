package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/inference-gateway/adk/client"
	adk "github.com/inference-gateway/adk/types"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
)

type Config struct {
	ServerURL        string        `env:"A2A_SERVER_URL,default=http://localhost:8080"`
	StreamingTimeout time.Duration `env:"STREAMING_TIMEOUT,default=2m"`
}

func main() {
	// Load configuration from environment variables
	ctx := context.Background()
	var config Config
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatal("failed to process configuration", zap.Error(err))
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("starting a2a paused task streaming example",
		zap.String("server_url", config.ServerURL),
		zap.Duration("streaming_timeout", config.StreamingTimeout))

	// Create A2A client
	a2aClient := client.NewClientWithLogger(config.ServerURL, logger)

	// Check agent capabilities first
	logger.Info("checking agent capabilities")
	agentCard, err := a2aClient.GetAgentCard(ctx)
	if err != nil {
		logger.Fatal("failed to get agent card", zap.Error(err))
	}

	logger.Info("agent card retrieved",
		zap.String("agent_name", agentCard.Name),
		zap.String("agent_version", agentCard.Version),
		zap.String("agent_description", agentCard.Description))

	// Verify streaming capability
	if agentCard.Capabilities.Streaming == nil || !*agentCard.Capabilities.Streaming {
		logger.Fatal("agent does not support streaming capabilities",
			zap.String("agent_name", agentCard.Name),
			zap.Bool("streaming_supported", agentCard.Capabilities.Streaming != nil && *agentCard.Capabilities.Streaming))
	}

	logger.Info("agent streaming capability verified",
		zap.Bool("streaming_supported", *agentCard.Capabilities.Streaming))

	// Create context with timeout for streaming
	ctx, cancel := context.WithTimeout(context.Background(), config.StreamingTimeout)
	defer cancel()

	// Prepare initial message that will trigger input requirements
	msgParams := adk.MessageSendParams{
		Message: adk.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("paused-streaming-msg-%d", time.Now().Unix()),
			Role:      "user",
			Parts: []adk.Part{
				map[string]any{
					"kind": "text",
					"text": "I need help planning a vacation. Please ask me questions to understand my preferences and then create a detailed itinerary.",
				},
			},
		},
		Configuration: &adk.MessageSendConfiguration{
			Blocking:            boolPtr(false),
			AcceptedOutputModes: []string{"text"},
		},
	}

	logger.Info("starting paused task streaming",
		zap.String("message_id", msgParams.Message.MessageID))

	fmt.Printf("🚀 Starting paused task streaming example...\n")
	fmt.Printf("📝 Initial request: %s\n\n", msgParams.Message.Parts[0].(map[string]any)["text"])

	// Track streaming progress
	var eventCount int
	var streamError error
	var currentTaskID string
	var taskPaused bool
	var pauseMessage string

	// Start streaming task
	logger.Info("initiating streaming request")
	eventChan, err := a2aClient.SendTaskStreaming(ctx, msgParams)
	if err != nil {
		streamError = err
		logger.Error("streaming task failed", zap.Error(err))
		fmt.Printf("❌ Streaming failed: %v\n", err)
	} else {
		logger.Info("streaming task started successfully")
		fmt.Printf("✅ Initial streaming started successfully\n")

		logger.Info("=== Starting Stream Processing ===")
		fmt.Printf("📡 Processing streaming events...\n\n")

		// Process streaming events from the returned channel
		for {
			select {
			case event, ok := <-eventChan:
				if !ok {
					logger.Info("=== Stream Channel Closed ===")
					fmt.Printf("\n📡 Stream completed.\n")
					goto streamComplete
				}

				eventCount++

				// Handle different types of streaming events
				switch v := event.(type) {
				case string:
					// Simple string events (rare in A2A)
					logger.Info("received streaming text",
						zap.Int("event_number", eventCount),
						zap.String("content", v))
					fmt.Printf("💬 Text: %s\n", v)

				case map[string]any:
					// Complex event objects (task updates, messages)
					if eventType, exists := v["kind"]; exists {
						switch eventType {
						case "status-update":
							handleStatusUpdate(v, &currentTaskID, &taskPaused, &pauseMessage, eventCount, logger)

						default:
							logger.Info("received unknown event type",
								zap.Int("event_number", eventCount),
								zap.String("type", fmt.Sprintf("%v", eventType)),
								zap.Any("event", v))
							fmt.Printf("❓ Unknown Event: %v\n", v)
						}
					} else {
						logger.Info("received untyped object event",
							zap.Int("event_number", eventCount),
							zap.Any("event", v))
						fmt.Printf("📦 Object: %v\n", v)
					}

				default:
					// Handle any other type of event
					logger.Info("received generic event",
						zap.Int("event_number", eventCount),
						zap.Any("event", v))
					fmt.Printf("🔗 Generic: %v\n", v)
				}

			case <-ctx.Done():
				logger.Info("stream processing cancelled due to context timeout")
				fmt.Printf("\n⏰ Stream processing timed out\n")
				goto streamComplete
			}
		}

		streamComplete:
		logger.Info("streaming task completed successfully")
	}

	// Handle paused task if needed
	if taskPaused && currentTaskID != "" {
		fmt.Printf("\n⏸️  Task paused for input!\n")
		fmt.Printf("📋 Task ID: %s\n", currentTaskID)

		if pauseMessage != "" {
			fmt.Printf("💭 Agent says: %s\n", pauseMessage)
		}

		// Show conversation history
		showConversationHistory(ctx, a2aClient, currentTaskID, logger)

		// Get user input and resume
		for {
			userInput, err := getUserInput()
			if err != nil {
				logger.Error("failed to get user input", zap.Error(err))
				break
			}

			if strings.ToLower(userInput) == "quit" || strings.ToLower(userInput) == "exit" {
				fmt.Printf("👋 Exiting...\n")
				break
			}

			// Resume with streaming
			err = resumeTaskWithStreaming(ctx, a2aClient, currentTaskID, userInput, config.StreamingTimeout, logger)
			if err != nil {
				logger.Error("failed to resume task with streaming", zap.Error(err))
				fmt.Printf("❌ Failed to resume task: %v\n", err)
				break
			}

			// Check if task is still paused
			taskResp, err := a2aClient.GetTask(ctx, adk.TaskQueryParams{ID: currentTaskID})
			if err != nil {
				logger.Error("failed to get task status", zap.Error(err))
				break
			}

			// Extract task from response
			var task adk.Task
			if taskResultBytes, ok := taskResp.Result.(json.RawMessage); ok {
				if err := json.Unmarshal(taskResultBytes, &task); err != nil {
					logger.Error("failed to unmarshal task", zap.Error(err))
					break
				}
			} else {
				logger.Error("unexpected task response format")
				break
			}

			if task.Status.State != adk.TaskStateInputRequired {
				fmt.Printf("✅ Task completed!\n")
				showFinalResult(&task)
				break
			} else {
				fmt.Printf("\n⏸️  Task still needs more input...\n")
				if task.Status.Message != nil {
					extractedText := extractTextFromMessage(task.Status.Message)
					if extractedText != "" {
						fmt.Printf("💭 Agent says: %s\n", extractedText)
					}
				}
			}
		}
	}

	// Display final results
	logger.Info("=== Final Summary ===")
	fmt.Printf("\n📊 Final Summary:\n")
	fmt.Printf("   Total events: %d\n", eventCount)

	if streamError != nil {
		logger.Fatal("streaming failed",
			zap.Error(streamError),
			zap.Int("events_received", eventCount))
		fmt.Printf("   Status: ❌ Failed\n")
		fmt.Printf("   Error: %v\n", streamError)
	} else {
		fmt.Printf("   Status: ✅ Success\n")
		fmt.Printf("   Task handling: %s\n", func() string {
			if taskPaused {
				return "⏸️  Paused and handled"
			}
			return "🏃 Completed without pause"
		}())
	}

	if eventCount == 0 {
		fmt.Printf("\n⚠️  No streaming events received. This could indicate:\n")
		fmt.Printf("   - The server doesn't support streaming\n")
		fmt.Printf("   - The server is not configured for streaming responses\n")
		fmt.Printf("   - Network or connection issues\n")
		logger.Warn("no streaming events received - check server streaming capabilities")
	}

	fmt.Printf("\n🎉 Paused task streaming example completed!\n")
}

func handleStatusUpdate(event map[string]any, currentTaskID *string, taskPaused *bool, pauseMessage *string, eventCount int, logger *zap.Logger) {
	if taskId, exists := event["taskId"]; exists {
		*currentTaskID = fmt.Sprintf("%v", taskId)

		if status, exists := event["status"]; exists {
			if statusMap, ok := status.(map[string]any); ok {
				if state, exists := statusMap["state"]; exists {
					stateStr := fmt.Sprintf("%v", state)

					logger.Info("received task status update",
						zap.Int("event_number", eventCount),
						zap.String("task_id", *currentTaskID),
						zap.String("state", stateStr))

					switch stateStr {
					case "input-required":
						*taskPaused = true
						fmt.Printf("⏸️  [Event %d] Task paused - input required (Task: %s)\n", eventCount, *currentTaskID)

						// Extract pause message if available
						if message, exists := statusMap["message"]; exists {
							if msgMap, ok := message.(map[string]any); ok {
								*pauseMessage = extractTextFromMessageMap(msgMap)
							}
						}

					case "working":
						fmt.Printf("⚡ [Event %d] Task working (Task: %s)\n", eventCount, *currentTaskID)

						// Show streaming content
						if message, exists := statusMap["message"]; exists {
							if msgMap, ok := message.(map[string]any); ok {
								if parts, exists := msgMap["parts"]; exists {
									if partsArray, ok := parts.([]any); ok {
										for _, part := range partsArray {
											if partMap, ok := part.(map[string]any); ok {
												if text, exists := partMap["text"]; exists {
													fmt.Printf("💬 %v", text)
												}
											}
										}
									}
								}
							}
						}

					case "completed":
						fmt.Printf("✅ [Event %d] Task completed (Task: %s)\n", eventCount, *currentTaskID)

					case "failed":
						fmt.Printf("❌ [Event %d] Task failed (Task: %s)\n", eventCount, *currentTaskID)

					default:
						fmt.Printf("🔄 [Event %d] Task state: %s (Task: %s)\n", eventCount, stateStr, *currentTaskID)
					}
				}
			}
		}
	}
}

func resumeTaskWithStreaming(ctx context.Context, a2aClient client.A2AClient, taskID, userInput string, timeout time.Duration, logger *zap.Logger) error {
	// Create resume message
	resumeParams := adk.MessageSendParams{
		Message: adk.Message{
			Kind:      "message",
			MessageID: fmt.Sprintf("resume-msg-%d", time.Now().Unix()),
			Role:      "user",
			TaskID:    &taskID,
			Parts: []adk.Part{
				map[string]any{
					"kind": "text",
					"text": userInput,
				},
			},
		},
		Configuration: &adk.MessageSendConfiguration{
			Blocking:            boolPtr(false),
			AcceptedOutputModes: []string{"text"},
		},
	}

	fmt.Printf("\n🔄 Resuming task with streaming...\n")
	logger.Info("resuming task with streaming input",
		zap.String("task_id", taskID),
		zap.String("input", userInput))

	// Create new context for resume operation
	resumeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Send resume with streaming
	eventChan, err := a2aClient.SendTaskStreaming(resumeCtx, resumeParams)
	if err != nil {
		return fmt.Errorf("failed to resume with streaming: %w", err)
	}

	// Process streaming events for resume
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				goto resumeComplete
			}

			switch v := event.(type) {
			case map[string]any:
				if eventType, exists := v["kind"]; exists && eventType == "status-update" {
					if status, exists := v["status"]; exists {
						if statusMap, ok := status.(map[string]any); ok {
							if message, exists := statusMap["message"]; exists {
								if msgMap, ok := message.(map[string]any); ok {
									if parts, exists := msgMap["parts"]; exists {
										if partsArray, ok := parts.([]any); ok {
											for _, part := range partsArray {
												if partMap, ok := part.(map[string]any); ok {
													if text, exists := partMap["text"]; exists {
														fmt.Printf("💬 %v", text)
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}

		case <-resumeCtx.Done():
			goto resumeComplete
		}
	}

	resumeComplete:
	fmt.Printf("\n✅ Resume streaming completed\n")
	return nil
}

func showConversationHistory(ctx context.Context, a2aClient client.A2AClient, taskID string, logger *zap.Logger) {
	taskResp, err := a2aClient.GetTask(ctx, adk.TaskQueryParams{ID: taskID})
	if err != nil {
		logger.Error("failed to get task for conversation history", zap.Error(err))
		return
	}

	var task adk.Task
	if taskResultBytes, ok := taskResp.Result.(json.RawMessage); ok {
		if err := json.Unmarshal(taskResultBytes, &task); err != nil {
			logger.Error("failed to parse task response", zap.Error(err))
			return
		}
	} else {
		logger.Error("unexpected task response format")
		return
	}

	if len(task.History) == 0 {
		fmt.Printf("📝 (No conversation history available)\n")
		return
	}

	fmt.Printf("\n📝 Recent conversation:\n")
	fmt.Printf("%s\n", strings.Repeat("-", 50))

	// Show last few messages for context
	start := len(task.History) - 5
	if start < 0 {
		start = 0
	}

	for i := start; i < len(task.History); i++ {
		msg := task.History[i]
		role := "👤 User"
		if msg.Role == "assistant" {
			role = "🤖 Assistant"
		} else if msg.Role == "tool" {
			role = "🔧 Tool"
		}

		textContent := extractTextFromMessage(&msg)
		if textContent != "" {
			// Truncate very long messages for context
			if len(textContent) > 500 {
				textContent = textContent[:497] + "..."
			}
			fmt.Printf("%s: %s\n", role, textContent)
		}
	}
	fmt.Printf("%s\n", strings.Repeat("-", 50))
}

func getUserInput() (string, error) {
	fmt.Printf("\n💬 Please provide your input (or 'quit' to exit): ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func extractTextFromMessage(message *adk.Message) string {
	if message == nil || len(message.Parts) == 0 {
		return ""
	}

	var texts []string
	for _, part := range message.Parts {
		if partMap, ok := part.(map[string]any); ok {
			if text, exists := partMap["text"]; exists {
				if textStr, ok := text.(string); ok && textStr != "" {
					texts = append(texts, textStr)
				}
			}
		}
	}

	return strings.Join(texts, " ")
}

func extractTextFromMessageMap(msgMap map[string]any) string {
	if parts, exists := msgMap["parts"]; exists {
		if partsArray, ok := parts.([]any); ok {
			var texts []string
			for _, part := range partsArray {
				if partMap, ok := part.(map[string]any); ok {
					if text, exists := partMap["text"]; exists {
						if textStr, ok := text.(string); ok && textStr != "" {
							texts = append(texts, textStr)
						}
					}
				}
			}
			return strings.Join(texts, " ")
		}
	}
	return ""
}

func showFinalResult(task *adk.Task) {
	fmt.Printf("\n🎯 Final Result:\n")
	fmt.Printf("%s\n", strings.Repeat("=", 60))

	if task.Status.Message != nil {
		finalText := extractTextFromMessage(task.Status.Message)
		if finalText != "" {
			fmt.Printf("%s\n", finalText)
		}
	}

	fmt.Printf("%s\n", strings.Repeat("=", 60))
	fmt.Printf("📊 Conversation included %d messages\n", len(task.History))
}

func boolPtr(b bool) *bool {
	return &b
}
