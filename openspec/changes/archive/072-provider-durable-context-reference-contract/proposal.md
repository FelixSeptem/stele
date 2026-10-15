## Why

PC1–PC4 provide a typed, authorized and budgeted context projection with a
versioned digest, but a runtime still cannot rely on a documented public
contract for writing and later resolving task/run/handoff/compilation
references. HTTP event acceptance or a ranked search hit does not prove that
governance completed, that a non-root `memory_path` survived canonical
promotion, or that the reference remains readable after an API/worker restart.
PC5 closes that reliability gap while keeping Danny's business objects and
their current state authoritative in Danny.

## What Changes

- Add a versioned Provider durable-reference capability with an explicit,
  supported write-to-read path built on governed events/intents, canonical
  memory, paths, versions and provenance.
- Define the supported reference event types and promotion rules. Distinguish
  accepted, pending, failed, completed-governance and visible-canonical states;
  an accepted event or arbitrary raw event type is not silently treated as a
  searchable reference.
- Add an exact-scope, exact-path reference lookup that returns bounded recorded
  identifier/value/path and source event, memory and version identities where
  authorized. Keep ranked top-k retrieval separate from deterministic lookup.
- Preserve non-root `memory_path`, UUID lineage, provenance and reference
  identity through candidate creation, canonical promotion, deduplication,
  consolidation and restart recovery.
- Specify idempotent replay and conflicting-key behavior for reference writes,
  including bounded completion/status inspection over durable governance state.
- Define explicit pending, failed, unavailable, suppressed, forgotten and
  deleted outcomes. Every read rechecks exact scope, current grant and
  lifecycle; a reference never resurrects hidden content.
- Add fresh PostgreSQL + pgvector public fixtures for compilation and task/run
  references, API/worker restart recovery, lifecycle removal, path preservation,
  and UUID/provenance lineage. Update OpenAPI, README, Provider integration,
  architecture, self-hosting, best practices, MCP and the agent skill in the
  same change for all externally visible semantics.

## Capabilities

### New Capabilities

- `provider-durable-context-reference-contract`: versioned durable reference
  write, governance status and exact-path read semantics for runtime context
  associations.

### Modified Capabilities

- `event-ingestion`: reference-capable event types, path/provenance lineage and
  durable governance completion semantics.
- `memory-history-and-provenance`: reference lineage across raw events,
  candidates, canonical versions, lifecycle transitions and deduplication.
- `memory-search-contract`: deterministic exact-path reference resolution is
  distinct from ranked search and remains lifecycle-safe.
- `agent-runtime-provider-adapter`: Provider capability negotiation, reference
  write/read DTOs, bounded statuses, replay/conflict outcomes and strict
  consumer behavior.
- `runtime-memory-provider-contract`: PC5 conformance, fresh public
  PostgreSQL/pgvector evidence and restart/lifecycle acceptance gates.

## Impact

Changes affect Provider OpenAPI DTOs and routes, event/intents and governance
status surfaces, canonical memory/path promotion, provenance and reference
repositories, migrations or indexes if required, assurance fixtures, and the
public live verifier. Existing scope, grant, lifecycle, disclosure, budget and
PC4 digest checks remain prerequisites. Danny stores returned event/memory/
version identifiers, paths, operation IDs, effect keys and digests, but does not
add a local canonical reference store or treat Stele insight/governance status as
task, schedule, lease, compilation, artifact or webhook state.

## Non-goals

- Do not make Stele the authority for task status, task revision, schedule
  occurrences, execution leases, generation commits, artifact publication or
  webhook delivery.
- Do not make arbitrary provider event types searchable, bypass governance, or
  create a second local fact/reference database in Danny.
- Do not introduce a new ranking, embedding, context-packing or digest
  algorithm; PC5 consumes the existing retrieval and PC4 identity contracts.
- Do not bypass scope, grant, disclosure, lifecycle, provenance or retention
  checks through an opaque reference token.
- Do not add exact historical snapshot replay, watermark continuity, or a new
  autonomous reflection/workflow engine.
