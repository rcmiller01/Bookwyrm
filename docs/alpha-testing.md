# Alpha Testing Guide

This guide is for external alpha testers validating end-to-end behavior.

Primary operator checklist: [alpha-validation.md](alpha-validation.md).

## What To Test

- Setup flow (first-run checklist and dependency readiness)
- Metadata search and author/work discovery
- Manual search candidate quality and explainability
- Grab to download-client handoff
- Import pipeline (including `needs_review`)
- Upgrade/cutoff behavior for monitored wanted items
- Queue and status page behavior under normal and degraded conditions

## Known Alpha Limitations

- UX polish and visual consistency are still in progress.
- Some provider/indexer combinations may require manual tuning.
- Windows installer flow is prioritized; other packaging modes may evolve faster than docs.

## Recommended Validation Sequence

1. Install Bookwyrm and complete first-run checklist.
2. Add one metadata provider, one search backend, one download client.
3. Monitor a small set of authors/books.
4. Trigger search/manual search and grab one release.
5. Validate download state transitions and import results.
6. Force one needs-review case and resolve it.
7. Export support bundle and confirm no sensitive secrets are exposed.

## Reporting Issues

Please include:

- Bookwyrm version
- OS version
- Deployment mode (Windows native, Docker, hybrid)
- Download client and indexer/backend used
- Support bundle (`Status -> Download Support Bundle`)
- Steps to reproduce and expected vs actual behavior

Use the GitHub bug report template for alpha issues.
