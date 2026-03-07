# Postgres Hybrid Mode

Use this mode when Bookwyrm services run natively and Postgres runs in Docker Desktop.

## Why use hybrid

- Easier Postgres backup/restore lifecycle
- Native service startup behavior for Bookwyrm
- Lower operational burden than bundled database management

## Connection examples

```env
DATABASE_DSN=postgres://bookwyrm:bookwyrm@localhost:5432/bookwyrm_backend?sslmode=disable
```

Adjust DB names per service where needed.

## Ops checklist

- Confirm container auto-start behavior for Postgres.
- Monitor free disk on Docker data volume.
- Take backups before upgrades.
- Validate `readyz` after restarts.
- Validate `GET /api/v1/system/dependencies` returns `can_function_now=true`.
- Validate `GET /api/v1/system/migration-status` is `ok` before and after upgrades.

## Troubleshooting quick hits

- `pq: password authentication failed`:
  - verify DSN credentials
  - reset password/user grants in Postgres
- `connection refused`:
  - confirm Docker Desktop is running
  - check Postgres port mapping (`5432`)
- migrations pending after upgrade:
  - check service startup logs
  - inspect `/api/v1/system/migration-status`
  - restore backup if migration failed irrecoverably
