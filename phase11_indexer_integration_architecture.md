# Phase 11 Companion — Indexer Integration Architecture (AI Handoff)

## Purpose

Define how to extend Bookwyrm with a separate `IndexerService` that enriches canonical metadata with external availability signals, while keeping `metadata-service` authoritative for bibliographic data.

This document complements:

- `phase11_application_blueprint.md`
- `metadata-service/docs/api_v1.md`

---

## Design Principles

1. `metadata-service` remains canonical for work/edition/graph metadata.
2. `IndexerService` is a separate bounded context for source discovery.
3. Integration is contract-first (versioned DTOs, explicit provenance).
4. Source adapters are pluggable and policy-controlled.
5. Compliance and terms-of-service controls are mandatory.

---

## High-Level Architecture

```
App Backend (BFF)
  ├─ calls metadata-service (/v1) for canonical entities
  ├─ calls indexer-service (/v1/indexer/*) for availability signals
  └─ merges results for UI

metadata-service (existing)
  └─ authoritative metadata graph + quality + recommendations

indexer-service (new)
  ├─ query planner
  ├─ adapter registry (source backends)
  ├─ matcher/scorer
  ├─ caching + rate limiter
  └─ provenance/event store
```

---

## Contract Model (Source-Agnostic)

## 1) Metadata Snapshot (input)

A normalized metadata snapshot from Bookwyrm domain:

- `work_id`
- `edition_id` (optional)
- `isbn_10` / `isbn_13` (optional)
- `title`
- `authors[]`
- `language` (optional)
- `publication_year` (optional)

## 2) Indexer Query

- `metadata_snapshot`
- `requested_capabilities[]` (e.g., `availability`, `files`, `news`)
- `priority` (`speed` | `completeness` | `recency`)
- `policy_profile` (which adapters are allowed)

## 3) Indexer Result

- `work_id`
- `source`
- `found`
- `candidates[]`
- `searched_at`
- `trace` (adapter timing/errors for observability)

## 4) Candidate

- `candidate_id`
- `title`
- `format`
- `size_bytes` (optional)
- `added_at` (optional)
- `provider_link` (optional)
- `match_confidence` (0–1)
- `provenance` (adapter + source identifier)

---

## Service Boundaries and Responsibilities

## `metadata-service` (already built)

- Canonical metadata resolution and graph intelligence
- Recommendations and quality engine
- No source-specific scraping orchestration

## `indexer-service` (new)

- External source querying through adapters
- Normalization + scoring of candidate signals
- Adapter health, retries, and circuit-breaking
- Caching and per-adapter rate limits

## App Backend (BFF)

- User auth/session
- Calls both services and composes responses
- Applies product policy on what is surfaced

---

## API Surface (IndexerService)

Suggested endpoints:

- `POST /v1/indexer/search`
  - Body: `IndexerQuery`
  - Returns: `IndexerResult`

- `POST /v1/indexer/batch-search`
  - Body: list of `IndexerQuery`
  - Returns: async job handle or streamed results

- `GET /v1/indexer/jobs/{id}`
  - Returns job status + partial/final results

- `GET /v1/indexer/providers`
  - Adapter availability + health + policy status

- `GET /v1/indexer/health`
  - Service health and dependency checks

---

## Adapter Interface (Go-first)

```go
type Adapter interface {
    Name() string
    Capabilities() []string
    Search(ctx context.Context, query IndexerQuery) (IndexerResult, error)
    HealthCheck(ctx context.Context) error
}
```

Implementation notes:

- Keep adapters stateless where possible.
- Share an HTTP client with timeouts and retry budget.
- Apply per-adapter limiter and fallback policy.

---

## Matching and Scoring Strategy

Weighted scoring baseline:

- Identifier exact match (ISBN): high weight
- Title similarity (normalized + token overlap): medium weight
- Author overlap: medium weight
- Year proximity: low-to-medium weight
- Language/format match: low weight

Output:

- `match_confidence` with reason codes (`identifier_exact`, `title_fuzzy`, etc.)

---

## Reliability and Performance

1. Cache search results by query fingerprint + policy profile.
2. Use short request deadlines and bounded retries.
3. Track adapter SLIs:
   - latency
   - success rate
   - empty-result rate
4. Quarantine unstable adapters automatically.
5. Support partial results if one adapter fails.

---

## Security and Compliance Controls

1. Policy-gated adapter enablement in config.
2. Per-adapter `User-Agent`, timeout, and rate-limit settings.
3. Explicit source provenance in every candidate.
4. Audit logging for outbound queries and decision path.
5. Respect legal/compliance requirements and terms of use for each source.

---

## Data Persistence (IndexerService)

Recommended tables:

- `indexer_jobs`
- `indexer_job_events`
- `indexer_candidates`
- `indexer_provider_status`
- `indexer_cache_entries`

Keep these separate from metadata-service canonical tables.

---

## Integration Sequence (App Request)

1. App receives search/intelligence request.
2. App backend resolves canonical metadata via `metadata-service`.
3. App backend sends normalized snapshot to `indexer-service`.
4. App backend merges and ranks final response.
5. UI displays canonical metadata + indexed availability signals with provenance.

---

## Delivery Plan for AI Agent

### Milestone 1 — Contract + Skeleton

- Create `indexer-service` module scaffold
- Implement DTOs and OpenAPI contract
- Add adapter registry and health endpoint

### Milestone 2 — Core Search Path

- Implement `POST /v1/indexer/search`
- Add one adapter stub + matcher/scorer
- Add cache and basic rate limiter

### Milestone 3 — Batch + Jobs

- Add async batch-search job model
- Add job status endpoint and event log

### Milestone 4 — Hardening

- Add integration tests with app backend and metadata-service
- Add observability dashboards and alert thresholds
- Add policy toggles and compliance checks

---

## Acceptance Criteria

1. App backend can query indexer-service using canonical metadata snapshots.
2. Indexer returns normalized candidate records with confidence and provenance.
3. Adapter failures do not break end-to-end response (partial results allowed).
4. Per-adapter rate limits and policy toggles are enforced.
5. Integration tests validate metadata-service + indexer-service orchestration.

---

## Handoff Prompt for Downstream AI Agent

Implement a new `indexer-service` for Bookwyrm as a separate bounded context.

Constraints:

- Do not modify canonical metadata responsibilities in `metadata-service`.
- Build contract-first (`/v1/indexer/*`) with typed DTOs and tests.
- Keep adapters pluggable and policy-gated.
- Include reliability controls (timeouts, retries, rate limits, cache).
- Ensure results include confidence and provenance fields.

Definition of done:

- Meets acceptance criteria in this document.
- Includes setup docs, configuration docs, and integration tests.
