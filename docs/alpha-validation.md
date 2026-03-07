# Alpha Validation Matrix

This document records required validation for each alpha candidate release.

## Installer Validation (Clean Machines)

| Scenario | Status | Notes |
|---|---|---|
| Windows 11 clean VM | Pending | Validate installer, service registration, browser launch, setup wizard |
| Windows with existing media tools | Pending | Validate path coexistence and port conflicts |
| Docker Desktop installed (hybrid DB) | Pending | Validate Postgres connectivity and first-run completion |
| Docker Desktop not installed (external DB) | Pending | Validate external DSN flow and error guidance |

## Functional Pipeline Validation

| Test case | Status | Notes |
|---|---|---|
| Metadata search for common authors | Pending | |
| Manual search scoring + explainability | Pending | |
| Grab and download handoff | Pending | |
| Import pipeline success path | Pending | |
| Needs-review decision workflow | Pending | |
| Upgrade/cutoff behavior | Pending | |
| Wanted automation checks | Pending | |
| Recommendation graph generation | Pending | |

## Required Go/No-Go Checks

- `GET /api/v1/system/dependencies` returns `can_function_now=true`
- `GET /api/v1/system/migration-status` returns `status=ok`
- Support bundle export succeeds and redacts secrets
- Installer and zip artifacts are attached to release

## Signoff

- QA Owner:
- Build SHA:
- Release Tag:
- Date:
