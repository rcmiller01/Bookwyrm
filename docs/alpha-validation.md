# Phase 23 Alpha Validation Test Matrix

This document is the operational validation gate for alpha candidates.

Goal: prove Bookwyrm works outside the dev machine across zip distribution, launcher/service lifecycle, pipeline behavior, failure recovery, scale, and supportability.

## Scope

- Zip extraction/setup behavior
- Launcher and service behavior
- Startup and readiness correctness
- End-to-end media pipeline
- Failure injection and recovery
- Upgrade/cutoff workflows
- Performance at realistic scale
- Support bundle and docs usability

## Test Environments

| Environment | Description | Purpose |
|---|---|---|
| A | Clean Windows 11 VM | First-time non-technical install experience |
| B | Existing Windows media server | Power-user integration with real paths and existing tools |
| C | Hybrid deployment (native services + Docker Postgres) | Validate recommended deployment shape |
| D | Failure injection environment | Controlled dependency and process breakage testing |

## Slice A: Clean Install Validation

### A1) Zip Validation

Steps:

1. Extract `bookwyrm-<version>-windows.zip` on Environment A.
2. Confirm folders are created (`bin`, `config`, `logs`, `data`).
3. Start launcher using `bookwyrm-launcher.exe run --base-dir <extract-path>`.
4. Confirm setup wizard/checklist appears.


Expected:

- Setup is possible from extracted files and config templates.
- UI reachable at `http://localhost:8090`.
- Logs/config files present.

### A2) Setup Checklist Validation

Validate checklist accuracy for:

- Library root missing/invalid
- Metadata/indexer availability
- At least one metadata provider enabled
- At least one search backend enabled
- At least one download client enabled
- DB ready and migrations applied

Expected:

- Checklist reflects actual state.
- Incomplete items deep-link to the correct settings page.

## Slice B: Launcher / Service Validation

### B1) Startup Sequence

Test cases:

- Normal startup
- Metadata delayed by 15s
- Indexer delayed by 15s
- Backend delayed by 15s
- Readiness failure due to invalid/missing library root

Expected:

- Launcher waits for readiness, not only liveness.
- Failures provide actionable diagnostics.

### B2) Service Lifecycle

Validate:

- `install-service`
- `start-service`
- `stop-service`
- `uninstall-service`
- Reboot persistence (if available in environment)

Expected:

- Stable service mode
- Accessible logs after restart/reboot cycles

## Slice C: End-to-End Pipeline Validation

### C1) Metadata Flow

Test:

- Common author/work search
- Metadata detail rendering
- Recommendations / next-in-series
- ISBN lookup
- DOI lookup (if used)
- Series with multiple entries

Expected:

- Results accurate enough for normal use
- Recommendation graph quality acceptable for alpha

### C2) Manual Search -> Grab

Validate:

- Candidate ranking quality
- One manual grab => one download job
- Staged search threshold behavior
- Preferred sources influence ordering
- No duplicate active grabs for same release

### C3) Download Handoff

Per enabled client (as available in environment):

- NZBGet
- SABnzbd
- qBittorrent

Validate:

- Add succeeds
- Downstream ID tracked
- Status mapping is coherent
- Output path captured

### C4) Import Pipeline

Validate:

- Correct final import path
- Naming template applied
- `_incoming` behavior correct
- `download_job.imported=true`
- `library_items` row created

Expected:

- Import is idempotent and deterministic

## Slice D: Edge Cases & Human Review

### D1) `needs_review` Flow

Trigger:

- Ambiguous title/author
- Collision with different size/format
- Low-confidence match

Validate:

- Item enters `needs_review`
- Candidate comparison is usable
- `approve & rerun` works
- `keep_both` / `replace_existing` / `skip` work

### D2) Upgrade / Cutoff Unmet

Validate:

- Profile cutoff flags unmet items
- Upgrade search flow runs
- Replacement/keep-both behavior is correct

## Slice E: Failure & Recovery Validation

### E1) Dependency Outage Tests

Simulate:

- Prowlarr down
- Download client down
- Metadata service down
- Indexer service down
- Postgres unavailable

Validate:

- `health-detail` shows cause
- Dashboard/Status guidance is actionable
- Support bundle captures evidence
- "Fix It" actions remain safe

### E2) Mid-flight Interruption

Simulate:

- Kill launcher during download
- Kill backend during import
- Kill indexer during queued searches
- Reboot host mid-pipeline

Validate:

- Stuck jobs recover
- Reconciliation resumes correctly
- No permanent "running forever" state

## Slice F: Performance / Scale Validation

### F1) Moderate Library Scale

Dataset target:

- 1,000 books
- 100 authors
- Mixed ebook/audiobook

Validate:

- List/filter responsiveness
- Queue page polling behavior
- Import throughput
- Search responsiveness

### F2) Large-list UI

Validate:

- Virtualization on Books/Authors/Wanted pages
- Saved views behavior
- Filters + bulk actions responsiveness

## Slice G: Supportability Validation

### G1) Support Bundle Drill

Generate bundles for:

- Healthy system
- Degraded system
- Failed downloader scenario

Validate:

- Logs present
- Redaction correct
- Version/migration/health snapshots included
- Bundle sufficient for triage

### G2) Docs Usability

Run a fresh-user docs walkthrough:

- [windows-native.md](windows-native.md)
- [windows-service.md](windows-service.md)
- [windows-installer.md](windows-installer.md)
- [postgres-hybrid.md](postgres-hybrid.md)

Capture confusion points and missing steps.

## Slice H: Release Candidate Gate

Alpha candidate requires all of:

- Clean install passes on at least one clean Windows 11 VM
- Hybrid deployment passes on one real Windows host
- Full pipeline run passes for:
  - ebook
  - audiobook
  - upgrade/collision workflow
- Support bundle validation passes
- Service restart/reboot recovery verified
- Automated tests green
- Release artifact produced:
  - zip

## Suggested Validation Matrix

| Area | Test | Env A | Env B | Env C | Pass Criteria |
|---|---|---|---|---|---|
| Install | Zip extraction + startup | ☐ | ☐ | — | UI opens, checklist visible |
| Launcher | Startup + readiness | ☐ | ☐ | ☐ | Ready within timeout or actionable failure |
| Metadata | Search + graph + recs | ☐ | ☐ | ☐ | Accurate enough for use |
| Search | Manual search + grab | ☐ | ☐ | ☐ | One grab, no duplicates |
| Download | NZBGet/SAB/qB handoff | — | ☐ | ☐ | Output path captured |
| Import | Rename + library_items | ☐ | ☐ | ☐ | Final path correct |
| Review | `needs_review` resolve | ☐ | ☐ | ☐ | Approve/decide works |
| Recovery | Restart mid-flight | ☐ | ☐ | ☐ | Jobs recover |
| Support | Bundle + docs | ☐ | ☐ | ☐ | Actionable support output |

## Execution Record Template

For each validation run, capture:

- Build SHA:
- Release tag:
- Date/time:
- Tester:
- Environment:
- Failed cases:
- Notes:
- Support bundle attached (Y/N):
- Final result (`pass`/`fail`):

