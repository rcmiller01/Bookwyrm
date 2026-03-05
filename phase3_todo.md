# Phase 3 Implementation Todo List

## Project: Bookwyrm — Provider Reliability Engine

---

## Tasks

| # | Task | Status |
|---|---|---|
| 1 | Provider metrics collection struct (`internal/provider/metrics.go`) | [x] |
| 2 | Provider metrics store (`internal/store/provider_metrics.go`) | [x] |
| 3 | Database schema migration (`migrations/000003_provider_metrics.up.sql`) | [x] |
| 4 | Reliability score engine (`internal/provider/reliability.go`) | [x] |
| 5 | Reliability score store (`internal/store/provider_reliability.go`) | [x] |
| 6 | Reliability update worker (`internal/provider/reliability_worker.go`) | [x] |
| 7 | Resolver integration — sort providers by reliability DESC (`internal/resolver/resolver.go`) | [x] |
| 8 | Resolver integration — record success/failure to metrics store (`internal/resolver/resolver.go`) | [x] |
| 9 | Merge weighting — reliability-weighted field conflict resolution (`internal/resolver/merge.go`) | [x] |
| 10 | Provider health status thresholds updated (`internal/provider/health.go`) | [x] |
| 11 | Quarantine providers with score < 0.40 and apply dispatch policy (`last_resort` or `disabled`) | [x] |
| 12 | Reliability decay — score regresses toward 0.7 after 30 days inactivity (`internal/provider/reliability.go`) | [x] |
| 13 | API: `GET /v1/providers/reliability` (`internal/api/handlers.go`) | [x] |
| 14 | API: `GET /v1/providers/{name}/reliability` (`internal/api/handlers.go`) | [x] |
| 15 | Router registration for reliability endpoints (`internal/api/router.go`) | [x] |
| 16 | Observability: `provider_success_total`, `provider_reliability_score` gauges (`internal/metrics/provider_metrics.go`) | [x] |
| 17 | Wire-up in `cmd/server/main.go` (new stores + reliability worker) | [x] |
| 18 | Tests: reliability score calculation (`internal/provider/reliability_test.go`) | [x] |
| 19 | Tests: metrics store operations (`internal/provider/metrics_test.go`) | [x] |
| 20 | Tests: resolver integration — sort, merge weighting, decay, disabling (`internal/resolver/reliability_integration_test.go`) | [x] |

---

## Phase 3 Success Criteria

- [x] `GET /v1/providers/reliability` returns a score for every registered provider
- [x] Resolver dispatches providers sorted by `composite_score DESC`
- [x] Merge engine resolves field conflicts using reliability weights
- [x] Repeated successes increase score; repeated failures decrease score
- [x] Score decays toward 0.7 after 30 days of inactivity
- [x] Providers with score < 0.40 are quarantined and handled by dispatch policy (`last_resort` by default, optionally `disabled`)
- [x] All Phase 3 tests pass (`go test ./internal/provider/... ./internal/resolver/...`)

---

## Files Created

| File | Purpose |
|---|---|
| `migrations/000003_provider_metrics.up.sql` | `provider_metrics` + `provider_reliability` tables |
| `migrations/000003_provider_metrics.down.sql` | Down migration |
| `internal/store/provider_metrics.go` | `ProviderMetrics` struct + `ProviderMetricsStore` interface + pgx implementation |
| `internal/store/provider_reliability.go` | `ReliabilityScore` struct + `ReliabilityStore` interface + pgx implementation |
| `internal/provider/metrics.go` | Type alias `provider.ProviderMetrics = store.ProviderMetrics` |
| `internal/provider/reliability.go` | `ComputeScore()`, `HealthStatus()`, decay logic, score constants |
| `internal/provider/reliability_worker.go` | Background worker — recomputes scores every 5 min and updates reliability status |
| `internal/metrics/provider_metrics.go` | `provider_success_total` counter + `provider_reliability_score` gauge |
| `internal/provider/reliability_test.go` | Unit tests: score calculation, decay, health status |
| `internal/provider/metrics_test.go` | Unit tests: metrics store operations, worker integration |
| `internal/resolver/reliability_integration_test.go` | Integration tests: sort order, merge weighting, quarantine behavior, decay |

## Files Modified

| File | Change |
|---|---|
| `internal/resolver/merge.go` | Added `ProviderResult` type; upgraded `Merger` interface with `MergeWorksWeighted` |
| `internal/resolver/resolver.go` | Sorts providers by reliability, records to metrics store, passes `ProviderResult` to weighted merge |
| `internal/provider/health.go` | Phase 3 score thresholds; `WithReliabilityStore` method for status label updates |
| `internal/api/types.go` | `ReliabilityInfo`, `ReliabilityListResponse`, `ReliabilityDetailResponse` types |
| `internal/api/handlers.go` | `ListReliabilityScores`, `GetProviderReliability` handlers; `reliabilityStore` field |
| `internal/api/router.go` | `GET /v1/providers/reliability` + `GET /v1/providers/{name}/reliability` routes |
| `cmd/server/main.go` | Wires `ProviderMetricsStore`, `ReliabilityStore`, `ReliabilityWorker`, updated handler constructor |

---

## Reference Documents

- [PRD.md](PRD.md)
- [implementation_plan.md](implementation_plan.md)
- [Metadata_Service_Query_Flow.md](Metadata_Service_Query_Flow.md)
- [architecture_diagram.md](architecture_diagram.md)
