# Product Requirements Document

## Project: Metadata Backbone Service

---

## Working Name

**Bookwyrm** *(placeholder)*

---

## Product Type

Self-hosted metadata resolution and enrichment service for books.

---

## Core Goal

Build a reliable, extensible metadata backbone that resolves book metadata across multiple providers, merges results into a canonical database, and continuously enriches a metadata graph that supports discovery, automation, and recommendations.

This service replaces the fragile single-provider metadata layer used by existing book automation tools.

---

## Problem Statement

Current book automation tools rely on one or two fragile metadata providers. When those providers fail, searches break, metadata quality degrades, and automation becomes unreliable.

**Specific issues:**

- Metadata provider outages break search
- Inconsistent metadata across providers
- Poor edition resolution
- Missing identifiers (ISBN/ASIN/etc)
- No metadata enrichment over time
- No provider reliability management

The result is an unreliable user experience.

---

## Solution Overview

The Metadata Backbone Service will:

- Aggregate metadata from multiple providers
- Resolve and merge results into canonical records
- Deduplicate works and editions
- Cache metadata locally
- Expand metadata using background enrichment
- Maintain provider reliability scoring
- Build a metadata graph enabling discovery and recommendations

The service will run as a lightweight Go application backed by PostgreSQL.

---

## Target Users

**Primary users:**

- Self-hosted media server operators
- Readarr / automation users
- Ebook and audiobook collectors

**Secondary users:**

- Library management software
- Automation tools
- Metadata-driven applications

---

## System Architecture

**Core runtime components:**

- API Server
- Resolver Engine
- Provider Plugin System
- Metadata Graph Engine
- Enrichment Workers
- Provider Reliability Monitor
- PostgreSQL Database

The Go service runs as a single binary containing these modules.

**Deployment model:** Docker Compose stack

**Services:**
- `metadata-service`
- `postgresql`

---

## Core Design Principles

| Principle | Description |
|---|---|
| **Resilient** | Provider failures never break search. |
| **Self-improving** | Metadata graph improves over time. |
| **Modular** | Providers are plug-in modules. |
| **Self-host friendly** | Runs quietly for months. |
| **Transparent** | Provider health and reliability visible. |

---

## Technology Stack

| Component | Technology |
|---|---|
| Language | Go |
| Database | PostgreSQL |
| Cache | In-memory (Ristretto) |
| Containerization | Docker |
| Metrics | Prometheus endpoint |
| Configuration | YAML + database overrides |

---

## Data Model Overview

**Key entities:**

- Author
- Work
- Edition
- Identifier
- Series
- Subject
- ProviderMapping
- WorkRelationships

**Example structure:**

```
Author → Work → Edition → Identifier
```

Canonical IDs are generated internally and never change.

---

## Provider System

Providers act as adapters that convert external metadata APIs into the canonical schema.

**Initial providers:**

- Open Library
- Google Books
- Hardcover
- Anna's Archive

**Provider capabilities:**

- `search`
- `identifier lookup`
- `edition lookup`
- `content sources`

Each provider exposes a standard interface. Provider configuration is stored in PostgreSQL and adjustable by the user.

---

## Resolver Engine

The resolver orchestrates metadata resolution.

**Pipeline:**

```
normalize query
       ↓
  check cache
       ↓
search canonical database
       ↓
if low confidence → query providers
       ↓
merge and score results
       ↓
store canonical metadata
       ↓
  return response
```

Provider queries execute concurrently using goroutines.

---

## Identity Resolution

Duplicate prevention uses four mechanisms:

1. Provider ID mapping
2. Fingerprint matching
3. Similarity scoring
4. Canonical ID stability

Fingerprints combine normalized title, author, and year.

**Example:**

```
dune|frankherbert|1965
```

---

## Metadata Graph Layer

Relationships between works are stored explicitly.

**Examples:**

- Author relationships
- Series relationships
- Subject relationships
- Related works

**This graph enables:**

- Recommendations
- Series discovery
- Author exploration

---

## Prefetch / Enrichment Engine

A background worker system expands metadata automatically.

**Example expansions:**

- Author catalog expansion
- Series expansion
- Edition enrichment
- Identifier completion

Jobs are stored in PostgreSQL and processed by worker threads. Prefetch behavior respects user preferences and provider rate limits.

---

## Provider Reliability System

Providers are dynamically scored using:

- Availability
- Latency
- Metadata agreement
- Identifier usefulness

**Composite reliability score determines:**

- Provider execution priority
- Metadata merge weighting

**Provider status states:**

- `Healthy`
- `Degraded`
- `Unreliable`
- `Disabled`

---

## Query Interpretation Layer

Incoming queries are classified before resolution.

**Types:**

- Identifier queries
- Title/author queries
- Format-aware queries
- Fuzzy searches

**Examples:**

```
9780441013593           → identifier query
dune herbert            → title/author query
dune audiobook          → format-aware query
```

PostgreSQL full-text search handles fuzzy queries.

---

## API Overview

**Primary endpoints:**

```
# Search works
GET /v1/search?q=dune

# Resolve identifier
GET /v1/resolve?isbn=9780441013593

# Get work
GET /v1/work/{id}

# Provider management
GET  /v1/providers
POST /v1/providers
POST /v1/providers/{id}/test
```

---

## Observability

**Metrics exposed via:** `/metrics`

**Metrics include:**

- Provider latency
- Resolver execution time
- Cache hit rate
- Provider success rate

Structured logging used throughout.

---

## Phase Roadmap

### Phase 1 — Core Metadata Resolver (MVP)

**Goal:** Deliver a functional metadata resolver capable of answering book metadata queries.

**Deliverables:**

- PostgreSQL schema
- Canonical data models
- Resolver pipeline
- Query normalization system
- Provider plugin architecture
- Open Library provider adapter
- Basic caching layer
- Identifier resolution
- Minimal REST API

**Capabilities:**

- Search works
- Resolve ISBN
- Store canonical metadata
- Prevent duplicates
- Return normalized metadata responses

> No enrichment or graph traversal yet.

**Success Criteria:** Service can resolve metadata for common books with reliable results.

---

### Phase 2 — Multi-Provider Integration

Add additional providers.

**Deliverables:**

- Google Books provider
- Hardcover provider
- Provider configuration system
- Provider health monitoring
- Provider rate limiting
- Provider reliability scoring

**Capabilities:**

- Multi-provider resolution
- Provider prioritization
- Provider failure resilience

---

### Phase 3 — Metadata Enrichment Engine

Introduce background workers.

**Deliverables:**

- Enrichment job queue
- Author expansion jobs
- Series expansion jobs
- Edition enrichment
- Metadata refresh scheduler

**Capabilities:**

- Automatic metadata expansion
- Prefetching based on user preferences

---

### Phase 4 — Metadata Graph

Introduce graph relationships.

**Deliverables:**

- Work relationship tables
- Subject indexing
- Series graph expansion
- Graph traversal queries

**Capabilities:**

- Series discovery
- Related works discovery

---

### Phase 5 — Recommendation Engine

Use metadata graph for discovery.

**Deliverables:**

- Recommendation scoring engine
- User preference modeling
- Subject-based recommendations
- Series progression suggestions

**Capabilities:**

- Personalized discovery

---

### Phase 6 — Full Metadata Service

Finalize the metadata backbone as a reusable infrastructure service.

**Deliverables:**

- Anna's Archive provider
- Content source indexing
- Advanced query interpretation
- API stabilization
- Documentation for external clients

**Capabilities:**

- Production-ready metadata backbone
- Integration target for Readarr replacement

---

## Non-Goals (Initial Phases)

This service will not initially handle:

- Downloading books
- Library file management
- Torrent management
- User interfaces

Those belong in automation clients.

---

## Success Metrics

| Metric | Target |
|---|---|
| Search success rate | >95% |
| Provider failure resilience | ✓ |
| Metadata resolution latency (cached) | <500ms |
| Resolver uptime | >99% |

---

## Long-Term Vision

This metadata service becomes the core infrastructure layer for book automation tools.

Automation systems (Readarr replacement, audiobook managers, ebook libraries) can consume the API and benefit from the shared metadata graph.

Over time the system evolves into a knowledge graph for books, enabling automation, discovery, and recommendations.
