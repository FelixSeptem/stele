# Stele v1 Memory Service Plan

## Summary

Stele is a Go-based, self-hosted, API-first agent memory service. This repository only owns the service itself; SDK, UI, and end-user application logic are explicitly out of scope for v1.

The product posture should align with Supabase at the service layer: deploy one service, expose stable HTTP APIs, and allow external SDKs or applications to connect directly. The main design focus is not just storage, but memory governance: what gets written, how it is consolidated, how it is retrieved, how it is forgotten, and how it remains auditable.

## Product Direction

- Deployment model: self-hosted service with HTTP APIs.
- Repository scope: service only.
- Primary workload: general-purpose memory backend for multiple agent types.
- Product boundary: managed memory engine, not low-level storage primitives only.
- Governance mode: automatic governance with audit trail.
- Graph model: enhancement layer, not the primary persistence model.
- Write path: hot write plus async consolidation.
- Isolation model: built-in `project + api_key + tenant + namespace`.

## Architecture

Stele should run as one Go codebase with three runtime modes:

- `api`: serves public and admin HTTP endpoints.
- `worker`: executes extraction, consolidation, summarization, forgetting, and graph update jobs.
- `scheduler`: triggers periodic retention, compaction, and maintenance jobs.

Core subsystems:

- `auth`: API key validation and request scoping.
- `memory`: domain models, state transitions, and versioning rules.
- `governance`: extraction, scoring, dedupe, contradiction handling, promotion, suppression, summarization.
- `retrieval`: hybrid recall, ranking, scope filtering, and context assembly.
- `storage/postgres`: PostgreSQL persistence, migrations, and query access.
- `policy`: retention, TTL, mutability, sensitivity, and forgetting rules.
- `jobs`: background queues, schedulers, and job execution.

## Storage Strategy

PostgreSQL is the only system of record.

Required capabilities:

- `pgvector` for semantic retrieval.
- PostgreSQL full-text search for lexical retrieval.
- standard relational tables for versioned memory records, provenance, and policy state.
- relational entity and relation tables for graph-enhanced retrieval in v1.

Graph support should start as a relational enhancement layer. If future workloads justify it, traversal can later evolve toward AGE or a comparable PostgreSQL graph extension without changing the public API model.

## Memory Model

### Scope hierarchy

Memory organization should follow a strict scope hierarchy:

`tenant -> project -> namespace -> agent -> user -> session -> run`

Writes must land in an explicit scope. Retrieval may search within a scope and optionally climb allowed parent scopes.

### Memory classes

First-class memory classes:

- `profile`: durable facts, preferences, and state about a user, project, or agent.
- `episodic`: concrete events, outcomes, observations, and interactions.
- `procedural`: learned instructions, heuristics, and operating guidance.
- `summary`: compactions of runs, sessions, or topic clusters.
- `relation`: entity and relationship facts used to enrich retrieval and reasoning.

### Memory states

Each memory record should move through explicit lifecycle states:

- `event`: immutable raw source input.
- `candidate`: extracted but not yet consolidated memory.
- `active`: canonical retrievable memory.
- `suppressed`: retained for audit, hidden from normal retrieval.
- `forgotten`: behaviorally removed according to policy.
- `deleted`: hard-erased content for compliance.

### Versioning

Canonical memory must be append-only versioned:

- `memory_id` stays stable.
- each material change creates a new `memory_version`.
- canonical lookup resolves to the latest active version.
- provenance and audit history are preserved.

## Governance And Lifecycle

The service should emphasize governance as a first-class concern.

### Write admission

- accept raw events immediately.
- record provenance, scope, timestamps, and metadata at ingest time.
- perform extraction and consolidation asynchronously.

### Consolidation

Worker jobs should be responsible for:

- extracting candidate memories from raw events.
- classifying memory type.
- assigning confidence, importance, freshness, sensitivity, mutability, and retention metadata.
- deduplicating near-identical facts.
- resolving mutable profile updates by version supersession rather than destructive overwrite.
- preserving conflicting episodic evidence with temporal validity where appropriate.
- generating summaries from stale or dense episodic clusters.
- updating entity/relation projections used for retrieval enhancement.

### Forgetting

Support three distinct forgetting semantics:

- `suppress`: hide from normal recall, retain for audit.
- `expire`: suppress automatically after TTL or retention policy.
- `delete`: remove payloads and index projections for compliance or operator action.

## Retrieval Design

Retrieval should be hybrid and scope-aware by default.

Pipeline:

1. apply scope and metadata filters.
2. run semantic recall over canonical memory and summaries.
3. run lexical recall over full-text indexed content.
4. optionally expand through related entities and relations.
5. rerank and assemble a bounded response.

The retrieval layer should avoid returning a flat, undifferentiated list. Context assembly should instead return structured sections such as:

- `profile`
- `recent_session`
- `recent_episodes`
- `relevant_summaries`
- `related_entities`
- `citations`

## Public API Direction

Initial v1 endpoints:

- `POST /v1/events`
- `POST /v1/memories/search`
- `POST /v1/context/assemble`
- `POST /v1/memories/forget`
- `GET /v1/memories/{id}`
- `GET /v1/memories/{id}/history`
- `PATCH /v1/memories/{id}`
- `DELETE /v1/memories/{id}`

Important contract expectations:

- every request is scoped by API key plus explicit tenant/project/namespace inputs or derived defaults.
- every mutation is auditable.
- default retrieval never returns suppressed or forgotten memory.
- context assembly is opinionated and agent-ready.

## Repository Layout

Planned top-level structure:

- `cmd/stele`: application entrypoint.
- `internal/app`: runtime bootstrapping by mode.
- `internal/auth`: API key auth and scope enforcement.
- `internal/memory`: domain models, repositories, services.
- `internal/governance`: extraction and consolidation flows.
- `internal/retrieval`: search and context assembly.
- `internal/storage/postgres`: migrations, queries, repositories.
- `internal/jobs`: worker and scheduler execution.
- `openapi/`: API contract source.
- `deploy/`: self-host deployment assets.
- `docs/`: design, planning, and operational docs.

## Delivery Phases

### Phase 1: Foundation

- initialize Go module and service layout.
- add configuration loading and runtime mode selection.
- add PostgreSQL bootstrap and migration runner.
- add health and readiness endpoints.
- add API key auth and scope primitives.
- publish initial OpenAPI contract.

### Phase 2: Memory Model

- define memory classes, states, identifiers, and scope model.
- create raw event, canonical memory, version, provenance, and policy schemas.
- implement event ingestion API and persistence path.

### Phase 3: Governance

- implement candidate extraction pipeline.
- implement scoring, dedupe, contradiction handling, and promotion/suppression rules.
- implement summary generation and retention processing.

### Phase 4: Retrieval

- implement hybrid semantic plus lexical recall.
- add metadata and scope filters.
- add context assembly endpoint.
- add relation-aware retrieval enhancement.

### Phase 5: Operations

- add worker and scheduler orchestration.
- add observability, audit inspection, and admin endpoints.
- add deployment manifests and bootstrap documentation.

## Acceptance Criteria

- raw events can be ingested and assigned stable identifiers.
- canonical memory supports version history and provenance.
- mutable profile memory can supersede prior versions without losing auditability.
- episodic contradictions remain queryable with time semantics intact.
- background consolidation promotes candidates into active memory.
- stale episodic clusters can be summarized and compacted.
- forgetting removes memory from default retrieval according to action semantics.
- isolation boundaries hold across tenants, projects, and namespaces.
- retrieval combines semantic, lexical, and relation-aware signals without leaking suppressed memory.

## Explicit Non-Goals

- SDK implementation.
- hosted control plane.
- dashboard UI.
- multi-database support.
- human approval workflows for v1 governance.
- graph-first persistence model in v1.

## Immediate Next Step

Before implementing code, land this plan document, add a concise repository contract in root `AGENTS.md`, then start foundation work from Phase 1.
