# How to Run ZaaS Locally

## API Only (fastest)

No Docker required. All environment variables have sensible defaults.

```bash
ZAAS_OTEL_ENABLED=false go run ./api/cmd/server
# -> listening on http://localhost:8080
```

Test it:

```bash
curl http://localhost:8080/api/v1/dice?sides=20
```

## Minimal Stack (API + web, no monitoring)

Runs the API, web frontend, and Caddy. No database, no Redis, no observability backends.

```bash
make dev-minimal
```

URLs after startup:

| Service | URL |
| ------- | --- |
| Main site | `http://localhost` |
| API | `http://localhost/api/v1/` |

Trade-offs vs the full stack:
- Auth and API key endpoints are disabled (no database)
- Rate limiting is in-memory (resets on restart)
- No traces, metrics, or logs shipped anywhere
- No Grafana dashboard

## Full Stack

Runs the API, web frontend, and the full observability stack (Grafana, Prometheus, Tempo, Loki).

```bash
make dev
```

URLs after startup:

| Service | URL |
| ------- | --- |
| Main site | `http://localhost` |
| API | `http://localhost/api/v1/` |
| Grafana | `http://grafana.localhost` |
| Mailpit (local SMTP) | `http://localhost:8025` |

No `.env` file is needed - all variables have defaults for local development. See `.env.example` for the full list with comments.

## Setup (first time)

Install tools, npm dependencies, and git hooks:

```bash
make setup
```

## Common Development Tasks

```bash
make test          # run Go tests
make lint          # run golangci-lint
make generate      # regenerate code from docs/reference/openapi.yaml (after spec changes)
make fmt-all-fix   # auto-fix formatting for all files
make web-dev       # start Astro dev server (hot reload)
make web-check     # TypeScript check + build web
```

## Configuration

All settings are environment variables. Key variables:

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `ZAAS_BASE_URL` | `http://localhost:8080` | Public-facing base URL |
| `ZAAS_OTEL_ENABLED` | `true` | Set `false` when running without an OTel Collector |
| `ZAAS_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `ZAAS_RATE_LIMIT_BACKEND` | `memory` | `memory` or `redis` |

Production-only (no defaults - must be set in production):

| Variable | Description |
| -------- | ----------- |
| `DEPLOY_WEBHOOK_SECRET` | Shared secret for the deploy webhook |
| `GRAFANA_ADMIN_PASSWORD` | Admin password for Grafana UI |
| `PUBLIC_IMPRINT_*` | Legal name, address, email for the `/imprint` page |

See `.env.example` for the complete list.

## Stopping the Stack

```bash
make stop        # stop containers, keep volumes
make clean       # stop containers and remove volumes
make distclean   # stop containers, remove volumes, and delete node_modules / build artifacts
```
