# RATE_LIMITED

**Type URI:** `https://zaas.at/errors/rate-limited`

**HTTP status:** `429 Too Many Requests`

## What it means

The client has exceeded the allowed request rate for the current time window.
The `retry_after` field contains the number of seconds to wait before retrying.
The `Retry-After` HTTP response header carries the same value.

## Rate limits

- **Anonymous requests** are rate-limited per IP address.
- **Authenticated requests** (with `Authorization: Bearer <api_key>`) are
  rate-limited per API key with a higher default limit.

Current limits are returned in the `X-RateLimit-Limit`,
`X-RateLimit-Remaining`, and `X-RateLimit-Reset` response headers on every
request.

## How to fix

Wait `retry_after` seconds before sending the next request. To increase your
limit, email contact@zaas.at to request a free API key.

## Example response

```json
{
  "type": "https://zaas.at/errors/rate-limited",
  "title": "Too Many Requests",
  "status": 429,
  "detail": "Rate limit exceeded (60 req/min). Email contact@zaas.at to request a free API key for higher limits.",
  "instance": "/api/v1/dice",
  "code": "RATE_LIMITED",
  "retry_after": 42,
  "request_id": "abc123"
}
```
