package server_test

import (
	"testing"

	"github.com/inference-gateway/a2a/adk/server"
	"github.com/inference-gateway/a2a/adk/server/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewToolsHandler(t *testing.T) {
	logger := zap.NewNop()

	handler := server.NewToolsHandler(logger)
	assert.NotNil(t, handler)

	provider1 := &mocks.FakeToolsProvider{}
	provider2 := &mocks.FakeToolsProvider{}

	handler = server.NewToolsHandler(logger, provider1, provider2)
	assert.NotNil(t, handler)
}

func TestToolsHandler_GetAllToolDefinitions(t *testing.T) {
	tests := []struct {
		name               string
		setupProviders     func() []*mocks.FakeToolsProvider
		expectedToolsCount int
	}{
		{
			name: "no providers",
			setupProviders: func() []*mocks.FakeToolsProvider {
				return []*mocks.FakeToolsProvider{}
			},
			expectedToolsCount: 0,
		},
		{
			name: "single provider with tools",
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider := &mocks.FakeToolsProvider{}
				provider.GetToolDefinitionsReturns([]server.A2ATool{
					{
						Name:        "test-tool-1",
						Description: "Test tool 1",
						Parameters: map[string]interface{}{
							"type": "object",
						},
					},
					{
						Name:        "test-tool-2",
						Description: "Test tool 2",
						Parameters: map[string]interface{}{
							"type": "string",
						},
					},
				})
				return []*mocks.FakeToolsProvider{provider}
			},
			expectedToolsCount: 2,
		},
		{
			name: "multiple providers with tools",
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider1 := &mocks.FakeToolsProvider{}
				provider1.GetToolDefinitionsReturns([]server.A2ATool{
					{
						Name:        "provider1-tool",
						Description: "Tool from provider 1",
					},
				})

				provider2 := &mocks.FakeToolsProvider{}
				provider2.GetToolDefinitionsReturns([]server.A2ATool{
					{
						Name:        "provider2-tool",
						Description: "Tool from provider 2",
					},
				})

				return []*mocks.FakeToolsProvider{provider1, provider2}
			},
			expectedToolsCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			providers := tt.setupProviders()

			interfaceProviders := make([]server.ToolsProvider, len(providers))
			for i, p := range providers {
				interfaceProviders[i] = p
			}

			handler := server.NewToolsHandler(logger, interfaceProviders...)
			tools := handler.GetAllToolDefinitions()

			assert.Len(t, tools, tt.expectedToolsCount)

			for _, provider := range providers {
				assert.Equal(t, 1, provider.GetToolDefinitionsCallCount())
			}
		})
	}
}

func TestToolsHandler_HandleToolCall(t *testing.T) {
	tests := []struct {
		name           string
		toolCall       server.A2AToolCall
		setupProviders func() []*mocks.FakeToolsProvider
		expectError    bool
		expectedResult string
	}{
		{
			name: "successful tool call",
			toolCall: server.A2AToolCall{
				Name:      "test-tool",
				Arguments: `{"param": "value"}`,
			},
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider := &mocks.FakeToolsProvider{}
				provider.IsToolSupportedReturns(true)
				provider.HandleToolCallReturns("Tool executed successfully", nil)
				return []*mocks.FakeToolsProvider{provider}
			},
			expectError:    false,
			expectedResult: "Tool executed successfully",
		},
		{
			name: "tool not supported by any provider",
			toolCall: server.A2AToolCall{
				Name:      "unknown-tool",
				Arguments: `{}`,
			},
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider := &mocks.FakeToolsProvider{}
				provider.IsToolSupportedReturns(false)
				return []*mocks.FakeToolsProvider{provider}
			},
			expectError:    true,
			expectedResult: "",
		},
		{
			name: "tool call returns error",
			toolCall: server.A2AToolCall{
				Name:      "error-tool",
				Arguments: `{"param": "value"}`,
			},
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider := &mocks.FakeToolsProvider{}
				provider.IsToolSupportedReturns(true)
				provider.HandleToolCallReturns("", assert.AnError)
				return []*mocks.FakeToolsProvider{provider}
			},
			expectError:    true,
			expectedResult: "",
		},
		{
			name: "multiple providers, second one handles tool",
			toolCall: server.A2AToolCall{
				Name:      "provider2-tool",
				Arguments: `{"data": "test"}`,
			},
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider1 := &mocks.FakeToolsProvider{}
				provider1.IsToolSupportedReturns(false)

				provider2 := &mocks.FakeToolsProvider{}
				provider2.IsToolSupportedReturns(true)
				provider2.HandleToolCallReturns("Provider 2 result", nil)

				return []*mocks.FakeToolsProvider{provider1, provider2}
			},
			expectError:    false,
			expectedResult: "Provider 2 result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			providers := tt.setupProviders()

			interfaceProviders := make([]server.ToolsProvider, len(providers))
			for i, p := range providers {
				interfaceProviders[i] = p
			}

			handler := server.NewToolsHandler(logger, interfaceProviders...)
			result, err := handler.HandleToolCall(tt.toolCall)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestToolsHandler_GetAllSupportedTools(t *testing.T) {
	tests := []struct {
		name               string
		setupProviders     func() []*mocks.FakeToolsProvider
		expectedToolsCount int
		expectedTools      []string
	}{
		{
			name: "no providers",
			setupProviders: func() []*mocks.FakeToolsProvider {
				return []*mocks.FakeToolsProvider{}
			},
			expectedToolsCount: 0,
			expectedTools:      []string{},
		},
		{
			name: "single provider with tools",
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider := &mocks.FakeToolsProvider{}
				provider.GetSupportedToolsReturns([]string{"tool1", "tool2"})
				return []*mocks.FakeToolsProvider{provider}
			},
			expectedToolsCount: 2,
			expectedTools:      []string{"tool1", "tool2"},
		},
		{
			name: "multiple providers with different tools",
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider1 := &mocks.FakeToolsProvider{}
				provider1.GetSupportedToolsReturns([]string{"tool1", "tool2"})

				provider2 := &mocks.FakeToolsProvider{}
				provider2.GetSupportedToolsReturns([]string{"tool3", "tool4"})

				return []*mocks.FakeToolsProvider{provider1, provider2}
			},
			expectedToolsCount: 4,
			expectedTools:      []string{"tool1", "tool2", "tool3", "tool4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			providers := tt.setupProviders()

			interfaceProviders := make([]server.ToolsProvider, len(providers))
			for i, p := range providers {
				interfaceProviders[i] = p
			}

			handler := server.NewToolsHandler(logger, interfaceProviders...)
			tools := handler.GetAllSupportedTools()

			assert.Len(t, tools, tt.expectedToolsCount)

			for _, expectedTool := range tt.expectedTools {
				assert.Contains(t, tools, expectedTool)
			}
		})
	}
}

func TestToolsHandler_IsToolSupported(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		setupProviders func() []*mocks.FakeToolsProvider
		expectedResult bool
	}{
		{
			name:     "tool supported by first provider",
			toolName: "supported-tool",
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider := &mocks.FakeToolsProvider{}
				provider.IsToolSupportedReturns(true)
				return []*mocks.FakeToolsProvider{provider}
			},
			expectedResult: true,
		},
		{
			name:     "tool not supported by any provider",
			toolName: "unsupported-tool",
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider := &mocks.FakeToolsProvider{}
				provider.IsToolSupportedReturns(false)
				return []*mocks.FakeToolsProvider{provider}
			},
			expectedResult: false,
		},
		{
			name:     "tool supported by second provider",
			toolName: "provider2-tool",
			setupProviders: func() []*mocks.FakeToolsProvider {
				provider1 := &mocks.FakeToolsProvider{}
				provider1.IsToolSupportedReturns(false)

				provider2 := &mocks.FakeToolsProvider{}
				provider2.IsToolSupportedReturns(true)

				return []*mocks.FakeToolsProvider{provider1, provider2}
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			providers := tt.setupProviders()

			interfaceProviders := make([]server.ToolsProvider, len(providers))
			for i, p := range providers {
				interfaceProviders[i] = p
			}

			handler := server.NewToolsHandler(logger, interfaceProviders...)
			result := handler.IsToolSupported(tt.toolName)

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
