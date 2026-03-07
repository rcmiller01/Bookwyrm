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

