# Phase 2 Implementation Todo List

## Project: Bookwyrm — Multi-Provider System

**Goal:** Introduce redundancy and improve metadata coverage. The resolver works even if one provider fails.

---

## Prerequisites

- [ ] Validate Phase 1 success criteria (deferred — requires live DB)
- [ ] Phase 1 service running stably

---

## Database

- [x] Write migration: `provider_configs` table (provider name, enabled, priority, timeout, rate limit)
- [x] Write migration: `provider_status` table (provider name, status, last_checked, failure_count)

---

## Provider System

- [x] Implement `ProviderConfigStore` interface and PostgreSQL implementation
- [x] Implement `ProviderStatusStore` interface and PostgreSQL implementation
- [x] Extend `Registry` to load provider config from database at startup
- [x] Add provider priority ordering to `Registry.EnabledProviders()`
- [x] Implement provider rate limiter (token bucket per provider)
- [x] Implement provider health monitor (background ping / failure tracking)

---

## New Provider Adapters

### Google Books

- [x] Implement `internal/provider/googlebooks/provider.go`
- [x] Map `SearchWorks` using Google Books Volumes API (`https://www.googleapis.com/books/v1/volumes?q=`)
- [x] Map `ResolveIdentifier` using ISBN lookup (`?q=isbn:`)
- [x] Map fields: title, authors, publisher, publication year, ISBN identifiers
- [x] Add Google Books to provider registration in `main.go`

### Hardcover

- [x] Research Hardcover API availability and authentication
- [x] Implement `internal/provider/hardcover/provider.go`
- [x] Map canonical fields to Hardcover response schema
- [x] Add Hardcover to provider registration in `main.go`

---

## Provider Configuration API

- [x] Add `GET /v1/providers` — list all registered providers and their status
- [x] Add `POST /v1/providers/{name}` — create/update provider configuration
- [x] Add `POST /v1/providers/{name}/test` — trigger a test query against a specific provider
- [x] Add provider API handlers to `internal/api/handlers.go`
- [x] Add provider routes to `internal/api/router.go`
- [x] Add provider API types to `internal/api/types.go`

---

## Resolver Improvements

- [x] Update resolver to respect provider priority ordering during dispatch
- [x] Add per-provider timeout enforcement in concurrent dispatch goroutines
- [x] Add provider failure fallback: if all providers fail, return cached/DB result
- [x] Track provider success/failure counts after each query

---

## Observability

- [x] Add `provider_requests_total` Prometheus counter (label: provider name)
- [x] Add `provider_failures_total` Prometheus counter (label: provider name)
- [x] Add `provider_latency_ms` Prometheus histogram (label: provider name)
- [x] Expose provider health status via `GET /v1/providers`

---

## Testing

- [x] Write unit tests for provider registry priority ordering
- [x] Write unit tests for rate limiter
- [x] Write integration test: resolver falls back correctly when one provider returns error
- [ ] Write integration test: `GET /v1/providers` returns correct provider list

---

## Phase 2 Success Criteria

- [ ] Resolver returns results when Open Library is intentionally disabled
- [ ] Google Books and Hardcover providers integrated and returning canonical metadata
- [ ] Provider health monitoring updates `provider_status` table
- [ ] Rate limiting prevents exceeding provider API limits
- [ ] `GET /v1/providers` returns live provider status

---

## Commit

- [x] `feat: Phase 2 multi-provider system` — committed `4f165b4` (16 files, +1161/-46)

## Reference Documents

- [PRD.md](PRD.md) — Phase 2 section
- [implementation_plan.md](implementation_plan.md) — Phase 2 Multi-Provider System
- [Metadata_Service_Query_Flow.md](Metadata_Service_Query_Flow.md) — Provider Dispatcher pattern
- [phase1_todo.md](phase1_todo.md)
