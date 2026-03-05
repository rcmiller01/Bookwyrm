# Bookwyrm Public API v1

This document describes the stable public contract for `v1` endpoints.

## Versioning

- Base path: `/v1`
- Response header: `X-Bookwyrm-API-Version: v1`

## Authentication

When enabled in configuration, clients must provide one of:

- `X-API-Key: <key>`
- `Authorization: Bearer <key>`

Unauthenticated requests return `401` with JSON error body.

## Rate limiting

When enabled, requests are limited per client identity (API key when present, otherwise source IP).

Response headers:

- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `Retry-After` (on `429`)

## Core endpoints

- `GET /v1/search?q=<query>`
- `GET /v1/work/{id}`
- `GET /v1/work/{id}/recommendations`
- `GET /v1/work/{id}/next`
- `GET /v1/work/{id}/similar`
- `GET /v1/work/{id}/graph`
- `GET /v1/quality/report`
- `POST /v1/quality/repair`
- `GET /v1/providers`
- `GET /v1/providers/reliability`

## Quality repair request

`POST /v1/quality/repair`

```json
{
  "dry_run": true,
  "limit": 25,
  "remove_invalid_identifiers": true
}
```

## Error format

Errors use a consistent JSON structure:

```json
{
  "error": "human-readable message"
}
```
