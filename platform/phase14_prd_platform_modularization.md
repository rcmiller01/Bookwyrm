# Phase 14 PRD — Platform Modularization

## Project: Bookwyrm

---

## Objective

Refactor the system into a stable platform core plus swappable domain packs, without changing runtime behavior.

- **Platform core:** queues, retries/backoff, reliability/tiering/quarantine policy, registries/config overlays, metrics patterns.
- **Domain pack:** query construction, matching, naming variables, file rules, metadata shaping.

**Definition of success:** after Phase 14, a second domain (for example TV) can be added by implementing `domain/tv` and wiring it, without rewriting Indexer/Download/Import architecture.

---

## Non-Goals

- No new end-user features
- No breaking API changes
- No performance tuning beyond regression avoidance

---

## Slice A — Standardize Go Version + Module Hygiene (No Behavior Change)

### Goal

Everything builds/tests cleanly with Go 1.23 and avoids toolchain surprises.

### Tasks

- Set `go 1.23` in:
  - `metadata-service/go.mod`
  - `indexer-service/go.mod`
  - `app/backend/go.mod`
- Run `go mod tidy` in each module.
- Add a small `CONTRIBUTING.md` section:
  - "Go 1.23 required"
  - "Run tests with `go test ./... -count=1`"

### Acceptance

- `go test ./...` passes in each module locally without toolchain fetch attempts.

---

## Slice B — Extract Shared Queue Primitives (Within Each Service First)

### Goal

Create one internal queue toolkit per service to remove copy/paste and standardize semantics.

### Scope

Applicable queues include enrichment, indexer search, download, and import where present.

### Tasks (Per Service)

Create `internal/queue/` package containing:

- `BackoffPolicy` (exponential backoff + jitter + max cap)
- `LockNext(ctx, tx, ...)` helpers for `FOR UPDATE SKIP LOCKED`
- Stats helpers:
  - `CountByStatus()`
  - `NextRunnableAt()`
- Dedupe helper pattern (unique index convention)

Refactor queue stores to use helpers without schema changes if existing fields already support this.

### Acceptance

- No behavioral changes; existing queue tests still pass.
- Semantics for `not_before` / `locked_at` / `locked_by` / `attempt_count` / `max_attempts` remain identical across queues.

---

## Slice C — Extract Policy + Reliability Primitives (Tiering/Quarantine/Recompute)

### Goal

Converge provider/indexer/download policy logic into shared reliability primitives per service.

### Tasks

Create `internal/policy/` (or `internal/reliability/`) package containing:

- Tier enum + `TierForScore(score)`
- `QuarantineMode` config (`last_resort|disabled`)
- `DispatchSortKey(tier, score, priority)`
- `ComputeAvailability`, `ComputeLatencyScore`, and related score composition utilities
- `DecayTowardBaseline()`

Update registries to use the shared policy package.

Standardize policy provenance fields where policy is surfaced:

- `value`
- `source`
- `mode`

### Acceptance

- Provider/indexer/download dispatch order remains unchanged.
- Tests remain green; no new endpoints required.

---

## Slice D — Introduce Domain Boundary Contract (Books as First Domain Module)

### Goal

Define and apply a minimal domain contract so platform pipeline stages are reusable for additional media domains.

### D1) Contract Package

Create:

- `app/backend/internal/domain/contract/`

Interfaces:

```go
type Domain interface {
  Name() string
  QueryBuilder() QueryBuilder
  MatchEngine() MatchEngine
  NamingEngine() NamingEngine
  ImportRules() ImportRules
}

type QueryBuilder interface {
  BuildSearch(workID string, prefs Preferences) QuerySpec
}

type MatchEngine interface {
  Match(ctx context.Context, input MatchInput) MatchResult
}

type NamingEngine interface {
  Plan(ctx context.Context, input NamingInput) (NamingPlan, error)
}

type ImportRules interface {
  SupportedExtensions() map[string]bool
  IsJunk(filename string) bool
  GroupFiles(files []string) []FileGroup
}
```

### D2) Books Domain Pack

Create:

- `app/backend/internal/domain/books/query_builder.go`
- `app/backend/internal/domain/books/match_engine.go`
- `app/backend/internal/domain/books/naming_engine.go`
- `app/backend/internal/domain/books/import_rules.go`

Implementation should preserve existing books behavior:

- query building from title/author/isbn
- matching via filename ISBN/title/author against metadata-service data
- current author-title naming logic
- current ebook/audiobook extensions and junk-file rules

### D3) Pipeline Wiring

Update ImportService and RenameService to consume domain interfaces (not direct book-specific helpers).

Repository note: in this codebase, Import/Rename behavior is currently implemented inside the app/backend importer/renamer pipeline paths rather than split into standalone services. Phase 14 wiring work therefore focuses on domain contract integration and packaging boundaries, not net-new feature capability.

### Acceptance

- Behavior remains identical; existing tests pass.
- Domain switching is feasible by replacing `Domain` implementation at startup.

---

## Slice E — Normalize Cross-Service Entity Identity Shape

### Goal

Standardize internal identity shape for current (books) and future (TV/movies) entity types.

### Tasks

Create `internal/domain/contract/entity.go`:

```go
type EntityRef struct {
  Type string // "work","edition" now; "series","episode" later
  ID string
  ParentIDs map[string]string // e.g. work->edition, series->episode
}
```

Update indexer-service and download queue internals to consistently store:

- `entity_type`
- `entity_id`

(no external API changes)

### Acceptance

- Internal normalization only; no API breaks.
- Phase 12/13 behavior remains intact.

---

## Slice F — Shared Platform Libraries at Repo Root (Optional)

### Goal

Optionally centralize common primitives after per-service standardization is complete.

### Candidate shared modules

- `/platform/queue`
- `/platform/policy`
- `/platform/normalize`
- `/platform/metrics`

### Constraint

Do this only if monorepo shared-module versioning and ownership are acceptable.

### Acceptance

- No circular dependencies
- Test suite remains green
- Stable, clear imports without cross-module spaghetti

---

## Success Criteria (Phase 14)

- Go 1.23 pinned everywhere and tests run offline.
- Queue semantics standardized (`SKIP LOCKED`, backoff, `next_runnable_at`) across services.
- Reliability/tier/quarantine behavior uses shared primitives with no drift.
- Books domain logic is encapsulated under `domain/books` implementing a contract.
- Import/rename pipeline becomes domain-driven and swappable.
- Full suite green:
  - `metadata-service`: `go test ./... -count=1`
  - `indexer-service`: `go test ./... -count=1`
  - `app/backend`: `go test ./... -count=1`

---

## Rollout Notes

- Deliver in slices with strict non-regression checks after each slice.
- Prefer adapter wrappers first, then internal call-site migration, then cleanup.
- Keep all externally visible APIs and runtime behavior stable until post-Phase 14 hardening.

---

## Implementation Status (March 2026)

Current repository state after incremental non-breaking rollout:

- **Slice A:** complete
  - Go 1.23 pinned across `metadata-service`, `indexer-service`, and `app/backend`
  - CI and local test workflows run with `GOTOOLCHAIN=local` compatibility
- **Slice B:** complete (metadata-service)
  - Shared queue primitives standardized and consumed through internal queue package
- **Slice C:** complete (metadata-service)
  - Reliability/tiering/quarantine logic centralized under shared policy primitives
- **Slice D:** complete (app/backend)
  - Domain contract + `domain/books` + domain-driven importer/renamer pipeline wiring
- **Slice E:** complete (indexer-service/app/backend integration path)
  - Internal entity identity normalization (`entity_type`, `entity_id`) with backward compatibility
- **Slice F (optional):** implemented via root shared modules
  - `platform/normalize`
  - `platform/policy`
  - `platform/queue`
  - `platform/metrics`

### Shared Module Boundary Rules

- `platform/*` modules contain reusable primitives only (no service-specific orchestration).
- Service-owned packages (`metadata-service/internal/*`, etc.) remain the compatibility surface and may re-export/wrap shared primitives.
- External APIs remain unchanged; migrations are wrapper-first and behavior-preserving.
- New domain additions (e.g., `domain/tv`) should integrate through `app/backend/internal/domain/contract` without altering platform pipeline architecture.
