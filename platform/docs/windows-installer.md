# Windows Zip Distribution

Bookwyrm alpha distributions are zip-only to avoid unsigned installer confusion.

## Artifact

- `bookwyrm-<version>-windows.zip`

## Zip structure

```text
Bookwyrm/
  bin/
  config/
    bookwyrm.env.example
    metadata-service.yaml.example
  docs/
    RUN-BOOKWYRM-WINDOWS.md
  scripts/
    start-bookwyrm.ps1
    install-service.ps1
    uninstall-service.ps1
  logs/
  data/
```

## Install from zip

1. Extract zip to a stable folder root, for example `C:\ProgramData` (creates `C:\ProgramData\Bookwyrm`).
2. Ensure these subfolders exist after extraction:
   - `bin\`
   - `config\`
   - `logs\`
   - `data\`
3. Edit `config\bookwyrm.env` and `config\metadata-service.yaml` for your environment.
   - Required in `bookwyrm.env`: `LIBRARY_ROOT`, `DOWNLOADS_COMPLETED_PATH`, `DATABASE_DSN`, `UI_DIST_DIR`.
   - Configure at least one download client in `bookwyrm.env`:
     - qBittorrent: `QBITTORRENT_BASE_URL`, `QBITTORRENT_USERNAME`, `QBITTORRENT_PASSWORD`
     - SABnzbd: `SABNZBD_BASE_URL`, `SABNZBD_API_KEY`, `SABNZBD_CATEGORY`
     - NZBGet: `NZBGET_BASE_URL`, `NZBGET_USERNAME`, `NZBGET_PASSWORD`, `NZBGET_CATEGORY`
   - Required in `metadata-service.yaml`: `database.host`, `database.port`, `database.user`, `database.password`, `database.dbname`.
4. Start Bookwyrm using the helper script:

```powershell
cd C:\ProgramData\Bookwyrm
.\scripts\start-bookwyrm.ps1
```

5. Open `http://localhost:8090` and complete setup.

Example Postgres bootstrap:

```sql
CREATE USER bookwyrm WITH PASSWORD 'bookwyrm';
CREATE DATABASE bookwyrm_backend OWNER bookwyrm;
```

## Upgrade behavior

- Replace `bin\` with files from the new zip.
- Keep `config\`, `data\`, and `logs\`.
- Do not overwrite secrets unless intentionally rotating values.

## Post-install validation

```powershell
Invoke-RestMethod http://localhost:8090/api/v1/system/dependencies
Invoke-RestMethod http://localhost:8090/api/v1/system/migration-status
```

- `can_function_now=true` confirms startup success.
- Use [alpha-validation.md](alpha-validation.md) to record clean-machine results per release tag.
