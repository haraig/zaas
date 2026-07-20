# Design Decisions

This document explains the non-obvious architectural choices in ZaaS - the reasoning behind how things are built, not just what they are. For the structural facts (package layout, database schema, rate limiter interface), see [reference/architecture.md](../reference/architecture.md).

## API-First Code Generation

The API spec (`docs/reference/openapi.yaml`) is the contractual source of truth, not the Go code. This inversion - spec drives implementation rather than implementation driving spec - gives several advantages:

- The spec is the client contract. Changing behavior without changing the spec is a bug.
- Code generation (`oapi-codegen`) produces `ServerInterface` and all model types. The compile-time assertion `var _ gen.ServerInterface = (*Server)(nil)` makes it impossible to ship code that doesn't implement the full spec.
- Route registration is a single call (`gen.HandlerFromMux`), so a new endpoint added to the spec is automatically routed once the handler method is implemented.

The tradeoff: generated code in `internal/gen/` must never be hand-edited. Changes to the API contract always start with the spec, then run `make generate`.

## Response Envelope and oneOf Handling

### The problem

Each endpoint's 200 response is a `oneOf` between a single-result and a multi-result schema. `oapi-codegen` generates these as separate concrete structs per endpoint (e.g., `SingleIntResult`, `MultiIntResult`, `SingleStringResult`, `MultiStringResult`). Using them directly would require every handler to branch on a concrete named type - significant boilerplate with no runtime benefit, since non-strict mode handlers write directly to `http.ResponseWriter` with no return-type enforcement anyway.

### The approach

`respond.go` defines two uniform structs instead:

```go
type singleResponse struct {
    Result any      `json:"result"`
    Meta   gen.Meta `json:"meta"`
}

type multiResponse struct {
    Results []any    `json:"results"`
    Meta    gen.Meta `json:"meta"`
}
```

The `any`-typed field accepts any value. JSON output is byte-identical to the generated per-type structs - same field names, same `meta` shape. The generated `oneOf` structs exist in `openapi.gen.go` but are never referenced.

This trades compile-time response-type checking (which non-strict mode does not provide anyway) for a single uniform response path across all handlers. `gen.Meta` and `gen.Problem` are still used directly from generated code.

## API Naming Conventions

These conventions apply to all current and future v1 endpoints and are enforced by code review. Deviations require a new major version.

### Property names

Body schema properties use `snake_case` (e.g., `private_key`, `display_name`, `retry_after`). This is the dominant style across all existing schemas and aligns with common REST API practice.

### Query parameter names

Query parameters use `snake_case` (e.g., `land_only`, `count`, `float`). Kebab-case parameter names (`land-only`) are rejected because some HTTP client libraries cannot bind them automatically.

### Boolean flags

Boolean query parameters use bare adjectives without an `is` or `use` prefix: `float`, `uppercase`, `lowercase`, `digits`, `symbols`, `capitalize`, `land_only`. The bare form is idiomatic for query strings and is documented here so the convention is applied consistently to future parameters.

### Resource paths

Paths follow a "singular for the primitive being produced" rule: `/dice`, `/coin`, `/number`, `/uuid`, `/color`, `/password`, `/ssh-key`. The paths `/coordinates` and `/words` are retained as collective nouns because each response is inherently a set (a coordinate pair, a word string). The `count` parameter controls multiplicity for all endpoints.

### Operation IDs

`operationId` values follow these verb rules:
- `generate*` for artifact-producing endpoints: `generateUUID`, `generateColor`, `generatePassword`, `generateCoordinates`, `generateWords`, `generateLorem`, `generateSSHKey`, `generateNumber`.
- Domain verbs for gameplay primitives: `rollDice`, `flipCoin`.
- `auth*` for auth lifecycle: `authRegister`, `authVerify`, `authReissue`.

## Error Responses: RFC 9457 Problem Details

All error responses use `Content-Type: application/problem+json` and follow the [RFC 9457 Problem Details](https://datatracker.ietf.org/doc/html/rfc9457) schema:

```json
{
  "type": "https://zaas.at/errors/rate-limited",
  "title": "Too Many Requests",
  "status": 429,
  "detail": "Rate limit exceeded. Try again in 42 seconds.",
  "instance": "/api/v1/dice",
  "code": "RATE_LIMITED",
  "retry_after": 42,
  "request_id": "..."
}
```

The `type` URI is stable and resolves to human-readable documentation under `docs/reference/errors/<slug>.md`. The `code` field is a ZaaS extension carrying the machine-readable error code. The `retry_after` field is a ZaaS extension that mirrors the `Retry-After` HTTP header.

The choice to adopt RFC 9457 over a custom envelope is driven by tooling support: Postman, Insomnia, and many SDK generators have built-in Problem Details rendering. A custom envelope provides no benefit that RFC 9457 does not also provide.

## Versioning and Deprecation Policy

The API uses URL versioning under `/api/v1`. This makes the version explicit in every request and avoids header-based negotiation, which is harder to use from browsers and curl.

- **v1** is the current stable version. The contract is frozen at the point of initial public launch. No breaking changes will be made to v1.
- A breaking change is: removing or renaming a field, changing a field type, removing an endpoint, changing the semantics of a parameter, or removing a previously documented error code.
- **v2** will be introduced as a parallel path (`/api/v2`) when breaking changes are needed. v1 will continue to be served for at least 12 months after v2 is announced.
- Deprecation is signaled via `Deprecation` and `Sunset` HTTP response headers on affected endpoints before removal.
- Non-breaking additions (new optional fields, new endpoints, new enum values) are made to v1 without a new version.

## Rate Limiter Interface Design

Two backends (`memory`, `redis`) satisfy the same `RateLimiter` interface. The memory backend is the default for local development - no dependencies, identical algorithm. The redis backend is required for multi-replica production deployments to maintain a shared counter across instances.

The middleware is unaware of which backend is active. Switching is purely a configuration change (`ZAAS_RATE_LIMIT_BACKEND=redis`).

The choice of a sliding window counter over a fixed window is deliberate: fixed windows allow burst traffic at window boundaries (up to 2x the rate limit in a short span). Sliding windows bound the rate continuously.

**Considered alternative: Redis 8.8 `INCREX`.** The `INCREX` command (introduced in Redis OSS 8.8, 2026) atomically increments a counter with an upper bound and TTL-on-create (`ENX`), collapsing the rate limit pattern into a single command and removing the need for the Lua script. We do not use it because (a) it implements a fixed-window counter, which reintroduces the boundary burst problem the sliding window exists to solve, (b) it requires Redis 8.8+, which is not yet available on Redis Cloud or Redis Software, and (c) the current Lua-based implementation is small, atomic, and well-tested. Revisit only if we deliberately choose to relax the algorithm to fixed-window semantics.

## Auth: No Plaintext Key Storage

API keys are never stored in plaintext. Only the SHA-256 hash and a short prefix (for human identification in logs/admin UIs) are persisted. On every authenticated request the incoming key is hashed and looked up by hash.

The consequence: a lost API key cannot be recovered. The re-issue flow exists specifically because of this. This is the same model used by GitHub personal access tokens and similar systems. The operational overhead (users must store their key) is acceptable given the security benefit (a database breach does not expose usable keys).

## Auth: Registration and Reissue Are Issued On Request

`POST /auth/register` and `POST /auth/reissue` were originally fully public and
unauthenticated, protected only by a shared per-IP rate limit (5 req/min, covering
all three auth routes together). That protection stops one attacker hammering one
target, but not the actual abuse vector: each call sends a real email to an
attacker-supplied address, and an attacker who sprays one request each across a
large list of distinct addresses - by rotating source IPs or simply staying under
the per-IP limit - faces no throttle at all, since nothing limits how many *distinct*
recipients get emailed. That's a mail-relay/spam vector and a risk to the service's
sending domain reputation, not merely an annoyance.

Per-email cooldowns and tighter per-IP limits don't close this gap either: they bound
repetition from one caller, not the number of distinct targets a caller can reach.
The two mitigations that actually address distinct-target spam are CAPTCHA (gates on
"is this a human," independent of IP or target) and admin-only access (no public
caller can trigger a send at all). Self-service signup with CAPTCHA is planned; it's
separate, larger work than adding a gate.

We're issuing keys manually in the meantime: `X-Admin-Token` gates registration and
reissue, a prospective user emails to request a key, and the admin runs the request
(see `docs/reference/runbook.md`, "Issuing an API Key on Request"). `POST /auth/verify`
stays public, since it only completes a flow the admin already started and can't be
used to originate new outbound email on its own.

## Email: Stdlib Only

The email implementation uses `net/smtp` from the standard library with no external dependency. The tradeoff is manual MIME construction (multipart, quoted-printable encoding) versus convenience libraries like `gomail`. The choice keeps the dependency count low for a non-critical path.

The `Sender` interface is defined in `api/internal/email/` so the handler layer depends on an interface, not a concrete SMTP implementation. This makes the email sender substitutable in tests and allows the nil-sender pattern (when SMTP is not configured, the sender is `nil` and auth endpoints are disabled with a clear error rather than a panic).
