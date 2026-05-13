package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	uuid "github.com/google/uuid"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
)

// Config holds client configuration.
type Config struct {
	Environment string `env:"ENVIRONMENT,default=development"`
	ServerURL   string `env:"SERVER_URL,default=http://localhost:8080"`
	// WebhookURL1 and WebhookURL2 are the URLs the server POSTs push
	// notifications to. Two sinks demonstrate that the server fans out
	// notifications to every registered webhook.
	WebhookURL1 string `env:"WEBHOOK_URL_1,default=http://localhost:9000/webhook"`
	WebhookURL2 string `env:"WEBHOOK_URL_2,default=http://localhost:9001/webhook"`
}

// submitTask sends a single message/send request and returns the created task.
func submitTask(ctx context.Context, a2a client.A2AClient, text string, logger *zap.Logger) (*types.Task, error) {
	resp, err := a2a.SendTask(ctx, types.MessageSendParams{
		Message: types.Message{
			MessageID: uuid.New().String(),
			Role:      types.RoleUser,
			Parts:     []types.Part{types.CreateTextPart(text)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("send task: %w", err)
	}
	taskBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var task types.Task
	if err := json.Unmarshal(taskBytes, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}
	logger.Info("task submitted", zap.String("task_id", task.ID), zap.String("state", string(task.Status.State)))
	return &task, nil
}

// demonstrateAuthenticatedExtendedCard calls `agent/getAuthenticatedExtendedCard`.
//
// This is the JSON-RPC counterpart of the public `/.well-known/agent-card.json`
// endpoint. It returns the same agent card the server is configured with, but
// the call travels through the JSON-RPC route - which means it is subject to
// whatever authentication middleware the server has installed.
func demonstrateAuthenticatedExtendedCard(ctx context.Context, a2a client.A2AClient, logger *zap.Logger) {
	fmt.Println("\n=== agent/getAuthenticatedExtendedCard ===")
	resp, err := a2a.GetAuthenticatedExtendedCard(ctx, types.GetAuthenticatedExtendedCardParams{})
	if err != nil {
		logger.Error("failed to fetch authenticated extended card", zap.Error(err))
		return
	}
	cardBytes, _ := json.MarshalIndent(resp.Result, "", "  ")
	fmt.Println(string(cardBytes))
}

// demonstrateListTasks calls `tasks/list` twice to walk through paginated results.
//
// The server caps the page size internally; what we control from the client is
// `Limit` (per-page size) and `Offset` (where to start). Iterating until we
// have collected `TotalSize` entries is the canonical pagination loop.
func demonstrateListTasks(ctx context.Context, a2a client.A2AClient, logger *zap.Logger) {
	fmt.Println("\n=== tasks/list (with pagination) ===")
	const pageSize = 2
	offset := 0
	page := 1
	for {
		resp, err := a2a.ListTasks(ctx, types.TaskListParams{
			Limit:  pageSize,
			Offset: offset,
		})
		if err != nil {
			logger.Error("failed to list tasks", zap.Error(err))
			return
		}

		listBytes, err := json.Marshal(resp.Result)
		if err != nil {
			logger.Error("failed to marshal list result", zap.Error(err))
			return
		}

		var taskList types.TaskList
		if err := json.Unmarshal(listBytes, &taskList); err != nil {
			logger.Error("failed to decode task list", zap.Error(err))
			return
		}

		fmt.Printf("Page %d (offset=%d, returned=%d, total=%d):\n",
			page, offset, len(taskList.Tasks), taskList.TotalSize)
		for _, t := range taskList.Tasks {
			fmt.Printf("  - %s [state=%s]\n", t.ID, t.Status.State)
		}

		offset += len(taskList.Tasks)
		page++

		// Stop once we've seen every task or the server stopped returning rows.
		if len(taskList.Tasks) == 0 || offset >= taskList.TotalSize {
			break
		}
	}
}

// setupPushNotificationConfig registers two webhooks and demonstrates the
// set/get/list config methods. It intentionally does NOT delete the configs so
// that both webhooks are still active when the task completes and real push
// notifications are delivered to both webhook sinks.
func setupPushNotificationConfig(ctx context.Context, a2a client.A2AClient, taskID string, webhookURLs []string, logger *zap.Logger) {
	fmt.Println("\n=== tasks/pushNotificationConfig/{set,get,list} ===")

	authToken := "demo-shared-secret"

	// 1. set: register a push notification webhook for each URL.
	for i, url := range webhookURLs {
		configID := uuid.New().String()
		setResp, err := a2a.SetTaskPushNotificationConfig(ctx, types.TaskPushNotificationConfig{
			Name: taskID,
			PushNotificationConfig: types.PushNotificationConfig{
				ID:    &configID,
				URL:   url,
				Token: &authToken,
			},
		})
		if err != nil {
			logger.Error("failed to set push notification config", zap.Error(err), zap.String("url", url))
			return
		}
		setBytes, _ := json.MarshalIndent(setResp.Result, "", "  ")
		fmt.Printf("set[%d] → registered webhook %s for task %s\n%s\n", i+1, url, taskID, string(setBytes))
	}

	// 2. get: read back the first config to verify the round-trip.
	getResp, err := a2a.GetTaskPushNotificationConfig(ctx, types.GetTaskPushNotificationConfigParams{
		Name: taskID,
	})
	if err != nil {
		logger.Error("failed to get push notification config", zap.Error(err))
		return
	}
	getBytes, _ := json.MarshalIndent(getResp.Result, "", "  ")
	fmt.Printf("get → \n%s\n", string(getBytes))

	// 3. list: show every config attached to this task — should contain both.
	listResp, err := a2a.ListTaskPushNotificationConfig(ctx, types.ListTaskPushNotificationConfigParams{
		Parent: taskID,
	})
	if err != nil {
		logger.Error("failed to list push notification configs", zap.Error(err))
		return
	}
	listBytes, _ := json.MarshalIndent(listResp.Result, "", "  ")
	fmt.Printf("list → \n%s\n", string(listBytes))

	fmt.Printf("(keeping %d configs alive so both webhook sinks receive notifications)\n", len(webhookURLs))
}

// waitForTaskAndCleanupPushConfig polls the task until it reaches a terminal
// state, then deletes the push notification config. By the time this function
// returns, the webhook-sink should have received at least one notification.
func waitForTaskAndCleanupPushConfig(ctx context.Context, a2a client.A2AClient, taskID string, logger *zap.Logger) {
	fmt.Println("\n=== waiting for push notification delivery ===")

	// Poll until the task reaches a terminal state.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Error("context cancelled while waiting for task", zap.String("task_id", taskID))
			return
		case <-ticker.C:
			resp, err := a2a.GetTask(ctx, types.TaskQueryParams{ID: taskID})
			if err != nil {
				logger.Error("failed to get task", zap.Error(err))
				continue
			}
			taskBytes, _ := json.Marshal(resp.Result)
			var task types.Task
			if err := json.Unmarshal(taskBytes, &task); err != nil {
				logger.Error("failed to decode task", zap.Error(err))
				continue
			}
			if task.Status.State == types.TaskStateCompleted ||
				task.Status.State == types.TaskStateFailed ||
				task.Status.State == types.TaskStateCancelled {
				fmt.Printf("task %s reached terminal state: %s\n", taskID, task.Status.State)
				fmt.Println("→ webhook-sink should have received a push notification")
				deletePushNotificationConfig(ctx, a2a, taskID, logger)
				return
			}
		}
	}
}

func deletePushNotificationConfig(ctx context.Context, a2a client.A2AClient, taskID string, logger *zap.Logger) {
	fmt.Println("\n=== tasks/pushNotificationConfig/delete ===")
	if _, err := a2a.DeleteTaskPushNotificationConfig(ctx, types.DeleteTaskPushNotificationConfigParams{
		Name: taskID,
	}); err != nil {
		logger.Error("failed to delete push notification config", zap.Error(err))
		return
	}
	fmt.Printf("delete → webhook config for task %s removed\n", taskID)
}

// demonstrateCancel cancels an in-flight task via `tasks/cancel`.
//
// The bundled server delays task completion for several seconds, so calling
// cancel immediately after `message/send` is enough to flip the task into
// the CANCELLED state.
func demonstrateCancel(ctx context.Context, a2a client.A2AClient, taskID string, logger *zap.Logger) {
	fmt.Println("\n=== tasks/cancel ===")
	resp, err := a2a.CancelTask(ctx, types.TaskIdParams{ID: taskID})
	if err != nil {
		logger.Error("failed to cancel task", zap.Error(err))
		return
	}
	taskBytes, _ := json.Marshal(resp.Result)
	var task types.Task
	if err := json.Unmarshal(taskBytes, &task); err != nil {
		logger.Error("failed to decode cancelled task", zap.Error(err))
		return
	}
	fmt.Printf("cancelled task %s → state=%s\n", task.ID, task.Status.State)
}

// demonstrateResubscribe opens a streaming task, drops the connection, and
// then reattaches with `tasks/resubscribe`.
func demonstrateResubscribe(ctx context.Context, a2a client.A2AClient, logger *zap.Logger) {
	fmt.Println("\n=== tasks/resubscribe ===")

	streamCtx, cancelStream := context.WithCancel(ctx)
	streamCh, err := a2a.SendTaskStreaming(streamCtx, types.MessageSendParams{
		Message: types.Message{
			MessageID: uuid.New().String(),
			Role:      types.RoleUser,
			Parts:     []types.Part{types.CreateTextPart("stream please")},
		},
	})
	if err != nil {
		cancelStream()
		logger.Error("failed to open initial stream", zap.Error(err))
		return
	}

	// Read the first event so the server creates the task and starts emitting.
	var taskID string
	select {
	case <-ctx.Done():
		cancelStream()
		return
	case evt, ok := <-streamCh:
		if !ok {
			cancelStream()
			logger.Warn("stream closed before first event")
			return
		}
		// The first envelope carries a TaskStatusUpdateEvent (with taskId),
		// not a full Task object (with id). Try both to be robust.
		resultBytes, _ := json.Marshal(evt.Result)
		var task types.Task
		if err := json.Unmarshal(resultBytes, &task); err == nil && task.ID != "" {
			taskID = task.ID
		}
		if taskID == "" {
			var statusUpdate types.TaskStatusUpdateEvent
			if err := json.Unmarshal(resultBytes, &statusUpdate); err == nil && statusUpdate.TaskID != "" {
				taskID = statusUpdate.TaskID
			}
		}
	}

	// Simulate a dropped connection by cancelling the original stream.
	cancelStream()
	fmt.Printf("dropped initial stream for task %s; re-attaching...\n", taskID)

	// Reattach with tasks/resubscribe. The server first re-emits the current
	// task state, then forwards any further streaming events.
	resubCh, err := a2a.ResubscribeTask(ctx, types.TaskResubscriptionParams{Name: taskID})
	if err != nil {
		logger.Error("resubscribe failed", zap.Error(err))
		return
	}

	deadline := time.After(15 * time.Second)
	events := 0
	for {
		select {
		case <-deadline:
			fmt.Printf("stopped reading after %d resubscribe events\n", events)
			return
		case evt, ok := <-resubCh:
			if !ok {
				fmt.Printf("resubscribe stream ended after %d events\n", events)
				return
			}
			events++
			payload, _ := json.Marshal(evt.Result)
			fmt.Printf("  event %d: %s\n", events, string(payload))
		}
	}
}

// Protocol Methods A2A Client Example
//
// Walks through every JSON-RPC method that is supported by the ADK beyond
// `message/send`, `message/stream`, and `tasks/get`. See the example README
// for an end-to-end description.
func main() {
	ctx := context.Background()
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

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
	defer func() { _ = logger.Sync() }()

	logger.Info("client starting", zap.String("server_url", cfg.ServerURL))

	a2a := client.NewClientWithLogger(cfg.ServerURL, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 1. Pull the authenticated/extended agent card.
	demonstrateAuthenticatedExtendedCard(ctx, a2a, logger)

	// 2. Submit a handful of tasks so tasks/list has something to page through.
	prompts := []string{
		"first protocol-methods task",
		"second protocol-methods task",
		"third protocol-methods task",
		"fourth protocol-methods task",
	}
	var submitted []*types.Task
	for _, prompt := range prompts {
		task, err := submitTask(ctx, a2a, prompt, logger)
		if err != nil {
			logger.Error("failed to submit task", zap.Error(err))
			continue
		}
		submitted = append(submitted, task)
	}

	if len(submitted) == 0 {
		logger.Fatal("no tasks were submitted")
	}

	// 3. List tasks with pagination.
	demonstrateListTasks(ctx, a2a, logger)

	// 4. Register a push notification webhook for the first task.
	//    The config stays active so the webhook-sink receives a real notification
	//    when the task completes (it has a ~6 s processing delay).
	setupPushNotificationConfig(ctx, a2a, submitted[0].ID, []string{cfg.WebhookURL1, cfg.WebhookURL2}, logger)

	// 5. Cancel a different task (it's still working, so the cancel sticks).
	demonstrateCancel(ctx, a2a, submitted[1].ID, logger)

	// 6. Open and resubscribe to a streaming task.
	demonstrateResubscribe(ctx, a2a, logger)

	// 7. Wait for the first task to finish so the push notification is
	//    delivered, then clean up the config.
	waitForTaskAndCleanupPushConfig(ctx, a2a, submitted[0].ID, logger)

	fmt.Println("\nAll protocol method demonstrations completed.")
}
