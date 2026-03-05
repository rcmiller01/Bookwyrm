# Phase 9 Implementation Todo List

## Project: Bookwyrm — Metadata Quality Engine

---

## Phase 9 Objective

Detect and repair metadata inconsistencies across graph relationships, publication years, duplicate editions, and identifiers.

---

## Deliverables

- Quality engine package with audit + repair flows
- Graph anomaly detection for malformed series ordering/indexes
- Conflicting publication year detection
- Duplicate edition detection
- Identifier verification (ISBN-10 / ISBN-13 checksum + format)
- Quality API endpoints for reporting and repair execution

---

## Tasks

| # | Task | Status |
|---|---|---|
| 1 | Add `internal/quality` engine, repository, and types | [x] |
| 2 | Implement series anomaly detection queries | [x] |
| 3 | Implement publication year conflict detection queries | [x] |
| 4 | Implement duplicate edition detection queries | [x] |
| 5 | Implement identifier verification logic (ISBN-10/13) | [x] |
| 6 | Implement repair actions (series reorder, first pub year sync, invalid identifier removal) | [x] |
| 7 | Add endpoint `GET /v1/quality/report` | [x] |
| 8 | Add endpoint `POST /v1/quality/repair` | [x] |
| 9 | Add unit tests for quality engine + identifier verification | [x] |
| 10 | Validate full suite `go test ./... -count=1` | [ ] |

---

## Notes

- Repair endpoint supports dry-run mode (`dry_run=true`) to preview changes before mutation.
- Duplicate edition detection is currently report-only; automatic merges are intentionally deferred.
- Invalid identifier repair targets malformed ISBN records and can be toggled via request payload.
