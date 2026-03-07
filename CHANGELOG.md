# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
- metadata-service Dockerfile moved from `docker/Dockerfile` to `Dockerfile` with version stamping
- metadata-service `/health` endpoint now returns JSON with version info (was plain text "ok")
- Backend `/api/v1/health` now delegates to `Healthz` handler for consistency

### Fixed
- 5 ESLint `exhaustive-deps` warnings in React pages (AuthorsPage, BooksPage, ProfilesPage, QueuePage)
