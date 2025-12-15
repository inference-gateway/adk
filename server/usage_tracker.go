package server

import (
	"sync"

	sdk "github.com/inference-gateway/sdk"
)

// UsageTracker tracks token usage and execution statistics during agent execution
type UsageTracker struct {
	mu sync.Mutex

	// Token usage from LLM
	promptTokens     int64
	completionTokens int64
	totalTokens      int64

	// Execution statistics
	iterations  int
	messages    int
	toolCalls   int
	failedTools int
	llmCalls    int
}

// NewUsageTracker creates a new usage tracker
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{}
}

// AddTokenUsage adds token usage from an LLM response
func (ut *UsageTracker) AddTokenUsage(usage sdk.CompletionUsage) {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	ut.promptTokens += usage.PromptTokens
	ut.completionTokens += usage.CompletionTokens
	ut.totalTokens += usage.TotalTokens
	ut.llmCalls++
}

// IncrementIteration increments the iteration counter
func (ut *UsageTracker) IncrementIteration() {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.iterations++
}

// AddMessages adds to the message count
func (ut *UsageTracker) AddMessages(count int) {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.messages += count
}

// IncrementToolCalls increments the tool call counter
func (ut *UsageTracker) IncrementToolCalls() {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.toolCalls++
}

// IncrementFailedTools increments the failed tool counter
func (ut *UsageTracker) IncrementFailedTools() {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.failedTools++
}

// GetMetadata returns the collected metrics as a metadata map
func (ut *UsageTracker) GetMetadata() map[string]any {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	metadata := make(map[string]any)

	if ut.llmCalls > 0 {
		metadata["usage"] = map[string]any{
			"prompt_tokens":     ut.promptTokens,
			"completion_tokens": ut.completionTokens,
			"total_tokens":      ut.totalTokens,
		}
	}

	metadata["execution_stats"] = map[string]any{
		"iterations":   ut.iterations,
		"messages":     ut.messages,
		"tool_calls":   ut.toolCalls,
		"failed_tools": ut.failedTools,
	}

	return metadata
}

// HasUsage returns true if any metrics have been collected
func (ut *UsageTracker) HasUsage() bool {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	return ut.llmCalls > 0 || ut.iterations > 0 || ut.messages > 0 || ut.toolCalls > 0
}
