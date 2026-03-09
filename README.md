# Bookwyrm

Bookwyrm is a modern automated book management system inspired by the Arr ecosystem. It is built for ebooks and audiobooks, with Sonarr/Radarr-style monitoring, search, download orchestration, import/rename workflows, and explainable decisioning.

## Database Model (Current Alpha)

Current development defaults use a single Postgres database connection for app and indexer via `DATABASE_DSN`, while `metadata-service` uses YAML config (`metadata-service/configs/config.yaml`) with optional `DATABASE_*` environment overrides.

For local/shared-db setups, all three services may safely target the same database. Each service now tracks migrations in its own migration history table:

- backend: `backend_schema_migrations`
- indexer: `indexer_schema_migrations`
- metadata: `metadata_schema_migrations`

This avoids cross-service migration version collisions when a shared Postgres database is used.

Migration startup behavior:

- backend: runs backend migrations when `DATABASE_DSN` is set
- indexer: runs indexer migrations when `DATABASE_DSN` is set
- metadata-service: runs metadata migrations on startup after DB connect

## Docker Quickstart (Shared DB)

Use the root compose file to run all core services against one Postgres database:

```bash
cp .env.example .env
docker compose up --build
```

Compose automatically reads `.env` from the repo root. Override DB credentials, DB name, and host port mappings there.

Services:

- app-backend: `http://localhost:8090`
- indexer-service: `http://localhost:8091`
- metadata-service: `http://localhost:8080`
- postgres: `localhost:5432` (`bookwyrm` / `bookwyrm`, db `bookwyrm`)

The stack intentionally uses one shared DB for alpha install stability. Migration history is isolated per service (`backend_schema_migrations`, `indexer_schema_migrations`, `metadata_schema_migrations`) so startup migrations do not conflict.

Startup readiness behavior in root compose:

- `indexer-service` waits for Postgres `service_healthy`
- `metadata-service` waits for Postgres `service_healthy`
- `app-backend` waits for Postgres, metadata-service, and indexer-service `service_healthy`

## Migration Behavior

Migrations are automatic at service startup:

- backend applies embedded migrations when `DATABASE_DSN` is set
- indexer-service applies embedded migrations when `DATABASE_DSN` is set
- metadata-service applies embedded migrations on startup after DB connect

Manual migration execution is not required for normal local startup.

See `docs/migrations.md` for details and troubleshooting queries.

## Troubleshooting First-Run Issues

If services are running but features fail (for example wanted items, import queue, or metadata queries), verify all services point to the same intended database for your selected install mode.

Common failure patterns:

- DB exists but tables are missing for one feature: service is likely pointed at the wrong DB
- Compose variables changed but behavior did not change: existing Postgres volume may still contain old state

When changing DB layout or credentials in Docker, reinitialize volumes deliberately:

```bash
docker compose down -v
docker compose up --build
```

Warning: `down -v` removes local Postgres data for this compose project.

## Platform Modules (Phase 14)

## Why Bookwyrm

Bookwyrm exists to provide an Arr-style experience for books with stronger explainability and operational support:

- Reliable long-running automation across metadata, search, download, and import stages
- Modular services with clear boundaries for easier debugging and upgrades
- Windows-native launcher flow with zip distribution
- Flexible search pipelines (Prowlarr and direct providers)
- Transparent scoring and review workflows for low-confidence imports

## How It Differs from Readarr

- Three-service modular architecture (`metadata-service`, `indexer-service`, `app/backend`) supervised by a single launcher
- Explainable candidate scoring and recommendation reasons surfaced in UI
- Reliability/tier-aware provider and backend routing
- Built-in support bundle export and remediation actions for supportability
- Native Windows launcher/service target from day one

## Key Features

- Automated metadata discovery and enrichment
- Multi-source metadata aggregation with reliability scoring
- Sonarr-style wanted monitoring model for authors and works
- Profiles with format quality ordering and cutoff upgrades
- Manual search with scoring explainability
- Needs-review workflow with keep/replace/skip decisions
- Download client integration (SABnzbd, NZBGet, qBittorrent)
- Import pipeline with naming/path previews
- Timeline/history visibility for works
- Recommendation graph APIs and UI reasons
- Support bundle diagnostics export (redacted)
- Windows-native launcher + service flow

## Screenshots

Screenshots are published in alpha release notes and docs once each tagged alpha artifact is produced. The first set includes:

- Dashboard
- Book detail (overview/search/history)
- Manual search scoring panel
- Import needs-review comparison
- System status/recovery page

## Installation

### Windows (Recommended)

Windows Alpha Distribution

Bookwyrm currently ships as a ZIP package for alpha testing. Extract it to a stable folder, configure the included env/config files, and launch it with the provided launcher scripts.

Installer packaging is planned for a later release once code-signing and broader distribution are justified.

Downloads:

- [Latest release downloads](https://github.com/rcmiller01/Bookwyrm/releases/latest)
- [v0.1.0-alpha release](https://github.com/rcmiller01/Bookwyrm/releases/tag/v0.1.0-alpha)

1. Download `bookwyrm-<version>-windows.zip` from Releases.
2. Extract to a stable folder root (example: `C:\ProgramData`, resulting in `C:\ProgramData\Bookwyrm`).
3. Create Postgres DB/user (example):

```sql
CREATE USER bookwyrm WITH PASSWORD 'bookwyrm';
CREATE DATABASE bookwyrm_backend OWNER bookwyrm;
```

4. Edit `config\bookwyrm.env` and set at minimum:
   - `LIBRARY_ROOT`
   - `DOWNLOADS_COMPLETED_PATH`
   - `DATABASE_DSN`
   - `UI_DIST_DIR=C:\ProgramData\Bookwyrm\web\dist`
   - one download client block (qBittorrent, SABnzbd, or NZBGet)
5. Edit `config\metadata-service.yaml` database credentials to match your Postgres user/password.
6. Run `scripts\start-bookwyrm.ps1` from `C:\ProgramData\Bookwyrm` (or `bin\bookwyrm-launcher.exe run --base-dir C:\ProgramData\Bookwyrm`).
7. Open `http://localhost:8090` and complete the setup checklist.

Recommended DB mode for Windows alpha: native Bookwyrm + Postgres in Docker Desktop (hybrid).

### Docker / Hybrid

Use `docker-compose.yml` for full-stack local deployment, or run Bookwyrm services natively and Postgres in Docker:

- [Docker hybrid guide](docs/docker-hybrid.md)
- [Postgres hybrid details](docs/postgres-hybrid.md)

## Architecture Overview

Bookwyrm keeps service boundaries explicit:

- `metadata-service`: metadata providers, normalization, enrichment, graph/recommendations
- `indexer-service`: wanted model, indexer routing, search orchestration, reliability
- `app/backend`: UI/API gateway, queue/import orchestration, system status, support tools

Windows packaging adds:

- `bookwyrm-launcher`: supervises the 3 services, health-waits startup, manages logs/service lifecycle

This yields one user-facing app without collapsing internal modularity.

## Configuration Overview

Primary setup areas:

- Library root and staging/trash behavior
- Metadata providers
- Indexer backends and staged search controls
- Download clients and protocol defaults
- Profiles and monitoring defaults

Secrets are env/YAML driven and are not written back from UI as plain values.

## Troubleshooting and Support

- [Troubleshooting](docs/troubleshooting.md)
- [Windows native deployment](docs/windows-native.md)
- [Postgres hybrid mode](docs/postgres-hybrid.md)
- [Backup and restore](docs/backup-restore.md)

For bug reports, export `Status -> Download Support Bundle` and attach it to the issue.

## Development

Stack:

- Go services
- React + TypeScript frontend
- PostgreSQL

Typical local checks:

```bash
# Go modules
cd metadata-service && go test ./... -count=1
cd ../indexer-service && go test ./... -count=1
cd ../app/backend && go test ./... -count=1

# Web app
cd web
npm ci
npm run lint
npm test
npm run build
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution and review expectations.

## License

This repository is currently distributed for alpha testing and development review. A formal open-source license declaration will be finalized before broader public release.
