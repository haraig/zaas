---
description: Modify an existing OpenAPI operation in ZaaS. Use when adding parameters, changing response shapes, or renaming fields on an endpoint that already exists. Covers spec edit -> make generate -> handler update -> test update -> single commit.
---

# Update an Existing ZaaS Endpoint Spec

Use this skill when changing an existing endpoint: adding or removing a query parameter,
changing a response schema, renaming a field, adjusting validation rules, or adding a new
response code. The spec is still the source of truth - everything flows from it.

## Reference

OpenAPI spec (source of truth): `docs/reference/openapi.yaml`
Generated code (never hand-edit): `api/internal/gen/openapi.gen.go`
Handler: `api/internal/handler/server.go`
Response helpers: `api/internal/handler/respond.go` (`WriteSingle`, `WriteMultiple`, `WriteError`)

## Workflow

### 1. Edit the OpenAPI spec

Edit `docs/reference/openapi.yaml`. Common changes:

- **Add a query parameter:** add an entry under the operation's `parameters:` list. Mark it
  `required: false` with a `default:` if it is optional.
- **Change a response shape:** edit the `$ref` schema under `components/schemas/` or inline
  the change in the operation's `responses:` block.
- **Rename a field:** rename in the schema and update every `$ref` that points to it.
- **Add an error case:** add the status code entry and `$ref` to the matching response
  component, or define an inline response.

Always keep:
- Required responses: `"200"`, `"400"` (`$ref: "#/components/responses/BadRequest"`),
  `"429"` (`$ref: "#/components/responses/TooManyRequests"`)
- `operationId` unchanged unless the endpoint is being renamed (that is a breaking change)

### 2. Regenerate code

```bash
make generate
```

If you changed a type used in the handler, the build will now fail - expected.

### 3. Update the handler

Open `api/internal/handler/server.go`. Adjust the affected method to match the new generated
signature or new parameter. Use the response helpers:

- `WriteSingle(w, r, result)` - single-value response
- `WriteMultiple(w, r, results)` - slice response
- `WriteError(w, statusCode, errorCode, message)` - error response

Do not import `net/http` status codes by hand - use the constants from the package.

The compile-time assertion `var _ gen.ServerInterface = (*Server)(nil)` will catch any
missing or mismatched method.

### 4. Update tests

Open the matching `api/internal/handler/<name>_test.go`. Adjust:

- Any test that asserts on changed field names or shapes
- Any test that passes the old parameter form
- Add a new test case for the new parameter or code path if one was added

Run `make test` - all tests must pass before continuing.

### 5. Verify end-to-end

```bash
ZAAS_OTEL_ENABLED=false go run ./api/cmd/server &
curl "http://localhost:8080/api/v1/<endpoint>?<new-param>=<value>"
```

Confirm the response envelope matches the updated spec.

### 6. Commit

Stage exactly: spec, generated files, handler, tests. No other files unless a doc update
is required (see below).

```bash
git add docs/reference/openapi.yaml \
        api/internal/gen/openapi.gen.go \
        api/internal/handler/server.go \
        api/internal/handler/<name>_test.go
git commit -m "fix(api): <concise description of the spec change>"
```

### Doc updates required?

- Parameter or response shape changed -> `docs/reference/openapi.yaml` is already updated
  (done in step 1); no separate doc file needed.
- New env var introduced -> `.env.example`, `README.md` env table,
  `docs/reference/architecture.md`.
- Behavior rationale changed -> `docs/explanation/design-decisions.md`.
- Non-obvious pitfall encountered -> `docs/reference/gotchas.md`.

## Checklist

- [ ] Spec edited in `docs/reference/openapi.yaml`
- [ ] `make generate` run, generated files committed
- [ ] Handler updated to match new generated types
- [ ] Existing tests updated to match new shapes
- [ ] New test cases added for new paths/parameters
- [ ] `make test` passes (all tests, not just affected ones)
- [ ] `make lint` passes
- [ ] Manual curl verification done
- [ ] Doc updates committed if required
