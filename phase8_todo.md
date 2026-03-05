# Phase 8 Implementation Todo List

## Project: Bookwyrm — Advanced Metadata Sources

---

## Phase 8 Objective

Integrate richer optional metadata sources to improve edition discovery coverage while preserving safe defaults.

---

## Deliverables

- New optional providers: Anna's Archive, LibraryThing, WorldCat
- Provider wiring and config support in service startup
- Database additions: `content_sources`, `file_metadata`
- Domain + store support for source/file metadata persistence
- Unit tests for provider result parsing helpers

---

## Tasks

| # | Task | Status |
|---|---|---|
| 1 | Add provider adapter `annasarchive` | [x] |
| 2 | Add provider adapter `librarything` | [x] |
| 3 | Add provider adapter `worldcat` | [x] |
| 4 | Wire new providers in startup registration flow | [x] |
| 5 | Extend provider config with optional `base_url` | [x] |
| 6 | Add migration `000006_advanced_metadata_sources.up.sql` | [x] |
| 7 | Add migration `000006_advanced_metadata_sources.down.sql` | [x] |
| 8 | Add models for `ContentSource` and `FileMetadata` | [x] |
| 9 | Add store for source/file metadata CRUD reads/writes | [x] |
| 10 | Add parser-focused unit tests for new provider adapters | [x] |
| 11 | Validate full suite `go test ./... -count=1` | [x] |

---

## Notes

- New providers are disabled by default in `configs/config.yaml`.
- Provider adapters are intentionally best-effort and non-blocking to existing providers.
- `content_sources` and `file_metadata` tables are additive and do not change existing resolver write paths yet.
