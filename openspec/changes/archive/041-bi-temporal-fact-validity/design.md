## Context

See `proposal.md` for motivation and externally visible behavior. The current
repository has append-only `memory_versions`, raw-event `source_timestamp`,
canonical `created_at`/`updated_at`, source-version-aware embedding rebuilds,
relation projections, derived chunks, citations, and RQ1 temporal query-family
planning. It does not yet store fact-valid intervals, temporal identity, or
version-aware predicates across all derived retrieval paths. Existing
`time_from`/`time_to` search filters are recorded/update-time filters and must
remain compatible.

## Goals / Non-Goals

**Goals:**

- Add a version-level valid-time model with deterministic current and historical
  selection while preserving PostgreSQL as the only system of record.
- Keep factual writes append-only, corrections auditable, and derived artifacts
  bound to immutable source-version/validity identities.
- Make historical access explicit, exact-scope, lifecycle-safe, bounded, and
  replayable through the RQ1 planner and public OpenAPI contract.
- Preserve legacy current behavior, ordinary response shapes, recorded-time
  filters, and baseline-safe rollback when temporal support is disabled or
  incompatible.
- Produce deterministic current/historical fixtures and hard release gates for
  stale-fact, provenance, isolation, lifecycle, latency, and rollback failures.

**Non-Goals:**

- No graph traversal, graph database, online learning, popularity ranking, or
  autonomous validity inference.
- No temporal semantics owned by derived summaries or relation projections;
  they carry source snapshots only.
- No destructive update of canonical content, memory history, raw events, or
  prior derived artifacts.

## Decisions

### 1. Store validity on canonical versions, not only the canonical head

Add temporal metadata to the version-level contract. Factual source classes in
the first slice are `profile`, `episodic`, and `procedural`; `summary` remains a
derived class and `relation` remains a derived projection. Each version has a
stable `temporal_fact_id`, `ingested_at` (recorded time), `valid_from`, and
optional `valid_to`. Validity uses `[valid_from, valid_to)` and open-ended
`valid_to` for a current interval.

The canonical head remains a fast current projection, but it references the
selected source version and repeats only the fields needed for ordinary reads.
Version rows are authoritative for temporal selection. This avoids treating a
mutable head timestamp as proof that a fact was true at a historical instant.

Alternative considered: put validity only on `canonical_memories`. Rejected
because historical selection would be unable to distinguish versions and would
make chunk, citation, and embedding lineage ambiguous.

### 2. Use an append-only temporal correction ledger for supersession

Canonical version content and validity are immutable after insertion. A
correction appends a successor version plus an append-only temporal operation
record containing `superseded_at`/correction time, actor, reason, predecessor,
successor, and conflict disposition. The current-head projection may advance
transactionally, but no prior version payload is overwritten. If compatibility
requires a nullable `superseded_at` read field, it is derived from the ledger,
not treated as mutable source history.

For one `temporal_fact_id`, non-conflicting intervals cannot overlap. A
retroactive correction must close the logical interval by appending successor
evidence; an overlap is rejected or recorded as an explicit conflict that is
excluded from ordinary current retrieval. Separate facts retain separate
temporal identities and may coexist.

Alternative considered: update `memory_versions.superseded_at` in place.
Rejected because it weakens append-only history and makes replay dependent on
mutation order.

### 3. Migrate legacy rows with an explicit current-compatible interpretation

The additive migration backfills `temporal_fact_id = memory_id`,
`ingested_at = memory_versions.created_at`, `valid_from = created_at`, and
open-ended `valid_to` for legacy visible versions selected as current. It marks
the validity source as `legacy_current_compatible`. Historical claims before
the migration are not invented; a privileged audit can distinguish inferred
compatibility from explicit source validity.

The migration is idempotent, scope-indexed, and leaves existing content,
provenance, embeddings, chunks, and relation rows intact. A failed backfill
does not make ordinary current retrieval hide legacy memories.

### 4. Separate recorded-time filters from valid-time constraints

The public search request retains `time_from`/`time_to` as recorded/update-time
filters. Add optional `as_of` and `valid_during` selectors with mutual
validation. The planner accepts a valid-time constraint only when it is explicit
and authorized. A current plan resolves one request evaluation instant; an
historical plan carries the caller-supplied instant or half-open interval.

Ordinary current retrieval applies lifecycle plus validity-at-evaluation-time
before lexical/semantic/relation/chunk fusion. Historical retrieval reads
version-addressable candidates, applies the same scope/lifecycle predicate, and
never widens constraints. If a derived index lacks the requested source
version, the channel reports a bounded omission and baseline-safe fallback.

Alternative considered: reinterpret existing `time_from`/`time_to` as valid
time. Rejected as a silent public-contract break and because existing callers
use those fields for recorded/update-time windows.

### 5. Bind every derived artifact to source-version and validity identity

Relation projections, chunks, embeddings/rebuild items, citations, context
projections, and evaluation aliases carry `source_version` plus
`temporal_fact_id` and a validity snapshot. Rebuilds are keyed by source version,
policy/renderer identity, and scope. A successor invalidates ordinary use of a
prior derived artifact through the shared predicate, but prior derived rows stay
available for privileged audit and deterministic replay.

No derived artifact is promoted to canonical truth. Missing historical vector or
chunk materialization cannot be repaired by reading a different scope or hidden
version; it produces a bounded channel fallback.

### 6. Keep temporal authorization inside the RQ1 plan and rollout contract

Extend the immutable retrieval plan with a temporal constraint identity,
evaluation instant/interval, and a historical-access disposition. A temporal
family without an explicit selector falls back to current baseline. Planner
diagnostics expose only family, temporal disposition, channel availability,
stale/conflict counts, and fallback buckets. Active historical behavior requires
the same exact-scope rollout, compatible policy identities, owned-stack evidence,
and rollback controls as RQ1.

### 7. Evaluate safety before quality and keep rollback data-preserving

Add fixture families for current, as-of, interval, legacy, retroactive,
stale-similarity, and temporal isolation cases. A stale-fact win, hidden or
foreign version, validity ambiguity, provenance mismatch, or resource overflow
is a hard failure even if Recall/MRR improves. Without an explicitly owned
PostgreSQL + pgvector DSN, evidence is a stable non-pass and cannot authorize
active rollout.

Rollback disables temporal-aware policy resolution and returns the approved
current baseline. It never deletes temporal history or rewrites canonical data.

### 8. Dependency and library choice

No new runtime dependency is needed. Go's `time` validation and immutable value
types cover interval checks; PostgreSQL `timestamptz`, range predicates, and
existing migration/repository infrastructure cover persistence and indexing.
Adding a generic temporal ORM or external event store would weaken the
PostgreSQL-only architecture and add migration risk without improving the
contract.

## Risks / Trade-offs

- **Legacy rows may look current despite unknown historical truth** → Mark the
  compatibility source explicitly, keep historical queries opt-in, and measure
  inferred-vs-explicit validity in diagnostics.
- **Retroactive corrections can produce overlapping intervals** → Reject
  implicit overlap, record explicit conflict dispositions, and fail closed for
  ordinary retrieval.
- **Historical semantic/chunk indexes may be incomplete during rollout** → Bind
  derived indexes to source versions, report bounded omission categories, and
  retain lexical/canonical baseline fallback.
- **Temporal predicates add joins and can increase p95 latency** → Add scope,
  fact-identity, validity, and source-version indexes; enforce RQ1 candidate and
  latency envelopes; use diagnostics/shadow before active rollout.
- **Many derived contracts need synchronized identity fields** → Introduce a
  shared source-validity value contract and migration/rebuild checkpoints; reject
  mismatched provenance at repository boundaries.
- **Current-time replay is not inherently stable** → Capture one evaluation
  instant per request, inject a fixed clock for fixtures, and include the logical
  instant in authorized evidence identities rather than raw timestamps in public
  responses.

## Migration Plan

1. Add domain interval/temporal identity contracts and pure validation tests.
2. Add an additive PostgreSQL migration, manifest entry, scope-safe indexes, and
   idempotent legacy compatibility backfill.
3. Add append-only temporal correction/provenance persistence and privileged
   history views.
4. Extend canonical current projection and all repository predicates while
   preserving recorded-time filters and ordinary response shapes.
5. Propagate source validity identity through relation, chunk, embedding,
   citation, projection, and rebuild paths; make missing derived history
   baseline-safe.
6. Add additive OpenAPI selectors and planner temporal constraints, initially
   diagnostics-only, then shadow-only.
7. Add current/historical fixtures, reports, release gates, and owned-stack
   evidence. Keep active rollout disabled when DSN/evidence is absent.
8. If rollback is required, disable the temporal rollout policy and rebuild
   derived artifacts from PostgreSQL source records; do not delete temporal rows.

## Open Questions

None. Legacy interpretation, source classes, interval semantics, correction
conflict handling, public selectors, and rollback behavior are fixed for this
proposal so implementation tasks remain independently verifiable.
