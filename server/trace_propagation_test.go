package server

import (
	"context"
	"encoding/json"
	"testing"

	types "github.com/inference-gateway/adk/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	otel "go.opentelemetry.io/otel"
	propagation "go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	trace "go.opentelemetry.io/otel/trace"
	zap "go.uber.org/zap"
)

// TestTraceContextPropagation verifies that the trace context of the request
// that enqueued a task survives the queue (including a JSON round-trip, as in
// the Redis storage) and is restored for background processing.
func TestTraceContextPropagation(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	tracerProvider := sdktrace.NewTracerProvider()
	ctx, span := tracerProvider.Tracer("test").Start(context.Background(), "a2a.request")
	defer span.End()
	wantTraceID := span.SpanContext().TraceID()

	t.Run("inject and extract round-trip", func(t *testing.T) {
		carrier := injectTraceContext(ctx)
		require.NotEmpty(t, carrier)
		assert.Contains(t, carrier, "traceparent")

		restored := extractTraceContext(context.Background(), carrier)
		assert.Equal(t, wantTraceID, trace.SpanContextFromContext(restored).TraceID())
	})

	t.Run("empty context yields nil carrier and no-op extract", func(t *testing.T) {
		assert.Nil(t, injectTraceContext(context.Background()))
		base := context.Background()
		assert.Equal(t, base, extractTraceContext(base, nil))
	})

	t.Run("survives storage enqueue/dequeue and JSON round-trip", func(t *testing.T) {
		storage := NewInMemoryStorage(zap.NewNop(), 10)
		task := &types.Task{
			ID:        "task-trace",
			ContextID: "context-trace",
			Status:    types.TaskStatus{State: types.TaskStateSubmitted},
		}
		require.NoError(t, storage.EnqueueTask(ctx, task, "req-trace"))

		queued, err := storage.DequeueTask(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, queued.TraceContext)

		data, err := json.Marshal(queued)
		require.NoError(t, err)
		var roundTripped QueuedTask
		require.NoError(t, json.Unmarshal(data, &roundTripped))

		restored := extractTraceContext(context.Background(), roundTripped.TraceContext)
		assert.Equal(t, wantTraceID, trace.SpanContextFromContext(restored).TraceID())
	})
}
