# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Telemetry-free diagnostics counters endpoint:
  - `GET /v1/indexer/stats` (searches executed, candidates evaluated, grabs performed)
  - `GET /api/v1/system/stats` (aggregated search/candidate/grab/download/import counters)
- Expanded Phase 23 alpha validation matrix with environment-specific gates and RC signoff checklist (`docs/alpha-validation.md`).
- Phase 22 alpha-release documentation set:
  - README overhaul with project overview, architecture, install, troubleshooting, and development sections.
  - `docs/release-workflow.md`
  - `docs/alpha-testing.md`
  - `docs/alpha-validation.md`
  - `docs/observability.md`
  - `docs/support-workflow.md`
- Alpha support intake template: `.github/ISSUE_TEMPLATE/alpha-bug-report.yml`.
- Windows alpha packaging scripts:
  - `scripts/release/build-alpha-windows.ps1`
  - `scripts/release/validate-alpha-install.ps1`
- GitHub Actions alpha packaging workflow: `.github/workflows/release-alpha.yml`.
- Installer packaging now supports versioned output naming (`bookwyrm-<version>-setup.exe`) and ships metadata config template scaffolding.
- `GET /api/v1/system/support-bundle` endpoint producing a redacted diagnostics zip for support.
- Support bundle now includes `system/readyz.json`, `system/dependencies.json`, and `system/migration-status.json` snapshots for faster incident diagnosis.
- `GET /api/v1/system/migration-status` endpoint for runtime migration readiness (ok/pending/failed).
- `GET /api/v1/system/dependencies` endpoint with a single functional dependency summary (`can_function_now`).
- Startup diagnostics now emit explicit `startup warning:` log lines for missing/unreachable metadata/indexer/DB/download-client/indexer-backend dependencies.
- New `launcher` module with `bookwyrm-launcher`:
  - supervises metadata/indexer/backend child processes
  - waits for health checks before reporting startup success
  - restarts crashed services with capped backoff policy
  - supports Windows service lifecycle (`install-service`, `start-service`, `stop-service`, `uninstall-service`)
  - writes `launcher.log` and per-service logs with rotation
  - opens browser on first successful startup using persisted first-run flag
- Added `GET /api/v1/system/logs-location` endpoint and Status page `Open Logs Folder` action.
- Added Inno Setup packaging scaffold: `launcher/packaging/windows/bookwyrm.iss`.
- System remediation actions:
  - `POST /api/v1/system/actions/retry-failed-downloads`
  - `POST /api/v1/system/actions/retry-failed-imports`
  - `POST /api/v1/system/actions/test-connections`
  - `POST /api/v1/system/actions/run-cleanup`
  - `POST /api/v1/system/actions/recompute-reliability`
  - `POST /api/v1/system/actions/rerun-wanted-searches`
  - `POST /api/v1/system/actions/rerun-enrichment`
- Status page "Support & Recovery" controls for support bundle download and one-click remediation.
- Status page migration warnings for pending/failed migrations, plus upgrade notes link and backup reminder.
- Status page degraded-mode messaging now reflects backend dependency summary.
- Operational docs:
  - `docs/backup-restore.md`
  - `docs/troubleshooting.md`
  - `docs/docker-hybrid.md`
  - `docs/windows-native.md`
  - `docs/windows-installer.md`
  - `docs/windows-service.md`
  - `docs/windows-paths.md`
  - `docs/postgres-hybrid.md`

## [0.18.0] - 2026-03-06

### Added
- Graceful shutdown with SIGINT/SIGTERM handling for all 3 services
- Version stamping via `-ldflags` for all 3 services (logged at startup)
- Unified `/healthz` (liveness) and `/readyz` (readiness) endpoints across all services
- `GET /api/v1/system/status` endpoint on app-backend with aggregated system overview
- Version info displayed on Status page and Dashboard footer in the UI
- Dockerfiles for app-backend (3-stage with UI build) and indexer-service
- Root `docker-compose.yml` for single-command full-stack deployment
- `.env.example` with documented configuration template
- `.dockerignore` to keep images lean
- `POST /api/v1/test-connection/download-client/{id}` endpoint with Test buttons in UI
- Prometheus HTTP metrics (`bookwyrm_http_requests_total`, `bookwyrm_http_request_duration_seconds`) on all 3 services
- `/metrics` endpoint on indexer-service (metadata-service and app-backend already had it)
- First-run welcome banner on Dashboard when setup is incomplete
- Prowlarr connection status in Dashboard setup checklist
- Inline setup tips and links to configuration pages in checklist items
- Migration documentation (`docs/migrations.md`)
- Upgrade guide (`docs/upgrading.md`)
- This changelog

### Changed
- CI now includes `launcher` in Go test/lint matrix and runs frontend production build (`npm run build`).
- metadata-service Dockerfile moved from `docker/Dockerfile` to `Dockerfile` with version stamping
- metadata-service `/health` endpoint now returns JSON with version info (was plain text "ok")
- Backend `/api/v1/health` now delegates to `Healthz` handler for consistency

### Fixed
- 5 ESLint `exhaustive-deps` warnings in React pages (AuthorsPage, BooksPage, ProfilesPage, QueuePage)
