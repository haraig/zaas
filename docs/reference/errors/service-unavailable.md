# SERVICE_UNAVAILABLE

**Type URI:** `https://zaas.at/errors/service-unavailable`

**HTTP status:** `503 Service Unavailable`

## What it means

A required backing service (database, rate-limiter) is temporarily unavailable.
The `retry_after` field (when present) indicates how many seconds to wait before
retrying.

## Common causes

- The rate-limiter backend is unreachable and the server is configured to fail
  closed (authentication routes always fail closed for safety).
- The database is not configured for auth endpoints.

## How to fix

Wait the number of seconds indicated by `retry_after` (or `Retry-After`
header) and retry. If the problem persists, report it with the `request_id`.

## Example response

```json
{
  "type": "https://zaas.at/errors/service-unavailable",
  "title": "Service Unavailable",
  "status": 503,
  "detail": "Rate limiter temporarily unavailable. Please retry shortly.",
  "instance": "/api/v1/dice",
  "code": "SERVICE_UNAVAILABLE",
  "retry_after": 5,
  "request_id": "abc123"
}
```
