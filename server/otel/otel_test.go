package otel_test

import (
	"context"
	"testing"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	require "github.com/stretchr/testify/require"
	zap "go.uber.org/zap"

	config "github.com/inference-gateway/adk/server/config"
	adkotel "github.com/inference-gateway/adk/server/otel"
)

// TestNewOpenTelemetry_Exporters verifies that the OTLP and none exporter paths
// build a working telemetry instance. The prometheus path is intentionally not
// exercised here because prometheus.New registers with the global default
// registry, which would conflict across repeated runs.
func TestNewOpenTelemetry_Exporters(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
	}{
		{
			name: "otlp http metrics and traces",
			envVars: map[string]string{
				"OTEL_METRICS_EXPORTER":       "otlp",
				"OTEL_TRACES_EXPORTER":        "otlp",
				"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
			},
		},
		{
			name: "otlp grpc metrics and traces",
			envVars: map[string]string{
				"OTEL_METRICS_EXPORTER":       "otlp",
				"OTEL_TRACES_EXPORTER":        "otlp",
				"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
			},
		},
		{
			name: "metrics none, traces none",
			envVars: map[string]string{
				"OTEL_METRICS_EXPORTER": "none",
				"OTEL_TRACES_EXPORTER":  "none",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.LoadWithLookuper(context.Background(), nil, envconfig.MapLookuper(tt.envVars))
			require.NoError(t, err)
			cfg.AgentName = "test-agent"
			cfg.AgentVersion = "0.0.1"

			tel, err := adkotel.NewOpenTelemetry(cfg, zap.NewNop())
			require.NoError(t, err)
			require.NotNil(t, tel)
			require.NotNil(t, tel.TracerProvider())

			// Recording must not panic regardless of the exporter selection,
			// including when metrics are dropped (none).
			ctx := context.Background()
			attrs := adkotel.TelemetryAttributes{Provider: "test", Model: "test"}
			require.NotPanics(t, func() {
				tel.RecordRequestCount(ctx, attrs, "POST")
			})

			// Shut down with an already-cancelled context so the OTLP exporters
			// return immediately instead of blocking on a flush to an absent
			// collector; the resulting error is expected and ignored.
			shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			cancel()
			_ = tel.ShutDown(shutdownCtx)
		})
	}
}
