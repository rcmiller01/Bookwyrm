# Bookwyrm

## Provider Dispatch Policy (Phase 3)

Provider reliability now supports a quarantine tier (`score < 0.40`).

- Default behavior: quarantine providers are **last-resort** (still dispatchable, ordered last).
- Optional behavior: quarantine providers are **disabled** (skipped from dispatch).

### Recommended config (extensible)

```yaml
providers:
	dispatch_policy:
		quarantine_mode: last_resort # last_resort | disabled
```

### Alternate boolean config (also supported)

```yaml
providers:
	quarantine:
		disable_dispatch: false
```

If both shapes are present, `dispatch_policy.quarantine_mode` takes precedence.

## Recommendation Engine (Phase 7)

Recommendation APIs are available for graph-based discovery:

- `GET /v1/work/{id}/recommendations`
- `GET /v1/work/{id}/next`
- `GET /v1/work/{id}/similar`

Supported query parameters:

- `limit` (bounded to 100)
- `include` (comma-separated: `series`, `author`, `subjects`, `relationships`)
- `formats` (comma-separated preference values)
- `languages` (comma-separated preference values)

Runtime behavior:

- Results are deterministic (`score DESC`, then `work_id ASC`).
- Responses include explainability through `reasons`.
- Recommendation caching and scoring defaults are controlled via the `recommendation` config block in `configs/config.yaml` (or defaults in code when omitted).

## Advanced Metadata Sources (Phase 8)

Optional provider adapters were added for broader edition discovery coverage:

- `annasarchive`
- `librarything`
- `worldcat`

These providers are disabled by default and can be enabled in `configs/config.yaml`.

Phase 8 also adds additive schema tables:

- `content_sources`
- `file_metadata`

Migration files:

- `migrations/000006_advanced_metadata_sources.up.sql`
- `migrations/000006_advanced_metadata_sources.down.sql`

## Metadata Quality Engine (Phase 9)

Quality APIs are available for inconsistency detection and targeted repair:

- `GET /v1/quality/report`
- `POST /v1/quality/repair`

Supported capabilities:

- Graph anomaly detection for series entries with missing/duplicate ordering indexes
- Conflicting publication year detection across editions of the same work
- Duplicate edition cluster detection (report-only)
- Identifier verification for ISBN-10 and ISBN-13 values

Repair behavior:

- Supports dry-run mode via `{"dry_run": true}`
- Reorders malformed series entry indexes into deterministic sequence order
- Normalizes `works.first_pub_year` to the minimum known edition publication year for conflicted works
- Optionally removes invalid ISBN identifiers (`remove_invalid_identifiers`, default true)

## Metadata Service Platform (Phase 10)

The public `v1` API is now treated as a stable integration surface.

Platform features:

- Stable API version header on all `v1` responses: `X-Bookwyrm-API-Version: v1`
- Optional API authentication via `X-API-Key` or `Authorization: Bearer <key>`
- Configurable API rate limiting with standard headers
- Starter client SDKs for Go and Python
- Public API contract documentation

Configuration (`configs/config.yaml`):

```yaml
api:
	auth:
		enabled: false
		keys: []
	rate_limit:
		enabled: true
		requests_per_minute: 120
		burst: 20
```

Environment overrides:

- `API_AUTH_ENABLED`
- `API_AUTH_KEYS` (comma-separated)
- `API_RATE_LIMIT_ENABLED`
- `API_RATE_LIMIT_RPM`
- `API_RATE_LIMIT_BURST`

Additional docs and SDKs:

- `metadata-service/docs/api_v1.md`
- `metadata-service/sdk/README.md`