# Bookwyrm Indexer Service (Phase 11)

Source-agnostic indexer orchestration service for concurrent provider groups.

## Endpoints

- `GET /v1/indexer/health`
- `GET /v1/indexer/providers`
- `POST /v1/indexer/search`

## Concurrent backend groups

Request payload supports `backend_groups`, e.g.:

```json
{
  "metadata": {"work_id": "work-1", "title": "Dune"},
  "backend_groups": ["prowlarr", "non_prowlarr"]
}
```

If omitted, both groups are queried. Results are merged and ranked deterministically by confidence.

## Run

```bash
cd indexer-service
go mod tidy
go run ./cmd/server
```

## Prowlarr Integration

By default, a mock `prowlarr` adapter is used for local development.

Set these env vars to enable live Prowlarr integration:

- `PROWLARR_BASE_URL` (e.g. `http://localhost:9696`)
- `PROWLARR_API_KEY`
