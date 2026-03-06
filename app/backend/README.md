# Bookwyrm App Backend (Phase 11)

Application BFF layer that composes metadata-service APIs into app workflows.

## Endpoints

- `GET /api/v1/health`
- `GET /api/v1/search?q=...`
- `GET /api/v1/works/{id}/intelligence`
- `GET /api/v1/works/{id}/availability?groups=prowlarr,non_prowlarr`
- `GET /api/v1/quality/report`
- `POST /api/v1/quality/repair` (dry-run only in current phase)
- `GET /api/v1/watchlists`
- `POST /api/v1/watchlists`
- `DELETE /api/v1/watchlists/{id}`
- `GET /api/v1/jobs`
- `POST /api/v1/jobs`
- `GET /api/v1/jobs/{id}`
- `POST /api/v1/jobs/{id}/retry`
- `POST /api/v1/jobs/{id}/cancel`

## Environment Variables

- `APP_BACKEND_ADDR` (default `:8090`)
- `APP_DOMAIN` (default `books`)
- `METADATA_SERVICE_URL` (default `http://localhost:8080`)
- `METADATA_SERVICE_API_KEY` (optional)
- `INDEXER_SERVICE_URL` (default `http://localhost:8091`)
- `INDEXER_SERVICE_API_KEY` (optional)
- `QBITTORRENT_BASE_URL` (optional; enables qBittorrent adapter)
- `QBITTORRENT_USERNAME`
- `QBITTORRENT_PASSWORD`
- `SABNZBD_BASE_URL` (optional; enables SABnzbd adapter)
- `SABNZBD_API_KEY`
- `SABNZBD_CATEGORY` (default `books`)
- `NZBGET_BASE_URL` (optional; enables NZBGet adapter)
- `NZBGET_USERNAME`
- `NZBGET_PASSWORD`
- `NZBGET_CATEGORY` (default `books`)

## Run

```bash
cd app/backend
go mod tidy
go run ./cmd/server
```
