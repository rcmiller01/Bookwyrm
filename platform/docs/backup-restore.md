# Backup and Restore

This guide covers backup and restore for beta deployments.

## What to back up

- Postgres databases:
  - `bookwyrm_metadata`
  - `bookwyrm_indexer`
  - `bookwyrm_backend`
- Config files (`.env`, service YAML files)
- `C:\ProgramData\Bookwyrm\config\` (Windows native deployments)
- `C:\ProgramData\Bookwyrm\logs\` (optional, for incident forensics)
- Optional: `LIBRARY_ROOT/_incoming` and `LIBRARY_ROOT/_trash` if you need job-staging history

## Recommended cadence

- Daily logical backups
- Before every upgrade
- Before migration-heavy changes

## Postgres backup (Docker)

```bash
docker compose exec postgres pg_dumpall -U bookwyrm > bookwyrm_full_$(date +%Y%m%d_%H%M%S).sql
```

## Postgres restore (Docker)

```bash
cat bookwyrm_full_YYYYMMDD_HHMMSS.sql | docker compose exec -T postgres psql -U bookwyrm
```

## Validate restore

```bash
curl -s http://localhost:8090/api/v1/healthz
curl -s http://localhost:8090/api/v1/readyz
curl -s http://localhost:8090/api/v1/system/status
curl -s http://localhost:8090/api/v1/system/dependencies
curl -s http://localhost:8090/api/v1/system/migration-status
```

Expected post-restore state:

- `/system/dependencies` returns `can_function_now=true`
- `/system/migration-status` returns `status=ok`

## Support bundle

When opening support requests, download:

- `Status -> Support & Recovery -> Download Support Bundle`

The bundle is redacted and includes queue/health/config summaries.
