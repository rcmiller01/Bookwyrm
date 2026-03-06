# Phase 13 TODO — ImportService + RenameService (Library Pipeline)

## Objective

Deliver a safe library pipeline that turns completed `download_jobs` into managed library items:

`search -> grab -> download -> import/rename -> library`

## Scope

### In scope

- detect completed downloads ready for import (`status=completed`, `imported=false`)
- create and process `import_jobs`
- scan supported ebook/audiobook files from `source_path`
- best-effort match to `work_id`/`edition_id`
- rename/move into configured library root via templates
- record `library_items` and import history/events
- support retry/approve/skip API workflows
- idempotent behavior (rerun safe, no duplicates)

### Out of scope (Phase 14)

- advanced upgrade/replacement rules
- deep duplicate fingerprinting/checksum strategy
- advanced metadata extraction from media internals

## Data Model

Add migration pair:

- `migrations/00000Y_import_rename.up.sql`
- `migrations/00000Y_import_rename.down.sql`

Tables:

- `import_jobs`
- `import_events`
- `library_items`

## Configuration

```yaml
library:
  root: "/data/books"
  staging_root: "/data/downloads/completed"
  allow_cross_device_move: true

naming:
  template_ebook: "{Author}/{Series}/{SeriesIndex} - {Title} ({Year})/{Title} - {Author}.{Ext}"
  template_audiobook: "{Author}/{Series}/{Title} ({Year})/{Title} - {Author} ({Narrator})/{Track:02} - {PartTitle}.{Ext}"
  sanitize:
    replace_colon: true
    max_path_len: 240
```

## Modules

- `internal/importer/types.go`
- `internal/importer/engine.go`
- `internal/importer/worker.go`
- `internal/importer/matcher.go`
- `internal/importer/renamer.go`
- `internal/importer/filescan.go`
- `internal/importer/store.go` (+ PG implementation)

## State Machine

1. Trigger job creation from completed, unimported download jobs.
2. Worker claims queued import job (`SKIP LOCKED`) and runs:
   - scan files
   - match work/edition (or set `needs_review`)
   - build naming plan
   - collision/idempotency checks
   - move/copy into library root
   - write `library_items`
   - mark import done and set `download_jobs.imported=true`

## APIs

- `GET /v1/import/jobs`
- `GET /v1/import/jobs/{id}`
- `POST /v1/import/jobs/{id}/retry`
- `POST /v1/import/jobs/{id}/approve`
- `POST /v1/import/jobs/{id}/skip`
- `POST /v1/import/jobs/{id}/set-template`
- `GET /v1/library/items`

## Slices

1. Slice A: schema + trigger + file scan + minimal move
2. Slice B: templated renamer + sanitize + collision handling
3. Slice C: matcher confidence + `needs_review` + approve flow
4. Slice D: full API + metrics + end-to-end validation

## Definition of Done

- completed download automatically creates import job
- import job moves/renames into library root and writes `library_items`
- low-confidence matches become `needs_review` (non-destructive)
- reruns are idempotent
- import metrics/logging present
- `go test ./... -count=1` green
