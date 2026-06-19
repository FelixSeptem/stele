## Context

Stele already persists lexical search projections and pgvector-backed semantic projections inside PostgreSQL, but today semantic projection ownership is intentionally incomplete. Material manual mutation updates `search_text` immediately and clears `embedding`, which prevents stale vectors from participating in retrieval but leaves no durable path to regenerate them. The repository also stores only the currently active vector directly on `canonical_memories`, so provider rotation, model routing, and vector lineage are not first-class governed concepts.

This change closes that gap without changing the public retrieval surface and without introducing a second system of record. The design must preserve the repository's existing architectural defaults:

- PostgreSQL remains the only durable truth source.
- Retrieval stays scoped by `tenant`, `project`, and `namespace`.
- Default reads continue excluding suppressed, forgotten, expired, and deleted memory.
- Worker and scheduler modes remain the mechanism for background execution.
- Current retrieval behavior keeps working even when no semantic embedding is available.

The proposal is intentionally an umbrella design. It defines the full vector lifecycle contract now, but it remains implementation-phaseable so the repository can land schema, provider abstraction, orchestration, and telemetry in staged work.

## Goals / Non-Goals

**Goals:**

- Define a durable internal model for embedding providers, model-aware routing, and current-projection vector revisions.
- Persist reindex eligibility and append-only vector revision audit state in PostgreSQL.
- Support asynchronous backfill, rebuild, and provider-rotation execution using the existing runtime modes.
- Ensure semantic retrieval only uses one active vector revision for the current canonical projection at a time.
- Preserve safe fallback to lexical or relation retrieval when semantic projection is missing, stale, pending, failed, or superseded.
- Allow future provider or model changes without rewriting public retrieval contracts.

**Non-Goals:**

- New admin APIs, dashboards, or operator-triggered control surfaces.
- Mandatory re-embedding of historical canonical versions.
- A second external vector database or non-PostgreSQL persistence layer.
- Learned reranking, online experiments, or adaptive query-planning changes.
- Changes to canonical memory history semantics beyond recording which canonical version produced the active vector revision.

## Decisions

### Decision: Govern vector lifecycle as a first-class current-projection subsystem

The service will treat semantic projection as a governed derivative of the current canonical memory projection rather than as an inline field that is overwritten in place with no audit. A new append-only vector revision record will represent each generated embedding for a memory's current canonical projection. The `canonical_memories.embedding` column can remain as the active retrieval fast path during migration, but it becomes a projection of the currently active vector revision rather than the only authoritative vector state.

Why this over keeping a single mutable embedding field:

- It preserves traceability for provider, model, dimensions, source version, and content hash.
- It allows controlled provider rotation and rebuild semantics without guessing which vector is active.
- It keeps retrieval fast while still preserving an auditable vector lineage.

Alternative considered: store vector lineage only in provenance metadata. Rejected because provenance records are too generic for active-vector lookup, supersession semantics, and failure or retry lifecycle.

### Decision: Scope vector audit to the current canonical projection, not all historical versions

Each vector revision will reference the canonical `memory_id` and the source canonical `version_number` used to generate it, but the system will not require every historical canonical version to have its own semantic rebuild chain. If a memory changes materially, the current projection becomes eligible for a new vector revision; old revisions remain auditable as superseded records but are not automatically replayed against every historical version.

Why this over full historical vector replay:

- Retrieval reads only the current visible canonical projection, so current-projection correctness has the highest operational value.
- Full historical replay multiplies compute cost and semantic ambiguity for deleted, merged-away, or hidden versions.
- It still preserves enough audit history to explain the active semantic state.

Alternative considered: rebuild vectors for all historical canonical versions during provider rotation. Rejected as too expensive and semantically unclear for v1 governance.

### Decision: Separate eligibility tracking from execution records

The design will persist both:

- a lightweight reindex eligibility or queue state for each canonical memory that indicates whether semantic projection is missing, stale, rebuilding, failed, or provider-mismatched
- append-only vector revision records that capture successful or failed embedding generation attempts and active or superseded lineage

This separation keeps scheduling efficient while preserving durable audit of actual work attempts.

Alternative considered: derive all pending work by scanning `canonical_memories` and `vector_revisions` ad hoc. Rejected because provider rotation and partial failure recovery would make scans expensive and operationally ambiguous.

### Decision: Introduce provider abstraction with policy-based model routing

Embedding generation will sit behind an internal provider interface that returns provider name, model name, dimensions, and vector payload. Routing rules will be policy-based and deterministic from durable inputs such as memory class, namespace-level defaults, or runtime configuration. The routing result becomes part of the persisted vector revision metadata so future rotations can compare the active projection against the desired provider or model target.

Why this over hard-coding one embedding backend:

- Provider decoupling is explicitly required by the chosen scope.
- Multi-model routing lets summary, relation-adjacent, or canonical classes evolve independently later without changing retrieval API shape.
- Persisted routing decisions make drift detection and rebuild eligibility deterministic.

Alternative considered: let callers provide provider or model per search request. Rejected because Stele owns memory governance and should not expose provider orchestration through public retrieval APIs.

### Decision: Use asynchronous worker or scheduler execution, never inline mutation-time embedding generation

Material mutation will keep invalidating stale semantic projection synchronously, but new vector generation will happen asynchronously. This aligns with the repository's hot-write plus async-consolidation preference and avoids putting network-bound embedding generation onto admin or ingest write paths.

Execution model:

- mutation or policy drift marks a memory as requiring rebuild
- scheduler or worker claims eligible reindex work
- provider generates a candidate embedding
- repository writes a new vector revision and atomically promotes it to active if it still matches the current canonical source version and content hash

Alternative considered: regenerate embedding inline during mutation. Rejected because it increases latency, introduces external dependency failure into write paths, and conflicts with existing governance architecture.

### Decision: Use compare-and-promote semantics to avoid stale vector activation

Every reindex attempt will capture the source canonical version and content hash at claim time. Promotion of a generated vector revision to active will succeed only if the canonical memory still matches that source. If the memory changed again during execution, the attempt is recorded but not activated; the memory remains eligible for a newer rebuild.

Why this over last-write-wins:

- It avoids semantic projection races during repeated mutation or provider rotation.
- It preserves append-only audit while ensuring retrieval does not activate a vector for stale content.

Alternative considered: lock the canonical row for the full embedding generation window. Rejected because embedding generation may be slow and should not hold long-lived database locks.

### Decision: Keep retrieval fallback semantics explicit

Semantic retrieval will select only active vector revisions that match the memory's current canonical projection and the runtime's eligibility rules. If no active vector exists, the retrieval pipeline simply contributes no semantic candidate for that memory and continues merging lexical and relation signals normally.

This keeps the hybrid retrieval contract stable while making semantic readiness explicit rather than accidental.

### Decision: Extend observability at the internal queue and cutover layer

The telemetry contract will grow to include vector backlog and execution events:

- pending reindex count
- failed rebuild count
- active provider or model drift count
- rebuild duration and status
- provider rotation batch outcomes

This keeps the system operable even before any admin surface exists.

## Risks / Trade-offs

- [Schema complexity increases around vector revisions and eligibility state] → Keep the current canonical projection as the only active retrieval target and phase schema changes so the initial implementation can dual-write active embedding state before any later cleanup.
- [Provider or model rotation can create large background backlogs] → Persist desired-target drift explicitly and process rebuilds incrementally through worker or scheduler limits rather than full-table blocking migrations.
- [Embedding generation depends on external providers and can fail repeatedly] → Treat failures as durable attempt records with retry-safe scheduling and observability rather than retrying blindly in write paths.
- [Multiple routing rules can make vector provenance hard to reason about] → Persist the exact provider, model, dimensions, and source version on every revision and keep routing policy deterministic from stored inputs.
- [Current canonical row may change while a rebuild is in flight] → Use compare-and-promote semantics keyed by source version and content hash so stale rebuilds are recorded but never activated.
- [Keeping active embedding on `canonical_memories` and in revision history duplicates state] → Accept temporary duplication because retrieval fast paths are simpler and safer during migration; later cleanup can collapse storage once active revision lookup is stable.

## Migration Plan

1. Add PostgreSQL schema for vector revision audit and reindex eligibility metadata while preserving existing retrieval behavior.
2. Backfill existing active `canonical_memories.embedding` rows into initial active vector revisions.
3. Update mutation paths to mark rebuild eligibility and invalidate semantic projection through the new lifecycle model.
4. Introduce provider abstraction and background execution that can backfill missing vectors and promote new active revisions.
5. Add drift detection for provider or model target changes and process rotation through the same lifecycle path.
6. Extend telemetry and diagnostics to expose vector backlog and rebuild outcomes.
7. Optionally, in a later implementation phase, simplify the active retrieval storage path once vector revision activation is fully trusted.

Rollback strategy:

- If background execution fails, retrieval continues to fall back to lexical and relation signals.
- If provider routing changes are faulty, the desired target can be reverted in config while preserving existing active revisions.
- If revision activation logic is faulty, the repository can continue reading the last known active embedding from the canonical projection until the new lifecycle path is repaired.

## Open Questions

- Whether the initial implementation should store active vector payload only on revisions and join at query time, or continue mirroring the active payload onto `canonical_memories.embedding` for a transition period.
- Whether model routing rules need to differ by memory class in v1, or whether the first implementation should support a single default route with durable metadata that allows later diversification.
- Whether provider rotation should support partial-scope targeting in internal config from the start, or begin as runtime-global desired-target drift detection.
