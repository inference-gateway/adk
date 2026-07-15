# Telemetry & Observability in the ADK

This document explains the telemetry surface of the A2A Agent Development Kit (ADK): Prometheus metrics, OTLP metric/trace export, W3C trace-context propagation, and how library consumers can reuse a pre-configured OpenTelemetry instance.

## Table of Contents

- [Overview](#overview)
- [Enabling Telemetry](#enabling-telemetry)
- [Configuration](#configuration)
  - [Standard OTEL Variables](#standard-otel-variables)
  - [Deprecated TELEMETRY Aliases](#deprecated-telemetry-aliases)
  - [Attribute Keys](#attribute-keys)
  - [Precedence](#precedence)
- [Metrics](#metrics)
  - [Prometheus Pull](#prometheus-pull)
  - [OTLP Push](#otlp-push)
- [Tracing](#tracing)
  - [OTLP Trace Export](#otlp-trace-export)
  - [Trace Context Propagation](#trace-context-propagation)
  - [Baggage Attributes](#baggage-attributes)
  - [Trace and Span IDs](#trace-and-span-ids)
  - [Span Status](#span-status)
- [Reusing Telemetry in a Library](#reusing-telemetry-in-a-library)
- [Example: Local OTLP Collector](#example-local-otlp-collector)

## Overview

The telemetry layer is built on [OpenTelemetry](https://opentelemetry.io/). When enabled it provides:

- **Metrics** exported either by a **Prometheus pull** endpoint (default port `9090`) or **pushed via OTLP** to a collector.
- **OTLP trace export** over HTTP or gRPC to a collector (on by default; opt out with `OTEL_TRACES_EXPORTER=none`).
- **W3C Trace Context propagation** - incoming `traceparent`/`baggage` headers are extracted and a request-scoped span is created for every `/a2a` request.

Exporters are selected with the standard OpenTelemetry `OTEL_*` environment variables. The original `TELEMETRY_*` variables remain supported as deprecated aliases so existing deployments keep working.

The tracing service name is derived from the agent card `name` (the build-time agent identity), so traces are attributed to the agent without an extra variable.

## Enabling Telemetry

Telemetry is off by default. `TELEMETRY_ENABLE=true` is the master switch that turns the telemetry subsystem on. Once enabled, the `OTEL_*` variables choose which exporters run per signal.

```bash
export TELEMETRY_ENABLE=true
export OTEL_METRICS_EXPORTER=prometheus      # prometheus | otlp | none
export OTEL_TRACES_EXPORTER=otlp             # otlp | none
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

> The variable names below are the library's own (`TELEMETRY_*`). Consumers that embed the ADK config under an `A2A_` prefix will see them as `A2A_TELEMETRY_*`. The `OTEL_*` variables follow the OpenTelemetry specification and are read without a prefix.

## Configuration

### Standard OTEL Variables

| Variable                        | Default                 | Description                                                    |
| ------------------------------- | ----------------------- | -------------------------------------------------------------- |
| `OTEL_METRICS_EXPORTER`         | `prometheus`            | Metrics exporter: `prometheus`, `otlp`, or `none`              |
| `OTEL_TRACES_EXPORTER`          | `otlp`                  | Traces exporter: `otlp` or `none`                              |
| `OTEL_EXPORTER_OTLP_ENDPOINT`   | `http://localhost:4318` | OTLP endpoint base URL shared by traces and metrics            |
| `OTEL_EXPORTER_OTLP_PROTOCOL`   | `http/protobuf`         | OTLP transport: `http/protobuf` or `grpc`                      |
| `OTEL_EXPORTER_PROMETHEUS_HOST` | -                       | Host for the Prometheus pull endpoint (empty = all interfaces) |
| `OTEL_EXPORTER_PROMETHEUS_PORT` | `9090`                  | Port for the Prometheus pull endpoint                          |

The OTLP path (`/v1/traces`, `/v1/metrics`) is appended to `OTEL_EXPORTER_OTLP_ENDPOINT` automatically. When using `grpc`, point the endpoint at the collector's gRPC receiver (typically port `4317`).

### Deprecated TELEMETRY Aliases

These still work but are superseded by the `OTEL_*` variables above. Prefer the standard names in new deployments.

| Variable                   | Default                 | Superseded by                   |
| -------------------------- | ----------------------- | ------------------------------- |
| `TELEMETRY_METRICS_PORT`   | `9090`                  | `OTEL_EXPORTER_PROMETHEUS_PORT` |
| `TELEMETRY_METRICS_HOST`   | -                       | `OTEL_EXPORTER_PROMETHEUS_HOST` |
| `TELEMETRY_TRACE_ENDPOINT` | `http://localhost:4318` | `OTEL_EXPORTER_OTLP_ENDPOINT`   |
| `TELEMETRY_TRACE_HEADERS`  | -                       | `OTEL_EXPORTER_OTLP_HEADERS`    |

`TELEMETRY_TRACE_HEADERS` accepts a comma-separated list of `key:value` pairs, for example `Authorization:Bearer <token>,X-Scope-OrgID:tenant-a`. This is useful when exporting to an authenticated collector such as Grafana Cloud or a self-hosted gateway. When set, these headers are applied to the OTLP trace exporter.

`TELEMETRY_LOG_*` remains reserved: `LogConfig` is parsed but not yet wired to an exporter, because the OpenTelemetry Go log SDK is still experimental. The existing zap logger remains the log path. These variables are accepted for forward compatibility and currently have no effect.

### Attribute Keys

The session-id and tool-call-id keys used for both the **baggage member read** and the **span attribute written** are configurable. Defaults follow the OTel semantic conventions.

| Variable                          | Default               | Description                                                |
| --------------------------------- | --------------------- | ---------------------------------------------------------- |
| `TELEMETRY_ATTR_SESSION_ID_KEY`   | `session.id`          | Baggage member and span attribute key for the session id   |
| `TELEMETRY_ATTR_TOOL_CALL_ID_KEY` | `gen_ai.tool.call.id` | Baggage member and span attribute key for the tool call id |

Baggage member names must stay in sync with the producer side (the `infer` orchestrator). If you override these keys, override them identically on the producer.

### Precedence

For each setting, the standard `OTEL_*` variable wins when set; otherwise the deprecated `TELEMETRY_*` alias (and its default) applies:

- **Metrics exporter** - `OTEL_METRICS_EXPORTER`, else `prometheus`.
- **Traces exporter** - `OTEL_TRACES_EXPORTER`, else `otlp` when telemetry is enabled (`TELEMETRY_ENABLE=true`), else `none`.
- **OTLP endpoint** - `OTEL_EXPORTER_OTLP_ENDPOINT`, else `TELEMETRY_TRACE_ENDPOINT`.
- **OTLP protocol** - `OTEL_EXPORTER_OTLP_PROTOCOL`, else `http/protobuf`.
- **Prometheus host/port** - `OTEL_EXPORTER_PROMETHEUS_HOST`/`PORT`, else `TELEMETRY_METRICS_HOST`/`PORT`.

## Metrics

Recorded instruments include token usage, request counts, response status, request duration, task lifecycle, and tool-call failures (all prefixed `a2a.`).

### Prometheus Pull

When `OTEL_METRICS_EXPORTER=prometheus` (the default), the server starts a separate HTTP server exposing Prometheus metrics at `http://<host>:<port>/metrics`, where host/port come from `OTEL_EXPORTER_PROMETHEUS_HOST`/`PORT` (or the deprecated `TELEMETRY_METRICS_HOST`/`PORT`).

### OTLP Push

When `OTEL_METRICS_EXPORTER=otlp`, metrics are pushed to the collector at `OTEL_EXPORTER_OTLP_ENDPOINT` over the configured protocol using a periodic reader. The Prometheus pull server is not started in this mode.

Set `OTEL_METRICS_EXPORTER=none` to disable metrics export entirely while still allowing traces.

```bash
# Push metrics over OTLP instead of exposing a Prometheus endpoint
export TELEMETRY_ENABLE=true
export OTEL_METRICS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

## Tracing

### OTLP Trace Export

When telemetry is enabled (`TELEMETRY_ENABLE=true`) traces default to OTLP; setting `OTEL_TRACES_EXPORTER=none` opts traces out. With OTLP selected, a batching OTLP trace exporter is configured against `OTEL_EXPORTER_OTLP_ENDPOINT` over the selected protocol (`http/protobuf` or `grpc`) and registered as the global `TracerProvider`. Spans are exported in the background and flushed on shutdown.

### Trace Context Propagation

A composite [W3C Trace Context](https://www.w3.org/TR/trace-context/) + [Baggage](https://www.w3.org/TR/baggage/) propagator is installed globally. For every request to `/a2a`, the middleware extracts `traceparent` and `baggage` headers from the incoming request and starts a server-kind span named `a2a.request`. When the caller supplies a `traceparent`, the ADK span becomes a child of the caller's span, giving you an end-to-end distributed trace across agents.

The span carries HTTP semantic-convention attributes (`http.request.method`, `url.full`, `http.route`, `http.response.status_code`).

### Baggage Attributes

Two baggage items are promoted to span attributes when present, so they are queryable in your tracing backend. The keys are configurable (see [Attribute Keys](#attribute-keys)) and default to the OTel semantic conventions:

| Baggage key (default) | Span attribute (default) |
| --------------------- | ------------------------ |
| `session.id`          | `session.id`             |
| `gen_ai.tool.call.id` | `gen_ai.tool.call.id`    |

A caller propagates them by setting the standard `baggage` header:

```http
baggage: session.id=session-123,gen_ai.tool.call.id=tool-456
```

### Trace and Span IDs

Trace id and span id are intrinsic span fields, not configurable attributes. They are generated by the tracing SDK and propagated across services via the W3C `traceparent` header, so no additional configuration is needed to correlate spans in your backend.

### Span Status

The request span status is set to `Error` only for `5xx` responses (a server fault), following HTTP-server semantic conventions; `4xx` client errors leave the span status unset. On a `5xx` the `error.type` attribute is set to the status code so backends such as Jaeger or Tempo flag the span as failed.

## Reusing Telemetry in a Library

If your application already configures OpenTelemetry, inject your instance with `WithTelemetry()` rather than letting the ADK build one from the environment. The builder wires the provided instance into the request middleware, request spans, and the `/metrics` endpoint - regardless of the `TELEMETRY_ENABLE` flag.

```go
package main

import (
	server "github.com/inference-gateway/adk/server"
	otel "github.com/inference-gateway/adk/server/otel"
)

func build(myOtel otel.OpenTelemetry) (server.A2AServer, error) {
	return server.NewA2AServerBuilder(cfg, logger).
		WithTelemetry(myOtel).
		WithAgentCard(card).
		WithDefaultTaskHandlers().
		Build()
}
```

`myOtel` must satisfy the `server/otel.OpenTelemetry` interface. The interface exposes `TracerProvider()` so the middleware can create spans from your provider, alongside the metric-recording methods.

## Example: Local OTLP Collector

Run an OpenTelemetry Collector locally and point the agent at it:

```bash
export TELEMETRY_ENABLE=true
export OTEL_TRACES_EXPORTER=otlp
export OTEL_METRICS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

The default endpoint (`http://localhost:4318`) matches the collector's OTLP/HTTP receiver. Traces and metrics then appear in whatever backend the collector exports to (Jaeger, Tempo, Prometheus, Honeycomb, etc.), correlated by session and tool-call baggage.
