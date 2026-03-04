# Phase 1 Implementation Todo List

## Project: Bookwyrm — Metadata Backbone Service

---

## Tasks

| # | Task | Status |
|---|---|---|
| 1 | Initialize Go module & repo structure | [ ] |
| 2 | Set up `go.mod` with dependencies (`gorilla/mux`, `pgx`, `ristretto`, `golang-migrate`, `zerolog`) | [ ] |
| 3 | Create Docker Compose stack (`metadata-service` + `postgres`) | [ ] |
| 4 | Write database migration files (authors, works, work_authors, editions, identifiers, provider_mappings) | [ ] |
| 5 | Implement canonical data models (`Author`, `Work`, `Edition`, `Identifier`) | [ ] |
| 6 | Implement config loader (YAML → `Config` struct) | [ ] |
| 7 | Implement PostgreSQL store layer (`WorkStore`, `AuthorStore`, `EditionStore`, `IdentifierStore`, `ProviderMappingStore`) | [ ] |
| 8 | Implement cache layer (`ristretto` backed `Cache` interface) | [ ] |
| 9 | Implement provider interface & registry | [ ] |
| 10 | Implement Open Library provider adapter | [ ] |
| 11 | Implement query normalization (`NormalizeQuery`) | [ ] |
| 12 | Implement query classifier (identifier vs. text) | [ ] |
| 13 | Implement fingerprint generation | [ ] |
| 14 | Implement merge engine | [ ] |
| 15 | Implement identity resolver (duplicate prevention) | [ ] |
| 16 | Implement resolver engine (full pipeline with concurrent provider dispatch) | [ ] |
| 17 | Implement API router & handlers (`/v1/search`, `/v1/resolve`, `/v1/work/{id}`) | [ ] |
| 18 | Wire up `main.go` entrypoint | [ ] |
| 19 | Add Prometheus `/metrics` endpoint | [ ] |
| 20 | Validate Phase 1 success criteria (<500ms cached, <2s provider queries, no crashes) | [ ] |

---

## Phase 1 Success Criteria

- [ ] `GET /v1/search?q=dune` returns canonical metadata
- [ ] `GET /v1/resolve?isbn=9780441013593` resolves a known edition
- [ ] `GET /v1/work/{id}` returns work with authors, editions, and identifiers
- [ ] Duplicate works are prevented via fingerprint matching
- [ ] Service runs continuously without crashes
- [ ] Cached query latency < 500ms
- [ ] Provider query latency < 2s

---

## Reference Documents

- [PRD.md](PRD.md)
- [implementation_plan.md](implementation_plan.md)
- [Metadata_Service_Query_Flow.md](Metadata_Service_Query_Flow.md)
- [architecture_diagram.md](architecture_diagram.md)
