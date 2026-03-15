# Bookwyrm Launcher

`bookwyrm-launcher` supervises the existing three-service architecture on Windows:

- `metadata-service`
- `indexer-service`
- `backend`

It is intentionally not a service merge. It starts all services, waits for health, restarts crashed children with capped backoff, rotates logs, and can run as a Windows service.

## Commands

```powershell
# foreground mode
bookwyrm-launcher.exe run --base-dir C:\ProgramData\Bookwyrm

# windows service lifecycle
bookwyrm-launcher.exe install-service --base-dir C:\ProgramData\Bookwyrm
bookwyrm-launcher.exe start-service --base-dir C:\ProgramData\Bookwyrm
bookwyrm-launcher.exe stop-service --base-dir C:\ProgramData\Bookwyrm
bookwyrm-launcher.exe uninstall-service --base-dir C:\ProgramData\Bookwyrm
```

## Expected layout

```
C:\ProgramData\Bookwyrm\
  bin\
    bookwyrm-launcher.exe
    metadata-service.exe
    indexer-service.exe
    backend.exe
  config\
    bookwyrm.env
    metadata-service.yaml
  logs\
    launcher.log
    metadata-service.log
    indexer-service.log
    backend.log
  data\
    first_run_complete.json
```

## Config

The launcher reads `config\bookwyrm.env` and merges with process env.

Important keys:

- `BOOKWYRM_HOME`
- `BOOKWYRM_LOG_DIR`
- `BOOKWYRM_METADATA_EXE`
- `BOOKWYRM_INDEXER_EXE`
- `BOOKWYRM_BACKEND_EXE`
- `BOOKWYRM_LAUNCH_URL`
- `BOOKWYRM_OPEN_BROWSER_ON_FIRST_START`
- `BOOKWYRM_RESTART_LIMIT`
- `BOOKWYRM_RESTART_WINDOW_SEC`
- `BOOKWYRM_RESTART_BASE_DELAY_SEC`
- `BOOKWYRM_RESTART_MAX_DELAY_SEC`

