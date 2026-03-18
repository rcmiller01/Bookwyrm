# Implementation Plan

## Project: Bookwyrm — Metadata Backbone Service

---

## Phase 1 Developer Implementation Specification

### Objective

Deliver a working metadata resolver service that:

- Accepts book queries
- Resolves metadata using one provider (Open Library)
- Stores canonical metadata in PostgreSQL
- Prevents duplicates
- Exposes a REST API for querying metadata

This version establishes the architecture that future phases expand.

---

## Phase 1 System Architecture

Runtime components inside one Go binary:

- API Server
- Resolver Engine
- Provider Registry
- OpenLibrary Provider
- PostgreSQL Store
- In-memory Cache

No background workers yet. All requests flow through the resolver.

---

## Project Repository Structure

```
metadata-service/
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── api/
│   │   ├── handlers.go
│   │   └── router.go
│   │
│   ├── resolver/
│   │   ├── resolver.go
│   │   ├── merge.go
│   │   └── fingerprint.go
│   │
│   ├── provider/
│   │   ├── provider.go
│   │   ├── registry.go
│   │   └── openlibrary/
│   │       └── provider.go
│   │
│   ├── model/
│   │   ├── author.go
│   │   ├── work.go
│   │   ├── edition.go
│   │   └── identifier.go
│   │
│   ├── store/
│   │   ├── db.go
│   │   ├── works.go
│   │   ├── authors.go
│   │   ├── editions.go
│   │   └── identifiers.go
│   │
│   ├── cache/
│   │   └── cache.go
│   │
│   └── config/
│       └── config.go
│
├── migrations/
│
├── configs/
│   └── config.yaml
│
├── docker/
│   └── docker-compose.yml
│
└── go.mod
```

---

## Database Setup

- **Database:** PostgreSQL
- **Migration tool:** `golang-migrate`
- **Migration files:** `/migrations`

---

## Phase 1 Database Schema

### Authors

```sql
CREATE TABLE authors (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    sort_name   TEXT,
    created_at  TIMESTAMP,
    updated_at  TIMESTAMP
);
```

### Works

```sql
CREATE TABLE works (
    id                TEXT PRIMARY KEY,
    title             TEXT NOT NULL,
    normalized_title  TEXT,
    fingerprint       TEXT,
    first_pub_year    INTEGER,
    created_at        TIMESTAMP,
    updated_at        TIMESTAMP
);

CREATE INDEX idx_works_title       ON works(normalized_title);
CREATE INDEX idx_works_fingerprint ON works(fingerprint);
```

### Work Authors

```sql
CREATE TABLE work_authors (
    work_id    TEXT,
    author_id  TEXT,
    PRIMARY KEY(work_id, author_id)
);
```

### Editions

```sql
CREATE TABLE editions (
    id                TEXT PRIMARY KEY,
    work_id           TEXT,
    title             TEXT,
    format            TEXT,
    publisher         TEXT,
    publication_year  INTEGER,
    created_at        TIMESTAMP,
    updated_at        TIMESTAMP
);
```

### Identifiers

```sql
CREATE TABLE identifiers (
    id          SERIAL PRIMARY KEY,
    edition_id  TEXT,
    type        TEXT,
    value       TEXT
);

CREATE UNIQUE INDEX idx_identifier_unique ON identifiers(type, value);
```

### Provider Mappings

```sql
CREATE TABLE provider_mappings (
    id            SERIAL PRIMARY KEY,
    provider      TEXT,
    provider_id   TEXT,
    entity_type   TEXT,
    canonical_id  TEXT
);

CREATE UNIQUE INDEX idx_provider_mapping ON provider_mappings(provider, provider_id);
```

---

## Configuration

**File:** `configs/config.yaml`

```yaml
server:
  port: 8080

database:
  host: localhost
  port: 5432
  user: metadata
  password: metadata
  dbname: metadata

providers:
  openlibrary:
    enabled: true
    timeout_seconds: 5
```

---

## Provider Interface

**Location:** `internal/provider/provider.go`

```go
type Provider interface {
    Name() string

    SearchWorks(ctx context.Context, query string) ([]model.Work, error)

    GetWork(ctx context.Context, providerID string) (*model.Work, error)

    GetEditions(ctx context.Context, providerWorkID string) ([]model.Edition, error)

    ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error)
}
```

---

## Provider Registry

**Location:** `internal/provider/registry.go`

**Purpose:**

- Register providers
- Return enabled providers
- Allow resolver to iterate providers

```go
type Registry struct {
    providers map[string]Provider
}
```

---

## Open Library Provider

Initial metadata provider.

**Endpoints used:**

```
https://openlibrary.org/search.json
https://openlibrary.org/works/{id}.json
https://openlibrary.org/isbn/{isbn}.json
```

**Responsibilities:**

- Convert Open Library responses to canonical models
- Return normalized results

**Location:** `internal/provider/openlibrary/provider.go`

---

## Query Normalization

All queries are normalized before processing.

**Normalization rules:**

- Lowercase
- Remove punctuation
- Collapse whitespace
- Normalize author formatting

```go
func NormalizeQuery(q string) string
```

Used in both the resolver and provider adapters.

---

## Fingerprint Generation

**Location:** `internal/resolver/fingerprint.go`

**Formula:**

```
normalized_title + "|" + normalized_author + "|" + publication_year
```

**Example:**

```
dune|frankherbert|1965
```

Used for duplicate detection.

---

## Resolver Engine

**Location:** `internal/resolver/resolver.go`

**Pipeline:**

```
normalize query
       ↓
  check cache
       ↓
search canonical DB
       ↓
if confident result → return
       ↓
  query providers
       ↓
  merge results
       ↓
store canonical metadata
       ↓
  return response
```

Providers run concurrently using goroutines.

---

## Merge Logic

**Location:** `internal/resolver/merge.go`

**Responsibilities:**

- Cluster results by similarity
- Merge metadata fields
- Attach editions
- Assign confidence score

---

## Cache Layer

**Location:** `internal/cache/cache.go`

**Library:** `github.com/dgraph-io/ristretto`

**Used for:**

- Query results
- Identifier lookups

---

## API Server

| File | Location |
|---|---|
| Router | `internal/api/router.go` |
| Handlers | `internal/api/handlers.go` |

**Router library:** `gorilla/mux`

---

## Phase 1 API Endpoints

### Search Works

```
GET /v1/search?q=dune
```

**Response:**

```json
{
  "works": [...]
}
```

### Resolve Identifier

```
GET /v1/resolve?isbn=9780441013593
```

### Get Work

```
GET /v1/work/{id}
```

Returns: work, authors, editions, identifiers.

---

## Docker Deployment

**File:** `docker/docker-compose.yml`

**Services:** `metadata-service`, `postgres`

```yaml
version: '3'

services:

  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: metadata
      POSTGRES_PASSWORD: metadata
      POSTGRES_DB: metadata

  metadata-service:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - postgres
```

---

## Phase 1 Success Criteria

The service can:

- Search books by title
- Resolve ISBN
- Store canonical metadata
- Prevent duplicate works
- Run continuously without crashes

**Latency targets:**

| Query type | Target |
|---|---|
| Cached queries | <500ms |
| Provider queries | <2s |

---

## Phase 1 Deliverable

Running metadata resolver accessible via REST API.

**Example query:**

```
GET /v1/search?q=dune
```

Returns canonical metadata for *Dune*.

---

## Phase 2 Build-On

Once Phase 1 is stable, Phase 2 introduces:

- Multiple providers
- Provider reliability scoring
- Provider configuration API

Phase 1 already contains the architecture to support it.

---

## Full Implementation Roadmap

### Phase 1 — Core Metadata Resolver (MVP)

**Goal:** A working service that can resolve metadata queries and persist canonical data.

**Capabilities:**

- PostgreSQL schema
- Query normalization
- OpenLibrary provider
- Resolver pipeline
- Canonical metadata storage
- Duplicate prevention
- REST API
- In-memory cache

**Deliverable:** `GET /v1/search?q=dune` returns normalized metadata. This proves the architecture.

---

### Phase 2 — Multi-Provider System

**Goal:** Introduce redundancy and improve metadata coverage.

**Capabilities:**

- Provider registry
- Provider configuration system
- Provider priority ordering
- Parallel provider resolution
- Provider health monitoring
- Provider rate limiting

**Providers added:** Google Books, Hardcover

**Database additions:** `provider_configs`, `provider_status`

**Deliverable:** The resolver works even if one provider fails.

---

### Phase 3 — Provider Reliability Engine

**Goal:** Automatically determine which providers are trustworthy.

**Capabilities:**

- Reliability scoring algorithm
- Provider availability tracking
- Latency measurement
- Metadata agreement scoring
- Identifier usefulness tracking

**Resolver improvements:**

- Weighted provider execution
- Merge weighting based on reliability

**Database additions:** `provider_metrics`, `provider_reliability_scores`

**Deliverable:** Resolver automatically adapts to provider quality.

---

### Phase 4 — Query Interpretation Engine

**Goal:** Improve how user queries are understood.

**Capabilities:**

Query classification:
- Identifier detection
- Title/author parsing
- Format hints (audiobook, epub)
- Fuzzy search routing

**Database upgrades:**

- PostgreSQL trigram indexes
- FTS search tables

**Deliverable:** User searches feel dramatically smarter.

**Examples:**

```
dune audiobook
9780441013593
herbert dune messiah
```

---

### Phase 5 — Metadata Enrichment Engine

**Goal:** Move from reactive metadata to proactive expansion.

**Capabilities:**

- Background job queue
- Enrichment workers
- Author expansion
- Series expansion
- Edition enrichment
- Metadata refresh scheduler

**Database additions:** `enrichment_jobs`, `job_history`

**Deliverable:** Searching one book quietly populates dozens more.

---

### Phase 6 — Metadata Graph Layer

**Goal:** Convert relational metadata into a navigable knowledge graph.

**Capabilities:**

Relationship tables:
- `work_relationships`
- `work_subjects`
- `series_entries`

Graph relationships:
- Author connections
- Series ordering
- Subject similarity
- Related works

**Deliverable:** Graph traversal queries become possible.

---

### Phase 7 — Recommendation Engine

**Goal:** Use the metadata graph for discovery.

**Capabilities:**

- Graph traversal algorithms
- Recommendation scoring
- Subject similarity
- Author similarity
- Series continuation

**Example output:**

```
Because you searched Dune:
- Dune Messiah
- Hyperion
- Foundation
```

This phase leverages everything built earlier.

---

### Phase 8 — Advanced Metadata Sources

**Goal:** Integrate richer metadata sources.

**Providers added:** Anna's Archive, LibraryThing *(optional)*, WorldCat *(if feasible)*

**Capabilities:**

- Edition discovery from shadow libraries
- Torrent/mirror metadata
- Rare book coverage

**Database additions:** `content_sources`, `file_metadata`

**Deliverable:** Edition coverage becomes dramatically better.

---

### Phase 9 — Metadata Quality Engine

**Goal:** Detect and repair metadata inconsistencies.

**Capabilities:**

- Graph anomaly detection
- Conflicting publication year detection
- Duplicate edition detection
- Identifier verification

**Example:** Series entry appears out of order → flagged.

This improves long-term metadata quality.

---

### Phase 10 — Metadata Service Platform

**Goal:** Turn the system into a general-purpose metadata service.

**Capabilities:**

- Stable public API
- API authentication
- API rate limiting
- Client SDKs
- Documentation

This allows other tools to integrate easily.

---

### Phase 11 — Automation Client *(Future Project)*

Not part of this service but enabled by it.

A new Readarr-like system would use the metadata backbone for:

- Monitoring authors
- Monitoring series
- Searching indexers
- Downloading releases

The metadata service becomes the core intelligence layer.

---

## Why This Roadmap Works

Each phase adds a new capability layer:

| Phase | Capability |
|---|---|
| 1 | Metadata storage |
| 2 | Redundancy |
| 3 | Provider intelligence |
| 4 | Smarter queries |
| 5 | Proactive expansion |
| 6 | Knowledge graph |
| 7 | Discovery |
| 8 | Rare metadata |
| 9 | Metadata quality |
| 10 | Ecosystem platform |

Every phase is independently useful.

---

## Development Order Recommendation

Build through Phases 1–7 before tackling advanced metadata sources. That sequence delivers the biggest functional gain early.

---

## Important Observation

By Phase 6 you will have built something that no current self-hosted book system has: **a continuously improving metadata knowledge graph.**

Readarr, Calibre, Kavita, and similar tools all rely on static metadata. This system would be alive.
