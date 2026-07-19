# Migrating to Dynatrace

Dynatrace is a commercial **all-in-one observability platform** that consolidates traces, metrics, logs, synthetic monitoring, AIOps, and infrastructure monitoring in a single SaaS product.

Because ZaaS uses the vendor-neutral OTel SDK, migrating to Dynatrace requires no application code changes.

## What Dynatrace Replaces

All four backend components can be eliminated:

| Current Component | Replaced By | Notes |
| ----------------- | ----------- | ----- |
| Tempo | Dynatrace Distributed Tracing | Built into the Dynatrace SaaS platform. Richer AI-assisted root cause analysis. |
| Prometheus | Dynatrace Metrics | Stores metrics natively. Supports Prometheus remote-write ingestion. |
| Loki | Dynatrace Log Management | Structured log ingestion. Log-to-trace correlation natively. |
| Grafana | Dynatrace Dashboards + Notebooks | Built-in dashboards, notebooks, and a query language (DQL). |

## What Stays

| Current Component | Status | Notes |
| ----------------- | ------ | ----- |
| **OTel SDK** (in the Go API) | **Unchanged** | Zero application code changes. The OTel SDK is vendor-neutral. The slog bridge, `otelhttp` instrumentation, and telemetry initialization code are identical. |
| **OTel Collector** | Optional | Dynatrace can ingest OTLP directly. Point `OTEL_EXPORTER_OTLP_ENDPOINT` at the Dynatrace endpoint and remove the Collector. Alternatively, keep the Collector for preprocessing, fan-out, or buffering. |

## Configuration Changes

### 1. OTel Exporter Endpoint

```bash
# Before
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317

# After (direct to Dynatrace, no Collector)
OTEL_EXPORTER_OTLP_ENDPOINT=https://{your-env-id}.live.dynatrace.com/api/v2/otlp
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Api-Token {your-api-token}
```

Dynatrace requires an API token with the `openTelemetryTrace.ingest`, `metrics.ingest`, and `logs.ingest` scopes.

### 2. OTel Collector (if retained)

Replace the three exporters with a single Dynatrace OTLP exporter:

```yaml
# otel-collector.yaml
exporters:
  otlphttp/dynatrace:
    endpoint: "https://{your-env-id}.live.dynatrace.com/api/v2/otlp"
    headers:
      Authorization: "Api-Token {your-api-token}"

pipelines:
  traces:
    receivers: [otlp]
    processors: [batch]
    exporters: [otlphttp/dynatrace]
  metrics:
    receivers: [otlp]
    processors: [batch]
    exporters: [otlphttp/dynatrace]
  logs:
    receivers: [otlp]
    processors: [batch]
    exporters: [otlphttp/dynatrace]
```

### 3. deploy/docker-compose.yaml

Remove the `tempo`, `prometheus`, `loki`, and `grafana` service definitions and their associated volumes. Remove Grafana provisioning files. The `otel-collector` service remains (or is also removed if going direct).

## What Does Not Change

- All Go source code in `api/` - zero changes.
- The Caddy configuration.
- `api/internal/telemetry/telemetry.go` initialization logic.
- The `otelhttp` router wrapping.
- The `otelslog` logging bridge.
- The `ZAAS_OTEL_ENABLED` kill switch behavior.

## Optional: Dynatrace OneAgent

Dynatrace also offers **OneAgent**, a host-level agent that instruments processes via bytecode injection (JVM/Node.js/.NET) or eBPF (any process). For Go, OneAgent provides infrastructure-level metrics (CPU, memory, disk I/O, network) and can instrument Go binaries without code changes via eBPF. This would add Caddy monitoring and system metrics without configuration changes to either application.

## Migration Summary

```
Migration cost: LOW
- Application code changes: NONE
- Infrastructure changes: Remove 4 containers, update 2 env vars
- Data migration: Not applicable (observability data is ephemeral)
- Risk: LOW - the OTel SDK is the only app dependency, and it is vendor-neutral
```
