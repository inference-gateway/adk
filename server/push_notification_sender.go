package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	types "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// PushNotificationSender handles sending push notifications
type PushNotificationSender interface {
	SendTaskUpdate(ctx context.Context, config types.PushNotificationConfig, task *types.Task) error
}

// HTTPPushNotificationSender implements push notifications via HTTP webhooks
type HTTPPushNotificationSender struct {
	httpClient *http.Client
	logger     *zap.Logger
}

// NewHTTPPushNotificationSender creates a new HTTP-based push notification sender
func NewHTTPPushNotificationSender(logger *zap.Logger) *HTTPPushNotificationSender {
	return &HTTPPushNotificationSender{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// TaskUpdateNotification represents the payload sent to webhook URLs
type TaskUpdateNotification struct {
	Type      string      `json:"type"`
	TaskID    string      `json:"taskId"`
	State     string      `json:"state"`
	Timestamp string      `json:"timestamp"`
	Task      *types.Task `json:"task,omitempty"`
}

// SendTaskUpdate sends a push notification about a task update
func (s *HTTPPushNotificationSender) SendTaskUpdate(ctx context.Context, config types.PushNotificationConfig, task *types.Task) error {
	timestamp := ""
	if task.Status.Timestamp != nil {
		timestamp = time.Now().Format(time.RFC3339)
	}

	notification := TaskUpdateNotification{
		Type:      "task_update",
		TaskID:    task.ID,
		State:     string(task.Status.State),
		Timestamp: timestamp,
		Task:      task,
	}

	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", config.URL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "A2A-Server/1.0")

	if config.Token != nil && *config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+*config.Token)
	}

	if config.Authentication != nil {
		for _, scheme := range config.Authentication.Schemes {
			switch scheme {
			case "bearer":
				if config.Authentication.Credentials != nil {
					req.Header.Set("Authorization", "Bearer "+*config.Authentication.Credentials)
				}
			case "basic":
				if config.Authentication.Credentials != nil {
					req.Header.Set("Authorization", "Basic "+*config.Authentication.Credentials)
				}
			}
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send push notification: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.logger.Warn("failed to close response body", zap.Error(closeErr))
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("push notification webhook returned status %d", resp.StatusCode)
	}

	s.logger.Info("push notification sent successfully",
		zap.String("task_id", task.ID),
		zap.String("webhook_url", config.URL),
		zap.String("state", string(task.Status.State)),
		zap.Int("status_code", resp.StatusCode))

	return nil
}
