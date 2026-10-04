package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	visible := task.WithoutExtension(types.UsageExtensionURI)
	payload, err := json.Marshal(types.StreamResponse{Task: &visible})
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
		zap.String("webhook_url", loggableURL(config.URL)),
		zap.String("state", string(task.Status.State)),
		zap.Int("status_code", resp.StatusCode))

	return nil
}

// loggableURL drops userinfo, query and fragment from a client-supplied URL
// before it is logged: webhook URLs often carry tokens there, and A2A section
// 13.4 forbids credentials in logs.
func loggableURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parsed.User, parsed.RawQuery, parsed.Fragment = nil, "", ""
	return parsed.String()
}
