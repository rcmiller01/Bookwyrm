# Phase 16 TODO — Sonarr-Style Web UI (Books)

## Phase 16 Objective

Build a full-featured web UI with Sonarr/Radarr look-and-feel that makes the system deployable and usable without Postman:

- Configuration (indexers, download clients, metadata providers, media management)
- Library browsing (authors, books, series)
- Manual search + grab (staged search + preferred sources)
- Activity pages (download queue, history, import `needs_review`)
- Wanted/monitoring controls
- System status/tasks/logs

No auth (LAN-style, Arr stack parity).
UI should be a separate frontend bundle served by `app/backend` (recommended) or as a static site behind a simple proxy.

## Non-goals

- Multi-user auth/permissions
- Native mobile app
- Rebuilding core queue/indexer/import logic (UI should consume existing APIs where possible)

---

## 0) Decisions / Constraints

### UI/UX constraints

- Arr feel: left nav, table-first pages, detail pages with tabs, filter bars, bulk actions
- Non-breaking APIs: UI consumes existing APIs; add endpoints only where UX requires missing data/actions

### Tech choices (recommended)

- React + Vite
- TypeScript
- Tailwind
- TanStack Query
- React Router

### Deployment

- UI compiled to static assets and served by `app/backend` at `/`
- APIs single-origin through backend:
  - `/ui-api/metadata/*` → `metadata-service /v1/*`
  - `/ui-api/indexer/*` → `indexer-service /v1/*`
  - keep backend native APIs at `/api/v1/*`

---

## Slice A — Backend UI Serving & API Gateway

Status:

- [x] A1 static UI serving + SPA fallback implemented in backend router.
- [x] A2 `/ui-api/metadata/*` and `/ui-api/indexer/*` single-origin proxy routes implemented.
- [x] A-tests added for SPA fallback and proxy path rewriting.

### A1) Serve UI bundle from backend

- Add static file serving in `app/backend`
- `GET /` serves `index.html`
- `GET /assets/*` serves static assets
- SPA fallback: all non-API routes return `index.html`

### A2) Add backend proxy routes (single-origin UI)

- `/ui-api/metadata/*` proxy to metadata-service
- `/ui-api/indexer/*` proxy to indexer-service
- Preserve `/api/v1/*` behavior for backend-native endpoints

### A3) Acceptance

- Refresh-safe SPA routing works
- UI can call all services from one origin without CORS setup

### Tests

- route fallback test (`/authors/123` returns `index.html`)
- proxy smoke tests for metadata and indexer paths

---

## Slice B — UI Shell & Shared Patterns

Status:

- [x] B1 React + Vite + TypeScript app shell scaffolded with Sonarr-style left navigation.
- [x] B2 shared primitives implemented (`DataTable`, toast provider, confirm dialog, polling hook).
- [x] B-build validated (`npm run build`) producing `web/dist` assets.

### B1) App shell and nav

Create Sonarr-style left nav sections:

- Dashboard
- Library
  - Authors
  - Books
  - Series
- Activity
  - Queue
  - History
  - Import List (`needs_review`)
- Wanted
  - Missing
  - (Optional) Cutoff Unmet (16.2)
- Settings
  - Media Management
  - Profiles (16.2)
  - Quality (16.2)
  - Indexers
  - Download Clients
  - Metadata
  - General
- System
  - Status
  - Tasks
  - Logs

### B2) Shared UI primitives

- Reusable table component:
  - sorting/filtering/pagination
  - bulk selection/actions
- Toast notifications
- Confirmation dialog for destructive actions
- Polling hooks for queue/activity pages

### B3) Acceptance

- Shell loads and navigates all planned routes
- Shared table patterns are used consistently

---

## Slice C — Dashboard

Status:

- [x] C1 setup checklist widget implemented (library root, enabled indexers, enabled download clients).
- [x] C2 activity summary implemented (downloads in progress, imports needs_review).
- [x] C3 health quick view implemented (metadata providers, indexer backends, download clients).
- [x] C4 wired to real APIs via `/api/v1/*` and `/ui-api/*`.
- [x] C-dashboard build/tests validated.

### C1) Setup checklist widget

- show missing configuration checks:
  - library root
  - at least one enabled indexer backend
  - at least one enabled download client

### C2) Activity summary widget

- downloads in progress
- imports in `needs_review`

### C3) Health quick view

- metadata provider health/reliability
- indexer backend health/reliability
- download client reliability

### C4) APIs

- backend stats endpoints (`/api/v1/*/stats` where available)
- metadata provider reliability (`/ui-api/metadata/providers/*`)
- indexer backend status (`/ui-api/indexer/backends/*`)
- download queue (`/api/v1/download/jobs?...`)

### C5) Acceptance

- Dashboard is actionable for setup + current activity at a glance

---

## Slice D — Library Pages

### D1) Authors list

- list known authors
- columns: author, book count, monitored state, last/next search
- bulk actions: monitor/unmonitor, search monitored

API gap note:

- Add lightweight author list endpoint if missing (metadata-service or backend aggregation)

### D2) Author detail

Tabs:

- Overview
- Books
- Series
- History

Actions:

- monitor/unmonitor author
- search author now

### D3) Books list

- list monitored/all books and/or library items
- filters: monitored, missing, format
- actions: manual search, monitor toggle, view files

### D4) Book detail

Tabs:

- Overview
- Files
- Search
- History
- Recommendations

### D5) Acceptance

- user can browse authors/books/series and navigate into actionable detail pages

---

## Slice E — Manual Search & Grab (Flow 4)

Status:

- [x] E1 staged manual search UI implemented (`enqueue`, `poll request`, `list candidates`) with configurable thresholds/timeouts.
- [x] E2 candidate table implemented with source badges, score/reasons panel, and `Grab` action (optional auto-handoff to downloader).
- [x] E3 preferred-source heart toggles implemented in both candidate rows and Settings → Indexers.
- [x] Preferred-source ordering enforced in indexer orchestrator (hearted backends run before non-hearted backends).
- [x] Preferred backend update endpoint added (`POST /v1/indexer/backends/{id}/preferred` via `/ui-api/indexer/...`).

### E1) Staged search flow wiring

- enqueue: `POST /ui-api/indexer/search/work/{workID}`
- poll request state: `GET /ui-api/indexer/search/{requestID}`
- list candidates: `GET /ui-api/indexer/candidates/{requestID}?limit=...`

Expose staged search settings in UI:

- min candidates
- min score threshold
- stage timeout

### E2) Candidate table UX

Columns:

- title, source, protocol, size, seeders, score, reasons

Actions:

- grab candidate (`POST /ui-api/indexer/grab/{candidateID}`)
- optional auto handoff to downloader (`POST /api/v1/download/from-grab/{grabID}`)

### E3) Preferred sources (heart)

- preferred toggle per backend in Settings and candidate rows
- preferred backends searched before non-preferred group

API gap note:

- add/extend backend update endpoint for `preferred` if missing

### E4) Acceptance

- staged search and preferred-source ordering behave as expected

### Tests

- enqueue → candidates render → grab → download job appears

---

## Slice F — Activity Pages

Status:

- [x] F1 Queue page implemented with 3s polling and `cancel`/`retry` actions.
- [x] F2 Import List page implemented for `needs_review` with detail, naming/decision previews, and `approve`/`retry`/`skip`/`decide` actions.
- [x] F3 History page implemented as aggregated recent download/import activity.
- [x] F-pages wired to real `/api/v1` endpoints.

### F1) Queue (downloads)

- source: `GET /api/v1/download/jobs?status=...`
- auto-poll every 2–5 seconds
- actions: cancel, retry

### F2) Import List (`needs_review`)

- source: `GET /api/v1/import/jobs?status=needs_review`
- per-item detail: decision context + naming preview
- actions: approve/retry/skip

### F3) History

- aggregate recent search/grab/download/import events
- simple first version; deep timeline links can follow

### F4) Acceptance

- operators can resolve queue and import review flows entirely in UI

---

## Slice G — Wanted & Monitoring

### G1) Wire Phase 15 wanted model

- list wanted works/authors
- monitor/unmonitor actions
- trigger search all / search item

### G2) Missing page

- monitored items not in library
- actions: search all, manual search per item

### G3) Feature-flag fallback

- if wanted backend is unavailable, hide pages behind experimental toggle

### G4) Acceptance

- wanted/monitoring is usable from UI without direct API calls

---

## Slice H — Settings (Sonarr-like)

Status:

- [x] H1 media management page implemented with live runtime settings view (`/api/v1/import/stats`).
- [x] H2 indexer settings complete (enable/disable, priority, reliability, preferred heart).
- [x] H3 download client settings implemented (list + `enabled`/`priority` updates via backend API).
- [x] H4 metadata settings implemented (provider enable/disable and priority updates).
- [x] H5 general settings/status page implemented (backend/metadata/indexer health + enrichment runtime stats).
- [x] H-build/tests validated (`go test ./internal/api ./internal/downloadqueue`, `npm run build`).

### H1) Media Management

- `LIBRARY_ROOT`
- `IMPORT_KEEP_INCOMING`
- naming templates and sanitization/path options

### H2) Indexers

- list backends (prowlarr + MCP)
- enable/disable, priority, reliability/tier, preferred (heart)
- global staged-search knobs:
  - min candidates
  - min score threshold
  - per-stage timeout
  - quarantine mode

### H3) Download Clients

- list/configure NZBGet/SAB/qB
- enable/disable, priority, reliability/tier
- quarantine mode
- test connection

### H4) Metadata

- provider enable/disable
- reliability views
- quarantine policy

### H5) General/System

- task intervals
- log level
- optional explicit upstream URLs (if proxy disabled)

### H6) Acceptance

- all key runtime configuration can be managed from UI

---

## Slice I — System Pages

### I1) Status

- provider/indexer/download client health
- DB connectivity status
- optional disk space

### I2) Tasks

- list background tasks + last run
- run-now where supported

### I3) Logs

- filter by service/level/correlation IDs
- optional deep-link from queue/import job rows

### I4) Acceptance

- system diagnostics are available without shell/database access

---

## Slice J — UI Testing & Quality

### J1) Contract/smoke tests

- each route loads
- key data-fetching hooks handle loading/error/success states

### J2) Integration tests (mocked APIs)

- manual search enqueue → candidates → grab → queue reflects job
- needs_review approve → item leaves review list and becomes imported

### J3) Optional E2E (Playwright)

- dashboard loads
- search + grab + queue update
- needs_review approve flow

### J4) Acceptance

- high-confidence regression safety for critical operator flows

---

## API Additions (Only if Missing)

- Author list/summary endpoint for library UI (`authors` table view with counts)
- Backend preferred-source flag update endpoint
- Optional history aggregation endpoint for activity page
- Optional task run-now endpoint(s)

Principle: prefer backend aggregation/proxy instead of adding many direct UI-service contracts.

---

## Phase 16 Success Criteria

A user can fully operate the system from UI:

- configure library root and naming
- configure indexers and download clients
- add/monitor author/book
- run manual search
- grab candidate
- watch download queue
- resolve `needs_review` items
- confirm file imported into library

Staged search + preferred sources works:

- hearted Prowlarr sources searched first
- system advances to non-Prowlarr only when candidate thresholds are not met

UI feels Sonarr-like:

- navigation, tables, filters, bulk actions, detail tabs

No auth required; single-origin deployment works.

---

## Recommended Build Order

1. Slice A — backend serving + proxy (foundation)
2. Slice B — shell + shared table primitives
3. Slice C — dashboard and health visibility
4. Slice F — queue/import activity pages (highest operator value)
5. Slice E — manual search + grab flow
6. Slice H — settings pages
7. Slice D/G — library + wanted pages
8. Slice I/J — system pages + quality hardening

---

## Tracking

- [x] A — Backend UI serving/proxy
- [x] B — UI shell/shared patterns
- [x] C — Dashboard
- [x] D — Library pages
- [x] E — Manual search/grab
- [x] F — Activity pages
- [x] G — Wanted pages
- [x] H — Settings pages
- [x] I — System pages
- [ ] J — Testing/quality
