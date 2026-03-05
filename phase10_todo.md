# Phase 10 Implementation Todo List

## Project: Bookwyrm — Metadata Service Platform

---

## Phase 10 Objective

Turn Bookwyrm into a general-purpose metadata platform with a stable public API, authentication, rate limiting, client SDKs, and integration documentation.

---

## Deliverables

- Stable public API contract for `/v1`
- API authentication middleware
- API rate-limiting middleware
- Configurable API platform settings
- Starter Go and Python SDK clients
- Public API documentation for integrations

---

## Tasks

| # | Task | Status |
|---|---|---|
| 1 | Add API config block (`auth`, `rate_limit`) to config model and defaults | [x] |
| 2 | Add environment variable overrides for API platform settings | [x] |
| 3 | Add API version middleware for stable v1 contract header | [x] |
| 4 | Add API authentication middleware (`X-API-Key` / Bearer) | [x] |
| 5 | Add API rate limiting middleware with response headers and 429 behavior | [x] |
| 6 | Wire middleware into `/v1` router construction with options | [x] |
| 7 | Add middleware tests for auth, rate limit, and version header | [x] |
| 8 | Add public API contract doc (`metadata-service/docs/api_v1.md`) | [x] |
| 9 | Add starter SDKs (`sdk/go`, `sdk/python`) and SDK README | [x] |
| 10 | Update top-level README for Phase 10 usage/config | [x] |
| 11 | Validate full suite `go test ./... -count=1` | [ ] |

---

## Notes

- Authentication is optional and disabled by default to preserve local self-hosted developer flow.
- Rate limiting is enabled by default and applied to all `/v1` endpoints.
- SDKs are intentionally minimal bootstrap clients and can be expanded later.
