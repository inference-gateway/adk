package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	zap "go.uber.org/zap"

	types "github.com/inference-gateway/adk/types"
)

// PushNotificationSender handles sending push notifications
type PushNotificationSender interface {
	SendTaskUpdate(ctx context.Context, config types.TaskPushNotificationConfig, task *types.Task) error
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

// SendTaskUpdate posts the task to the webhook as an A2A StreamResponse (spec section 4.3.3).
func (s *HTTPPushNotificationSender) SendTaskUpdate(ctx context.Context, config types.TaskPushNotificationConfig, task *types.Task) error {
	payload, err := json.Marshal(types.StreamResponse{Task: task})
	if err != nil {
		return fmt.Errorf("failed to marshal notification payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", config.URL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/a2a+json")
	req.Header.Set("User-Agent", "A2A-Server/1.0")

	if config.Token != nil && *config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+*config.Token)
	}

	if config.Authentication != nil && config.Authentication.Credentials != nil {
		switch strings.ToLower(config.Authentication.Scheme) {
		case "bearer":
			req.Header.Set("Authorization", "Bearer "+*config.Authentication.Credentials)
		case "basic":
			req.Header.Set("Authorization", "Basic "+*config.Authentication.Credentials)
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
