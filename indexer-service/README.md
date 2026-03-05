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
