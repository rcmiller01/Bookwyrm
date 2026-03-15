# Bookwyrm Change Summary

This document summarizes the improvements made during local debugging and live deployment work against the Bookwyrm install. It is intended as a handoff for upstreaming these changes into the repository under `C:\Users\rcmiller\Downloads\Bookwyrm-main\Bookwyrm-main`.

## Recommended PR Breakdown

1. Fix Add New / wanted work payloads and monitoring defaults
2. Improve indexer search hydration, auto-grab, scoring, and fallback retry
3. Fix NZBGet handoff and download queue behavior
4. Improve importer timing, naming, cross-drive moves, and clearer missing-source errors
5. Improve metadata service support for Anna's Archive
6. Improve import review UI wording and approve flow
7. Improve metadata provider test health reporting

## 1. Add New / Wanted Work Payload Fixes

Problem:
- Add New was creating monitored wanted works with an outdated payload shape.
- The installed UI only sent `enabled`, `priority`, and `cadence_minutes`.
- The live indexer schema required `formats`, and in practice also needed `profile_id` and language/profile-derived context.

Result:
- Adding books works again.
- New books are monitored correctly and scheduler/manual search has the data it needs.

Source files:
- `app/backend/web/src/lib/wantedWork.ts`
- `app/backend/web/src/pages/AddNewPage.tsx`
- `app/backend/web/src/pages/BooksPage.tsx`
- `app/backend/web/src/pages/MissingPage.tsx`
- `app/backend/web/src/pages/BookDetailPage.tsx`

Implementation notes:
- Introduced a shared wanted-work payload builder.
- Payload now derives `profile_id`, ordered `formats`, and `languages` from the selected/default profile.
- Keeps create/monitor flows consistent across pages.

## 2. Manual Search UX Improvements

Problem:
- Search actions appeared to do nothing because search-only activity does not show up in queue/history.
- The UI gave poor visibility into the actual manual-search step.

Result:
- Search actions now route users to a real manual-search experience that auto-runs for the selected title.
- Users can see candidates immediately and explicitly grab one.

Source files:
- `app/backend/web/src/lib/manualSearch.ts`
- `app/backend/web/src/pages/AddNewPage.tsx`
- `app/backend/web/src/pages/BookDetailPage.tsx`
- `app/backend/web/src/pages/BooksPage.tsx`
- `app/backend/web/src/pages/MissingPage.tsx`
- `app/backend/web/src/pages/ManualSearchPage.tsx`

Implementation notes:
- Search buttons prefill manual search from book context.
- Manual search auto-runs after navigation.
- UI copy clarifies that searching alone does not download until a candidate is grabbed.

## 3. Indexer Search Hydration Fix

Problem:
- Fresh work searches were sometimes issued using raw `wrk_...` IDs or poor placeholder queries.
- That caused Prowlarr searches to look like they were not really searching for the book.

Result:
- Searches now hydrate the real title from metadata service before sending the query to indexers.

Source files:
- `indexer-service/internal/indexer/metadata_client.go`
- `indexer-service/internal/indexer/orchestrator.go`
- `indexer-service/internal/api/handlers.go`
- `indexer-service/cmd/server/main.go`

Implementation notes:
- Work-ID searches are resolved to metadata titles before dispatch.
- This fixed both manual and scheduled search behavior.

## 4. Auto-Grab Worker Fixes

Problem:
- Auto-grab was missing fresh completed searches after restart because backend polling timestamps and live indexer timestamps did not line up.
- Worker behavior was also too permissive and could grab low-quality junk matches.

Result:
- Auto-grab now sees fresh searches reliably.
- Startup replay window is constrained.
- Candidate filtering is stricter and avoids obvious non-book junk.

Source files:
- `app/backend/internal/autograb/worker.go`
- `app/backend/internal/autograb/worker_test.go`
- `app/backend/internal/integration/indexer/client.go`

Implementation notes:
- Narrowed startup lookback to avoid replay storms.
- Fixed `updated_after` formatting to match live indexer behavior.
- Strengthened title matching and rejected non-book style releases.

## 5. Prowlarr Query Construction and Candidate Quality

Problem:
- Ambiguous titles produced weak candidate ranking.
- Auto-grab sometimes picked poor matches because upstream search terms were too plain.

Result:
- Prowlarr search queries use quoted titles plus preferred-format hints.
- Real ebook-style candidates are easier to rank and grab.

Source files:
- `indexer-service/internal/indexer/prowlarr.go`
- `indexer-service/internal/indexer/prowlarr_test.go`
- `indexer-service/internal/indexer/backends/prowlarr/backend.go`
- `app/backend/internal/autograb/worker.go`
- `app/backend/internal/autograb/worker_test.go`

Implementation notes:
- Pass preferred formats from Bookwyrm into Prowlarr adapter.
- Improve query shaping for ebook releases.
- Auto-grab still remains conservative when search quality is poor.

## 6. Download Queue Fallback Retry

Problem:
- If the first acceptable usenet candidate failed with `403/404/410`, the job stopped instead of trying the next acceptable match.

Result:
- Download submission can now roll to the next candidate from the same search request when the first fetchable candidate is dead.

Source files:
- `app/backend/internal/downloadqueue/manager.go`
- `app/backend/internal/downloadqueue/manager_test.go`
- `app/backend/internal/integration/indexer/client.go`

Implementation notes:
- Jobs carry `search_request_id` and `attempted_candidate_ids`.
- Retry skips already-tried candidates from the same search request.

## 7. NZBGet Handoff Fixes

Problem:
- NZBGet RPC argument ordering was wrong for the deployed NZBGet version.
- Passing redirected Prowlarr download URLs directly to NZBGet caused fetch failures.
- Duplicate/quick-delete outcomes were not represented well in Bookwyrm.

Result:
- Bookwyrm now fetches the NZB content itself and submits NZB content to NZBGet.
- Handoff to NZBGet works for real grabs.
- Naming on submitted NZBs is better.

Source files:
- `app/backend/internal/integration/download/nzbget.go`
- `app/backend/internal/integration/download/nzbget_test.go`

Implementation notes:
- Fix RPC/request behavior for live NZBGet compatibility.
- Prefer fetched NZB content over URL passthrough.
- Some duplicate-history handling work was done locally as well, but verify the final version to upstream if that logic is desired.

## 8. Importer Timing and Cross-Drive Move Fixes

Problem:
- Bookwyrm was creating import jobs from NZBGet paths before post-processing had fully settled.
- Some imports referenced `intermediate` folders that later disappeared.
- Windows rename across `C:` to `H:` was not handled cleanly.

Result:
- Import waits for real terminal completion states.
- Final path selection is more reliable.
- Cross-volume move fallback works on Windows.

Source files:
- `app/backend/internal/integration/download/nzbget.go`
- `app/backend/internal/integration/download/nzbget_test.go`
- `app/backend/internal/importer/engine.go`
- `app/backend/internal/importer/engine_test.go`

Implementation notes:
- Prefer `FinalDir`, then `DestDir`, then `Directory` from NZBGet records.
- Do not mark downloads complete merely because progress reaches 100.
- Treat post-processing phases as still active.
- Recognize Windows cross-device move errors like `different disk drive`.

## 9. Importer Naming and Library Layout Fixes

Problem:
- Imported books fell under `_incoming`-style staging semantics too long.
- Final library layout used `Unknown Author` too often.
- Release naming heuristics assumed `Title - Author`, but many real releases are `Author - Title`.
- Audiobooks should live in a shared folder instead of per-track title folders.

Result:
- Final layout now matches the requested style: `Author/Title/File`.
- Naming uses metadata when available and falls back to release-name heuristics when author metadata is sparse.
- Audiobook multi-track imports land in a common `Author/Title` folder.

Source files:
- `app/backend/internal/importer/engine.go`
- `app/backend/internal/importer/engine_test.go`
- `app/backend/internal/importer/renamer.go`
- `app/backend/internal/importer/renamer_test.go`
- `app/backend/cmd/server/main.go`

Implementation notes:
- Added metadata-backed `BuildNamingPlanWithValues` flow.
- Added fallback parsing for release patterns such as:
  - `Author - Title (retail) (epub)`
  - `Author - 2021 - Title`
- Updated defaults:
  - `NAMING_TEMPLATE_EBOOK={Author}/{Title}/{Title} - {Author}.{Ext}`
  - `NAMING_TEMPLATE_AUDIOBOOK_SINGLE={Author}/{Title}/{Title} - {Author}.{Ext}`
  - `NAMING_TEMPLATE_AUDIOBOOK_FOLDER={Author}/{Title}`
  - `IMPORT_KEEP_INCOMING=false`

## 10. Clearer Missing-Source Import Errors

Problem:
- If an old review job pointed at a deleted NZBGet intermediate folder, the UI showed a generic scan failure that looked like approve did nothing.

Result:
- Import failures now clearly say the source path no longer exists and advise retrying the download.

Source files:
- `app/backend/internal/importer/engine.go`
- `app/backend/internal/importer/engine_test.go`

Implementation notes:
- `processJob` now wraps missing-folder scan failures with a human-readable recovery hint.

## 11. Anna's Archive Metadata Provider Improvements

Problem:
- Anna's Archive was only a scraper fallback with incomplete metadata support.
- Stable IDs and detailed work/edition retrieval were missing.

Result:
- Anna's Archive behaves much more like a proper metadata provider.
- Deterministic IDs are used.
- Detail pages are parsed for richer metadata.

Source files:
- `metadata-service/internal/provider/annasarchive/provider.go`
- `metadata-service/internal/provider/annasarchive/provider_test.go`

Implementation notes:
- Treat `/md5/...` as stable provider key material.
- Generate deterministic work/edition IDs.
- Implement `GetWork` and `GetEditions`.
- Parse title, author, year, publisher, format, and ISBNs from detail pages.

## 12. Metadata Provider Health / Test Behavior

Problem:
- Testing a provider from the UI did not update provider status to `healthy`, which made the provider page misleading.

Result:
- Successful provider tests now record provider health and latency so the UI reflects reality.

Source files:
- `metadata-service/internal/api/handlers.go`
- `metadata-service/cmd/server/main.go`

Implementation notes:
- After a successful test, provider metrics/health state are recorded and visible in provider listing.

## 13. Import Review UI Improvements

Problem:
- The import review page made the approve flow confusing.
- Candidate-row `Approve as this` looked like a real approve action, but only filled the Work ID field.
- The primary approve action did not clearly communicate rerun behavior.

Result:
- The page now better matches actual behavior.

Source files:
- `app/backend/web/src/pages/ImportListPage.tsx`
- `app/backend/web/src/components/CandidateComparisonTable.tsx`

Implementation notes:
- Candidate action renamed to `Use this work ID`.
- Primary button is `Approve & Run Import`.
- Secondary action is `Approve Only`.

## Live-Config / Deployment Notes

These were live-install details, not necessarily source changes to upstream directly:
- `C:\ProgramData\Bookwyrm\config\bookwyrm.env`
- `C:\ProgramData\Bookwyrm\config\metadata-service.yaml`
- Prowlarr and NZBGet live credentials/config
- Anna's Archive live mirror base URL set to `https://annas-archive.gd`

These should be represented upstream as documentation and safe defaults, not as committed local secrets or machine-specific paths.

## Suggested Upstream Verification

Run at minimum:
- `go test ./internal/importer ./internal/integration/download ./internal/downloadqueue` from `app/backend`
- `go test ./internal/autograb` from `app/backend`
- `go test ./internal/indexer` or relevant `indexer-service` packages
- `go test ./internal/provider/annasarchive` from `metadata-service`
- `npm run build` from `app/backend/web`

## Suggested Upstream Review Focus

Reviewers should pay special attention to:
- whether release-name fallback heuristics are too aggressive for imports
- whether auto-grab scoring is conservative enough for ambiguous titles
- whether Anna's Archive parsing should remain best-effort or be feature-flagged
- whether importer retry/approve semantics should explicitly distinguish dead-source jobs in the UI/API model

## Practical Next Step

Use this document to split the copied changes into a few reviewable PRs instead of one giant catch-all PR. The cleanest PR boundaries are:
- importer/naming
- search/autograb/indexer
- NZBGet/download integration
- metadata service / Anna's Archive
- web UI usability
