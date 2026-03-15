# Docker Hybrid Deployment

Hybrid mode runs services as follows:

- Bookwyrm services on host or Windows native launcher
- Postgres in Docker Desktop

This is the recommended beta path for Windows users who want simpler DB operations.

## Service topology

- Host/native:
  - `metadata-service` on `http://localhost:8080`
  - `indexer-service` on `http://localhost:8091`
  - `app-backend` on `http://localhost:8090`
- Docker Desktop:
  - `postgres` on `localhost:5432`

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

Example DSNs:

```env
# app/backend
DATABASE_DSN=postgres://bookwyrm:bookwyrm@localhost:5432/bookwyrm_backend?sslmode=disable

# metadata-service
DATABASE_DSN=postgres://bookwyrm:bookwyrm@localhost:5432/bookwyrm_metadata?sslmode=disable

# indexer-service
DATABASE_DSN=postgres://bookwyrm:bookwyrm@localhost:5432/bookwyrm_indexer?sslmode=disable
```

## Minimum env for app-backend

```env
METADATA_SERVICE_URL=http://localhost:8080
INDEXER_SERVICE_URL=http://localhost:8091
APP_BACKEND_ADDR=:8090
LIBRARY_ROOT=D:\Media\Books
BOOKWYRM_LOG_DIR=C:\ProgramData\Bookwyrm\logs
```

## Verification

```bash
curl -s http://localhost:8090/api/v1/healthz
curl -s http://localhost:8090/api/v1/readyz
curl -s http://localhost:8090/api/v1/system/dependencies
curl -s http://localhost:8090/api/v1/system/migration-status
```

`can_function_now=true` on `/system/dependencies` is the operational go/no-go signal.

## Backup

Follow [backup-restore.md](backup-restore.md).

## Common failures

- `readyz` degraded:
  - verify Docker Postgres is running
  - verify service URLs and ports
  - open Status page and run `Test Connections`
- no search results:
  - confirm at least one enabled indexer backend
  - check `/api/v1/system/dependencies` for `search_backend_enabled`
- downloads never queue:
  - confirm at least one enabled download client
  - test each client from Settings -> Download Clients
