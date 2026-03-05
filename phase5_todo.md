# Phase 5 Implementation Todo List

## Project: Bookwyrm — Metadata Graph Layer

---

## Phase 5 Objective

Implement a durable, queryable metadata graph in PostgreSQL representing:

- Works ↔ Series (ordered entries)
- Works ↔ Subjects (tags/topics)
- Works ↔ Works (derived/explicit relationships)

This phase must:

- integrate cleanly with Phase 4 enrichment and Phase 1/2 ingest paths
- remain provider-agnostic (graph derived from canonical metadata)
- avoid unbounded fanout
- preserve identity stability and idempotency

---

## Deliverables

- Graph schema migrations
- Graph stores (series, subjects, work relationships)
- Deterministic graph builder with fanout caps
- `graph_update_work` enrichment job + handler
- Graph query APIs
- Graph metrics and logs
- Tests across stores, builder, enrichment wiring, and API contracts

---

## Tasks

| # | Task | Status |
|---|---|---|
| 1 | Extend canonical work model with optional graph fields (`SeriesName`, `SeriesIndex`, `Subjects`) | [x] |
| 2 | Add normalization helpers (`NormalizeSubject`, `NormalizeSeriesName`) in shared module | [x] |
| 3 | Create migration `000005_metadata_graph.up.sql` | [x] |
| 4 | Create migration `000005_metadata_graph.down.sql` | [x] |
| 5 | Implement series store (`internal/store/series.go`) | [x] |
| 6 | Implement subject store (`internal/store/subjects.go`) | [x] |
| 7 | Implement work relationship store (`internal/store/work_relationships.go`) | [x] |
| 8 | Implement graph builder core (`internal/graph/builder.go`) | [x] |
| 9 | Enforce deterministic graph update algorithm (`UpdateGraphForWork`) | [x] |
| 10 | Enforce fanout caps (author/series/subject) in builder | [x] |
| 11 | Implement relationship provenance + confidence rules | [x] |
| 12 | Add enrichment job type constant `graph_update_work` | [x] |
| 13 | Add enrichment handler `internal/enrichment/handlers/graph_update_work.go` | [x] |
| 14 | Wire enqueue from resolver ingest path (deduped) | [x] |
| 15 | Wire enqueue from `author_expand` for newly ingested works (bounded) | [x] |
| 16 | Add API endpoint `GET /v1/work/{id}/graph` | [x] |
| 17 | Add API endpoint `GET /v1/series/{id}` | [x] |
| 18 | Add API endpoint `GET /v1/subjects/{id}/works?limit=50` | [x] |
| 19 | Add API endpoint `GET /v1/graph/stats` | [x] |
| 20 | Add graph metrics in `internal/metrics` | [x] |
| 21 | Add structured logging in `graph_update_work` handler | [x] |
| 22 | Add store tests (series/subjects/relationships) | [x] |
| 23 | Add graph builder tests (caps, idempotency, deterministic edges) | [x] |
| 24 | Add enrichment handler tests for `graph_update_work` | [x] |
| 25 | Add API contract tests for graph endpoints | [x] |
| 26 | Validate full suite `go test ./... -count=1` | [x] |

---

## Canonical Model Extensions (Non-Breaking)

Update work model to include optional graph-relevant fields when providers supply them:

- `SeriesName *string`
- `SeriesIndex *float64`
- `Subjects []string`
- `RelatedProviderIDs []string` (optional; can remain deferred in Phase 5)

Notes:

- Providers are not required to populate all graph fields.
- Merge must tolerate sparse/missing graph metadata.

---

## Normalization Helpers

Add shared helpers in `internal/normalize/normalize.go` (or existing normalize module):

- `NormalizeSubject(string) string`
- `NormalizeSeriesName(string) string`

Goals:

- deterministic keys
- stable upserts
- case/spacing/punctuation normalization consistency

---

## Database Migrations

Create:

- `migrations/000005_metadata_graph.up.sql`
- `migrations/000005_metadata_graph.down.sql`

### `series` + `series_entries`

```sql
CREATE TABLE series (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  normalized_name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_series_normalized
ON series(normalized_name);

CREATE TABLE series_entries (
  series_id TEXT NOT NULL REFERENCES series(id) ON DELETE CASCADE,
  work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  series_index DOUBLE PRECISION NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  PRIMARY KEY(series_id, work_id)
);

CREATE INDEX idx_series_entries_series
ON series_entries(series_id, series_index);

CREATE INDEX idx_series_entries_work
ON series_entries(work_id);
```

### `subjects` + `work_subjects`

```sql
CREATE TABLE subjects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  normalized_name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_subjects_normalized
ON subjects(normalized_name);

CREATE TABLE work_subjects (
  work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  subject_id TEXT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  PRIMARY KEY(work_id, subject_id)
);

CREATE INDEX idx_work_subjects_subject
ON work_subjects(subject_id);
```

### `work_relationships`

```sql
CREATE TABLE work_relationships (
  source_work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  target_work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  relationship_type TEXT NOT NULL,
  confidence DOUBLE PRECISION NOT NULL DEFAULT 0.5,
  provider TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  PRIMARY KEY(source_work_id, target_work_id, relationship_type)
);

CREATE INDEX idx_workrels_source
ON work_relationships(source_work_id);

CREATE INDEX idx_workrels_target
ON work_relationships(target_work_id);
```

---

## Store Layer

Create:

- `internal/store/series.go`
- `internal/store/subjects.go`
- `internal/store/work_relationships.go`

### SeriesStore

- `UpsertSeries(name, normalized) (seriesID)`
- `UpsertSeriesEntry(seriesID, workID, seriesIndex)`
- `GetSeriesByID(id)`
- `GetSeriesEntries(seriesID)`
- `GetSeriesForWork(workID)` (optional but useful)

### SubjectStore

- `UpsertSubject(name, normalized) (subjectID)`
- `SetWorkSubjects(workID, subjectIDs)` (replace mode; idempotent)
- `GetSubjectsForWork(workID)`
- `GetWorksForSubject(subjectID, limit, offset)`

### WorkRelationshipStore

- `UpsertRelationship(sourceID, targetID, type, confidence, provider)`
- `GetRelatedWorks(sourceID, type?, limit)`
- `DeleteRelationshipsForWork(sourceID, type?)` (for rebuilds)

---

## Graph Builder

Create `internal/graph/builder.go` with primary function:

- `UpdateGraphForWork(ctx, workID string) error`

Responsibilities:

1. Load canonical work + linked author data + graph fields.
2. Upsert series + series entry when series data exists.
3. Upsert subjects + set work-subject associations.
4. Derive bounded relationships:
   - `same_author`
   - `same_series`
   - `related_subject` (optional/low confidence)

### Fanout Caps (Required)

- `same_author`: top 25 works per author
- `same_series`: immediate neighbors by `series_index` and/or top 10
- `related_subject`: top 10 works per shared subject

### Confidence Rules (Derived Edges)

- `same_series` neighbor: `0.9`
- `same_author`: `0.7`
- `related_subject`: `0.5`

Provenance for derived edges:

- `provider = NULL`

---

## Enrichment Wiring

Preferred approach: enqueue graph updates (do not block interactive path).

### New job type

- Add constant: `graph_update_work`

### New handler

- `internal/enrichment/handlers/graph_update_work.go`
- Behavior: call `graph.UpdateGraphForWork(workID)` and return result

### Enqueue points

- Resolver ingest/persist success path: enqueue `graph_update_work` for persisted work (deduped)
- `author_expand`: enqueue `graph_update_work` for newly ingested works, capped per job

Note: `work_editions` does not need to trigger graph updates in Phase 5 unless work-level graph metadata changed.

---

## API Endpoints

- `GET /v1/work/{id}/graph`
  - series (if any), series neighbors, subjects, related works grouped by relationship type
- `GET /v1/series/{id}`
  - series metadata + ordered works
- `GET /v1/subjects/{id}/works?limit=50`
  - works attached to subject
- `GET /v1/graph/stats`
  - counts: series, subjects, relationships by type

---

## Observability

Add metrics:

- `graph_updates_total`
- `graph_update_failures_total`
- `graph_relationships_created_total{relationship_type}`
- `graph_queue_depth` (optional; enrichment queue may be sufficient)

Add structured logs in graph update handler:

- `work_id`
- `series_id` (if available)
- `subject_count`
- `relationships_added`

---

## Tests (Must-Have)

### Store tests

- series upsert/idempotency
- subject upsert + replace semantics for `SetWorkSubjects`
- relationship upsert/query dedupe behavior

### Graph builder tests

- series metadata creates series + entry
- subjects metadata creates subject nodes + links
- caps enforced
- same-series neighbor behavior
- idempotency under repeated `UpdateGraphForWork`

### Enrichment tests

- `graph_update_work` enqueue + execute
- dedupe behavior under repeated schedule

### API tests

- `/v1/work/{id}/graph` contract
- `/v1/series/{id}` ordered entries
- `/v1/graph/stats` stable shape

---

## Phase 5 Success Criteria

### Functional

- Ingest + enrichment produce graph data (`series`, `series_entries`, `subjects`, `work_subjects`, `work_relationships`)
- `graph_update_work` is enqueued reliably and deduped
- Graph endpoints return expected structures for operators/clients

### Safety & Performance

- No unbounded graph fanout (caps proven by tests)
- Graph updates are idempotent (no relationship count inflation)
- Graph layer remains provider-agnostic and canonical-first

### Observability

- Operators can track graph update success/failure and relationship growth
- `/v1/graph/stats` reflects graph growth over time

### Quality

- `go test ./... -count=1` passes

---

## Recommended Implementation Order (Lowest Risk)

1. Migrations + stores
2. Graph builder core (`UpdateGraphForWork`)
3. `graph_update_work` job + handler
4. Enqueue wiring (resolver + author_expand)
5. API endpoints
6. Metrics + full tests

---

## Reference Documents

- [PRD.md](PRD.md)
- [implementation_plan.md](implementation_plan.md)
- [Metadata_Service_Query_Flow.md](Metadata_Service_Query_Flow.md)
- [architecture_diagram.md](architecture_diagram.md)
