# Stele Self-Hosting

## Overview

`Stele` runs as three process modes against one PostgreSQL instance:

- `api`: public ingest, memory read, retrieval, context assembly, and admin inspection or lifecycle routes
- `worker`: continuous governance processing loop for claimed raw events
- `scheduler`: periodic maintenance dispatch for summary compaction, retention sweep, and job execution cleanup for the configured default scope

The repository ships a production-oriented `Dockerfile` plus a local `docker-compose.yml` so operators can boot the full stack without reconstructing runtime wiring by hand.

## Prerequisites

- Docker 26+ and Docker Compose v2
- PostgreSQL 17+ with:
  - `pgcrypto`
  - `vector` from `pgvector`
- One reachable DSN for all three Stele modes

The bundled compose file uses `pgvector/pgvector:pg17`, which already includes the required `vector` extension.

## Runtime Variables

Required in all modes:

- `STELE_MODE`: one of `api`, `worker`, `scheduler`
- `STELE_POSTGRES_DSN`: PostgreSQL connection string

Common optional variables:

- `STELE_HTTP_ADDR`: listen address for `api` mode, default `:8080`
- `STELE_AUTH_API_KEYS`: comma-separated public API keys for `/v1/events`, `/v1/memories/search`, `/v1/context/assemble`
- `STELE_AUTH_ADMIN_API_KEYS`: comma-separated admin API keys for `/v1/admin/...`
- `STELE_AUTH_DEFAULT_TENANT`: default scheduler tenant scope
- `STELE_AUTH_DEFAULT_PROJECT`: default scheduler project scope
- `STELE_AUTH_DEFAULT_NAMESPACE`: default scheduler namespace scope

Job tuning variables:

- `STELE_JOBS_MAINTENANCE_INTERVAL`: scheduler cadence, default `15m`
- `STELE_JOBS_SUMMARY_COMPACTION_INTERVAL`: summary compaction cadence, default `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_RETENTION_INTERVAL`: retention and expiry sweep cadence, default `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_CLEANUP_INTERVAL`: maintenance cleanup cadence, default `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_JOB_EXECUTION_RETENTION`: retention window for finished job execution records, default `168h`
- `STELE_JOBS_WORKER_POLL_INTERVAL`: worker idle poll delay, default `5s`
- `STELE_JOBS_WORKER_ERROR_BACKOFF`: worker retry backoff, default `15s`
- `STELE_JOBS_SCHEDULER_ERROR_BACKOFF`: scheduler retry backoff, default `30s`

## Local Bootstrap With Compose

Start the stack:

```bash
docker compose up --build -d
```

Watch the service come up:

```bash
docker compose ps
docker compose logs -f api worker scheduler
```

Stop the stack:

```bash
docker compose down
```

Reset the stack including PostgreSQL data:

```bash
docker compose down -v
```

## Manual Image Build

Build the service image directly:

```bash
docker build -t stele:local .
```

Run API mode manually:

```bash
docker run --rm -p 8080:8080 \
  -e STELE_MODE=api \
  -e STELE_HTTP_ADDR=:8080 \
  -e STELE_POSTGRES_DSN='postgres://stele:stele@host.docker.internal:5432/stele?sslmode=disable' \
  -e STELE_AUTH_API_KEYS=dev-public-key \
  -e STELE_AUTH_ADMIN_API_KEYS=dev-admin-key \
  stele:local
```

## Smoke Check

1. Check liveness:

```bash
curl http://localhost:8080/health
```

Expected:

```json
{"status":"ok"}
```

2. Check readiness:

```bash
curl http://localhost:8080/ready
```

Expected:

```json
{"status":"ready"}
```

3. Ingest one event:

```bash
curl -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-public-key' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"event_type":"conversation.message","content":"user prefers concise answers"}'
```

4. Inspect governance backlog:

```bash
curl http://localhost:8080/v1/admin/jobs/governance/status \
  -H 'X-API-Key: dev-admin-key'
```

5. Inspect recent scheduler and maintenance executions:

```bash
curl 'http://localhost:8080/v1/admin/jobs/status?limit=5' \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

6. Inspect a memory history after governance promotion:

```bash
curl http://localhost:8080/v1/admin/memories/<memory-id>/history \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

## Memory Management Surface

`Stele` now exposes two distinct memory management boundaries:

- public read surface, authenticated by `STELE_AUTH_API_KEYS`
- privileged lifecycle surface, authenticated by `STELE_AUTH_ADMIN_API_KEYS`

Public read routes:

- `GET /v1/memories`
- `GET /v1/memories/{memory_id}`
- `GET /v1/memories/{memory_id}/history`
- `GET /v1/memories/{memory_id}/provenance`

Privileged lifecycle routes:

- `POST /v1/admin/memories/{memory_id}:suppress`
- `POST /v1/admin/memories/{memory_id}:expire`
- `POST /v1/admin/memories/{memory_id}:delete`

Lifecycle actions require:

- admin API key
- scoped headers
- `X-Stele-Actor`
- JSON body with `reason`

## Example Flow

1. List visible canonical memory:

```bash
curl 'http://localhost:8080/v1/memories?class=profile&limit=10' \
  -H 'X-API-Key: dev-public-key' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

2. Inspect one memory history:

```bash
curl http://localhost:8080/v1/memories/<memory-id>/history \
  -H 'X-API-Key: dev-public-key' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

3. Inspect provenance lineage:

```bash
curl http://localhost:8080/v1/memories/<memory-id>/provenance \
  -H 'X-API-Key: dev-public-key' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

4. Suppress a memory through the admin boundary:

```bash
curl -X POST http://localhost:8080/v1/admin/memories/<memory-id>:suppress \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"manual override"}'
```

## Operational Notes

- `api` logs request completion and panic recovery in structured key-value style.
- `worker` logs polling loop failures and successful batch execution.
- `scheduler` logs maintenance job execution results and backoff retries.
- The scheduler dispatch path is independent from public request traffic and currently drives summary compaction, retention expiry evaluation, and job execution cleanup for the configured default scope.
- Telemetry hook points are wired for ingest, governance worker execution, retrieval, forgetting, and backlog inspection. The default runtime uses a no-op observer until a concrete metrics or tracing backend is attached.
