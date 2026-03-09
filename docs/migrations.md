# Bookwyrm Migration Semantics

This document is the source of truth for local migration behavior.

## Current Behavior

Migrations run automatically at startup:

- app-backend: runs embedded backend migrations when DATABASE_DSN is set
- indexer-service: runs embedded indexer migrations when DATABASE_DSN is set
- metadata-service: runs embedded metadata migrations during service startup

Manual migration execution is not required for normal local development.

## Migration Tracking Tables

For shared-DB mode, each service records migration state in a separate table:

- backend_schema_migrations
- indexer_schema_migrations
- metadata_schema_migrations

This prevents version-number collisions between services that each have migration files like 000001_*.sql.

## Shared Database Mode (Default Alpha Path)

In the root docker-compose path, all services target the same Postgres database by default.

Key requirement:

- each service must point to the intended database for your install mode

If one service points to a different DB, startup may look healthy while features fail with missing-table symptoms.

## Quick Checks

Connect to Postgres and inspect migration history:

SELECT * FROM backend_schema_migrations ORDER BY version;
SELECT * FROM indexer_schema_migrations ORDER BY version;
SELECT * FROM metadata_schema_migrations ORDER BY version;

If one table is empty while the service is running, that service likely did not run migrations in the DB you are checking.

## Docker Volume Caveat

Postgres initialization is persistent across restarts when a volume already exists.
Changing compose variables does not retroactively recreate schema or databases in an existing volume.

To fully reinitialize local Docker state:

1. docker compose down -v
2. docker compose up --build

Use caution: this deletes local compose-managed Postgres data.
