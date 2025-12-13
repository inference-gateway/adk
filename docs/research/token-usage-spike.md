# SPIKE: Token Usage in A2A Protocol

**Issue**: #124
**Date**: December 13, 2024
**Author**: Claude (AI Research Assistant)

## Executive Summary

This spike investigates how to expose token usage and related execution metrics to A2A clients. Currently, the ADK tracks token usage internally via OpenTelemetry metrics, but this information is not exposed to clients through the A2A protocol responses.

## Background

The A2A protocol is designed to be LLM-agnostic - servers are not required to use LLMs. However, when LLMs are used, token usage is valuable information for:

- **Cost tracking**: Understanding the cost of agent interactions
- **Performance monitoring**: Analyzing efficiency and optimization opportunities
- **Debugging**: Understanding why certain responses consumed more resources
- **Client-side analytics**: Enabling clients to track their usage patterns

Additional metrics of interest mentioned in the issue:
- Tool call failures count
- Total tool calls count
- Total iterations (LLM interaction rounds)
- Total messages in conversation

## Current State Analysis

### A2A Protocol Schema (v1.0)

The A2A protocol defines the following key structures:

#### Task Structure
```yaml
Task:
  properties:
    id: string              # Unique task identifier
    context_id: string      # Conversation context identifier
    status: TaskStatus      # Current task status
    artifacts: []Artifact   # Output artifacts
    history: []Message      # Interaction history
    metadata: object        # Custom metadata (key-value pairs)
```

#### Task Metadata Field

The **`metadata`** field is defined as:
- Type: `map[string]any` (arbitrary key-value object)
- Description: "A key/value object to store custom metadata about a task"
- Location: Available on:
  - `Task` object
  - `TaskArtifactUpdateEvent` object (streaming)
  - `TaskUpdateEvent` object (streaming)
  - `Message` object

This field is **explicitly designed** for custom, implementation-specific data.

#### Extensions Mechanism

The A2A protocol also defines an **extensions** mechanism:

```yaml
AgentExtension:
  properties:
    uri: string              # Unique URI identifying the extension
    description: string      # Human-readable description
    required: boolean        # Whether clients must understand this extension
    params: object          # Extension-specific configuration
```

Extensions are declared in the `AgentCapabilities` and allow agents to signal support for protocol extensions that clients may or may not understand.

### Current ADK Implementation

#### Token Usage Tracking

The ADK currently tracks token usage through OpenTelemetry:

**Location**: `server/otel/otel.go`

```go
type OpenTelemetry interface {
    RecordTokenUsage(ctx context.Context, attrs TelemetryAttributes, usage sdk.CompletionUsage)
    // ... other metrics
}

func (o *OpenTelemetryImpl) RecordTokenUsage(ctx context.Context, attrs TelemetryAttributes, usage sdk.CompletionUsage) {
    o.promptTokensCounter.Add(ctx, usage.PromptTokens, ...)
    o.completionTokensCounter.Add(ctx, usage.CompletionTokens, ...)
    o.totalTokensCounter.Add(ctx, usage.TotalTokens, ...)
}
```

**Token Usage Structure** (from inference-gateway/sdk):
```go
type CompletionUsage struct {
    PromptTokens     int64
    CompletionTokens int64
    TotalTokens      int64
}
```

#### Current Data Flow

1. **Agent execution**: LLM calls are made through the agent
2. **Telemetry recording**: Token usage is recorded to OpenTelemetry metrics
3. **Task completion**: Task is returned with status and results
4. **Gap**: Token usage is NOT propagated to the Task metadata

The token usage data exists but is only exposed through:
- Prometheus metrics (for monitoring/alerting)
- Internal telemetry system

It is **not** exposed to A2A clients in task responses.

### Metadata Usage in Codebase

**Current usage**: Minimal
- The `metadata` field exists in the protocol types
- No examples currently populate task metadata
- The field is preserved during task processing but not actively used

## Proposed Solutions

### Option 1: Task Metadata (Recommended)

**Approach**: Use the existing `metadata` field on Task responses to include execution metrics.

**Pros**:
- ✅ Works within existing A2A protocol (no schema changes needed)
- ✅ Simple to implement
- ✅ Backward compatible (clients can ignore unknown metadata)
- ✅ Flexible for future additions
- ✅ Follows the intended purpose of the metadata field

**Cons**:
- ⚠️ Not standardized - each agent might use different field names
- ⚠️ Clients need to know which metadata fields to expect
- ⚠️ No formal specification

**Implementation Example**:

```go
task.Metadata = map[string]any{
    "usage": map[string]any{
        "prompt_tokens":     1234,
        "completion_tokens": 567,
        "total_tokens":      1801,
    },
    "execution": map[string]any{
        "total_iterations": 3,
        "total_messages":   7,
        "tool_calls": map[string]any{
            "total":   5,
            "failed":  1,
        },
    },
    "timing": map[string]any{
        "processing_time_ms": 2345,
    },
}
```

### Option 2: Protocol Extension

**Approach**: Define a formal A2A protocol extension for usage metrics.

**Extension Definition**:
```yaml
Extension URI: "https://inference-gateway.com/extensions/usage-metrics/v1"
Description: "Provides token usage and execution metrics in task metadata"
Required: false
```

**Metadata Structure** (when extension is supported):
```json
{
  "x-usage-metrics": {
    "version": "1.0",
    "tokens": {
      "prompt": 1234,
      "completion": 567,
      "total": 1801
    },
    "execution": {
      "iterations": 3,
      "messages": 7,
      "tool_calls": {
        "total": 5,
        "successful": 4,
        "failed": 1
      }
    },
    "timing": {
      "total_ms": 2345,
      "llm_ms": 2100,
      "tool_execution_ms": 245
    }
  }
}
```

**Pros**:
- ✅ Formalized and discoverable (via agent card)
- ✅ Clients can check if agent supports the extension
- ✅ Versioned for evolution
- ✅ Clear specification
- ✅ Still backward compatible

**Cons**:
- ⚠️ More complex to implement
- ⚠️ Requires updating agent card
- ⚠️ Need to define extension specification
- ⚠️ Still uses metadata field (not a separate response field)

### Option 3: A2A Protocol Schema Change (Not Recommended)

**Approach**: Propose changes to the official A2A protocol schema to add a dedicated `usage` field to Task.

**Pros**:
- ✅ First-class protocol support
- ✅ Type-safe
- ✅ Standardized across all A2A implementations

**Cons**:
- ❌ Requires A2A protocol governance approval
- ❌ Long implementation timeline
- ❌ Breaking change or requires protocol version bump
- ❌ Would need community consensus
- ❌ Not all agents use LLMs (makes field optional anyway)

## Recommendations

### Immediate Action (Phase 1)

**Use Task Metadata approach (Option 1)**:

1. **Collect metrics during task execution**:
   - Token usage from LLM responses
   - Iteration count from agent loops
   - Tool call statistics
   - Message counts

2. **Populate task metadata before returning**:
   ```go
   task.Metadata = map[string]any{
       "usage": map[string]any{
           "prompt_tokens":     usage.PromptTokens,
           "completion_tokens": usage.CompletionTokens,
           "total_tokens":      usage.TotalTokens,
       },
       "execution_stats": map[string]any{
           "iterations":    iterationCount,
           "messages":      len(task.History),
           "tool_calls":    toolCallCount,
           "failed_tools":  failedToolCount,
       },
   }
   ```

3. **Document the metadata structure** in ADK documentation

4. **Update examples** to demonstrate usage metadata

### Future Enhancement (Phase 2)

**Define a formal extension (Option 2)**:

1. Create an extension specification document
2. Register the extension URI
3. Update agent card to declare extension support
4. Standardize the metadata structure
5. Version the extension for future evolution

### Long-term Consideration (Phase 3)

**Contribute to A2A protocol** (Option 3):

1. Gather real-world usage data from Phase 1 & 2
2. Propose standardization to A2A protocol maintainers
3. Work with community on schema changes
4. Implement when protocol is updated

## Implementation Details

### Data Collection Points

**Token Usage**:
- Already collected: `server/otel/otel.go` - `RecordTokenUsage()`
- Need to: Aggregate per-task and store for inclusion in metadata

**Tool Call Statistics**:
- Already tracked: `RecordToolCallFailure()` for failures
- Need to: Track total count and success/failure per task

**Iteration Count**:
- Need to: Track LLM interaction rounds in agent execution loop

**Message Count**:
- Already available: `len(task.History)`

### Code Changes Required

1. **Create usage tracking struct** in agent execution context
2. **Aggregate metrics** during task processing
3. **Populate metadata** before task return
4. **Update streaming responses** to include metadata in final events
5. **Add configuration** to enable/disable usage reporting
6. **Write tests** for metadata population

### Configuration

Add environment variables:
```bash
ENABLE_USAGE_METADATA=true    # Enable usage metadata in responses
USAGE_METADATA_VERBOSE=false  # Include detailed timing breakdowns
```

## Open Questions

1. **Privacy considerations**: Should token usage be considered sensitive information?
2. **Performance impact**: What is the overhead of tracking and aggregating these metrics?
3. **Streaming updates**: Should partial usage data be sent during streaming, or only in final response?
4. **Caching scenarios**: How should cached responses report token usage (0 tokens vs. showing original usage)?
5. **Multi-agent scenarios**: If an agent delegates to other agents, how should usage be aggregated?

## Acceptance Criteria Review

- [x] **A2A protocol documentation examined**: Schema reviewed, metadata and extensions mechanisms understood
- [x] **Clear process defined**: Three-phase approach (metadata → extension → protocol change)
- [ ] **Follow-up ticket created**: Will create feature request issue

## Next Steps

1. **Create feature request issue** with detailed implementation plan
2. **Prototype metadata population** in a branch
3. **Gather feedback** from ADK users on metadata structure
4. **Document the approach** in ADK usage guide
5. **Consider extension formalization** after initial implementation proves useful

## References

- A2A Protocol Schema: `schema.yaml` (downloaded from inference-gateway/schemas)
- ADK Telemetry: `server/otel/otel.go`
- Task Types: `types/generated_types.go`
- OpenTelemetry SDK: github.com/inference-gateway/sdk v1.14.0
