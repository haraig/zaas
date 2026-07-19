# Tutorial: Add a New API Endpoint

This tutorial walks through adding a complete new randomness endpoint to ZaaS - from the OpenAPI spec through to a working, tested handler. You will add a `GET /api/v1/coin` endpoint as an example (already exists in the codebase, so use a different name when following along for real).

By the end you will understand the full flow: spec -> generate -> service -> handler -> test.

## Prerequisites

- Go toolchain installed
- `make setup` has been run
- `ZAAS_OTEL_ENABLED=false go run ./api/cmd/server` starts without errors

---

## Step 1: Add the Endpoint to the OpenAPI Spec

The spec at `docs/reference/openapi.yaml` is the source of truth. All changes start here.

Add a path entry. Follow the existing patterns - look at `/coin` for a simple no-parameter endpoint or `/dice` for one with query parameters.

```yaml
# docs/reference/openapi.yaml

paths:
  /yourname:
    get:
      operationId: GetYourName
      summary: Brief description
      tags: [Random]
      parameters: []          # add query params here if needed
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                oneOf:
                  - $ref: "#/components/schemas/SingleStringResult"
                  - $ref: "#/components/schemas/MultiStringResult"
        "400":
          $ref: "#/components/responses/InvalidParam"
        "429":
          $ref: "#/components/responses/RateLimited"
```

Use existing `$ref` schemas (`SingleIntResult`, `MultiIntResult`, `SingleStringResult`, `MultiStringResult`, etc.) where possible. Only add new schemas to `components/schemas` when the result type is genuinely new.

## Step 2: Regenerate Code

```bash
make generate
```

This updates `api/internal/gen/openapi.gen.go` with a new `GetYourName` method on `ServerInterface` and copies the updated spec to `api/internal/handler/openapi.yaml`.

The build now fails with a compile error - `Server` in `handler/server.go` does not yet implement the new interface method. That is expected and correct.

## Step 3: Write a Failing Test

Before implementing, write a test that captures the expected behavior.

Create `api/internal/service/yourname_test.go`:

```go
package service_test

import (
    "testing"

    "zaas/api/internal/service"
)

func TestYourName(t *testing.T) {
    result, err := service.YourName()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // assert the shape of the result
    if result == "" {
        t.Error("expected non-empty result")
    }
}
```

Run it to confirm it fails:

```bash
make test
# -> FAIL: undefined: service.YourName
```

## Step 4: Implement the Service Function

Create `api/internal/service/yourname.go`:

```go
package service

// YourName returns a random <thing>.
func YourName() (string, error) {
    // pure logic here - no HTTP, no generated types
    return "result", nil
}
```

The `service/` package has no knowledge of HTTP or generated types. Keep it that way.

Run tests:

```bash
make test
# -> PASS
```

## Step 5: Implement the Handler

Add the handler method to `api/internal/handler/server.go`. The method signature is generated - find it in `api/internal/gen/openapi.gen.go` and implement it:

```go
// GetYourName implements gen.ServerInterface.
func (s *Server) GetYourName(w http.ResponseWriter, r *http.Request, params gen.GetYourNameParams) {
    result, err := service.YourName()
    if err != nil {
        WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
        return
    }
    WriteSingle(w, r, result)
}
```

Use `WriteSingle` for a single result or `WriteMultiple` for multiple results. Both are defined in `handler/respond.go`.

The build now compiles cleanly - the compile-time assertion in `server.go` (`var _ gen.ServerInterface = (*Server)(nil)`) verifies the interface is fully implemented.

## Step 6: Add an Integration Test

Add an HTTP-level test in `api/internal/handler/` alongside the existing handler tests:

```go
func TestGetYourName(t *testing.T) {
    srv := newTestServer(t)
    resp := srv.Get(t, "/api/v1/yourname")
    resp.AssertStatus(t, http.StatusOK)
    resp.AssertJSONField(t, "result") // verify envelope shape
}
```

Run all tests:

```bash
make test
# -> PASS
```

## Step 7: Verify End-to-End

```bash
ZAAS_OTEL_ENABLED=false go run ./api/cmd/server &
curl http://localhost:8080/api/v1/yourname
# -> {"result":"...","meta":{"endpoint":"yourname","timestamp":"..."}}
```

## Step 8: Commit

```bash
git add docs/reference/openapi.yaml \
        api/internal/gen/openapi.gen.go \
        api/internal/handler/openapi.yaml \
        api/internal/service/yourname.go \
        api/internal/service/yourname_test.go \
        api/internal/handler/server.go \
        api/internal/handler/yourname_test.go
git commit -m "feat: add GET /api/v1/yourname endpoint"
```

---

## What You Just Did

| Step | File(s) changed |
| ---- | --------------- |
| Spec | `docs/reference/openapi.yaml` |
| Generated | `api/internal/gen/openapi.gen.go`, `api/internal/handler/openapi.yaml` |
| Service | `api/internal/service/yourname.go` |
| Handler | `api/internal/handler/server.go` |
| Tests | `api/internal/service/yourname_test.go`, `api/internal/handler/yourname_test.go` |

The compile-time assertion on `ServerInterface` ensures you can never ship a spec change without a corresponding handler implementation.
