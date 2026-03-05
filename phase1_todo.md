# Phase 1 Implementation Todo List

## Project: Bookwyrm — Metadata Backbone Service

---

## Tasks

| # | Task | Status |
|---|---|---|
| 1 | Initialize Go module & repo structure | [x] |
| 2 | Set up `go.mod` with dependencies (`gorilla/mux`, `pgx`, `ristretto`, `golang-migrate`, `zerolog`) | [x] |
| 3 | Create Docker Compose stack (`metadata-service` + `postgres`) | [x] |
| 4 | Write database migration files (authors, works, work_authors, editions, identifiers, provider_mappings) | [x] |
| 5 | Implement canonical data models (`Author`, `Work`, `Edition`, `Identifier`) | [x] |
| 6 | Implement config loader (YAML → `Config` struct) | [x] |
| 7 | Implement PostgreSQL store layer (`WorkStore`, `AuthorStore`, `EditionStore`, `IdentifierStore`, `ProviderMappingStore`) | [x] |
| 8 | Implement cache layer (`ristretto` backed `Cache` interface) | [x] |
| 9 | Implement provider interface & registry | [x] |
| 10 | Implement Open Library provider adapter | [x] |
| 11 | Implement query normalization (`NormalizeQuery`) | [x] |
| 12 | Implement query classifier (identifier vs. text) | [x] |
| 13 | Implement fingerprint generation | [x] |
| 14 | Implement merge engine | [x] |
| 15 | Implement identity resolver (duplicate prevention) | [x] |
| 16 | Implement resolver engine (full pipeline with concurrent provider dispatch) | [x] |
| 17 | Implement API router & handlers (`/v1/search`, `/v1/resolve`, `/v1/work/{id}`) | [x] |
| 18 | Wire up `main.go` entrypoint | [x] |
| 19 | Add Prometheus `/metrics` endpoint | [x] |
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
