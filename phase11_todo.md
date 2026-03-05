# Phase 11 Implementation Todo List

## Project: Bookwyrm — Application + Multi-Indexer Orchestration

---

## Objective

Implement an application backend and indexer-service layer that can run concurrent backend groups (`prowlarr` and `non_prowlarr`) without modifying or regressing existing metadata-service behavior.

---

## Completed Deliverables

- App backend module scaffold (`app/backend`)
- App workflow APIs for search, intelligence, quality, watchlists, and availability fan-out
- Indexer-service scaffold (`indexer-service`) with health/providers/search endpoints
- Concurrent multi-group search orchestration and deterministic merge/ranking
- Integration tests for app backend and indexer-service modules
- Documentation for app backend and indexer-service setup

---

## Validation

- Existing metadata-service regression suite still passes.
- New module tests pass for `app/backend` and `indexer-service`.

---

## Notes

- `metadata-service` remains canonical for bibliographic metadata.
- App backend composes metadata-service + indexer-service outputs.
- Indexer groups can be selected per request via `groups` query (default both groups).
