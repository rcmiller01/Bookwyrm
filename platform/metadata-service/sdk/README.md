# Bookwyrm Client SDKs (Phase 10)

This folder provides lightweight client SDK starters for integrating with the stable `v1` API.

## Go SDK

Location: `sdk/go/bookwyrm/client.go`

```go
client := bookwyrm.NewClient("http://localhost:8080", bookwyrm.Options{
    APIKey: "your-api-key",
})

report, err := client.GetQualityReport(context.Background(), 25)
```

## Python SDK

Location: `sdk/python/bookwyrm_client.py`

```python
from bookwyrm_client import BookwyrmClient

client = BookwyrmClient("http://localhost:8080", api_key="your-api-key")
report = client.get_quality_report(limit=25)
```

## Supported methods (both SDKs)

- Search works (`GET /v1/search`)
- Fetch work (`GET /v1/work/{id}`)
- Get quality report (`GET /v1/quality/report`)
- Run quality repair (`POST /v1/quality/repair`)

These SDKs are intentionally small and can be expanded into full-featured clients later.
