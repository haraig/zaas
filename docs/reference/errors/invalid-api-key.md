# INVALID_API_KEY

**Type URI:** `https://zaas.at/errors/invalid-api-key`

**HTTP status:** `401 Unauthorized`

## What it means

The `Authorization: Bearer <api_key>` header was present but the key is
invalid, revoked, or not yet verified.

Anonymous requests (no `Authorization` header) are not rejected with this
error. This error only occurs when a key is explicitly provided.

## Common causes

- The key was typed or copied incorrectly.
- The key was revoked (e.g., a re-issue was requested).
- The email address was never verified after registration.

## How to fix

- Double-check that the key matches exactly what was received in the
  verification email.
- If the key has been revoked, email contact@zaas.at to request a re-issue.
- To use the API without a key (at lower rate limits), omit the
  `Authorization` header entirely.

## Example response

```json
{
  "type": "https://zaas.at/errors/invalid-api-key",
  "title": "Unauthorized",
  "status": 401,
  "detail": "Invalid or revoked API key. Email contact@zaas.at to request a reissue.",
  "instance": "/api/v1/uuid",
  "code": "INVALID_API_KEY",
  "request_id": "abc123"
}
```
