# INVALID_PARAM

**Type URI:** `https://zaas.at/errors/invalid-param`

**HTTP status:** `400 Bad Request`

## What it means

A query parameter or request body field did not pass validation. The `detail`
field in the problem response explains which parameter failed and why.

## Common causes

- A parameter value is outside its allowed range (e.g., `count=200` when the
  maximum is 100, `sides=7` when only 4, 6, 8, 10, 12, 20, and 100 are valid).
- A required field is missing from a request body.
- A field value does not match the expected type or format.

## How to fix

Read the `detail` field of the response for the specific constraint that was
violated, then correct the parameter value and retry.

## Example response

```json
{
  "type": "https://zaas.at/errors/invalid-param",
  "title": "Bad Request",
  "status": 400,
  "detail": "sides must be one of 4,6,8,10,12,20,100",
  "instance": "/api/v1/dice",
  "code": "INVALID_PARAM",
  "request_id": "abc123"
}
```
