# Observability Baseline

Bookwyrm exposes health, readiness, dependency, and metrics endpoints for operational monitoring.

## Required Endpoints

- `GET /api/v1/healthz`
- `GET /api/v1/readyz`
- `GET /api/v1/system/status`
- `GET /api/v1/system/health-detail`
- `GET /api/v1/system/dependencies`
- `GET /api/v1/system/migration-status`
- `GET /metrics` (service-level Prometheus format)

## Functional Readiness Invariant

Use `GET /api/v1/system/dependencies` as the operational go/no-go signal:

- `can_function_now=true` means the stack is actually usable (not just "up").
- It includes dependency checks for backend, metadata, indexer, DB, library root, search backend, and download client availability.

## Prometheus Integration

Scrape all running services:

- app/backend metrics endpoint
- indexer-service metrics endpoint
- metadata-service metrics endpoint

Track:

- request volume and latency
- queue depth and failure rates
- provider/backend reliability values
- readiness/dependency state transitions

## Minimal Ops Alerting

Alert when:

- `readyz` fails for >5 minutes
- `can_function_now` flips false
- failed imports/downloads climb continuously
- migration status reports pending/failed after startup grace period
