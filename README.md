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