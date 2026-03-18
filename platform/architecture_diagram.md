# Architecture Diagram

## Project: Bookwyrm — Metadata Backbone Service

                           ┌──────────────────────────────┐
                           │           CLIENTS            │
                           │                              │
                           │  Readarr Replacement         │
                           │  Library Managers            │
                           │  CLI Tools                   │
                           │  Automation Systems          │
                           │  Future APIs / SDKs          │
                           └──────────────┬───────────────┘
                                          │
                                          ▼
                          ┌────────────────────────────────┐
                          │           API SERVER           │
                          │                                │
                          │  REST Endpoints                │
                          │  Query Normalization           │
                          │  Query Classification          │
                          │  Authentication (future)       │
                          └──────────────┬─────────────────┘
                                         │
                                         ▼
                       ┌─────────────────────────────────────┐
                       │            RESOLVER ENGINE           │
                       │                                     │
                       │  Cache Lookup                        │
                       │  Canonical DB Search                 │
                       │  Provider Query Dispatcher           │
                       │  Merge & Scoring Logic               │
                       │  Identity Resolution                 │
                       └───────────────┬─────────────────────┘
                                       │
                 ┌─────────────────────┼─────────────────────┐
                 │                     │                     │
                 ▼                     ▼                     ▼
      ┌───────────────────┐  ┌────────────────────┐  ┌────────────────────┐
      │  PROVIDER SYSTEM  │  │ ENRICHMENT ENGINE  │  │ RELIABILITY ENGINE │
      │                   │  │                    │  │                    │
      │ Provider Registry │  │ Background Workers │  │ Provider Metrics   │
      │ Provider Adapters │  │ Author Expansion   │  │ Success Tracking   │
      │ Rate Limiting     │  │ Series Expansion   │  │ Latency Tracking   │
      │ Capability Flags  │  │ Edition Discovery  │  │ Agreement Scoring  │
      └───────────┬───────┘  └───────────┬────────┘  └───────────┬────────┘
                  │                      │                       │
                  └──────────────┬───────┴───────────────┬───────┘
                                 │                       │
                                 ▼                       ▼
                    ┌────────────────────────────────────────┐
                    │          METADATA GRAPH LAYER           │
                    │                                        │
                    │ Works                                  │
                    │ Authors                                │
                    │ Series                                 │
                    │ Subjects                               │
                    │ Work Relationships                     │
                    │ Recommendation Signals                 │
                    └───────────────┬────────────────────────┘
                                    │
                                    ▼
                     ┌───────────────────────────────────┐
                     │        POSTGRESQL DATABASE        │
                     │                                   │
                     │ Canonical Metadata                │
                     │ Provider Mappings                 │
                     │ Identifiers                       │
                     │ Editions                          │
                     │ Query Cache                       │
                     │ Enrichment Jobs                   │
                     │ Provider Reliability Metrics      │
                     └───────────────┬───────────────────┘
                                     │
                                     ▼
                         ┌─────────────────────────┐
                         │    METADATA PROVIDERS   │
                         │                         │
                         │ OpenLibrary             │
                         │ Google Books            │
                         │ Hardcover               │
                         │ Anna's Archive          │
                         │ Future Sources          │
                         └─────────────────────────┘