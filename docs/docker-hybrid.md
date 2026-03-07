# Docker Hybrid Deployment

Hybrid mode runs services as follows:

- Bookwyrm services on host or Windows native launcher
- Postgres in Docker Desktop

This is the recommended beta path for Windows users who want simpler DB operations.

## Port usage

- app-backend: `8090`
- indexer-service: `8091`
- metadata-service: `8080`
- postgres: `5432`

## Database connection

Set `DATABASE_DSN` for services that need Postgres:

```env
DATABASE_DSN=postgres://bookwyrm:bookwyrm@localhost:5432/bookwyrm_backend?sslmode=disable
```

Use separate DB names per service where configured.

## Verification

```bash
curl -s http://localhost:8090/api/v1/healthz
curl -s http://localhost:8090/api/v1/readyz
```

## Backup

Follow [backup-restore.md](backup-restore.md).

