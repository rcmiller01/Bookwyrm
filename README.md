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