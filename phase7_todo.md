# Phase 7 Implementation Todo List

## Project: Bookwyrm — Recommendation Engine

---

## Phase 7 Objective

Use the metadata graph to deliver deterministic, explainable discovery recommendations.

---

## Deliverables

- Recommendation traversal engine over graph/store read models
- Weighted scoring model with reason evidence
- Endpoints for recommendations, similar works, and next-in-series
- Recommendation metrics and cache behavior
- API + engine contract tests

---

## Tasks

| # | Task | Status |
|---|---|---|
| 1 | Implement recommendation domain types (`request`, `result`, `reason`) | [x] |
| 2 | Implement scoring weights and normalization helpers | [x] |
| 3 | Implement bounded candidate traversal reads in store layer | [x] |
| 4 | Implement recommendation engine ranking + dedupe | [x] |
| 5 | Implement preference bias (`formats`, `languages` input shape) | [x] |
| 6 | Add recommendation cache keying + TTL behavior | [x] |
| 7 | Add observability metrics for rec requests/cache/latency | [x] |
| 8 | Add API route `GET /v1/work/{id}/recommendations` | [x] |
| 9 | Add API route `GET /v1/work/{id}/next` | [x] |
| 10 | Add API route `GET /v1/work/{id}/similar` | [x] |
| 11 | Add API contract tests for recommendation endpoints | [x] |
| 12 | Validate full suite `go test ./... -count=1` | [x] |

---

## Notes

- Scores are deterministic by stable sorting (score desc, work id asc).
- Returned results include explainability via `reasons`.
- Language preference input is accepted and included in cache key; practical boost depends on available stored metadata.
