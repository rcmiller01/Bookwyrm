# Troubleshooting

## First checks

1. Open `Status` page and resolve degraded checks.
2. Check `GET /api/v1/system/dependencies` and confirm `can_function_now=true`.
3. Check `GET /api/v1/system/migration-status` and confirm migrations are not pending/failed.
4. Run `Support & Recovery` actions:
   - `Test Connections`
   - `Retry Failed Downloads`
   - `Retry Failed Imports`
   - `Run Cleanup`
   - `Open Logs Folder`
5. Download a support bundle and attach it to bug reports.

## Common issues

### `readyz` is degraded

- Check `Status` page service rows.
- Check backend startup logs for `startup warning:` entries (metadata/indexer/DB/download clients).
- Confirm service URLs:
  - `METADATA_SERVICE_URL`
  - `INDEXER_SERVICE_URL`

### Downloads fail repeatedly

- Verify download client credentials/URLs.
- Use `Test Connections`.
- Retry failed downloads from `Status` page.

### Imports stuck in `needs_review`

- Open `Import List`.
- Pick `keep_both`, `replace_existing`, or `skip`.
- Retry failed imports from `Status` page.

### Wanted searches not running

- Ensure indexer backends are enabled and not quarantined.
- Run `Rerun Wanted Searches`.
- Check Indexer reliability tiers.

### Enrichment lagging

- Check Metadata service health and enrichment stats.
- Run `Rerun Enrichment`.

## Collect diagnostics

- Use support bundle export.
- If running with local logs, provide:
  - launcher log (if applicable)
  - backend log
  - indexer log
  - metadata log
