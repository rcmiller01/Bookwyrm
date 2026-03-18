# Windows Paths and Permissions

## Recommended paths

- Library: `D:\Media\Books`
- Incoming staging: `D:\Media\Books\_incoming`
- Trash: `D:\Media\Books\_trash`
- Completed downloads: `D:\Downloads\Completed`
- Launcher home: `C:\ProgramData\Bookwyrm`
- Launcher first-run state: `C:\ProgramData\Bookwyrm\data\first_run_complete.json`

## Path rules

- Use absolute paths.
- Keep service account permissions for read/write/move on library and download folders.
- Avoid mixed UNC/local path combinations during initial setup.
- Watch long paths; keep `NAMING_MAX_PATH_LEN` conservative (default 240).

## Validation checklist

- Library root exists and is writable.
- Completed download path exists.
- Import can move/copy files from completed path to library root.
- Spaces in paths are tested before enabling automation.
- Logs should remain under `C:\ProgramData\Bookwyrm\logs` for support bundle capture.
