# Windows Service Operation

## Service name

- `Bookwyrm`

## Start/stop expectations

- Start launcher once; it should supervise:
  - metadata-service
  - indexer-service
  - app-backend
- Stop should terminate child processes gracefully.

## Logs

Service mode should write:

- `launcher.log`
- `metadata-service.log`
- `indexer-service.log`
- `backend.log`

## Health validation

After service start:

```powershell
Invoke-RestMethod http://localhost:8090/api/v1/readyz
```

