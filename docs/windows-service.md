# Windows Service Operation

## Service name

- `Bookwyrm`

## Start/stop expectations

- Start launcher once; it should supervise:
  - metadata-service
  - indexer-service
  - app-backend
- Stop should terminate child processes gracefully.

## Service commands

```powershell
bookwyrm-launcher.exe install-service --base-dir C:\ProgramData\Bookwyrm
bookwyrm-launcher.exe start-service --base-dir C:\ProgramData\Bookwyrm
bookwyrm-launcher.exe status --base-dir C:\ProgramData\Bookwyrm
bookwyrm-launcher.exe stop-service --base-dir C:\ProgramData\Bookwyrm
bookwyrm-launcher.exe uninstall-service --base-dir C:\ProgramData\Bookwyrm
```

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
Invoke-RestMethod http://localhost:8090/api/v1/system/dependencies
```

The service is operational when `/system/dependencies` reports `can_function_now=true`.
