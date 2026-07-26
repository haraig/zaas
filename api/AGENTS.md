# API - AI Assistant Configuration

<!-- Keep commands in sync with root AGENTS.md and Makefile -->

## Overview

Go REST API service using chi router, oapi-codegen, and OpenTelemetry.

## Commands

```bash
make test       # Run Go tests (cd api && go test ./...)
make lint       # Run golangci-lint on Go code
make generate   # Regenerate Go types from OpenAPI spec
make build      # Build API Docker image
```

Run locally without Docker:

```bash
ZAAS_OTEL_ENABLED=false go run ./api/cmd/server
```

## Coding Conventions

- **API-first:** `docs/reference/openapi.yaml` is the source of truth. Run `make generate` after spec changes, then implement the handler.
- **Router:** chi router for HTTP routing
- **Logging:** slog (structured logging) - never `fmt.Println` or `log` stdlib
- **Observability:** OpenTelemetry for traces and metrics
- **Rate limiting:** Sliding window via `internal/ratelimiter`
- **Config:** Environment variables only, loaded via `internal/config`
- **Generated code:** Never hand-edit files in `internal/gen/` - they are overwritten by `make generate`

## Local Testing

### Testing per-client rate limits without email registration

Session 4 adds email-based API key registration, but you can seed a test client directly into PostgreSQL to test per-client rate limits locally without going through the email flow.

**1. Start the full stack**

```bash
make dev
```

**2. Insert a test client with a known API key**

```bash
docker exec -it deploy-postgres-1 psql -U zaas -d zaas -c "
INSERT INTO clients (email, display_name, api_key_hash, api_key_prefix, rate_limit_rpm, verified_at)
VALUES (
  'local-test@example.com',
  'Local Test Client',
  encode(sha256('zaas_deadbeefdeadbeefdeadbeefdeadbeef'::bytea), 'hex'),
  'zaas_dea',
  10,
  now()
);
"
```

Adjust `rate_limit_rpm` to whatever limit you want to test. The key `zaas_deadbeefdeadbeefdeadbeefdeadbeef` is a fixed local-only value - never use it in production.

**3. Make requests with the key**

```bash
curl -H "Authorization: Bearer zaas_deadbeefdeadbeefdeadbeefdeadbeef" \
  http://localhost:8080/api/v1/roll/dice
```

**4. Watch the Redis sliding window counters**

```bash
docker exec -it deploy-redis-1 redis-cli
> KEYS rate:*
> ZCARD rate:client:<uuid-from-db>
```

To get the client UUID:

```bash
docker exec -it deploy-postgres-1 psql -U zaas -d zaas -c \
  "SELECT id FROM clients WHERE email = 'local-test@example.com';"
```

**5. Clean up**

```bash
docker exec -it deploy-postgres-1 psql -U zaas -d zaas -c \
  "DELETE FROM clients WHERE email = 'local-test@example.com';"
```

---

> **Note:** If you only need to test sliding window behavior and don't need per-client limits, set `ZAAS_RATE_LIMIT_BACKEND=memory` (the default) - no Redis or DB required. The algorithm is identical between backends.

## Structure

```
api/
├── cmd/server/             # Entrypoint (main.go)
├── internal/
│   ├── config/             # Env var config loading
│   ├── gen/                # oapi-codegen generated types + ServerInterface
│   ├── handler/            # HTTP handlers (server.go implements ServerInterface)
│   ├── middleware/         # CORS, logging, rate limiting, request ID
│   ├── ratelimiter/        # RateLimiter interface + MemoryRateLimiter
│   ├── service/            # Business logic (dice, coin, number, uuid, color, coordinates)
│   ├── email/              # Email sending (API key registration flow)
│   ├── store/              # Database access layer
│   └── telemetry/          # OpenTelemetry SDK initialization
├── Dockerfile
└── go.mod
```
