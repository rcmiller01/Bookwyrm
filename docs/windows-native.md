# Windows Native Deployment

This guide describes running Bookwyrm natively on Windows.

## Current beta shape

- `metadata-service.exe`
- `indexer-service.exe`
- `backend.exe`

Use a launcher/service wrapper when available. Until then, start all three services with consistent env/config.

## Recommended paths

- Library root: `D:\Media\Books`
- Downloads completed: `D:\Downloads\Completed`
- Optional logs dir: `C:\ProgramData\Bookwyrm\logs`

## Required env vars

- `METADATA_SERVICE_URL=http://localhost:8080`
- `INDEXER_SERVICE_URL=http://localhost:8091`
- `APP_BACKEND_ADDR=:8090`
- `LIBRARY_ROOT=D:\Media\Books`

## Health checks

```powershell
Invoke-RestMethod http://localhost:8090/api/v1/healthz
Invoke-RestMethod http://localhost:8090/api/v1/readyz
```

## Supportability

- Use `Status -> Download Support Bundle`.
- Keep logs in a stable folder and set `BOOKWYRM_LOG_DIR` for bundle pickup.

