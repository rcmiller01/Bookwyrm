# Metadata Service Query Flow

## Project: Bookwyrm — Metadata Backbone Service

Below is the canonical data flow for a single query using the example:

GET /v1/search?q=dune
Metadata Service Query Data Flow
User / Client
      │
      ▼
API Router
      │
      ▼
Query Normalization
      │
      ▼
Query Classification
      │
      ├──────── Identifier Query? ────────► Identifier Resolver
      │
      ▼
Cache Lookup
      │
      ├──────── Cache Hit ───────────────► Return Response
      │
      ▼
Canonical DB Search
      │
      ├──────── High Confidence Result ──► Cache Result → Return Response
      │
      ▼
Resolver Engine
      │
      ▼
Provider Dispatcher
      │
      ├──────── OpenLibrary Worker
      ├──────── GoogleBooks Worker (future)
      ├──────── Hardcover Worker (future)
      └──────── Anna's Archive Worker (future)
      │
      ▼
Provider Result Channel
      │
      ▼
Merge + Scoring Engine
      │
      ▼
Identity Resolution
      │
      ▼
Database Transaction
      │
      ├──────── Update canonical records
      ├──────── Insert identifiers
      └──────── Update provider mappings
      │
      ▼
Cache Result
      │
      ▼
Return API Response
Step-by-Step Execution Specification

This is the exact order of operations the resolver should follow.

The IDE should treat this as the authoritative flow.

Step 1 — API Entry

Endpoint:

GET /v1/search?q=dune

Handler location:

internal/api/handlers.go

Handler responsibilities:

• validate request
• extract query parameter
• pass query to resolver

Example:

func SearchHandler(w http.ResponseWriter, r *http.Request)
Step 2 — Query Normalization

Location:

internal/resolver/normalize.go

Normalize the query so comparisons are deterministic.

Rules:

• lowercase
• remove punctuation
• collapse whitespace

Example:

"Dune: Frank Herbert"

becomes

dune frank herbert

Function:

NormalizeQuery(query string) string
Step 3 — Query Classification

Location:

internal/resolver/query_classifier.go

Determine query type.

Possible types:

identifier
structured
fuzzy

Example rules:

13 digits → ISBN
10 alphanumeric → ASIN
otherwise → text search

Output structure:

type QueryType struct {
    IsIdentifier bool
    IdentifierType string
}
Step 4 — Cache Lookup

Location:

internal/cache/cache.go

Check in-memory cache first.

Key:

search:<normalized_query>

Example:

search:dune

If cache hit:

Return response immediately.

Step 5 — Canonical Database Search

Location:

internal/store/works.go

Search canonical metadata.

Example query:

SELECT *
FROM works
WHERE normalized_title ILIKE '%dune%'
LIMIT 10;

If high-confidence results exist:

• return results
• update cache

No provider queries needed.

Step 6 — Resolver Provider Dispatch

Location:

internal/resolver/resolver.go

Resolver retrieves enabled providers from registry.

Example:

providers := registry.EnabledProviders()

Workers are launched concurrently.

Example pattern:

for _, p := range providers {
    go worker.SearchWorks(query)
}

Provider responses return via channel.

Step 7 — Provider Adapters

Location:

internal/provider/openlibrary/provider.go

Provider responsibilities:

• call external API
• map response to canonical model

Example provider API:

https://openlibrary.org/search.json?q=dune

Mapped fields:

title
author
publication year
edition identifiers
Step 8 — Merge Engine

Location:

internal/resolver/merge.go

Responsibilities:

• cluster works by similarity
• merge metadata fields
• attach editions
• assign confidence score

Example clustering signals:

title similarity
author similarity
identifier match
Step 9 — Identity Resolution

Location:

internal/resolver/identity.go

Prevent duplicates.

Steps:

check provider mapping
check fingerprint index
run similarity comparison
create or attach canonical work

Fingerprint example:

dune|frankherbert|1965
Step 10 — Database Transaction

Location:

internal/store/

All writes occur inside a transaction.

Operations:

insert work
insert authors
insert editions
insert identifiers
insert provider mapping

Example transaction pattern:

tx, err := db.Begin()
defer tx.Rollback()

Commit only after successful merge.

Step 11 — Cache Result

Cache the final response.

Key:

search:<query>

TTL recommendation:

1 hour
Step 12 — API Response

Return normalized JSON response.

Example:

{
  "works": [
    {
      "id": "wrk_123",
      "title": "Dune",
      "authors": ["Frank Herbert"],
      "first_pub_year": 1965,
      "confidence": 0.94
    }
  ]
}
Expected Latency Behavior

Cold query:

1–2 seconds

Cached query:

<100ms
Observability Hooks (Phase 1)

Add basic metrics.

Expose:

/metrics

Metrics tracked:

resolver_requests_total
provider_requests_total
cache_hits_total
cache_misses_total
resolver_latency_ms
Phase 1 Runtime Flow Summary
Client Query
     │
Normalize
     │
Cache
     │
Canonical DB
     │
Providers
     │
Merge
     │
Identity Resolution
     │
DB Transaction
     │
Cache Result
     │
Return Response

---

## Phase 1 Core Interface Contracts

All interfaces live under `internal/`. These contracts define the metadata models, provider interfaces, resolver interfaces, storage interfaces, and cache interface.

---

### Metadata Models

**Location:** `internal/model/`

These structs represent canonical metadata used everywhere in the system.

#### Author

```go
package model

type Author struct {
    ID        string
    Name      string
    SortName  string
    CreatedAt int64
    UpdatedAt int64
}
```

#### Work

A work represents the intellectual work (not a specific edition).

```go
package model

type Work struct {
    ID              string
    Title           string
    NormalizedTitle string
    FirstPubYear    int
    Fingerprint     string

    Authors   []Author
    Editions  []Edition

    Confidence float64
}
```

#### Edition

Represents a specific published edition.

```go
package model

type Edition struct {
    ID              string
    WorkID          string
    Title           string
    Format          string
    Publisher       string
    PublicationYear int

    Identifiers []Identifier
}
```

#### Identifier

Identifiers connect metadata across providers.

```go
package model

type Identifier struct {
    Type  string
    Value string
}
```

**Examples:** `ISBN`, `ASIN`, `OpenLibraryID`

---

### Provider Interfaces

**Location:** `internal/provider/`

Providers adapt external APIs into canonical metadata.

#### Provider Interface

```go
package provider

import (
    "context"
    "metadata-service/internal/model"
)

type Provider interface {
    Name() string

    SearchWorks(ctx context.Context, query string) ([]model.Work, error)

    GetWork(ctx context.Context, providerID string) (*model.Work, error)

    GetEditions(ctx context.Context, providerWorkID string) ([]model.Edition, error)

    ResolveIdentifier(
        ctx context.Context,
        idType string,
        value string,
    ) (*model.Edition, error)
}
```

#### Provider Registry Interface

```go
package provider

type Registry interface {
    Register(p Provider)

    Get(name string) (Provider, bool)

    EnabledProviders() []Provider
}
```

This allows providers to register themselves.

---

### Resolver Interfaces

**Location:** `internal/resolver/`

#### Resolver Interface

```go
package resolver

import (
    "context"
    "metadata-service/internal/model"
)

type Resolver interface {
    SearchWorks(ctx context.Context, query string) ([]model.Work, error)

    ResolveIdentifier(
        ctx context.Context,
        idType string,
        value string,
    ) (*model.Edition, error)

    GetWork(ctx context.Context, id string) (*model.Work, error)
}
```

#### Merge Engine Interface

```go
package resolver

import "metadata-service/internal/model"

type Merger interface {
    MergeWorks(results [][]model.Work) ([]model.Work, error)
}
```

#### Identity Resolver Interface

```go
package resolver

import "metadata-service/internal/model"

type IdentityResolver interface {
    ResolveWork(work model.Work) (string, error)
}
```

Returns the canonical work ID.

---

### Storage Interfaces

**Location:** `internal/store/`

All database operations pass through this layer.

#### Work Store

```go
package store

import (
    "context"
    "metadata-service/internal/model"
)

type WorkStore interface {
    GetWorkByID(ctx context.Context, id string) (*model.Work, error)

    SearchWorks(ctx context.Context, query string) ([]model.Work, error)

    InsertWork(ctx context.Context, work model.Work) error

    UpdateWork(ctx context.Context, work model.Work) error
}
```

#### Author Store

```go
package store

import (
    "context"
    "metadata-service/internal/model"
)

type AuthorStore interface {
    InsertAuthor(ctx context.Context, author model.Author) error

    GetAuthorByName(ctx context.Context, name string) (*model.Author, error)
}
```

#### Edition Store

```go
package store

import (
    "context"
    "metadata-service/internal/model"
)

type EditionStore interface {
    InsertEdition(ctx context.Context, edition model.Edition) error

    GetEditionsByWork(ctx context.Context, workID string) ([]model.Edition, error)
}
```

#### Identifier Store

```go
package store

import (
    "context"
    "metadata-service/internal/model"
)

type IdentifierStore interface {
    InsertIdentifier(ctx context.Context, editionID string, id model.Identifier) error

    FindEditionByIdentifier(
        ctx context.Context,
        idType string,
        value string,
    ) (*model.Edition, error)
}
```

#### Provider Mapping Store

```go
package store

import "context"

type ProviderMappingStore interface {
    GetCanonicalID(
        ctx context.Context,
        provider string,
        providerID string,
    ) (string, error)

    InsertMapping(
        ctx context.Context,
        provider string,
        providerID string,
        canonicalID string,
    ) error
}
```

---

### Cache Interface

**Location:** `internal/cache/`

Abstract cache layer allows swapping implementations.

```go
package cache

import "time"

type Cache interface {
    Get(key string) (interface{}, bool)

    Set(key string, value interface{}, ttl time.Duration)

    Delete(key string)
}
```

Phase 1 implementation uses `ristretto`.

---

### Configuration

**Location:** `internal/config/`

```go
package config

type Config struct {
    Server struct {
        Port int
    }

    Database struct {
        Host     string
        Port     int
        User     string
        Password string
        DBName   string
    }

    Providers map[string]ProviderConfig
}

type ProviderConfig struct {
    Enabled bool
    Timeout int
}
```

---

### API Contract Types

**Location:** `internal/api/types.go`

#### Search Response

```go
package api

import "metadata-service/internal/model"

type SearchResponse struct {
    Works []model.Work `json:"works"`
}
```

#### Work Response

```go
package api

import "metadata-service/internal/model"

type WorkResponse struct {
    Work model.Work `json:"work"`
}
```

---

### Concurrency Pattern Contract

Provider queries must run concurrently.

```go
results := make(chan []model.Work)

for _, provider := range providers {
    go func(p Provider) {
        r, err := p.SearchWorks(ctx, query)
        if err == nil {
            results <- r
        }
    }(provider)
}
```

The resolver aggregates results from the channel.

---

### Phase 1 Dependency List

| Module | Purpose |
|---|---|
| `gorilla/mux` | HTTP router |
| `pgx` | PostgreSQL driver |
| `ristretto` | In-memory cache |
| `golang-migrate` | Database migrations |
| `zerolog` | Structured logging |
