# Run Bookwyrm on Windows (ZIP)

1. Extract `bookwyrm-<version>-windows.zip` to `C:\ProgramData` (this creates `C:\ProgramData\Bookwyrm`).
2. Create Postgres DB/user (example):

```sql
CREATE USER bookwyrm WITH PASSWORD 'bookwyrm';
CREATE DATABASE bookwyrm_backend OWNER bookwyrm;
```

3. Edit `config\bookwyrm.env` and set:
   - `LIBRARY_ROOT` (example `D:\Media\Books`)
   - `DOWNLOADS_COMPLETED_PATH` (example `D:\Downloads\Completed`)
   - `DATABASE_DSN` (example `postgres://bookwyrm:bookwyrm@localhost:5432/bookwyrm_backend?sslmode=disable`)
   - `UI_DIST_DIR` (example `C:\ProgramData\Bookwyrm\web\dist`)
   - At least one download client (examples):
     - qBittorrent: `QBITTORRENT_BASE_URL`, `QBITTORRENT_USERNAME`, `QBITTORRENT_PASSWORD`
     - SABnzbd: `SABNZBD_BASE_URL`, `SABNZBD_API_KEY`, `SABNZBD_CATEGORY`
     - NZBGet: `NZBGET_BASE_URL`, `NZBGET_USERNAME`, `NZBGET_PASSWORD`, `NZBGET_CATEGORY`
4. Edit `config\metadata-service.yaml` database values to match Postgres auth.
5. Run:

```powershell
cd C:\ProgramData\Bookwyrm
.\bin\bookwyrm-launcher.exe run --base-dir C:\ProgramData\Bookwyrm
```

6. Open `http://localhost:8090`.

Notes:
- `metadata-service` now runs migrations automatically at startup.
- If `UI_DIST_DIR` is missing, `scripts\start-bookwyrm.ps1` auto-adds it.

Alternative:

- Right-click `scripts\start-bookwyrm.ps1`
- Select `Run with PowerShell`
