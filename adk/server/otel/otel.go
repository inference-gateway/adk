package otel

import (
	"context"

	config "github.com/inference-gateway/a2a/adk/server/config"
	sdk "github.com/inference-gateway/sdk"
	otel "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	prometheus "go.opentelemetry.io/otel/exporters/prometheus"
	metric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	resource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	zap "go.uber.org/zap"
)

// OpenTelemetry defines the operations for telemetry
type OpenTelemetry interface {
	Init(config *config.Config, logger zap.Logger) error

	// Application level metrics
	RecordTokenUsage(ctx context.Context, attrs TelemetryAttributes, usage sdk.CompletionUsage)
	RecordRequestCount(ctx context.Context, attrs TelemetryAttributes, requestType string)
	RecordResponseStatus(ctx context.Context, attrs TelemetryAttributes, requestType, requestPath string, statusCode int)
	RecordRequestDuration(ctx context.Context, attrs TelemetryAttributes, requestType, requestPath string, durationMs float64)
	RecordTaskQueued(ctx context.Context, attrs TelemetryAttributes)
	RecordTaskCompleted(ctx context.Context, attrs TelemetryAttributes, success bool)
	RecordTaskFailure(ctx context.Context, attrs TelemetryAttributes, toolName string, errorMessage string)
	RecordToolCallFailure(ctx context.Context, attrs TelemetryAttributes, toolName string, errorMessage string)

	// Shutdown the telemetry system
	ShutDown(ctx context.Context) error
}

type OpenTelemetryImpl struct {
	logger        zap.Logger
	meterProvider *sdkmetric.MeterProvider
	meter         metric.Meter

	// Metrics
	promptTokensCounter      metric.Int64Counter
	completionTokensCounter  metric.Int64Counter
	totalTokensCounter       metric.Int64Counter
	queueTimeHistogram       metric.Float64Histogram
	promptTimeHistogram      metric.Float64Histogram
	completionTimeHistogram  metric.Float64Histogram
	totalTimeHistogram       metric.Float64Histogram
	requestCounter           metric.Int64Counter
	responseStatusCounter    metric.Int64Counter
	requestDurationHistogram metric.Float64Histogram
	toolCallFailureCounter   metric.Int64Counter
}

type TelemetryAttributes struct {
	Provider string
	Model    string
	TaskID   string
}

func (o *OpenTelemetryImpl) Init(cfg config.Config, log *zap.Logger) error {
	o.logger = *log

	o.logger.Info("initializing opentelemetry", zap.String("agent_name", cfg.AgentName), zap.String("version", cfg.AgentVersion))

	exporter, err := prometheus.New()
	if err != nil {
		o.logger.Error("failed to create prometheus exporter", zap.Error(err))
		return err
	}

	o.logger.Debug("prometheus exporter created successfully")

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.AgentName),
		semconv.ServiceVersion(cfg.AgentVersion),
	)

	o.logger.Debug("opentelemetry resource created", zap.String("agent_name", cfg.AgentName), zap.String("version", cfg.AgentVersion))

	histogramBoundaries := []float64{1, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000}

	latencyView := sdkmetric.NewView(
		sdkmetric.Instrument{
			Kind: sdkmetric.InstrumentKindHistogram,
		},
		sdkmetric.Stream{
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: histogramBoundaries,
			},
		},
	)

	o.logger.Debug("histogram boundaries configured", zap.Any("boundaries", histogramBoundaries))

	o.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
		sdkmetric.WithView(latencyView),
	)
	otel.SetMeterProvider(o.meterProvider)

	o.logger.Debug("meter provider created and set globally")

	o.meter = o.meterProvider.Meter(cfg.AgentName)

	o.logger.Debug("meter created", zap.String("name", cfg.AgentName))

	o.logger.Debug("initializing opentelemetry metrics")

	// TODO - Initialize metrics

	return nil
}

func (o *OpenTelemetryImpl) RecordTokenUsage(ctx context.Context, attrs TelemetryAttributes, usage sdk.CompletionUsage) {
	attributes := []attribute.KeyValue{
		attribute.String("provider", attrs.Provider),
		attribute.String("model", attrs.Model),
	}

	o.promptTokensCounter.Add(ctx, usage.PromptTokens, metric.WithAttributes(attributes...))
	o.completionTokensCounter.Add(ctx, usage.CompletionTokens, metric.WithAttributes(attributes...))
	o.totalTokensCounter.Add(ctx, usage.TotalTokens, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordRequestCount(ctx context.Context, attrs TelemetryAttributes, requestType string) {
	attributes := []attribute.KeyValue{
		attribute.String("provider", attrs.Provider),
		attribute.String("model", attrs.Model),
		attribute.String("request_type", requestType),
	}

	o.requestCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordResponseStatus(ctx context.Context, attrs TelemetryAttributes, requestType, requestPath string, statusCode int) {
	attributes := []attribute.KeyValue{
		attribute.String("provider", attrs.Provider),
		attribute.String("model", attrs.Model),
		attribute.String("request_method", requestType),
		attribute.String("request_path", requestPath),
		attribute.Int("status_code", statusCode),
	}

	o.responseStatusCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordRequestDuration(ctx context.Context, attrs TelemetryAttributes, requestType, requestPath string, durationMs float64) {
	attributes := []attribute.KeyValue{
		attribute.String("provider", attrs.Provider),
		attribute.String("model", attrs.Model),
		attribute.String("request_method", requestType),
		attribute.String("request_path", requestPath),
	}

	o.requestDurationHistogram.Record(ctx, durationMs, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) ShutDown(ctx context.Context) error {
	return o.meterProvider.Shutdown(ctx)
}
