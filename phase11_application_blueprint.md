# Phase 11 — Full Application Blueprint (AI Handoff)

## Project: Bookwyrm Application Layer

---

## Purpose

Define the implementation blueprint for building a full user-facing application on top of the completed metadata platform (Phases 1–10).

This document is intended to be handed to an AI coding/review agent as the source-of-truth scope for Phase 11.

---

## Current Baseline (Completed in this repo)

The metadata platform is already available with:

- Stable public API surface under `/v1`
- Optional API authentication (`X-API-Key` / Bearer)
- API rate limiting and version headers
- Metadata graph + recommendation engine
- Metadata quality audit/repair APIs
- Provider reliability + enrichment jobs
- Starter SDKs (`metadata-service/sdk/go`, `metadata-service/sdk/python`)

Reference docs:

- `README.md`
- `metadata-service/docs/api_v1.md`
- `phase9_todo.md`
- `phase10_todo.md`

---

## Phase 11 Goal

Build a complete application around the metadata service components, including:

- User-facing web UI
- Application backend/API facade
- Persistent app-domain data model
- Operational workflows (search, monitor, quality actions)
- End-to-end integration with `metadata-service`

---

## Scope Boundaries

### In Scope

1. Application architecture and project scaffolding
2. API facade layer for metadata-service integration
3. Core user workflows:
   - search/discovery
   - work detail and graph exploration
   - recommendations consumption
   - quality report and repair trigger
4. Monitoring/collection workflows (author/series/work watchlists)
5. Integration tests against live metadata-service
6. Deployment profile for local self-hosted stack

### Out of Scope (for initial Phase 11 delivery)

- Rewriting metadata-service internals
- Replacing current provider adapters
- Multi-tenant SaaS billing/organizations
- Native mobile apps
- Advanced ML ranking beyond existing recommendation APIs

---

## Recommended Target Architecture

## 1) Application Web UI

- Framework: modern SPA (React/Next.js or equivalent)
- Responsibility:
  - search UX
  - entity pages (author/work/series)
  - recommendation views
  - quality dashboard
  - watchlist management

## 2) Application Backend (BFF / API Facade)

- Responsibility:
  - auth/session (app users)
  - policy enforcement
  - orchestration across metadata-service calls
  - response shaping for UI
  - caching hot reads for UX responsiveness

## 3) Metadata Platform (existing)

- Keep as authoritative metadata backbone:
  - `/v1/search`
  - `/v1/work/{id}`
  - `/v1/work/{id}/recommendations`
  - `/v1/work/{id}/graph`
  - `/v1/quality/report`
  - `/v1/quality/repair`

## 4) App Database

- Store app-domain state only (not canonical metadata):
  - users
  - preferences
  - watchlists
  - saved searches
  - action history

---

## Canonical Integration Contract

Application should treat metadata-service as:

- Canonical source for bibliographic entities and graph data
- Eventual-consistency backend for enrichment/quality changes
- Stable v1 external dependency

Rules:

1. Do not duplicate canonical metadata tables in app DB.
2. Store only foreign references (`work_id`, `author_id`, `series_id`) in app DB.
3. Route all metadata mutations via metadata-service endpoints only.

---

## Primary User Workflows

1. Search and Explore
   - User enters query
   - App backend calls `/v1/search`
   - UI renders ranked works

2. Work Intelligence View
   - App backend fans out to:
     - `/v1/work/{id}`
     - `/v1/work/{id}/graph`
     - `/v1/work/{id}/recommendations`
   - UI renders metadata + related graph + recommendations

3. Quality Operations
   - Dashboard calls `/v1/quality/report`
   - User can execute dry-run and real repair via `/v1/quality/repair`
   - UI surfaces impact summaries and audit trail

4. Monitoring/Watchlists
   - User saves author/series/work targets
   - App job periodically queries metadata-service and stores changes/events
   - UI shows newly discovered items

---

## Suggested Repository Layout (Phase 11)

```
app/
  backend/
    cmd/
    internal/
      api/
      domain/
      integration/metadata/
      jobs/
      store/
  web/
    src/
      pages/
      components/
      features/
  docs/
    architecture.md
    api-contracts.md
```

---

## Incremental Delivery Plan

### Milestone A — Foundation

- Scaffold `app/backend` and `app/web`
- Add metadata-service client abstraction in backend
- Add health/status page proving end-to-end connectivity

### Milestone B — Core Read UX

- Implement search and work detail pages
- Integrate graph + recommendations
- Add backend response shaping and error handling

### Milestone C — Quality Dashboard

- Add quality report view
- Add dry-run repair action flow
- Add controlled production repair action flow

### Milestone D — Watchlists + Automation

- Add app DB + migrations
- Add watchlist entities and APIs
- Add periodic polling/enrichment jobs and event timeline

### Milestone E — Hardening

- End-to-end integration tests
- Observability/logging baselines
- Docker Compose profile for full stack

---

## Non-Functional Requirements

- All app-to-metadata calls include timeout + retry policy
- Strict typed DTO mapping at integration boundary
- Clear user-visible error categories (auth, rate-limit, upstream failure, validation)
- Idempotent job handlers for watchlist polling
- Structured logs with correlation IDs across app backend and metadata-service

---

## Acceptance Criteria (Phase 11)

1. User can search works and open a detailed work page.
2. Work page includes graph and recommendations data.
3. User can view metadata quality report and execute dry-run repair.
4. User can manage watchlists (create/list/delete).
5. Background job detects at least one new-item event for watched entities.
6. End-to-end tests cover search, work detail, quality report, and watchlist flow.
7. Full stack runs via documented local compose workflow.

---

## AI Agent Execution Checklist

Use this exact checklist when reviewing/implementing:

1. Confirm Phase 1–10 artifacts are intact and reused, not replaced.
2. Create app backend integration client for metadata-service first.
3. Implement vertical slices (API + backend + UI) one workflow at a time.
4. Add tests per slice before moving to next slice.
5. Keep app-domain data separate from canonical metadata records.
6. Validate all quality actions in dry-run mode before enabling destructive actions.
7. Document every added endpoint and environment variable.

---

## Handoff Prompt (for downstream AI agent)

You are implementing **Phase 11** for this repository.

Context:
- Phases 1–10 are complete.
- `metadata-service` is the canonical metadata platform.
- Your task is to build an application layer around it, not rewrite it.

Primary objective:
- Deliver a working app backend + web UI that integrates with existing `metadata-service` endpoints for search, work detail, recommendations, quality reporting, and watchlist workflows.

Constraints:
- Preserve existing metadata-service behavior and public API contract.
- Store only app-domain entities in new app persistence.
- Implement incrementally by milestone and keep each milestone testable.

Definition of done:
- Meet all acceptance criteria in this document.
- Produce documentation for setup, architecture, and API boundaries.

---

## Notes for Human Review

- This blueprint intentionally focuses on execution order and integration boundaries.
- Keep metadata-service as a platform dependency to avoid architecture drift.
- If the app eventually becomes multi-service, preserve the backend facade contract to avoid coupling UI directly to metadata internals.
