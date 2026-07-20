# ZaaS - Zufall as a Service

[![CI](https://github.com/haraig/zaas/actions/workflows/ci.yaml/badge.svg)](https://github.com/haraig/zaas/actions/workflows/ci.yaml)
[![Go version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**Randomness as a Service.** A free, open REST API for random data - dice rolls, coin flips, UUIDs, colors, coordinates, and more.

**Live:** [zaas.at](https://zaas.at) | **API Docs:** [zaas.at/docs](https://zaas.at/docs)

```bash
curl https://zaas.at/api/v1/dice?sides=20
# {"result":{"value":17,"sides":20},"meta":{"endpoint":"dice","timestamp":"..."}}

curl https://zaas.at/api/v1/uuid?count=3
# {"results":[{"uuid":"..."},{"uuid":"..."},{"uuid":"..."}],"meta":{...}}
```

## Endpoints

| Endpoint                  | Description             | Key params                                                |
| ------------------------- | ----------------------- | --------------------------------------------------------- |
| `GET /api/v1/dice`        | Roll dice               | `sides` (4/6/8/10/12/20/100, default 6), `count` (1-100) |
| `GET /api/v1/coin`        | Flip a coin             | -                                                         |
| `GET /api/v1/number`      | Random integer or float | `min`, `max`, `count`, `float`                            |
| `GET /api/v1/uuid`        | Generate UUIDs          | `count`, `format` (standard/no-dashes/urn)                |
| `GET /api/v1/color`       | Random color            | `count`, `format` (hex/rgb/hsl)                           |
| `GET /api/v1/coordinates` | Random geo coordinates  | `count`, `land-only`                                      |
| `GET /api/v1/password`    | Random password         | `length` (8-128), `uppercase`, `lowercase`, `digits`, `symbols`, `count` |
| `GET /api/v1/words`       | Random words            | `style` (random/docker/ubuntu), `words`, `separator`, `capitalize`, `count` |
| `GET /api/v1/lorem`       | Lorem ipsum text        | `paragraphs`, `sentences`, `count`                        |
| `GET /api/v1/ssh-key`     | SSH keypair             | `type` (ed25519/rsa), `bits`, `comment`, `count`          |

Response envelope: `{"result": ..., "meta": {"endpoint": "...", "timestamp": "...", "params": {...}}}` (single) or `{"results": [...], "meta": {...}}` (multiple).

**Rate limiting:** 60 req/min per IP. Free API keys give 600 req/min - email contact@zaas.at to request one. Authenticate with `Authorization: Bearer <key>`.

## Running Locally

```bash
# API only - no config needed, all env vars have defaults
cd api && ZAAS_OTEL_ENABLED=false go run ./cmd/server
# curl -s http://localhost:8080/api/v1/uuid | jq -r .result
# 8751ac24-97d0-4060-b716-5b4ee27293b5

# Full stack (API + web + Grafana observability)
make dev
# http://localhost         (main site)
# http://grafana.localhost (Grafana, no login in local mode)
```

## Development

```bash
make setup        # install golangci-lint, npm deps, lefthook git hooks
make test         # run Go tests
make lint         # run golangci-lint
make generate     # regenerate code from docs/reference/openapi.yaml (after spec changes)
make fmt-all-fix  # auto-fix formatting for all files
make web-dev      # start Astro dev server
make web-check    # TypeScript check + build web
```

## Configuration

All settings are environment variables with sensible defaults - no `.env` file needed for local development. For the local Docker compose workflows, set values in the repo root `.env` file (copy from [`.env.example`](.env.example)) if you want to override the defaults for the web build, especially `PUBLIC_IMPRINT_*` and `PUBLIC_API_BASE_URL`.

Key variables:

| Variable            | Default                 | Description                                        |
| ------------------- | ----------------------- | -------------------------------------------------- |
| `ZAAS_BASE_URL`     | `http://localhost:8080` | Public-facing base URL (single source of truth)    |
| `ZAAS_OTEL_ENABLED` | `true`                  | Set `false` when running without an OTel Collector |
| `ZAAS_LOG_LEVEL`    | `info`                  | `debug` / `info` / `warn` / `error`                |

**Production-only** (no defaults - must be set):

| Variable                 | Description                                         |
| ------------------------ | --------------------------------------------------- |
| `DEPLOY_WEBHOOK_SECRET`  | Shared secret for the deploy webhook (GitHub -> VPS) |
| `GRAFANA_ADMIN_PASSWORD` | Admin password for Grafana UI                       |
| `PUBLIC_IMPRINT_*`       | Legal name, address, email for the `/imprint` page  |
| `PUBLIC_PRIVACY_EMAIL`   | Email address for GDPR data subject requests        |
| `PUBLIC_API_BASE_URL`    | Public API base URL for browser-side requests       |

## Tech Stack

- **API**: Go 1.26, [chi](https://github.com/go-chi/chi), [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen), OpenTelemetry
- **Web**: [Astro](https://astro.build), Tailwind CSS v4, [Scalar](https://scalar.com) API playground
- **Observability**: OTel Collector, Prometheus, Tempo, Loki, Grafana
- **Deployment**: Docker, Caddy, Hetzner Cloud

See [`docs/how-to/deploy.md`](docs/how-to/deploy.md) for production deployment instructions.

## Documentation

Detailed documentation lives in [`docs/`](docs/):

- [Tutorial: Add a new endpoint](docs/tutorials/add-endpoint.md) - end-to-end walkthrough
- [How to run locally](docs/how-to/run-locally.md) - full stack setup
- [How to deploy](docs/how-to/deploy.md) - production deployment
- [Architecture reference](docs/reference/architecture.md) - package layout, code generation, auth
- [OpenAPI spec](docs/reference/openapi.yaml) - source of truth for the API contract

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a PR.

## Community

- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Security Policy](SECURITY.md)

## License

[MIT](LICENSE) © Harald Aigner
