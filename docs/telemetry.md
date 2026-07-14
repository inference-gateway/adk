# Telemetry & Observability in the ADK

This document explains the telemetry surface of the A2A Agent Development Kit (ADK): Prometheus metrics, OTLP trace export, W3C trace-context propagation, and how library consumers can reuse a pre-configured OpenTelemetry instance.

## Table of Contents

- [Overview](#overview)
- [Enabling Telemetry](#enabling-telemetry)
- [Configuration](#configuration)
- [Metrics](#metrics)
- [Tracing](#tracing)
  - [OTLP Trace Export](#otlp-trace-export)
  - [Trace Context Propagation](#trace-context-propagation)
  - [Baggage Attributes](#baggage-attributes)
  - [Span Status](#span-status)
- [Reusing Telemetry in a Library](#reusing-telemetry-in-a-library)
- [Example: Local OTLP Collector](#example-local-otlp-collector)

## Overview

The telemetry layer is built on [OpenTelemetry](https://opentelemetry.io/). When enabled it provides:

- **Prometheus metrics** served on a dedicated `/metrics` endpoint (default port `9090`).
- **OTLP trace export** over HTTP to a collector (optional, disabled by default).
- **W3C Trace Context propagation** - incoming `traceparent`/`baggage` headers are extracted and a request-scoped span is created for every `/a2a` request.

The tracing service name is derived from the agent card `name` (the build-time agent identity), so traces are attributed to the agent without an extra variable.

## Enabling Telemetry

Telemetry is off by default. Set `TELEMETRY_ENABLE=true` to turn on metrics collection and the `/metrics` endpoint. Trace export is gated separately by `TELEMETRY_TRACE_ENABLE` so you can run metrics without shipping traces.

```bash
export TELEMETRY_ENABLE=true
export TELEMETRY_TRACE_ENABLE=true
export TELEMETRY_TRACE_ENDPOINT=http://localhost:4318
```

## Configuration

| Variable                   | Default                 | Description                                            |
| -------------------------- | ----------------------- | ------------------------------------------------------ |
| `TELEMETRY_ENABLE`         | `false`                 | Enable telemetry (Prometheus metrics + tracing)        |
| `TELEMETRY_METRICS_PORT`   | `9090`                  | Port for the `/metrics` Prometheus endpoint            |
| `TELEMETRY_METRICS_HOST`   | -                       | Metrics server host (empty = all interfaces)           |
| `TELEMETRY_TRACE_ENABLE`   | `false`                 | Enable OTLP trace export                               |
| `TELEMETRY_TRACE_ENDPOINT` | `http://localhost:4318` | OTLP HTTP endpoint URL for traces                      |
| `TELEMETRY_TRACE_HEADERS`  | -                       | Custom headers for OTLP trace export (`key:value`)     |
| `TELEMETRY_LOG_ENABLE`     | `false`                 | Reserved - OTLP log export is not yet wired            |
| `TELEMETRY_LOG_ENDPOINT`   | `http://localhost:4318` | Reserved - OTLP log endpoint URL (not yet wired)       |
| `TELEMETRY_LOG_HEADERS`    | -                       | Reserved - headers for OTLP log export (not yet wired) |

`TELEMETRY_TRACE_HEADERS` accepts a comma-separated list of `key:value` pairs, for example `Authorization:Bearer <token>,X-Scope-OrgID:tenant-a`. This is useful when exporting to an authenticated collector such as Grafana Cloud or a self-hosted gateway.

> **Log export is reserved.** `LogConfig` (`TELEMETRY_LOG_*`) is parsed but not yet wired to an exporter, because the OpenTelemetry Go log SDK is still experimental. The existing zap logger remains the log path. These variables are accepted for forward compatibility and currently have no effect.

## Metrics

With `TELEMETRY_ENABLE=true`, the server starts a separate HTTP server exposing Prometheus metrics at `http://<host>:<TELEMETRY_METRICS_PORT>/metrics`. Recorded instruments include token usage, request counts, response status, request duration, task lifecycle, and tool-call failures (all prefixed `a2a.`).

## Tracing

### OTLP Trace Export

When `TELEMETRY_TRACE_ENABLE=true`, a batching OTLP/HTTP trace exporter is configured against `TELEMETRY_TRACE_ENDPOINT` and registered as the global `TracerProvider`. Spans are exported in the background and flushed on shutdown.

### Trace Context Propagation

A composite [W3C Trace Context](https://www.w3.org/TR/trace-context/) + [Baggage](https://www.w3.org/TR/baggage/) propagator is installed globally. For every request to `/a2a`, the middleware extracts `traceparent` and `baggage` headers from the incoming request and starts a server-kind span named `a2a.request`. When the caller supplies a `traceparent`, the ADK span becomes a child of the caller's span, giving you an end-to-end distributed trace across agents.

The span carries HTTP semantic-convention attributes (`http.request.method`, `url.full`, `http.route`, `http.response.status_code`).

### Baggage Attributes

Two baggage items are promoted to span attributes when present, so they are queryable in your tracing backend:

| Baggage key          | Span attribute       |
| -------------------- | -------------------- |
| `infer.session.id`   | `infer.session.id`   |
| `infer.tool.call.id` | `infer.tool.call.id` |

A caller propagates them by setting the standard `baggage` header:

```http
baggage: infer.session.id=session-123,infer.tool.call.id=tool-456
```

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
export TELEMETRY_TRACE_ENABLE=true
export TELEMETRY_TRACE_ENDPOINT=http://localhost:4318
```

The default endpoint (`http://localhost:4318`) matches the collector's OTLP/HTTP receiver. Traces then appear in whatever backend the collector exports to (Jaeger, Tempo, Honeycomb, etc.), correlated by session and tool-call baggage.
