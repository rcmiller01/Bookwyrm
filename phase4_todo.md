# Phase 4 Implementation Todo List

## Project: Bookwyrm — Enrichment & Prefetch Engine

---

## Phase 4 Objective

Add a background enrichment system that:

- Queues enrichment jobs in PostgreSQL
- Processes jobs with worker(s) inside the same Go binary
- Expands metadata (authors, series, editions, identifiers) over time
- Is preference-driven and provider-policy-aware
- Never blocks the API request path
- Provides visibility (status + metrics + job history)

---

## Phase 4 Deliverables

- Job queue + worker runtime
- Enrichment job types (at least `AuthorExpansion` + `WorkEditions`)
- Preference-driven scheduling (formats/languages + limits)
- Provider-policy-aware execution (tier ordering, quarantine policy, timeouts, rate limits)
- Observability (API endpoints + Prometheus metrics)
- Strong tests (unit + integration style) and clear success criteria

---

## 0 — Define Phase 4 Scope Boundaries

### Non-goals (Phase 4)

- No recommendation engine yet
- No full graph traversal APIs yet (that’s Phase 5/6)
- No Anna’s Archive integration required in Phase 4 (allowed later)
- No UI required (API only)

### Safety Requirements

- Must not hammer providers
- Must cap fanout per author/series/work
- Must pause/slow when providers degrade (via reliability tiers)

---

## Tasks

| # | Task | Status |
|---|---|---|
| 1 | Create migrations: `000004_enrichment_jobs.up.sql` + `.down.sql` | [x] |
| 2 | Implement queue schema + indexes + run history tables | [x] |
| 3 | Implement `SELECT ... FOR UPDATE SKIP LOCKED` locking flow | [x] |
| 4 | Add enrichment models in `internal/model/enrichment.go` | [x] |
| 5 | Add enrichment store interface + pgx implementation (`internal/store/enrichment_jobs.go`) | [x] |
| 6 | Implement deduped enqueue behavior (queued/running no-op on duplicate) | [x] |
| 7 | Implement retry strategy (exponential backoff + jitter, capped) | [x] |
| 8 | Implement dead-letter transition when `attempt_count >= max_attempts` | [x] |
| 9 | Create worker runtime (`internal/enrichment/worker.go`) | [x] |
| 10 | Create enrichment engine (`internal/enrichment/engine.go`) with configurable worker count | [x] |
| 11 | Add graceful shutdown semantics for in-flight jobs | [x] |
| 12 | Add job handler registry (`internal/enrichment/handlers/handlers.go`) | [x] |
| 13 | Implement `work_editions` handler (`internal/enrichment/handlers/work_editions.go`) | [x] |
| 14 | Implement `author_expand` handler (`internal/enrichment/handlers/author_expand.go`) | [x] |
| 15 | Wire resolver enqueue hook after successful merge+persist path | [x] |
| 16 | Add enqueue rules for search results and identifier resolution | [x] |
| 17 | Add enrichment config parsing (`enabled`, workers, limits, preferences) | [x] |
| 18 | Apply preference-aware edition fetch/storage prioritization | [ ] |
| 19 | Ensure handlers use policy-aware provider ordering via registry | [x] |
| 20 | Ensure per-provider timeout + rate limiter are respected in enrichment | [x] |
| 21 | Add enrichment Prometheus metrics (`internal/metrics/enrichment_metrics.go`) | [x] |
| 22 | Add API endpoint: `GET /v1/enrichment/jobs` | [x] |
| 23 | Add API endpoint: `GET /v1/enrichment/jobs/{id}` | [x] |
| 24 | Add API endpoint: `GET /v1/enrichment/stats` | [x] |
| 25 | Optional API endpoint: `POST /v1/enrichment/jobs` (manual enqueue) | [x] |
| 26 | Add store tests (`internal/store/enrichment_jobs_test.go`) | [x] |
| 27 | Add worker tests (`internal/enrichment/worker_test.go`) | [x] |
| 28 | Add handler tests (`work_editions_test.go`, `author_expand_test.go`) | [x] |
| 29 | Add resolver scheduling tests (`internal/resolver/enrichment_hook_test.go`) | [x] |
| 30 | Validate full Phase 4 success criteria + `go test ./... -count=1` | [x] |

---

## 1 — Database: Job Queue + History

### 1.1 Create migrations

Create:

- `migrations/000004_enrichment_jobs.up.sql`
- `migrations/000004_enrichment_jobs.down.sql`

Suggested schema:

```sql
CREATE TABLE enrichment_jobs (
  id BIGSERIAL PRIMARY KEY,

  job_type TEXT NOT NULL,          -- e.g., "author_expand", "work_editions"
  entity_type TEXT NOT NULL,       -- "author", "work"
  entity_id TEXT NOT NULL,         -- canonical ID

  status TEXT NOT NULL DEFAULT 'queued',  -- queued|running|succeeded|failed|dead|cancelled
  priority INT NOT NULL DEFAULT 100,      -- lower = sooner
  attempt_count INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 5,

  not_before TIMESTAMP NULL,       -- scheduling / backoff
  locked_at TIMESTAMP NULL,
  locked_by TEXT NULL,

  last_error TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- prevent duplicate queued/running jobs for same target
CREATE UNIQUE INDEX uq_enrichment_jobs_dedupe
ON enrichment_jobs(job_type, entity_type, entity_id)
WHERE status IN ('queued','running');

CREATE INDEX idx_enrichment_jobs_sched
ON enrichment_jobs(status, priority, not_before, created_at);

CREATE TABLE enrichment_job_runs (
  id BIGSERIAL PRIMARY KEY,
  job_id BIGINT NOT NULL REFERENCES enrichment_jobs(id) ON DELETE CASCADE,
  started_at TIMESTAMP NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMP NULL,
  outcome TEXT NOT NULL,           -- succeeded|failed
  error TEXT NULL
);

CREATE INDEX idx_enrichment_job_runs_job
ON enrichment_job_runs(job_id);
```

### 1.2 Concurrency-safe locking strategy

Use PostgreSQL `SELECT ... FOR UPDATE SKIP LOCKED`:

- Select next job where:
  - `status = queued`
  - `not_before IS NULL OR not_before <= NOW()`
- Order by `priority ASC, created_at ASC`
- `FOR UPDATE SKIP LOCKED`
- Worker sets `status = running`, `locked_at`, `locked_by`
- On completion: set `status = succeeded/failed/dead`, clear lock fields

---

## 2 — Add Enrichment Store Layer

Create:

- `internal/store/enrichment_jobs.go`

Interface:

- `EnrichmentJobStore`:
  - `EnqueueJob(ctx, job) (jobID, error)`
  - `GetJobByID(ctx, id) (*Job, error)`
  - `TryLockNextJob(ctx, workerID string) (*Job, error)`
  - `MarkSucceeded(ctx, jobID)`
  - `MarkFailed(ctx, jobID, errMsg string, backoff time.Duration)`
  - `MarkDead(ctx, jobID, errMsg string)`
  - `RecordRunStart/Finish(ctx, jobID, outcome, err)`
  - `ListJobs(ctx, filters...) ([]Job, error)`

Models:

- `internal/model/enrichment.go`:
  - `EnrichmentJob`
  - `EnrichmentJobRun`

Backoff policy:

- Exponential backoff with jitter
- Cap at ~6 hours
- Increment `attempt_count`
- If `attempt_count >= max_attempts` → `status = dead`

---

## 3 — Enrichment Worker Runtime

### 3.1 Create worker module

Create:

- `internal/enrichment/worker.go`
- `internal/enrichment/engine.go`

Engine responsibilities:

- Start N workers (configurable)
- Each worker loop:
  - `TryLockNextJob`
  - Execute via handler registry
  - Record run history
  - Mark success/failure/dead

Graceful shutdown on SIGTERM:

- Stop acquiring new jobs
- Finish current job if possible
- Release lock if aborted

### 3.2 Job handler registry

Create:

- `internal/enrichment/handlers/handlers.go`

Interface:

```go
type JobHandler interface {
  Type() string
  Handle(ctx context.Context, job EnrichmentJob) error
}
```

Register handlers at startup.

---

## 4 — Prefetch Scheduling from Resolver

### 4.1 Add an enrichment scheduler hook

Modify resolver enqueue path **after successful provider merge + DB write**, never before.

Location:

- `internal/resolver/resolver.go` (or canonical write completion point)

Rules (Phase 4 minimal):

- When `SearchWorks` returns >=1 work with confidence above threshold (e.g. `0.85`):
  - Enqueue `work_editions` for top 1–3 works
  - Enqueue `author_expand` for each author on top result (cap 1–2)
- When `ResolveIdentifier` succeeds:
  - Enqueue `work_editions` for that work
  - Optionally enqueue `author_expand`

Priority target:

- `~50` (higher than periodic refresh jobs later)

### 4.2 Deduped enqueue

- Do not create duplicates if job already queued/running
- Unique partial index enforces this
- Store should swallow duplicate-key errors as no-op success

---

## 5 — User Preferences & Prefetch Policy

### 5.1 Add preferences config (Phase 4 minimal)

Add config keys:

```yaml
enrichment:
  enabled: true
  worker_count: 2
  max_jobs_per_request: 5

  limits:
    max_author_works: 50
    max_work_editions: 100

  preferences:
    languages: ["en"]
    formats: ["epub", "audiobook"]
```

Implement parsing in `internal/config/config.go`.

### 5.2 Apply preferences in handlers

- Keep canonical records complete
- Use preferences to prioritize aggressive fetch/store strategy for editions

---

## 6 — Enrichment Job Types (Phase 4 Minimum Set)

### 6.1 `work_editions`

File:

- `internal/enrichment/handlers/work_editions.go`

Behavior:

- Input: canonical `work_id`
- Use policy-aware provider ordering from registry (tier → score → priority)
- For each enabled provider:
  - `GetEditions(providerWorkID)` if mapping exists
  - Best-effort discovery if mapping missing
- Merge/normalize editions
- Insert editions + identifiers
- Record identifier introduced/match metrics where applicable
- Respect `max_work_editions`

### 6.2 `author_expand`

File:

- `internal/enrichment/handlers/author_expand.go`

Behavior:

- Input: canonical `author_id`
- For each enabled provider in dispatch order:
  - `SearchWorks("author name")` and filter by author match
- Stop when `max_author_works` reached
- For each discovered work:
  - Insert canonical metadata via shared ingest/identity flow
  - Enqueue `work_editions` for top N (e.g. 5)
- Respect `max_author_works` and `max_jobs_per_request`

---

## 7 — Provider Policy Awareness in Enrichment

Rules:

- Always obtain providers via `registry.EnabledProviders()`
- Use registry ordering as single source of truth
- Apply per-provider timeout from registry
- Respect rate limiter gating
- If quarantine mode is `disabled`, enrichment skips quarantine providers

---

## 8 — Observability

### 8.1 Prometheus metrics

Create:

- `internal/metrics/enrichment_metrics.go`

Metrics:

- `enrichment_jobs_enqueued_total{job_type}`
- `enrichment_jobs_started_total{job_type}`
- `enrichment_jobs_succeeded_total{job_type}`
- `enrichment_jobs_failed_total{job_type}`
- `enrichment_job_duration_ms{job_type}`
- `enrichment_queue_depth{status}` (gauge)

### 8.2 API endpoints

Add:

- `GET /v1/enrichment/jobs?status=queued&limit=50`
- `GET /v1/enrichment/jobs/{id}`
- `GET /v1/enrichment/stats`

Optional:

- `POST /v1/enrichment/jobs` to manually enqueue
  - Body: `{job_type, entity_type, entity_id, priority}`

---

## 9 — Tests

### 9.1 Store tests

Create:

- `internal/store/enrichment_jobs_test.go`

Validate:

- Enqueue dedupe behavior
- Lock acquisition + `SKIP LOCKED` semantics
- Backoff scheduling (`not_before`)
- Status transitions: queued → running → succeeded/failed/dead

### 9.2 Worker tests

Create:

- `internal/enrichment/worker_test.go`

Validate:

- Worker executes handler
- Marks success/failure correctly
- Honors max attempts and backoff

### 9.3 Handler tests

Create:

- `internal/enrichment/handlers/work_editions_test.go`
- `internal/enrichment/handlers/author_expand_test.go`

Validate:

- Limits enforced
- Provider ordering respected (tier/score)
- Quarantine policy respected
- Dedupe insert behavior (identifier uniqueness)

### 9.4 Resolver scheduling tests

Create:

- `internal/resolver/enrichment_hook_test.go` (or extend resolver tests)

Validate:

- Successful query enqueues expected jobs
- Enqueue is deduped
- Scheduling behavior on cache-hit paths (as intended)
- Scheduling does not block response latency

---

## Phase 4 Success Criteria

### Functional

- Jobs are enqueued automatically from successful interactive flows
- Repeated search does not create duplicate queued/running jobs
- Workers process jobs through queued → running → succeeded
- `work_editions` persists additional editions + identifiers
- `author_expand` adds related works and queues follow-up `work_editions`

### Operational Safety

- Provider policy is respected:
  - `last_resort`: quarantine used only after higher tiers
  - `disabled`: quarantine never called
- Timeouts and rate limiting respected during enrichment
- Interactive query latency remains stable under background workload

### Observability

- `GET /v1/enrichment/stats` exposes worker/queue state
- Metrics reflect lifecycle progression
- Logs include `job_id`, `job_type`, `entity_id`, and outcome

### Quality

- `go test ./... -count=1` passes
- Includes concurrency/locking coverage
- Includes policy-awareness coverage

---

## Recommended Milestone Order

1. DB migrations + store layer
2. Worker engine + handler registry
3. Minimal `work_editions` handler (OpenLibrary-only first)
4. Resolver hook to enqueue `work_editions`
5. Add `author_expand` handler
6. Add API endpoints + metrics
7. Add tests, then expand handlers to all enabled providers

---

## Reference Documents

- [PRD.md](PRD.md)
- [implementation_plan.md](implementation_plan.md)
- [Metadata_Service_Query_Flow.md](Metadata_Service_Query_Flow.md)
- [architecture_diagram.md](architecture_diagram.md)
