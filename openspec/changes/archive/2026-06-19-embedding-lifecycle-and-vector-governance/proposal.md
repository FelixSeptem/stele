## Why

Stele already supports semantic retrieval, but its vector lifecycle is intentionally incomplete: manual mutation clears stale embeddings and the runtime falls back to lexical or relation recall until a later reindex mechanism exists. That gap is now the main consistency hole in retrieval quality and operational safety, especially as the service needs controlled provider rotation, model-specific routing, and durable vector audit semantics without weakening PostgreSQL-first governance rules.

## What Changes

- Introduce an internal embedding lifecycle capability that tracks when canonical memory requires semantic backfill, rebuild, or rotation.
- Add provider abstraction and model-aware routing so embedding generation is decoupled from retrieval storage and can evolve without changing public memory APIs.
- Add append-only vector revision and audit semantics for the current canonical projection, including provider, model, dimensions, source version, content hash, status, and supersession lineage.
- Add asynchronous reindex and backfill orchestration for missing, stale, or provider-rotated embeddings using the existing worker or scheduler runtime model.
- Define safe cutover semantics so semantic retrieval only uses the active vector revision for the current canonical projection and falls back cleanly when no active embedding is available.
- Explicitly defer admin or operator control surfaces for triggering rebuilds, cutovers, or viewing vector audit state to a later change.

## Non-goals

- New public retrieval APIs or changes to the existing search request or response contract.
- Admin endpoints for viewing vector state, manually triggering reindex, or performing provider cutover.
- Re-embedding every historical canonical memory version by default.
- Rewriting memory history semantics or replacing PostgreSQL as the system of record.
- Multi-tenant external vector stores, learned rerankers, or online experiment infrastructure.

## Capabilities

### New Capabilities
- `embedding-lifecycle-governance`: Internal vector lifecycle management for canonical memory, including backfill, rebuild, provider rotation, and current-projection vector revision audit.

### Modified Capabilities
- `hybrid-memory-retrieval`: Semantic retrieval requirements expand to require active-vector selection and safe fallback when embeddings are missing, stale, rebuilding, or superseded.
- `manual-mutation-governance-controls`: Retrieval consistency requirements expand from stale-vector invalidation to durable reindex eligibility and vector revision continuity after material mutation.
- `worker-orchestration-and-maintenance-jobs`: Maintenance requirements expand to cover asynchronous embedding backfill, rebuild, and provider-rotation execution paths.
- `service-observability`: Operational diagnostics expand to cover embedding queue state, rebuild outcomes, and provider or model cutover telemetry.

## Impact

- Affected code will include memory mutation services, retrieval services, background worker or scheduler orchestration, PostgreSQL schema and repository contracts, and telemetry plumbing.
- Public APIs remain stable, but internal runtime contracts will add embedding provider interfaces, vector revision persistence, and reindex job coordination.
- PostgreSQL remains the only durable store; vector governance metadata and active revision state will be persisted there alongside canonical memory.
- Related prior work and follow-up context: `openspec/changes/archive/006-manual-memory-mutation-and-reclassification/`, `/opsx:apply embedding-lifecycle-and-vector-governance`.
