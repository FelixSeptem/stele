# Stele Self-Hosting

## Overview

`Stele` runs as three process modes against one PostgreSQL instance:

- `api`: public ingest, memory read, retrieval, context assembly, and admin inspection or lifecycle routes
- `worker`: continuous governance processing loop for claimed raw events
- `scheduler`: periodic maintenance dispatch for embedding rebuild, summary compaction, retention sweep, and job execution cleanup

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

Embedding routing variables:

- `STELE_EMBEDDING_DEFAULT_PROVIDER`: default provider name recorded on rebuild eligibility and vector revisions
- `STELE_EMBEDDING_DEFAULT_MODEL`: default model name recorded on rebuild eligibility and vector revisions
- `STELE_EMBEDDING_DEFAULT_DIMENSIONS`: default vector dimension target used for drift detection and rebuild eligibility
- `STELE_EMBEDDING_CLASS_ROUTES`: optional comma-separated per-class overrides in `class=provider:model:dimensions` format

Job tuning variables:

- `STELE_JOBS_MAINTENANCE_INTERVAL`: scheduler cadence, default `15m`
- `STELE_JOBS_SUMMARY_COMPACTION_INTERVAL`: summary compaction cadence, default `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_RETENTION_INTERVAL`: retention and expiry sweep cadence, default `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_CLEANUP_INTERVAL`: maintenance cleanup cadence, default `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_JOB_EXECUTION_RETENTION`: retention window for finished job execution records, default `168h`
- `STELE_JOBS_WORKER_POLL_INTERVAL`: worker idle poll delay, default `5s`
- `STELE_JOBS_WORKER_ERROR_BACKOFF`: worker retry backoff, default `15s`
- `STELE_JOBS_SCHEDULER_ERROR_BACKOFF`: scheduler retry backoff, default `30s`
- `STELE_JOBS_GOVERNANCE_MAX_ATTEMPTS`: automatic retry budget for claimed governance raw events, default `5`
- `STELE_JOBS_GOVERNANCE_RETRY_BACKOFF`: retry wait after a failed governance attempt, default `30s`
- `STELE_JOBS_GOVERNANCE_LEASE_RENEW_INTERVAL`: cadence for renewing an in-flight governance claim lease, default `30s`
- `STELE_JOBS_MAINTENANCE_SCOPE_BATCH_LIMIT`: maximum discovered scopes evaluated per scheduler tick, default `100`

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

7. Inspect governance recovery candidates in one scope:

```bash
curl 'http://localhost:8080/v1/admin/governance/raw-events?state=retry_wait&attempt_gte=1&limit=10' \
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

- `POST /v1/admin/memories`
- `PATCH /v1/admin/memories/{memory_id}`
- `POST /v1/admin/memories/{memory_id}:merge`
- `POST /v1/admin/memories/{memory_id}:reclassify`
- `POST /v1/admin/memories/{memory_id}:suppress`
- `POST /v1/admin/memories/{memory_id}:expire`
- `POST /v1/admin/memories/{memory_id}:delete`

Privileged governance recovery routes:

- `GET /v1/admin/governance/raw-events`
- `GET /v1/admin/governance/raw-events/{raw_event_id}`
- `GET /v1/admin/governance/raw-events/{raw_event_id}/recovery-history`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:retry`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:reschedule`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:requeue`

Privileged manual mutation and lifecycle actions require:

- admin API key
- scoped headers
- `X-Stele-Actor`
- JSON body with `reason`

Privileged governance recovery actions use the same admin boundary and additionally require:

- `X-Stele-Actor` for every recovery action
- `reason` in the JSON body for every recovery action
- `scheduled_for` only for `:reschedule`

Governance recovery query filters:

- `state`
- `event_type`
- `attempt_gte`
- `attempt_lte`
- `failed_from`
- `failed_to`
- `next_attempt_from`
- `next_attempt_to`
- `limit`
- `cursor`

Manual mutation notes:

- `POST /v1/admin/memories` only supports operator-authored primary classes; derived `summary` memory is excluded.
- `PATCH /v1/admin/memories/{memory_id}` only updates canonical content. Scope, lifecycle, and class stay on dedicated surfaces.
- merge and reclassify require `expected_version` optimistic concurrency.
- `reclassify` only targets `profile`, `episodic`, or `procedural`; `relation` and `summary` stay excluded in this phase.
- material manual mutation clears stale semantic participation immediately and records durable rebuild eligibility for later background regeneration.

Governance recovery notes:

- Admin governance inspection returns derived raw event states: `pending`, `retry_wait`, `leased`, `exhausted`, and `processed`.
- `retry` only targets `retry_wait` items and pulls `next_attempt_at` forward to immediate worker eligibility.
- `reschedule` only targets `pending` or `retry_wait` items and moves `next_attempt_at` to the operator-provided future timestamp.
- `requeue` only targets `exhausted` items, clears `governance_exhausted_at`, resets the automatic attempt counter, and returns the event to the ordinary worker claim path.
- `leased` and `processed` items are rejected rather than being force-taken over.
- Every recovery action is written to the append-only `governance_recovery_ledger` with actor, reason, and before or after state snapshots.
- Recovery never triggers direct execution. The existing governance worker picks the event up on a later poll through the normal durable claim path.
- First phase boundaries stay narrow: single-item recovery only, no bulk remediation, no leased takeover, and no `ignore` or `drop` terminal action.

## Embedding Lifecycle

- Semantic retrieval reads only the active vector revision linked to the current canonical projection. Superseded, failed, stale, or missing revisions are excluded from default semantic recall.
- When a canonical memory has no active semantic projection, hybrid retrieval still completes through lexical and relation recall. The request does not fail just because semantic projection is unavailable.
- Manual create, update, merge, and reclassify paths invalidate stale active semantic participation synchronously, then rely on scheduler-driven rebuild execution to restore semantic recall later.
- The scheduler discovers missing embeddings and provider or model drift through durable `embedding_rebuilds` state, then claims rebuild work without blocking write paths.
- Every generation attempt is audited in append-only `vector_revisions`, including failed attempts and superseded lineage. Activation uses compare-and-promote guards so stale content never becomes the active semantic projection.
- Routing metadata comes from `STELE_EMBEDDING_DEFAULT_*` and optional `STELE_EMBEDDING_CLASS_ROUTES` overrides. These settings define the desired target used for eligibility, drift detection, and audit history.
- Provider implementations remain an internal runtime integration point. If no provider is registered for the configured target, rebuild attempts remain retryable failures and retrieval falls back to lexical plus relation signals.
- Admin rebuild controls, bulk rotation controls, and embedding-specific inspection endpoints remain an explicit non-goal in this phase.

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

5. Seed one canonical memory directly through the admin boundary:

```bash
curl -X POST http://localhost:8080/v1/admin/memories \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"class":"profile","content":"user prefers concise answers","reason":"seed curated memory"}'
```

6. Correct the memory content with optimistic concurrency:

```bash
curl -X PATCH http://localhost:8080/v1/admin/memories/<memory-id> \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"content":"user strongly prefers concise answers","expected_version":1,"reason":"correct curated fact"}'
```

7. Merge one duplicate memory into the surviving target:

```bash
curl -X POST http://localhost:8080/v1/admin/memories/<target-memory-id>:merge \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"source_memory_id":"<source-memory-id>","content":"user strongly prefers concise answers","expected_version":2,"reason":"merge duplicate canonical facts"}'
```

8. Reclassify a memory into the right governed class:

```bash
curl -X POST http://localhost:8080/v1/admin/memories/<memory-id>:reclassify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"target_class":"procedural","expected_version":3,"reason":"fix canonical class"}'
```

9. List governance raw events that are waiting for retry:

```bash
curl 'http://localhost:8080/v1/admin/governance/raw-events?state=retry_wait&attempt_gte=1&limit=20' \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

10. Read one raw event detail before remediation:

```bash
curl http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id> \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

11. Retry one waiting raw event immediately:

```bash
curl -X POST http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>:retry \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"retry now after transient dependency recovery"}'
```

12. Reschedule one raw event into a later maintenance window:

```bash
curl -X POST http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>:reschedule \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"delay until downstream system is stable","scheduled_for":"2026-06-13T03:00:00Z"}'
```

13. Requeue one exhausted raw event back into normal worker polling:

```bash
curl -X POST http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>:requeue \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"clear exhausted state after operator review"}'
```

14. Read recovery audit history for one raw event:

```bash
curl http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>/recovery-history \
  -H 'X-API-Key: dev-admin-key' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

## Operational Notes

- `api` logs request completion and panic recovery in structured key-value style.
- `worker` logs polling loop failures and successful batch execution.
- `scheduler` logs maintenance job execution results and backoff retries.
- The worker persists retryable governance failures with bounded retry state instead of relying only on lease expiry.
- Raw events that hit the retry ceiling are marked exhausted and stop automatic claim until an explicit admin recovery action intervenes.
- The scheduler dispatch path is independent from public request traffic.
- The embedding rebuild scheduler records backlog and execution telemetry for newly queued rebuild work, rebuild outcomes, and error paths through the shared observer hook.
- Summary compaction and retention sweep are dispatched per eligible discovered scope, with the configured default scope used only as a fallback when discovery returns none.
- Job execution cleanup remains runtime-global and runs once per cadence window instead of being fanned out per discovered scope.
- Telemetry hook points are wired for ingest, governance worker execution, retrieval, forgetting, governance backlog inspection, and embedding rebuild backlog plus execution inspection. The default runtime uses a no-op observer until a concrete metrics or tracing backend is attached.
