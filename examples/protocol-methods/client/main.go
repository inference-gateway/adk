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
	// WebhookURL is the URL the server is asked to POST push notifications to.
	// It does not need to be reachable for this example: we only use it to
	// demonstrate the `tasks/pushNotificationConfig/*` round-trip.
	WebhookURL string `env:"WEBHOOK_URL,default=http://localhost:9000/webhook"`
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

// demonstratePushNotificationConfig runs the full set/get/list/delete cycle
// against a single task.
func demonstratePushNotificationConfig(ctx context.Context, a2a client.A2AClient, taskID, webhookURL string, logger *zap.Logger) {
	fmt.Println("\n=== tasks/pushNotificationConfig/{set,get,list,delete} ===")

	configID := uuid.New().String()
	authToken := "demo-shared-secret"

	// 1. set: register a push notification webhook for this task.
	setResp, err := a2a.SetTaskPushNotificationConfig(ctx, types.TaskPushNotificationConfig{
		Name: taskID,
		PushNotificationConfig: types.PushNotificationConfig{
			ID:    &configID,
			URL:   webhookURL,
			Token: &authToken,
		},
	})
	if err != nil {
		logger.Error("failed to set push notification config", zap.Error(err))
		return
	}
	setBytes, _ := json.MarshalIndent(setResp.Result, "", "  ")
	fmt.Printf("set → registered webhook for task %s\n%s\n", taskID, string(setBytes))

	// 2. get: read the config we just registered.
	getResp, err := a2a.GetTaskPushNotificationConfig(ctx, types.GetTaskPushNotificationConfigParams{
		Name: taskID,
	})
	if err != nil {
		logger.Error("failed to get push notification config", zap.Error(err))
		return
	}
	getBytes, _ := json.MarshalIndent(getResp.Result, "", "  ")
	fmt.Printf("get → \n%s\n", string(getBytes))

	// 3. list: show every config attached to this task. With one registration
	// the result is a single-element slice, but the API supports many.
	listResp, err := a2a.ListTaskPushNotificationConfig(ctx, types.ListTaskPushNotificationConfigParams{
		Parent: taskID,
	})
	if err != nil {
		logger.Error("failed to list push notification configs", zap.Error(err))
		return
	}
	listBytes, _ := json.MarshalIndent(listResp.Result, "", "  ")
	fmt.Printf("list → \n%s\n", string(listBytes))

	// 4. delete: tear the config down.
	if _, err := a2a.DeleteTaskPushNotificationConfig(ctx, types.DeleteTaskPushNotificationConfigParams{
		Name: taskID,
	}); err != nil {
		logger.Error("failed to delete push notification config", zap.Error(err))
		return
	}
	fmt.Printf("delete → webhook for task %s removed\n", taskID)
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
		// The first envelope carries the freshly-created Task object.
		resultBytes, _ := json.Marshal(evt.Result)
		var task types.Task
		if err := json.Unmarshal(resultBytes, &task); err == nil && task.ID != "" {
			taskID = task.ID
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

	// 4. Run the push notification config family against the first task.
	demonstratePushNotificationConfig(ctx, a2a, submitted[0].ID, cfg.WebhookURL, logger)

	// 5. Cancel a different task (it's still working, so the cancel sticks).
	demonstrateCancel(ctx, a2a, submitted[1].ID, logger)

	// 6. Open and resubscribe to a streaming task.
	demonstrateResubscribe(ctx, a2a, logger)

	fmt.Println("\nAll protocol method demonstrations completed.")
}
