## Why

Stele has broad memory-domain coverage and durable evidence workflows, but its
self-hosted delivery path is not yet safe to treat as a product dependency: the
documented Compose configuration uses authentication settings the service now
rejects, database bootstrap is not a versioned upgrade mechanism, and no real
PostgreSQL stack proves the full lifecycle. Before an agent runtime adopts Stele
as its memory provider, operators need a repeatable path to deploy, bootstrap,
upgrade, restart, recover, and verify the service without source inspection.

## What Changes

- Make the documented single-node self-hosted deployment executable by replacing
  obsolete static key configuration with the constrained bootstrap-admin flow,
  separating development defaults from production-required secrets, and adding
  an end-to-end first-operator bootstrap procedure.
- Add a versioned PostgreSQL migration system with an explicit schema history,
  dirty-state handling, locking/concurrency safety, startup and standalone
  migration policies, and upgrade verification from an already-populated
  database. **BREAKING:** direct replay of an ever-changing base schema at
  startup is removed as the schema-management contract.
- Add a product-readiness verification harness that runs against a real
  PostgreSQL plus pgvector deployment and proves bootstrap, principal grants,
  idempotent ingest, async governance, retrieval/context, isolation, restart,
  graceful shutdown, and migration upgrade behavior.
- Harden service runtime boundaries with request-size and server-timeout limits,
  signal-driven cancellation, bounded graceful shutdown, dependency cleanup,
  and health/readiness behavior that remains useful during startup and drain.
- Publish the running API contract and service build/schema compatibility
  metadata through stable OpenAPI and version endpoints so a future agent
  runtime can discover and pin the contract it consumes.
- Provide executable, operator-owned PostgreSQL backup, restore, and restore
  verification runbooks. The service continues to use PostgreSQL as its sole
  system of record and does not become a backup scheduler or object-storage
  product.
- Bring Compose, README, self-hosting documentation, examples, OpenAPI, and
  automated contract checks into one canonical bootstrap and deployment story.

## Non-goals

- Do not add an Agent Runtime adapter, SDK, UI, hosted control plane, model
  invocation, prompt/tool orchestration, or final-answer generation. Those are
  consumers of the stable service contract established here.
- Do not add Redis, a new queue, an object store, or any system of record other
  than PostgreSQL.
- Do not build a managed backup service, backup scheduler, cross-region
  replication system, or vendor-specific deployment integration.
- Do not change canonical memory lifecycle semantics, retrieval ranking,
  tenant/project/namespace isolation, or public evidence-workflow semantics
  except where runtime safety or contract publication requires it.
- Do not claim Kubernetes, multi-region high availability, or arbitrary cloud
  infrastructure support; the supported product baseline is a documented
  single-node Compose deployment with either its bundled PostgreSQL for local
  evaluation or an operator-managed PostgreSQL instance for production.

## Capabilities

### New Capabilities

- `database-schema-migration-management`: Versioned, concurrency-safe
  PostgreSQL schema migration, status inspection, and upgrade compatibility.
- `self-hosted-product-delivery-verification`: Executable deployment,
  bootstrap, lifecycle, restart, and recovery conformance evidence for the
  supported self-hosted baseline.
- `runtime-api-contract-publication`: Runtime publication of the authoritative
  OpenAPI document and bounded build/schema compatibility metadata.
- `postgres-backup-recovery-operations`: Safe operator-run backup, restore,
  and restore-verification commands and documentation for PostgreSQL data.

### Modified Capabilities

- `self-hosting-bootstrap`: Require a working bootstrap-admin-first Compose
  flow, production secret boundaries, and a smoke path that proves the actual
  protected memory lifecycle rather than only health endpoints.
- `service-runtime-foundation`: Require server resource limits, signal-driven
  shutdown, bounded drain behavior, dependency cleanup, and explicit migration
  execution policy for all three runtime modes.
- `service-observability`: Add bounded migration, startup/drain, backup-proof,
  and product-verification telemetry without leaking credentials or scope data.
- `self-hosted-assurance-and-conformance`: Accept a successful executable
  restore verification as backup/restore proof while preserving its existing
  diagnostic-only behavior.

## Impact

- Affected deployment assets: `Dockerfile`, `docker-compose.yml`, Compose
  environment examples, runtime image entrypoints, and self-hosting documents.
- Affected runtime code: configuration, `cmd/stele`, API/worker/scheduler
  runners, HTTP middleware/server construction, dependency lifecycle, health,
  readiness, build metadata, and telemetry.
- Affected storage: migration ledger and locking strategy; conversion of the
  current bootstrap schema into an immutable initial migration plus future
  additive migrations.
- Affected public interfaces: stable `/openapi.yaml` or equivalent OpenAPI
  endpoint and a bounded version/compatibility endpoint; existing memory APIs
  remain backward compatible except for documented request-resource limits.
- Affected verification: real PostgreSQL/pgvector integration tests, Compose
  smoke tests, upgrade tests, shutdown tests, backup/restore verification, docs
  consistency checks, OpenAPI checks, and `openspec validate`.
- Artifact references: use `openspec instructions apply --change
  product-ready-self-hosting-foundation --json`, run `openspec validate
  product-ready-self-hosting-foundation --strict`, and archive only with
  `pwsh -File scripts/openspec-archive-seq.ps1 -ChangeName
  "product-ready-self-hosting-foundation"` after implementation.
