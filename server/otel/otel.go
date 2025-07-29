package otel

import (
	"context"
	"fmt"

	config "github.com/inference-gateway/adk/server/config"
	sdk "github.com/inference-gateway/sdk"
	otel "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	prometheus "go.opentelemetry.io/otel/exporters/prometheus"
	metric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	resource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
	zap "go.uber.org/zap"
)

// OpenTelemetry defines the operations for telemetry
type OpenTelemetry interface {
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
	logger        *zap.Logger
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

// NewOpenTelemetry creates a new OpenTelemetry implementation with proper dependency injection
func NewOpenTelemetry(cfg *config.Config, logger *zap.Logger) (OpenTelemetry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	o := &OpenTelemetryImpl{
		logger: logger,
	}

	if err := o.initialize(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize opentelemetry: %w", err)
	}

	return o, nil
}

func (o *OpenTelemetryImpl) initialize(cfg *config.Config) error {
	o.logger.Info("initializing opentelemetry",
		zap.String("agent_name", cfg.AgentName),
		zap.String("version", cfg.AgentVersion))

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

	o.logger.Debug("opentelemetry resource created",
		zap.String("agent_name", cfg.AgentName),
		zap.String("version", cfg.AgentVersion))

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

	if err := o.initializeMetrics(cfg.AgentName); err != nil {
		o.logger.Error("failed to initialize metrics", zap.Error(err))
		return err
	}

	o.logger.Info("opentelemetry initialized successfully")
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

func (o *OpenTelemetryImpl) RecordTaskQueued(ctx context.Context, attrs TelemetryAttributes) {
	attributes := []attribute.KeyValue{
		attribute.String("task_id", attrs.TaskID),
	}
	if attrs.Provider != "" {
		attributes = append(attributes, attribute.String("provider", attrs.Provider))
	}
	if attrs.Model != "" {
		attributes = append(attributes, attribute.String("model", attrs.Model))
	}

	o.queueTimeHistogram.Record(ctx, 1, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordTaskCompleted(ctx context.Context, attrs TelemetryAttributes, success bool) {
	attributes := []attribute.KeyValue{
		attribute.String("task_id", attrs.TaskID),
		attribute.Bool("success", success),
	}
	if attrs.Provider != "" {
		attributes = append(attributes, attribute.String("provider", attrs.Provider))
	}
	if attrs.Model != "" {
		attributes = append(attributes, attribute.String("model", attrs.Model))
	}

	o.requestCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordTaskFailure(ctx context.Context, attrs TelemetryAttributes, toolName string, errorMessage string) {
	attributes := []attribute.KeyValue{
		attribute.String("task_id", attrs.TaskID),
		attribute.String("tool_name", toolName),
		attribute.String("error_message", errorMessage),
	}
	if attrs.Provider != "" {
		attributes = append(attributes, attribute.String("provider", attrs.Provider))
	}
	if attrs.Model != "" {
		attributes = append(attributes, attribute.String("model", attrs.Model))
	}

	o.requestCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordToolCallFailure(ctx context.Context, attrs TelemetryAttributes, toolName string, errorMessage string) {
	attributes := []attribute.KeyValue{
		attribute.String("task_id", attrs.TaskID),
		attribute.String("tool_name", toolName),
		attribute.String("error_message", errorMessage),
	}
	if attrs.Provider != "" {
		attributes = append(attributes, attribute.String("provider", attrs.Provider))
	}
	if attrs.Model != "" {
		attributes = append(attributes, attribute.String("model", attrs.Model))
	}

	o.toolCallFailureCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) ShutDown(ctx context.Context) error {
	return o.meterProvider.Shutdown(ctx)
}

// initializeMetrics initializes all the OpenTelemetry metrics
func (o *OpenTelemetryImpl) initializeMetrics(serviceName string) error {
	var err error

	o.promptTokensCounter, err = o.meter.Int64Counter(
		"a2a.prompt_tokens.total",
		metric.WithDescription("Total number of prompt tokens consumed by A2A requests"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create prompt tokens counter: %w", err)
	}

	o.completionTokensCounter, err = o.meter.Int64Counter(
		"a2a.completion_tokens.total",
		metric.WithDescription("Total number of completion tokens generated by A2A requests"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create completion tokens counter: %w", err)
	}

	o.totalTokensCounter, err = o.meter.Int64Counter(
		"a2a.tokens.total",
		metric.WithDescription("Total number of tokens used in A2A requests"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create total tokens counter: %w", err)
	}

	o.queueTimeHistogram, err = o.meter.Float64Histogram(
		"a2a.task.queue_time",
		metric.WithDescription("Time tasks spend in queue before processing"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return fmt.Errorf("failed to create queue time histogram: %w", err)
	}

	o.promptTimeHistogram, err = o.meter.Float64Histogram(
		"a2a.prompt_time",
		metric.WithDescription("Time taken to process prompt requests in A2A"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return fmt.Errorf("failed to create prompt time histogram: %w", err)
	}

	o.completionTimeHistogram, err = o.meter.Float64Histogram(
		"a2a.completion_time",
		metric.WithDescription("Time taken to generate completions in A2A"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return fmt.Errorf("failed to create completion time histogram: %w", err)
	}

	o.totalTimeHistogram, err = o.meter.Float64Histogram(
		"a2a.total_time",
		metric.WithDescription("Total time for complete A2A request processing"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return fmt.Errorf("failed to create total time histogram: %w", err)
	}

	o.requestCounter, err = o.meter.Int64Counter(
		"a2a.requests.total",
		metric.WithDescription("Total number of A2A requests processed"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request counter: %w", err)
	}

	o.responseStatusCounter, err = o.meter.Int64Counter(
		"a2a.response_status.total",
		metric.WithDescription("Total number of responses by status code"),
		metric.WithUnit("{response}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create response status counter: %w", err)
	}

	o.requestDurationHistogram, err = o.meter.Float64Histogram(
		"a2a.request_duration",
		metric.WithDescription("Duration of A2A request processing"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	o.toolCallFailureCounter, err = o.meter.Int64Counter(
		"a2a.tool_call_failures.total",
		metric.WithDescription("Total number of tool call failures"),
		metric.WithUnit("{failure}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create tool call failure counter: %w", err)
	}

	o.logger.Debug("all opentelemetry metrics initialized successfully")
	return nil
}
