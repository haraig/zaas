# INTERNAL_ERROR

**Type URI:** `https://zaas.at/errors/internal-error`

**HTTP status:** `500 Internal Server Error`

## What it means

An unexpected server-side error occurred while processing the request. The
error is not caused by the request itself.

## How to fix

Retry the request. If the problem persists, check the ZaaS status page or
report the `request_id` to hello@zaas.at.

## Example response

```json
{
  "type": "https://zaas.at/errors/internal-error",
  "title": "Internal Server Error",
  "status": 500,
  "detail": "Registration failed",
  "instance": "/api/v1/auth/register",
  "code": "INTERNAL_ERROR",
  "request_id": "abc123"
}
```
