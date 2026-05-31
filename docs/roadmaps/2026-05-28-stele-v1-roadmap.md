# Stele v1 Roadmap

## Purpose

This roadmap decomposes the approved v1 plan into execution phases and concrete tasks. It is intended to guide implementation order, checkpoint reviews, and milestone acceptance before broader feature expansion.

Scope remains unchanged:

- service only
- Go implementation
- PostgreSQL as the only system of record
- built-in API key plus tenant isolation
- governance-first memory service

## Milestone View

### M0: Project Foundation

Goal: make the repository runnable as a service skeleton with clear runtime modes and operational boundaries.

Exit signal:

- service starts in `api`, `worker`, and `scheduler` modes
- configuration loads cleanly
- PostgreSQL connectivity and migration bootstrap are wired

### M1: Event And Canonical Memory Base

Goal: ingest raw events into durable storage and establish the canonical memory schema and versioning model.

Exit signal:

- `POST /v1/events` persists raw event records
- memory domain types and schemas exist
- provenance and version history are modeled

### M2: Governance Pipeline

Goal: turn raw events into governed memory through extraction, scoring, consolidation, suppression, and summarization.

Exit signal:

- async worker can promote candidates into active memory
- profile updates and episodic contradictions follow explicit rules
- retention and forgetting primitives are available

### M3: Retrieval And Context Assembly

Goal: provide hybrid, scoped retrieval and an agent-ready context assembly surface.

Exit signal:

- memory search works across semantic plus lexical signals
- context assembly returns structured output
- suppressed or forgotten memory is excluded by default

### M4: Operations And Self-Hosting

Goal: make the service operable as a self-hosted product with deployment assets and inspection tooling.

Exit signal:

- worker and scheduler operations are observable
- admin inspection surfaces exist
- deploy assets and bootstrap docs are usable

## Phase Breakdown

## Phase 1: Foundation

### Task 1.1: Repository bootstrap

- Initialize `go.mod`.
- Create top-level directories: `cmd/stele`, `internal/app`, `internal/storage/postgres`, `openapi`, `deploy`, `docs`.
- Define package boundaries for `auth`, `memory`, `governance`, `retrieval`, `jobs`, and `policy`.

Outputs:

- bootable Go module
- stable directory layout

Done when:

- `go test ./...` runs against an empty but valid module layout

### Task 1.2: Runtime mode entrypoint

- Add a single binary entrypoint with mode selection for `api`, `worker`, `scheduler`.
- Define startup lifecycle hooks: config load, logger init, database init, mode dispatch.
- Keep mode wiring minimal and testable.

Outputs:

- CLI/runtime contract for service startup

Done when:

- each mode can start with a no-op runner path

### Task 1.3: Configuration system

- Define config schema for service mode, HTTP listen address, PostgreSQL DSN, auth defaults, and job settings.
- Support environment-based loading.
- Add validation for required config.

Outputs:

- typed config package
- startup validation rules

Done when:

- invalid config fails fast with actionable errors

### Task 1.4: PostgreSQL bootstrap and migrations

- Select migration mechanism and folder layout.
- Add initial migration runner integration.
- Wire health checks for DB readiness.

Outputs:

- migration entrypoint
- DB bootstrap package

Done when:

- service can apply migrations on a clean database

### Task 1.5: Service baseline endpoints

- Add `health` and `ready` endpoints.
- Add request middleware scaffolding: request ID, logging, panic recovery, auth hook point.
- Publish initial OpenAPI skeleton for system endpoints and upcoming memory APIs.

Outputs:

- minimal HTTP server
- baseline middleware stack

Done when:

- API mode exposes health endpoints and OpenAPI stub

### Task 1.6: Auth and isolation primitives

- Define `project`, `tenant`, and `namespace` request context model.
- Implement API key parsing and request scoping middleware.
- Add rejection paths for invalid or missing scopes.

Outputs:

- auth middleware primitives
- scoped request context

Done when:

- protected routes can reject unauthenticated or unscoped requests

Dependencies:

- Tasks 1.2, 1.3, 1.5

## Phase 2: Memory Model And Event Ingestion

### Task 2.1: Domain model for memory

- Define enums and types for memory classes, memory states, scope hierarchy, identifiers, and timestamps.
- Define canonical memory aggregate and raw event aggregate.
- Define version and provenance records.

Outputs:

- stable domain types for memory

Done when:

- downstream packages can depend on domain types without guessing field semantics

### Task 2.2: Schema design and migrations

- Add relational schemas for raw events, canonical memories, memory versions, provenance links, policies, and deletion markers.
- Add indexes for scope, timestamps, and lifecycle state.
- Reserve extension hooks for `pgvector` and full-text search fields.

Outputs:

- database schema for the first usable vertical slice

Done when:

- migrations create all base memory tables successfully

Dependencies:

- Task 1.4

### Task 2.3: Repository interfaces and Postgres implementations

- Define repository interfaces for event ingest, memory lookup, version history, and provenance persistence.
- Implement PostgreSQL repositories for raw event creation and base memory reads.
- Keep repository contracts aligned with scope isolation rules.

Outputs:

- storage abstractions
- first Postgres-backed repositories

Done when:

- tests can persist and load raw events through repository interfaces

### Task 2.4: `POST /v1/events` API

- Define request and response shapes for event ingestion.
- Validate scope, event type, content, timestamps, and metadata.
- Persist raw events and return stable identifiers.

Outputs:

- first product API endpoint

Done when:

- API can ingest an event and write it to PostgreSQL

Dependencies:

- Tasks 1.5, 1.6, 2.1, 2.3

### Task 2.5: Audit and provenance baseline

- Record creation source, actor, timestamps, and request lineage for ingested events.
- Make provenance retrievable internally even before external history APIs are built.

Outputs:

- consistent provenance capture at ingest time

Done when:

- every ingested event has auditable origin metadata

## Phase 3: Governance Pipeline

### Task 3.1: Candidate extraction pipeline contract

- Define the worker input/output contract from raw event to candidate memory.
- Create extraction interfaces so summarization or LLM-backed extraction can be plugged in later.
- Keep v1 extraction path simple and deterministic where possible.

Outputs:

- extraction pipeline boundary

Done when:

- worker can load raw events and emit candidate memory records

### Task 3.2: Candidate persistence and scoring

- Add candidate memory storage.
- Attach governance fields: confidence, importance, freshness, sensitivity, mutability, retention class.
- Define initial scoring and normalization rules.

Outputs:

- governed candidate layer

Done when:

- candidates persist with required governance metadata

### Task 3.3: Consolidation rules

- Implement dedupe and merge rules for mutable profile memory.
- Implement temporal coexistence rules for episodic contradictions.
- Promote accepted candidates into active memory versions.
- Suppress losing or stale candidates without destroying audit history.

Outputs:

- first governed consolidation path

Done when:

- worker can transform candidates into active or suppressed records

Dependencies:

- Tasks 2.1, 2.2, 3.2

### Task 3.4: Summary generation and compaction

- Define summary records for sessions or topic clusters.
- Add compaction rules for stale episodic material.
- Preserve evidence links from summaries back to underlying events or memories.

Outputs:

- summary memory path

Done when:

- worker can produce summary memories with provenance links

### Task 3.5: Forgetting and retention actions

- Add suppression, expiry, and delete action models.
- Implement retention job contracts and mutation audit logging.
- Ensure default reads exclude forgotten and suppressed content.

Outputs:

- lifecycle control primitives

Done when:

- forgetting actions change retrieval visibility according to policy

## Phase 4: Retrieval And Context Assembly

### Task 4.1: Search query model

- Define search request model: query text, scope filters, class filters, time window, top-k, summary inclusion, relation inclusion.
- Define retrieval result model with scores and citations.

Outputs:

- stable search contract

Done when:

- query orchestration can run without ad hoc request shaping

### Task 4.2: Lexical and semantic retrieval base

- Add full-text indexing strategy for canonical content.
- Add vector storage and retrieval path for semantic similarity.
- Define merge and rerank logic across lexical and semantic results.

Outputs:

- hybrid retrieval core

Done when:

- search can combine both recall paths into a single ranked output

Dependencies:

- Tasks 2.2, 3.3

### Task 4.3: Scope-aware filtering and policy enforcement

- Enforce `tenant`, `project`, `namespace`, and optional lower-scope filters in retrieval.
- Exclude suppressed, forgotten, and expired memories by default.
- Respect class and time filters.

Outputs:

- safe default retrieval behavior

Done when:

- retrieval does not leak hidden or out-of-scope memory

### Task 4.4: `POST /v1/memories/search`

- Expose the first public retrieval API.
- Return canonical memory hits with citations and metadata.
- Add request validation and auth integration.

Outputs:

- first retrieval endpoint

Done when:

- clients can retrieve governed memory through HTTP

### Task 4.5: `POST /v1/context/assemble`

- Build structured context sections: `profile`, `recent_session`, `recent_episodes`, `relevant_summaries`, `related_entities`, `citations`.
- Add token-budget-aware packing strategy.
- Prefer summary plus evidence over flat chunk dumps.

Outputs:

- agent-ready context assembly endpoint

Done when:

- service can return structured context instead of raw hit lists

### Task 4.6: Relation-enhanced retrieval

- Add entity and relation projection reads.
- Expand local neighborhoods for entity-centric queries.
- Keep graph enhancement optional and bounded.

Outputs:

- graph-assisted retrieval layer

Done when:

- relation-aware search improves recall without becoming the primary dependency

## Phase 5: Operations And Self-Hosting

### Task 5.1: Worker orchestration

- Finalize worker loop, job reservation, retry policy, and failure handling.
- Add idempotency protections for repeated jobs.

Outputs:

- stable background execution path

Done when:

- governance jobs can run reliably under restart and retry scenarios

### Task 5.2: Scheduler and maintenance jobs

- Add periodic triggers for retention, expiry, compaction, and cleanup.
- Separate maintenance cadence from request path behavior.

Outputs:

- scheduled maintenance subsystem

Done when:

- lifecycle maintenance can run unattended

### Task 5.3: Observability

- Add structured logs, metrics, and tracing hooks for API, worker, and scheduler modes.
- Expose operational signals for ingest latency, consolidation lag, retrieval latency, and forgetting actions.

Outputs:

- baseline observability

Done when:

- operators can inspect service health and backlog status

### Task 5.4: Admin and inspection endpoints

- Add admin surfaces for job status, memory history inspection, and operational diagnostics.
- Keep admin paths clearly separated from public APIs.

Outputs:

- operational inspection surface

Done when:

- operators can inspect internal state without direct database access

### Task 5.5: Deployment and bootstrap docs

- Add deployment assets for local and self-hosted environments.
- Document required PostgreSQL extensions, startup modes, config variables, and bootstrapping steps.

Outputs:

- self-host onboarding assets

Done when:

- a new operator can bring up the service from documentation alone

## Execution Order

Recommended build order:

1. Phase 1
2. Phase 2
3. Phase 3 Tasks 3.1 to 3.3
4. Phase 4 Tasks 4.1 to 4.4
5. Phase 3 Tasks 3.4 to 3.5
6. Phase 4 Tasks 4.5 to 4.6
7. Phase 5

Reasoning:

- event ingestion and canonical memory must exist before governance can do real work.
- governance must exist before retrieval can be trustworthy.
- context assembly and graph enhancement should sit after the base retrieval path is stable.
- operations hardening should happen after the core runtime surfaces exist.

## Review Gates

Before moving between phases, verify:

- Phase 1 to 2: service boot, config, DB bootstrap, and auth scaffolding are stable.
- Phase 2 to 3: event ingest path persists raw events with scope and provenance intact.
- Phase 3 to 4: active memory promotion and suppression behavior are test-covered.
- Phase 4 to 5: retrieval and context assembly exclude hidden memory and respect scope boundaries.

## Immediate Next Step

If implementation begins next, start with Phase 1 Task 1.1 and 1.2 together, then move through Phase 1 in order before touching memory-specific logic.
