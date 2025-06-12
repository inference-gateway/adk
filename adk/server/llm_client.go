package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	adk "github.com/inference-gateway/a2a/adk"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// LLMClient defines the interface for Language Model clients
type LLMClient interface {
	// CreateChatCompletion sends a chat completion request and returns the response
	CreateChatCompletion(ctx context.Context, messages []adk.Message) (*adk.Message, error)

	// CreateStreamingChatCompletion sends a streaming chat completion request
	CreateStreamingChatCompletion(ctx context.Context, messages []adk.Message) (<-chan *adk.Message, <-chan error)

	// HealthCheck verifies if the LLM client is healthy and can connect
	HealthCheck(ctx context.Context) error
}

// OpenAICompatibleLLMClient implements LLMClient using an OpenAI-compatible API via the Inference Gateway SDK
type OpenAICompatibleLLMClient struct {
	client   sdk.Client
	config   *LLMProviderClientConfig
	logger   *zap.Logger
	provider sdk.Provider
	model    string
}

// NewOpenAICompatibleLLMClient creates a new OpenAI-compatible LLM client
func NewOpenAICompatibleLLMClient(config *LLMProviderClientConfig, logger *zap.Logger) (*OpenAICompatibleLLMClient, error) {
	if config == nil {
		return nil, fmt.Errorf("llm provider client config is required")
	}

	clientOptions := &sdk.ClientOptions{}

	if config.BaseURL != "" {
		clientOptions.BaseURL = config.BaseURL
	}

	if config.APIKey != "" {
		clientOptions.APIKey = config.APIKey
	}

	if config.Timeout > 0 {
		clientOptions.Timeout = config.Timeout
	}

	if len(config.CustomHeaders) > 0 {
		clientOptions.Headers = config.CustomHeaders
	}

	client := sdk.NewClient(clientOptions)

	provider, err := parseProvider(config.Provider)
	if err != nil {
		return nil, fmt.Errorf("invalid provider %s: %w", config.Provider, err)
	}

	model := parseModelName(config.Model, config.Provider)

	return &OpenAICompatibleLLMClient{
		client:   client,
		config:   config,
		logger:   logger,
		provider: provider,
		model:    model,
	}, nil
}

// CreateChatCompletion implements LLMClient.CreateChatCompletion
func (c *OpenAICompatibleLLMClient) CreateChatCompletion(ctx context.Context, messages []adk.Message) (*adk.Message, error) {
	sdkMessages, err := c.convertToSDKMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	options := &sdk.CreateChatCompletionRequest{}

	if c.config.MaxTokens > 0 {
		options.MaxTokens = &c.config.MaxTokens
	}

	var response *sdk.CreateChatCompletionResponse
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Debug("retrying llm request",
				zap.Int("attempt", attempt),
				zap.Int("max_retries", c.config.MaxRetries),
				zap.Error(lastErr))

			backoff := time.Duration(attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		response, lastErr = c.client.WithOptions(options).GenerateContent(
			ctx,
			c.provider,
			c.model,
			sdkMessages,
		)
		if lastErr == nil {
			break
		}

		c.logger.Debug("llm request failed",
			zap.Error(lastErr),
			zap.Int("attempt", attempt+1))
	}

	if lastErr != nil {
		return nil, fmt.Errorf("llm request failed after %d retries: %w", c.config.MaxRetries, lastErr)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from llm")
	}

	return c.convertFromSDKMessage(response.Choices[0].Message), nil
}

// CreateStreamingChatCompletion implements LLMClient.CreateStreamingChatCompletion
func (c *OpenAICompatibleLLMClient) CreateStreamingChatCompletion(ctx context.Context, messages []adk.Message) (<-chan *adk.Message, <-chan error) {
	messageChan := make(chan *adk.Message)
	errorChan := make(chan error, 1)

	go func() {
		defer close(messageChan)
		defer close(errorChan)

		sdkMessages, err := c.convertToSDKMessages(messages)
		if err != nil {
			errorChan <- fmt.Errorf("failed to convert messages: %w", err)
			return
		}

		options := &sdk.CreateChatCompletionRequest{}

		if c.config.MaxTokens > 0 {
			options.MaxTokens = &c.config.MaxTokens
		}

		events, err := c.client.WithOptions(options).GenerateContentStream(
			ctx,
			c.provider,
			c.model,
			sdkMessages,
		)
		if err != nil {
			errorChan <- fmt.Errorf("failed to create stream: %w", err)
			return
		}

		for event := range events {
			if event.Event == nil {
				continue
			}

			switch *event.Event {
			case sdk.ContentDelta:
				if event.Data != nil {
					var streamResponse sdk.CreateChatCompletionStreamResponse
					if err := json.Unmarshal(*event.Data, &streamResponse); err != nil {
						c.logger.Debug("error parsing stream response", zap.Error(err))
						continue
					}

					for _, choice := range streamResponse.Choices {
						if choice.Delta.Content != "" {
							message := &adk.Message{
								Kind:      "message",
								MessageID: streamResponse.ID,
								Role:      "assistant",
								Parts: []adk.Part{
									map[string]interface{}{
										"kind": "text",
										"text": choice.Delta.Content,
									},
								},
							}

							select {
							case messageChan <- message:
							case <-ctx.Done():
								return
							}
						}
					}
				}

			case sdk.StreamEnd:
				return

			default:
				if event.Data != nil {
					var errResp struct {
						Error string `json:"error"`
					}
					if err := json.Unmarshal(*event.Data, &errResp); err == nil && errResp.Error != "" {
						errorChan <- fmt.Errorf("llm error: %s", errResp.Error)
						return
					}
				}
			}
		}
	}()

	return messageChan, errorChan
}

// HealthCheck implements LLMClient.HealthCheck
func (c *OpenAICompatibleLLMClient) HealthCheck(ctx context.Context) error {
	return c.client.HealthCheck(ctx)
}

// convertToSDKMessages converts A2A messages to Inference Gateway SDK format
func (c *OpenAICompatibleLLMClient) convertToSDKMessages(messages []adk.Message) ([]sdk.Message, error) {
	var sdkMessages []sdk.Message

	for _, msg := range messages {
		role := msg.Role
		if role == "" {
			role = "user"
		}

		var sdkRole sdk.MessageRole
		switch role {
		case "user":
			sdkRole = sdk.User
		case "assistant":
			sdkRole = sdk.Assistant
		case "system":
			sdkRole = sdk.System
		default:
			sdkRole = sdk.User
		}

		var content string
		for _, part := range msg.Parts {
			if partMap, ok := part.(map[string]interface{}); ok {
				if text, exists := partMap["text"]; exists {
					if textStr, ok := text.(string); ok {
						content += textStr
					}
				}
			}
		}

		sdkMessages = append(sdkMessages, sdk.Message{
			Role:    sdkRole,
			Content: content,
		})
	}

	return sdkMessages, nil
}

// convertFromSDKMessage converts SDK message to A2A format
func (c *OpenAICompatibleLLMClient) convertFromSDKMessage(msg sdk.Message) *adk.Message {
	return &adk.Message{
		Kind:      "message",
		MessageID: fmt.Sprintf("llm-response-%d", time.Now().UnixNano()),
		Role:      string(msg.Role),
		Parts: []adk.Part{
			map[string]interface{}{
				"kind": "text",
				"text": msg.Content,
			},
		},
	}
}

// parseProvider converts a provider string to SDK Provider type
func parseProvider(provider string) (sdk.Provider, error) {
	p := sdk.Provider(strings.ToLower(provider))
	if p == "" {
		return "", fmt.Errorf("invalid provider: %s", provider)
	}
	return p, nil
}

// parseModelName removes provider prefix from model name if present
func parseModelName(model, provider string) string {
	if strings.Contains(model, "/") {
		parts := strings.SplitN(model, "/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return model
}
