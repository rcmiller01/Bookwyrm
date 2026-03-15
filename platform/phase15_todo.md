# Phase 15 TODO — Bookwyrm Hardening & Library Lifecycle

## Phase 15 Objective

Reduce manual intervention and make the system resilient over months of uptime by adding:

- Stuck job recovery + reconciliation across indexer/download/import queues
- Upgrade & duplicate workflows (replace/keep both/skip)
- Wanted/Monitoring model (author/work monitoring and scheduled searches)
- Cross-service correlation & observability polish
- End-to-end golden pipeline tests to prevent regressions

## Non-goals

- New media domains (TV/movies/music)
- UI beyond APIs you already have
- Deep content parsing (optional later)

---

## Slice A — Job Lifecycle Hardening (stuck recovery + reconciliation)

### A1) Add lease TTL to running jobs

Across:

- indexer search requests
- download jobs
- import jobs
- enrichment jobs (optional)

Add/update fields if missing:

- `locked_at`, `locked_by` already exist
- add `lease_expires_at` (or compute `locked_at + ttl`)

### A2) Recovery worker per subsystem

Status:

- [x] `indexer-service` recovery worker and store recovery paths
- [x] `app/backend` download recovery worker and store recovery paths
- [x] `app/backend` importer recovery worker and store recovery paths

Add periodic workers that:

- find `status=running` with expired lease → set back to `queued` with backoff + increment `attempt_count`
- record an event explaining why

### A3) Reconciliation routines

Status:

- [x] Downloader reconciliation: missing downstream job now uses tag fallback lookup and then fails with explicit reason/event when still missing.
- [x] Import reconciliation (idempotent existing target): verified by importer collision/idempotency tests.
- [x] Incoming-orphan to `needs_review` sweep implemented in importer reconciler.

**Downloader reconciliation**

- If `download_job` has `downstream_id` but client can’t find it:
  - attempt lookup by tags (you already have tag fallbacks)
  - if still missing, mark failed with reason `missing downstream job`

**Import reconciliation**

- If `_incoming` exists but job missing → create `needs_review` job (optional)
- If library target already exists and matches size → mark imported idempotently

### A4) API hooks (minimal)

- `POST /api/v1/*/reconcile` (optional) or keep internal only

### Success criteria

- Killing/restarting services does not leave jobs permanently stuck.
- Jobs recover automatically with bounded retries.

Slice A completion:

- [x] A1 complete
- [x] A2 complete
- [x] A3 complete
- [x] A4 left internal-only (no external reconcile endpoint required)

### Tests

- lease expiry resets `running` → `queued`
- downstream missing triggers fail + event (without panic)
- reconciliation is idempotent

---

## Slice B — Upgrade & Duplicate Policy (readarr-grade library lifecycle)

Status:

- [x] B1 decision model implemented (`keep_both`, `replace_existing`, `skip`) via importer decision action handling.
- [x] B2 endpoint implemented: `POST /api/v1/import/jobs/{id}/decide`.
- [x] B3 core behaviors implemented and tested (keep_both suffix copy, replace_existing to `_trash`, skip).

### B1) Introduce import decision model for collisions/upgrades

When import hits "target exists different size," capture decision options:

- `keep_both`
- `replace_existing`
- `skip`

Persist in `import_jobs.decision_json` (already present).

### B2) New endpoints for decisions

- `POST /api/v1/import/jobs/{id}/decide`
  - body: `{ action: "keep_both"|"replace_existing"|"skip" }`

Reuse existing approve/retry flows.

### B3) Implement behaviors

- `keep_both`: append suffix like `(copy)` or include edition id; store both in `library_items`
- `replace_existing`: move old file to quarantine/trash folder then move new in
- `skip`: mark skipped, leave new in `_incoming` or clean per policy

Add config:

- `LIBRARY_TRASH_DIR` (default `{library_root}/_trash`)
- `LIBRARY_KEEP_TRASH_DAYS` (future cleanup)

### Success criteria

- Collisions no longer require manual filesystem work; decisions are actionable.
- `library_items` remains consistent.

### Tests

- collision → decide `keep_both` creates second file with suffix
- collision → `replace` moves old to trash and installs new
- collision → `skip` leaves existing unchanged

Implemented test coverage:

- importer engine tests for `keep_both` and `replace_existing`
- API test for `/import/jobs/{id}/decide` with action validation + `skip`

---

## Slice C — Wanted/Monitoring Model (make it Arr-like)

Status:

- [x] C1 tables + migrations implemented in `indexer-service` (`indexer_wanted_works`, `indexer_wanted_authors`).
- [x] C2 scheduler loop implemented in `indexer-service` orchestrator (periodic due-item enqueue + enqueue timestamp update).
- [x] C3 APIs implemented in `indexer-service` (`/v1/indexer/wanted/works*`, `/v1/indexer/wanted/authors*`).
- [x] C-tests implemented (store due lifecycle, scheduler enqueue, API CRUD coverage).

Ownership note:

- Wanted/monitoring scheduling should live in `indexer-service` (or a thin orchestration layer) because it owns search scheduling.
- `app/backend` may own user-facing API surfaces for wanted management.

### C1) Add wanted tables

Migrations:

- `wanted_authors`
- `wanted_works`
- (optional) `wanted_series`

Fields:

- `enabled`, `priority`, `created_at`
- optional preferences per wanted: formats/languages

### C2) Scheduler jobs

New scheduled tasks (worker loops):

- periodically enqueue indexer searches for wanted items
- re-search on cadence (config), e.g. daily/weekly
- respect rate limits and provider/indexer reliability tiers

### C3) APIs

- `POST /v1/wanted/authors/{author_id}` enable monitoring
- `POST /v1/wanted/works/{work_id}`
- `GET /v1/wanted/*` list
- `DELETE /v1/wanted/*` disable

### Success criteria

- You can mark an author/work as monitored and the system schedules searches automatically.
- Searches respect your existing async indexer workflow.

### Tests

- enabling wanted creates schedule entry
- scheduler enqueues searches
- dedupe prevents duplicate queued searches

Slice C completion:

- [x] C1 complete
- [x] C2 complete
- [x] C3 complete

---

## Slice D — Correlation IDs & audit trail polish

Status:

- [x] D1 correlation fields standardized in downloader/importer event payloads (`work_id`, `edition_id`, `candidate_id`, `grab_id`, `download_job_id`, `import_job_id`).
- [x] D2 timeline endpoint implemented: `GET /api/v1/work/{id}/timeline` (plus `/api/v1/works/{id}/timeline` alias).
- [x] D-tests implemented and passing for timeline contract and correlation payload coverage.

### D1) Standardize correlation across the pipeline

Ensure these IDs are propagated and stored consistently:

- `work_id`, `edition_id`
- `search_request_id`
- `candidate_id`
- `grab_id`
- `download_job_id`
- `import_job_id`

Add in event payloads (`download_events`/`import_events`/indexer logs).

### D2) Add a timeline endpoint (operator gold)

- `GET /api/v1/work/{id}/timeline`

Returns:

- searches, grabs, download status, import results, library paths

This is a read-only aggregation from existing tables.

### Success criteria

- Operators can debug a single work end-to-end from one endpoint.

### Tests

- timeline shape contract test

Slice D completion:

- [x] D1 complete
- [x] D2 complete

---

## Slice E — End-to-end golden pipeline tests (regression shield)

Status:

- [x] E1 offline E2E harness implemented in backend importer tests with fake indexer + fake downloader and in-memory stores.
- [x] E2 golden fixtures implemented:
  - ebook pipeline
  - audiobook folder pipeline
  - collision upgrade pipeline (`replace_existing` decision path)

### E1) Add an offline E2E harness

Use in-memory stores + fake backends:

- fake indexer returns candidates
- fake downloader completes and writes `output_path`
- importer runs through

Assert:

- final library path matches template
- imported flags set
- events recorded

### E2) Add golden fixtures

- one ebook case
- one audiobook folder case
- one collision upgrade case

### Success criteria

- A single test run verifies the full core loop logic.
- Prevents future refactors from breaking the loop silently.

Implemented test coverage:

- `TestGoldenE2E_EbookPipeline`
- `TestGoldenE2E_AudiobookFolderPipeline`
- `TestGoldenE2E_CollisionUpgradeReplace`

---

## Slice F — Ops cleanup jobs (optional but helpful)

- cleanup `_incoming` older than N days (if `keep_incoming`)
- cleanup `_trash` older than N days
- cleanup stale indexer candidates (keep last N per request)

### Success criteria

- Library folder does not accumulate infinite debris.

Status:

- [x] Incoming cleanup implemented in importer (`_incoming` retention by age when `keep_incoming=true`).
- [x] Trash cleanup implemented in importer (`_trash` retention by age).
- [x] Indexer stale candidate pruning implemented (periodic per-request retention cap).

Implemented settings:

- `LIBRARY_KEEP_INCOMING_DAYS` (default `14`)
- `LIBRARY_KEEP_TRASH_DAYS` (default `30`)
- `INDEXER_CANDIDATE_RETENTION` (default `50`)

Slice F completion:

- [x] F1 incoming cleanup complete
- [x] F2 trash cleanup complete
- [x] F3 indexer candidate pruning complete

---

## Phase 15 Definition of Done

- Stuck jobs recover automatically after restart/interruption.
- Upgrade/duplicate decisions are supported via API and tested.
- Monitoring/wanted model schedules searches automatically.
- Correlation/audit trail makes debugging straightforward.
- End-to-end golden tests exist and pass.
- Full suite green across `metadata-service`, `indexer-service`, and `app/backend`.

Validation status (2026-03-06):

- [x] `metadata-service` full suite green
- [x] `indexer-service` full suite green
- [x] `app/backend` full suite green

## Recommended build order

1. Slice A
2. Slice B
3. Slice E
4. Slice C
5. Slice D
6. Slice F (optional)

Reason: recovery + upgrade + golden tests provide protection before adding monitoring automation.
