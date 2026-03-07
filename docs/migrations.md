# Database Migrations

Bookwyrm uses per-service PostgreSQL databases, each with its own migration set. Migrations run automatically on service startup when a `DATABASE_DSN` (or equivalent) is configured.

## Migration Ownership

| Service           | Database              | Migration Directory                        | Runner                                  |
|-------------------|-----------------------|--------------------------------------------|-----------------------------------------|
| metadata-service  | `bookwyrm_metadata`   | `metadata-service/migrations/`             | External (golang-migrate or manual)     |
| indexer-service   | `bookwyrm_indexer`    | `indexer-service/internal/indexer/migrations/` | `indexer.RunMigrations()` (embedded)  |
| app/backend       | `bookwyrm_backend`    | `app/backend/internal/downloadqueue/migrations/` | `downloadqueue.RunMigrations()` (embedded) |

## Numbering Scheme

Migration files follow the pattern `{NNNNNN}_{description}.{up|down}.sql`:

- **`NNNNNN`** - Zero-padded integer version (e.g., `000001`)
- **`description`** - Snake-case summary (e.g., `initial_schema`)
- **`up`** - Forward migration
- **`down`** - Rollback migration

Versions are globally unique within each service. The indexer-service starts at `000006` to leave room for future shared-schema migrations if needed.

## Migration Inventory

### metadata-service (7 migrations)

| Version | Name                          | Tables Created/Modified |
|---------|-------------------------------|------------------------|
| 000001  | initial_schema                | authors, works, work_authors, editions, identifiers, provider_mappings |
| 000002  | provider_management           | provider_configs, provider_status |
| 000003  | provider_metrics              | provider_metrics, provider_reliability |
| 000004  | enrichment_jobs               | enrichment_jobs, enrichment_job_runs |
| 000005  | metadata_graph                | series, series_entries, subjects, work_subjects, work_relationships |
| 000006  | advanced_metadata_sources     | Extends metadata source support |
| 000007  | provider_expansion            | Extends provider configuration |

### indexer-service (5 migrations)

| Version | Name                          | Tables Created/Modified |
|---------|-------------------------------|------------------------|
| 000006  | indexer_core                  | indexer_backends, mcp_servers, indexer_search_requests, indexer_candidates, indexer_grabs |
| 000007  | indexer_reliability           | indexer_metrics, indexer_reliability |
| 000008  | search_request_lease          | Adds lease columns to indexer_search_requests |
| 000009  | wanted_monitoring             | indexer_wanted_works, indexer_wanted_authors |
| 000010  | profiles_and_cutoff           | indexer_profiles, indexer_profile_qualities |

### app/backend (4 migrations)

| Version | Name                                       | Tables Created/Modified |
|---------|--------------------------------------------|------------------------|
| 000001  | download_core                              | download_clients, download_jobs, download_events |
| 000002  | download_reliability_and_import_flag       | download_client_metrics, download_client_reliability |
| 000003  | import_jobs_core                           | import_jobs, import_events, library_items |
| 000004  | job_leases                                 | Adds lease columns to download_jobs |

## Auto-Run Behavior

- **indexer-service** and **app/backend** embed migrations using Go's `//go:embed` directive and run them automatically at startup via their respective `RunMigrations()` functions.
- **metadata-service** migrations are in `metadata-service/migrations/` and can be applied with [golang-migrate](https://github.com/golang-migrate/migrate) or manually.
- All services track applied migrations in a `schema_migrations` table within their respective database.
- Migrations are idempotent: only unapplied migrations execute.
- Each migration runs inside a database transaction with automatic rollback on failure.
- Runtime migration visibility is surfaced through system diagnostics (Status page + support bundle export).

## Backup and Restore

### Backup all databases

```bash
# From the host or a container with pg_dump available:
pg_dump -h localhost -U bookwyrm -d bookwyrm_metadata -F c -f metadata_backup.dump
pg_dump -h localhost -U bookwyrm -d bookwyrm_indexer  -F c -f indexer_backup.dump
pg_dump -h localhost -U bookwyrm -d bookwyrm_backend  -F c -f backend_backup.dump
```

### Restore

```bash
pg_restore -h localhost -U bookwyrm -d bookwyrm_metadata --clean --if-exists metadata_backup.dump
pg_restore -h localhost -U bookwyrm -d bookwyrm_indexer  --clean --if-exists indexer_backup.dump
pg_restore -h localhost -U bookwyrm -d bookwyrm_backend  --clean --if-exists backend_backup.dump
```

### Docker Compose shortcut

```bash
# Backup
docker compose exec postgres pg_dumpall -U bookwyrm > bookwyrm_full_backup.sql

# Restore
cat bookwyrm_full_backup.sql | docker compose exec -T postgres psql -U bookwyrm
```

## Adding New Migrations

1. Create `{next_version}_{description}.up.sql` and `{next_version}_{description}.down.sql` in the service's migration directory.
2. For embedded runners (indexer-service, app/backend), the new files are automatically picked up at compile time.
3. For metadata-service, apply with `migrate -path migrations/ -database "$DATABASE_URL" up`.
4. Always test the down migration to ensure clean rollback.
