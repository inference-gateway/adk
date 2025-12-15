package server

import (
	"testing"

	sdk "github.com/inference-gateway/sdk"
	"github.com/stretchr/testify/assert"
)

func TestNewUsageTracker(t *testing.T) {
	tracker := NewUsageTracker()
	assert.NotNil(t, tracker)
	assert.False(t, tracker.HasUsage())
}

func TestUsageTracker_AddTokenUsage(t *testing.T) {
	tracker := NewUsageTracker()

	usage := sdk.CompletionUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	tracker.AddTokenUsage(usage)

	metadata := tracker.GetMetadata()
	assert.NotNil(t, metadata)
	assert.Contains(t, metadata, "usage")

	usageMap := metadata["usage"].(map[string]any)
	assert.Equal(t, int64(100), usageMap["prompt_tokens"])
	assert.Equal(t, int64(50), usageMap["completion_tokens"])
	assert.Equal(t, int64(150), usageMap["total_tokens"])
}

func TestUsageTracker_AddTokenUsage_Multiple(t *testing.T) {
	tracker := NewUsageTracker()

	// Add first usage
	tracker.AddTokenUsage(sdk.CompletionUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	})

	// Add second usage
	tracker.AddTokenUsage(sdk.CompletionUsage{
		PromptTokens:     200,
		CompletionTokens: 75,
		TotalTokens:      275,
	})

	metadata := tracker.GetMetadata()
	usageMap := metadata["usage"].(map[string]any)

	// Should be accumulated
	assert.Equal(t, int64(300), usageMap["prompt_tokens"])
	assert.Equal(t, int64(125), usageMap["completion_tokens"])
	assert.Equal(t, int64(425), usageMap["total_tokens"])
}

func TestUsageTracker_IncrementIteration(t *testing.T) {
	tracker := NewUsageTracker()

	tracker.IncrementIteration()
	tracker.IncrementIteration()
	tracker.IncrementIteration()

	metadata := tracker.GetMetadata()
	execStats := metadata["execution_stats"].(map[string]any)
	assert.Equal(t, 3, execStats["iterations"])
}

func TestUsageTracker_AddMessages(t *testing.T) {
	tracker := NewUsageTracker()

	tracker.AddMessages(5)
	tracker.AddMessages(3)

	metadata := tracker.GetMetadata()
	execStats := metadata["execution_stats"].(map[string]any)
	assert.Equal(t, 8, execStats["messages"])
}

func TestUsageTracker_IncrementToolCalls(t *testing.T) {
	tracker := NewUsageTracker()

	tracker.IncrementToolCalls()
	tracker.IncrementToolCalls()

	metadata := tracker.GetMetadata()
	execStats := metadata["execution_stats"].(map[string]any)
	assert.Equal(t, 2, execStats["tool_calls"])
}

func TestUsageTracker_IncrementFailedTools(t *testing.T) {
	tracker := NewUsageTracker()

	tracker.IncrementFailedTools()

	metadata := tracker.GetMetadata()
	execStats := metadata["execution_stats"].(map[string]any)
	assert.Equal(t, 1, execStats["failed_tools"])
}

func TestUsageTracker_GetMetadata_Complete(t *testing.T) {
	tracker := NewUsageTracker()

	// Add all types of metrics
	tracker.AddTokenUsage(sdk.CompletionUsage{
		PromptTokens:     156,
		CompletionTokens: 89,
		TotalTokens:      245,
	})
	tracker.IncrementIteration()
	tracker.IncrementIteration()
	tracker.AddMessages(4)
	tracker.IncrementToolCalls()
	tracker.IncrementFailedTools()

	metadata := tracker.GetMetadata()

	// Check usage
	assert.Contains(t, metadata, "usage")
	usageMap := metadata["usage"].(map[string]any)
	assert.Equal(t, int64(156), usageMap["prompt_tokens"])
	assert.Equal(t, int64(89), usageMap["completion_tokens"])
	assert.Equal(t, int64(245), usageMap["total_tokens"])

	// Check execution stats
	assert.Contains(t, metadata, "execution_stats")
	execStats := metadata["execution_stats"].(map[string]any)
	assert.Equal(t, 2, execStats["iterations"])
	assert.Equal(t, 4, execStats["messages"])
	assert.Equal(t, 1, execStats["tool_calls"])
	assert.Equal(t, 1, execStats["failed_tools"])
}

func TestUsageTracker_GetMetadata_NoLLMCalls(t *testing.T) {
	tracker := NewUsageTracker()

	// Only add execution stats, no token usage
	tracker.IncrementIteration()
	tracker.AddMessages(2)

	metadata := tracker.GetMetadata()

	// Should not include usage section if no LLM calls
	assert.NotContains(t, metadata, "usage")

	// Should include execution stats
	assert.Contains(t, metadata, "execution_stats")
	execStats := metadata["execution_stats"].(map[string]any)
	assert.Equal(t, 1, execStats["iterations"])
	assert.Equal(t, 2, execStats["messages"])
}

func TestUsageTracker_HasUsage(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*UsageTracker)
		expected bool
	}{
		{
			name:     "empty tracker",
			setup:    func(ut *UsageTracker) {},
			expected: false,
		},
		{
			name: "with token usage",
			setup: func(ut *UsageTracker) {
				ut.AddTokenUsage(sdk.CompletionUsage{PromptTokens: 100})
			},
			expected: true,
		},
		{
			name: "with iteration",
			setup: func(ut *UsageTracker) {
				ut.IncrementIteration()
			},
			expected: true,
		},
		{
			name: "with messages",
			setup: func(ut *UsageTracker) {
				ut.AddMessages(1)
			},
			expected: true,
		},
		{
			name: "with tool calls",
			setup: func(ut *UsageTracker) {
				ut.IncrementToolCalls()
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewUsageTracker()
			tt.setup(tracker)
			assert.Equal(t, tt.expected, tracker.HasUsage())
		})
	}
}

func TestUsageTracker_ThreadSafety(t *testing.T) {
	tracker := NewUsageTracker()
	done := make(chan bool)

	// Simulate concurrent updates
	for i := 0; i < 10; i++ {
		go func() {
			tracker.AddTokenUsage(sdk.CompletionUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			})
			tracker.IncrementIteration()
			tracker.AddMessages(1)
			tracker.IncrementToolCalls()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	metadata := tracker.GetMetadata()
	usageMap := metadata["usage"].(map[string]any)
	execStats := metadata["execution_stats"].(map[string]any)

	// Should have accumulated all values correctly
	assert.Equal(t, int64(100), usageMap["prompt_tokens"])
	assert.Equal(t, int64(50), usageMap["completion_tokens"])
	assert.Equal(t, int64(150), usageMap["total_tokens"])
	assert.Equal(t, 10, execStats["iterations"])
	assert.Equal(t, 10, execStats["messages"])
	assert.Equal(t, 10, execStats["tool_calls"])
}
