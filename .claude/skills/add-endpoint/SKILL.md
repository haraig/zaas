---
description: Add a new API endpoint to ZaaS. Use when asked to add, implement, or create a new endpoint or randomness type. Covers the full workflow: OpenAPI spec -> code generation -> service -> handler -> tests.
---

# Add a New ZaaS Endpoint

Follow these steps in order. The spec is the source of truth - everything flows from it.

## Reference

Full tutorial with examples: `docs/tutorials/add-endpoint.md`
OpenAPI spec (source of truth): `docs/reference/openapi.yaml`
Generated code (never hand-edit): `api/internal/gen/openapi.gen.go`

## Workflow

### 1. Update the OpenAPI spec

Edit `docs/reference/openapi.yaml`. Add a path entry under `paths:`. Follow existing patterns:
- Simple endpoint (no params): see `/coin`
- Endpoint with query params: see `/dice`
- Use existing `$ref` schemas where possible (`SingleIntResult`, `MultiIntResult`, `SingleStringResult`, `MultiStringResult`, `SingleCoordinateResult`, etc.)
- Required responses: `"200"`, `"400"` (`$ref: "#/components/responses/BadRequest"`), `"429"` (`$ref: "#/components/responses/TooManyRequests"`)
- `operationId` must be `Get<EndpointName>` in PascalCase

### 2. Regenerate code

```bash
make generate
```

The build now fails (expected) - `Server` does not yet implement the new `ServerInterface` method.

### 3. Write a failing service test first (TDD)

Create `api/internal/service/<name>_test.go`. Test the pure business logic only - no HTTP, no generated types.

Run `make test` and confirm it fails with "undefined".

### 4. Implement the service function

Create `api/internal/service/<name>.go`. Rules:
- No HTTP imports
- No `gen.*` types
- Pure functions only - accept primitives, return primitives or domain types
- Document non-obvious behavior with comments

Run `make test` - service tests must pass before continuing.

### 5. Implement the handler

Add the method to `api/internal/handler/server.go`. The signature is in `api/internal/gen/openapi.gen.go`.

Response helpers (defined in `handler/respond.go`):
- `WriteSingle(w, r, result)` - single result
- `WriteMultiple(w, r, results)` - slice of results
- `WriteError(w, statusCode, errorCode, message)` - error response

The compile-time assertion `var _ gen.ServerInterface = (*Server)(nil)` confirms full implementation.

### 6. Add a handler integration test

Add `api/internal/handler/<name>_test.go`. Test the HTTP layer:
- Status codes
- Response envelope shape (`result`/`results` + `meta` fields)
- Parameter validation (invalid values -> 400)
- Count parameter behavior if applicable

Run `make test` - all tests must pass.

### 7. Verify end-to-end

```bash
ZAAS_OTEL_ENABLED=false go run ./api/cmd/server &
curl "http://localhost:8080/api/v1/<name>"
```

Confirm the response envelope matches the spec.

### 8. Commit

Stage exactly: spec, generated files, service, handler, tests.

```bash
git add docs/reference/openapi.yaml \
        api/internal/gen/openapi.gen.go \
        api/internal/handler/openapi.yaml \
        api/internal/service/<name>.go \
        api/internal/service/<name>_test.go \
        api/internal/handler/server.go \
        api/internal/handler/<name>_test.go
git commit -m "feat: add GET /api/v1/<name> endpoint"
```

## Checklist

- [ ] Spec updated in `docs/reference/openapi.yaml`
- [ ] `make generate` run, generated files committed
- [ ] Service test written and passing
- [ ] Service function implemented
- [ ] Handler method implemented
- [ ] Handler integration test passing
- [ ] `make test` passes (all tests, not just new ones)
- [ ] `make lint` passes
- [ ] Manual curl verification done
