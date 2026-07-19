# Observability Concepts

This document explains what observability is in ZaaS, why the stack is built the way it is, and how the pieces fit together. For the concrete facts (which tools, which ports, which pipelines), see [reference/observability.md](../reference/observability.md).

## The Three Pillars

Modern observability rests on three complementary signal types:

- **Traces** - a request's journey through your system, as a tree of timed spans. Answers: "where did this request spend its time?"
- **Metrics** - numeric measurements over time. Answers: "what is the rate/duration/error-count right now and historically?"
- **Logs** - structured event records. Answers: "what exactly happened at this moment in this request?"

Each pillar has blind spots the others cover. Metrics tell you something is wrong but not where. Traces tell you where but not the surrounding context. Logs have full context but no aggregation. Using all three together, linked by a shared `trace_id`, gives a complete picture.

## Why OpenTelemetry

Without OTel, each observability backend requires its own SDK in the application: a Prometheus client for metrics, a Tempo/Jaeger client for traces, a custom log shipper for logs. Swapping backends means changing application code.

With OTel, the application uses only the **OTel API/SDK**. The SDK emits all signals in **OTLP** (OpenTelemetry Protocol) to the **OTel Collector**, which handles routing, batching, and format translation to whatever backends are configured. The application is decoupled from the backends entirely.

## OTel Components in ZaaS

```
+--------------------------------------------+
| Application (zaas-api)                     |
|                                            |
|  OTel SDK                                  |
|  +-- TracerProvider  -+                    |
|  +-- MeterProvider   -+---> OTLP gRPC :4317 --> OTel Collector
|  +-- LoggerProvider  -+                    |
|                                            |
|  Bridges:                                  |
|  +-- slog --> OTel LoggerProvider          |
|                                            |
|  Instrumentation:                          |
|  +-- otelhttp wraps the HTTP server        |
+--------------------------------------------+
```

**TracerProvider** - Creates `Tracer` instances. Configured with a batch span processor that forwards to an OTLP gRPC exporter.

**MeterProvider** - Creates `Meter` instances. Configured with two readers:
1. A `PeriodicReader` driving an OTLP gRPC exporter (pushes to Collector every 30s).
2. A `PrometheusReader` driving the optional `/metrics` HTTP endpoint (pull-based, only used if `ZAAS_METRICS_ENDPOINT_ENABLED=true`).

**LoggerProvider** - The `otelslog` bridge translates all standard `slog.Info/Error/...` calls into OTel log records. This means standard Go logging flows into Loki without any changes to call sites throughout the codebase - every `slog.InfoContext(ctx, "...")` call automatically includes the `trace_id` and `span_id` from the context, enabling log-to-trace correlation.

**No-op mode** - When `ZAAS_OTEL_ENABLED=false`, all three providers are replaced with their no-op implementations. No connections are made, no data is exported, zero overhead. This is the intended mode when running `go run ./api/cmd/server` without a Collector.

## Push vs. Pull

The OTel Collector uses a **push model** for all three signals: the application actively sends data to the Collector. Prometheus is natively pull-based (it scrapes targets), but the Collector bypasses this via **Remote Write**, pushing metrics directly into Prometheus's storage engine. This avoids exposing the API to Prometheus and simplifies network topology - only the API needs to know the Collector's address.

Some Prometheus scrape targets do exist (Caddy admin API, the API's own `/metrics` endpoint, exporters) for metrics that are not available via OTLP push.

## Application Integration

### Initialization

All OTel initialization is centralized in `api/internal/telemetry/telemetry.go`:

```go
func Init(ctx context.Context, enabled bool) (Result, error)
```

Returns a `Result` containing:
- `Shutdown func(context.Context) error` - gracefully flushes and shuts down all exporters on `SIGTERM`/`SIGINT`.
- `PrometheusHandler http.Handler` - the `/metrics` handler. `nil` when OTel is disabled.

### HTTP Instrumentation

The entire chi router is wrapped with `otelhttp.NewHandler`. This single call automatically:

- Creates a new trace span for every incoming HTTP request.
- Propagates incoming trace context from `traceparent`/`tracestate` headers (W3C Trace Context).
- Records standard HTTP attributes: `http.request.method`, `url.path`, `http.response.status_code`, `http.route`.
- Emits `http.server.request.duration` histogram and `http.server.active_requests` gauge metrics.
- Injects the active span into context so log lines carry the `trace_id`.

Health check paths (`/healthz`, `/readyz`) are excluded from instrumentation - they are polled every 10 seconds by Docker and would generate noise in dashboards.

### Adding Custom Spans

For business-logic-level tracing, acquire a tracer and create child spans:

```go
import "go.opentelemetry.io/otel"

tracer := otel.Tracer("zaas-api")

func (s *DiceService) Roll(ctx context.Context, n, sides int) ([]int, error) {
    ctx, span := tracer.Start(ctx, "dice.roll",
        trace.WithAttributes(
            attribute.Int("dice.count", n),
            attribute.Int("dice.sides", sides),
        ),
    )
    defer span.End()
    // ...
}
```

## End-to-End Trace Shape

With Caddy's `tracing` directive configured, a complete request trace looks like:

```
[caddy-ingress: GET /api/v1/dice]          <- Caddy span (root)
    +-- [HTTP span: GET /api/v1/dice]       <- Go API span (child, via traceparent header)
            +-- (any custom child spans)
```

Both spans share the same `trace_id`. In Grafana Explore (Tempo datasource), you see a waterfall with Caddy's processing time (TLS, routing, connection pooling) and the API's processing time as nested segments.

Trace propagation uses **W3C Trace Context** (`traceparent`/`tracestate` headers) - the OTel SDK default, supported by Caddy, `otelhttp`, and all modern OTel SDKs.

## Docker Container Logs

All container stdout/stderr is collected into Loki via the OTel Collector's `filelog` receiver. Docker's `json-file` log driver writes each container's output to `/var/lib/docker/containers/<id>/<id>-json.log`. The Collector mounts this directory read-only and tails all log files.

The filelog pipeline:
1. **Tail** - watches `*-json.log` files for new lines
2. **Parse** - extracts `log`, `stream`, and `time` from the Docker JSON envelope
3. **Timestamp** - uses the Docker timestamp as the log record timestamp
4. **Extract** - captures the container ID from the file path via regex
5. **Route** - feeds into the existing `logs` pipeline

Example LogQL queries in Grafana:

```logql
# All Docker container logs
{log_iostream=~".+"}

# Only stderr
{log_iostream="stderr"}

# Filter by container ID prefix
{log_iostream=~".+"} | container_id =~ "a1b2c3.*"
```

Note: the `filelog` receiver cannot resolve container IDs to human-readable names - it has no access to the Docker API. For a small Compose stack this is acceptable.

## Monitoring Caddy

Caddy is integrated via metrics (Option A) and distributed tracing (Option C):

**Option A: Prometheus metrics** - Caddy's built-in metrics module exposes `caddy_http_requests_total`, `caddy_http_request_duration_seconds`, and TLS metrics. Configured via `servers { metrics }` in the Caddyfile; scraped by Prometheus at `:2019`.

**Option C: Distributed tracing** - Caddy's `tracing` directive creates root spans and propagates `traceparent` to upstream services. Configured via `OTEL_EXPORTER_OTLP_ENDPOINT` pointing at the Collector. Produces the Caddy -> Go API trace chain described above.

**Option B (not implemented): Caddy access log shipping** - Caddy emits structured JSON access logs that could be shipped to Loki via a filelog receiver. Deferred because metrics and traces already cover request-level visibility.

## Tool Alternatives

### OTel Collector Alternatives

| Tool | Notes |
| ---- | ----- |
| **Vector** | Rust-based, high throughput. Stronger in log transformation pipelines. |
| **Fluent Bit** | Lightweight C agent, purpose-built for log collection. No native trace support. |
| **Grafana Alloy** | OTel-compatible collector from Grafana Labs. Drop-in replacement with native Grafana ecosystem integration. |

**Recommendation:** The OTel Collector (contrib distribution) is the standard choice. Grafana Alloy is a strong alternative if fully committed to the Grafana ecosystem.

### Tracing Backend Alternatives

| Tool | Notes |
| ---- | ----- |
| **Jaeger** | Mature open-source backend with a dedicated UI. Replaced by Tempo for tighter Grafana integration. |
| **Zipkin** | Older, less feature-rich. |
| **AWS X-Ray / Google Cloud Trace** | Managed cloud-native; require vendor-specific exporters. |

### Metrics Backend Alternatives

| Tool | Notes |
| ---- | ----- |
| **Grafana Mimir** | Horizontally scalable Prometheus-compatible storage. |
| **VictoriaMetrics** | High-performance, resource-efficient. Excellent for high-cardinality workloads. |
| **Thanos** | Adds long-term storage and global query on top of existing Prometheus. |

### Log Backend Alternatives

| Tool | Notes |
| ---- | ----- |
| **Elasticsearch / OpenSearch** | Full-text indexing of all fields. High resource cost. Right choice when full-text search across all fields is required. |
| **ClickHouse** | Columnar database, fast aggregation. More operational overhead. |
