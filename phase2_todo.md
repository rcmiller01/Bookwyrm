# Phase 2 Implementation Todo List

## Project: Bookwyrm — Multi-Provider System

**Goal:** Introduce redundancy and improve metadata coverage. The resolver works even if one provider fails.

---

## Prerequisites

- [ ] Validate Phase 1 success criteria (deferred)
- [ ] Phase 1 service running stably

---

## Database

- [ ] Write migration: `provider_configs` table (provider name, enabled, priority, timeout, rate limit)
- [ ] Write migration: `provider_status` table (provider name, status, last_checked, failure_count)

---

## Provider System

- [ ] Implement `ProviderConfigStore` interface and PostgreSQL implementation
- [ ] Implement `ProviderStatusStore` interface and PostgreSQL implementation
- [ ] Extend `Registry` to load provider config from database at startup
- [ ] Add provider priority ordering to `Registry.EnabledProviders()`
- [ ] Implement provider rate limiter (token bucket per provider)
- [ ] Implement provider health monitor (background ping / failure tracking)

---

## New Provider Adapters

### Google Books

- [ ] Implement `internal/provider/googlebooks/provider.go`
- [ ] Map `SearchWorks` using Google Books Volumes API (`https://www.googleapis.com/books/v1/volumes?q=`)
- [ ] Map `ResolveIdentifier` using ISBN lookup (`?q=isbn:`)
- [ ] Map fields: title, authors, publisher, publication year, ISBN identifiers
- [ ] Add Google Books to provider registration in `main.go`

### Hardcover

- [ ] Research Hardcover API availability and authentication
- [ ] Implement `internal/provider/hardcover/provider.go`
- [ ] Map canonical fields to Hardcover response schema
- [ ] Add Hardcover to provider registration in `main.go`

---

## Provider Configuration API

- [ ] Add `GET /v1/providers` — list all registered providers and their status
- [ ] Add `POST /v1/providers` — create/update provider configuration
- [ ] Add `POST /v1/providers/{id}/test` — trigger a test query against a specific provider
- [ ] Add provider API handlers to `internal/api/handlers.go`
- [ ] Add provider routes to `internal/api/router.go`
- [ ] Add provider API types to `internal/api/types.go`

---

## Resolver Improvements

- [ ] Update resolver to respect provider priority ordering during dispatch
- [ ] Add per-provider timeout enforcement in concurrent dispatch goroutines
- [ ] Add provider failure fallback: if all providers fail, return cached/DB result
- [ ] Track provider success/failure counts after each query

---

## Observability

- [ ] Add `provider_requests_total` Prometheus counter (label: provider name)
- [ ] Add `provider_failures_total` Prometheus counter (label: provider name)
- [ ] Add `provider_latency_ms` Prometheus histogram (label: provider name)
- [ ] Expose provider health status via `GET /v1/providers`

---

## Testing

- [ ] Write unit tests for provider registry priority ordering
- [ ] Write unit tests for rate limiter
- [ ] Write integration test: resolver falls back correctly when one provider returns error
- [ ] Write integration test: `GET /v1/providers` returns correct provider list

---

## Phase 2 Success Criteria

- [ ] Resolver returns results when Open Library is intentionally disabled
- [ ] Google Books and Hardcover providers integrated and returning canonical metadata
- [ ] Provider health monitoring updates `provider_status` table
- [ ] Rate limiting prevents exceeding provider API limits
- [ ] `GET /v1/providers` returns live provider status

---

## Reference Documents

- [PRD.md](PRD.md) — Phase 2 section
- [implementation_plan.md](implementation_plan.md) — Phase 2 Multi-Provider System
- [Metadata_Service_Query_Flow.md](Metadata_Service_Query_Flow.md) — Provider Dispatcher pattern
- [phase1_todo.md](phase1_todo.md)
