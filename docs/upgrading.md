# Upgrading Bookwyrm

This guide covers upgrading a Docker Compose deployment to a new release.

## Prerequisites

- Docker and Docker Compose installed
- Access to the Bookwyrm repository or pre-built images
- The current deployment is healthy (`curl localhost:8090/api/v1/readyz` returns 200)

## Upgrade Steps

### 1. Back up your databases

```bash
docker compose exec postgres pg_dumpall -U bookwyrm > bookwyrm_backup_$(date +%Y%m%d).sql
```

Verify the backup file is non-empty:

```bash
ls -lh bookwyrm_backup_*.sql
```

### 2. Pull the latest code or images

If building from source:

```bash
git pull origin main
```

If using pre-built images, pull the latest tags:

```bash
docker compose pull
```

### 3. Rebuild and restart

```bash
docker compose up -d --build
```

Docker Compose will rebuild changed images and restart containers in dependency order (postgres first, then metadata + indexer, then backend).

### 4. Verify health

Wait a few seconds for services to start, then check:

```bash
# Liveness check
curl -s localhost:8090/api/v1/healthz | jq .

# Readiness check (verifies all dependencies)
curl -s localhost:8090/api/v1/readyz | jq .

# Full system status
curl -s localhost:8090/api/v1/system/status | jq .
```

All checks should return `"status": "ok"`. The readyz endpoint returns 503 if any dependency is unreachable.

### 5. Verify the UI

Open `http://localhost:8090` in your browser. The Dashboard footer shows the running version. The Status page shows version details for all services.

Use `Status -> Support & Recovery -> Download Support Bundle` to capture a pre/post-upgrade diagnostics artifact.

## Rollback

If something goes wrong after upgrading:

### 1. Stop the new containers

```bash
docker compose down
```

### 2. Checkout the previous version

```bash
git checkout <previous-tag-or-commit>
```

### 3. Restore the database backup

```bash
docker compose up -d postgres
# Wait for postgres to be ready
docker compose exec -T postgres psql -U bookwyrm < bookwyrm_backup_YYYYMMDD.sql
```

### 4. Rebuild and start the old version

```bash
docker compose up -d --build
```

### 5. Verify

```bash
curl -s localhost:8090/api/v1/healthz | jq .
```

## Migration Notes

- Database migrations run automatically on service startup (see [migrations.md](migrations.md) for details).
- New migrations are forward-only in production. If you need to roll back a migration, restore from backup.
- Always back up before upgrading, especially across major version bumps.
- If migrations fail or services are degraded, capture a support bundle and review `system/migration-status.json` plus service health snapshots.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Service won't start | `docker compose logs <service-name>` for startup errors |
| readyz returns 503 | Check which dependency is failing in the response `checks` object |
| Migration failed | Check service logs; restore from backup and file an issue |
| UI shows old version | Hard-refresh the browser (Ctrl+Shift+R) to clear cached assets |
| Port conflict | Adjust `*_PORT` variables in `.env` |
