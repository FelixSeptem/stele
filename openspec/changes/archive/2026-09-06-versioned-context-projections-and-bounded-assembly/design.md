## Context

Stele's current context assembler already returns structured profile, session,
episode, summary, relation, citation, diagnostic, and optional insight
sections. It obtains those sections from live scoped retrieval and applies a
caller-provided character budget, but the result is not a durable projection
that can be inspected, replayed, or rebuilt independently of a request. The
roadmap's P2 stage needs a stable context hierarchy before memory intents,
reflection, or compaction add more writers.

The implementation must preserve PostgreSQL as the system of record, the
existing canonical-memory append-only version contract, lifecycle visibility
defaults, tenant/project/namespace isolation, and the current public context
shape. The projection layer is a derived, versioned read model; it is not a new
canonical memory class and it cannot become an alternate write path.

## Goals / Non-Goals

**Goals:**

- Persist scoped projection records for `always_visible`, `session`,
  `retrieval`, and `archival_history` with stable version and source metadata.
- Define deterministic class-aware eligibility and ordering for projections.
- Make projection materialization idempotent and rebuildable from PostgreSQL
  canonical-memory versions and raw-event evidence.
- Integrate authorized visible projections into context assembly with strict
  character/token budgets, lifecycle filtering, and redacted citations.
- Provide bounded diagnostics and operator documentation for projection state,
  rebuilds, omissions, and scope-safe behavior.

**Non-Goals:**

- No memory-intent API, reflection scheduler, automated extraction, review
  workflow, or compaction run. Those are separate P2 changes.
- No chunking, RRF/fusion, deduplication, reranking, or default ranking change.
- No direct canonical-memory mutation, lifecycle bypass, cross-scope lookup,
  raw-event payload exposure, SDK, UI, MCP, or provider adapter.

## Decisions

### Decision 1: Projection records are derived and append-only

Use a PostgreSQL `context_projections` header plus item rows (or an equivalent
repository-local normalized model) with a stable projection id, scope,
projection kind, schema version, source watermark, status, and timestamps.
Each item stores a canonical memory id/version or raw-event id, bounded rendered
text, class, lifecycle state observed at materialization, sort key, and redacted
citation references. A rebuild creates a new projection version and marks the
prior version superseded; it never updates canonical memory or deletes source
history.

Alternative: store only a JSON context blob. Rejected because item-level
authorization, lifecycle filtering, citation verification, and rebuild
diagnostics would be opaque and difficult to test.

### Decision 2: One policy resolver owns class and kind eligibility

Materialization first resolves a policy from projection kind, memory class,
confidence/size limits, session identity, and lifecycle state. The resolver
returns a bounded eligibility decision and stable omission reason. `profile` is
eligible for `always_visible` only when configured confidence and size gates
pass; `summary` can enter bounded session context; episodic, procedural, and
relation items stay on-demand through retrieval; raw history can populate only
`archival_history` evidence references. Suppressed, forgotten, expired, or
deleted sources are always excluded from ordinary context.

Alternative: let each caller decide eligibility. Rejected because API, worker,
scheduler, and future reflection callers would drift on lifecycle and scope
rules.

### Decision 3: Rebuild from source watermarks, not from rendered output

Materialization records an input watermark containing the source query/window,
canonical version ids, raw-event range or ids, policy version, and renderer
version. Rebuild re-reads those authorized source records, applies the current
policy, and emits a new deterministic projection version. Identical source,
policy, and renderer inputs produce identical item order and checksums.

Alternative: mutate the latest projection in place. Rejected because it loses
the context history needed to explain an agent session and makes rollback
ambiguous.

### Decision 4: Projection-backed assembly is additive and budget-first

The existing `AssembleContext` contract remains the public entrypoint. It may
read a verified projection for the exact request scope/session and use it as a
preferred source for always-visible and session sections, then fill remaining
budget from current retrieval. All sections pass through one budget packer that
counts UTF-8 characters and an optional token estimate, applies deterministic
priority/tie-breakers, and stops before the configured limit. If projection
validation, scope, lifecycle, or budget accounting fails, the item is omitted
with a bounded diagnostic; the assembler never expands the scope or budget to
compensate.

Alternative: replace live retrieval with projections entirely. Rejected because
retrieval remains the correct on-demand path and projections may be stale until
rebuild.

### Decision 5: Citations contain references, never source payloads

Projection citations reuse existing citation ids and operation categories and
must resolve against the request scope before being returned. Diagnostics may
report projection version, section, included/omitted counts, and stable reason
codes, but never raw event content, hidden ids, foreign scope values, or SQL/
database details.

### Decision 6: PostgreSQL migration and repository boundaries follow existing patterns

Add one forward migration for projection tables and indexes, repository methods
for create/read/rebuild and scoped item lookup, and focused service tests using
the existing pgx/pgxmock conventions. Runtime wiring remains inside existing
retrieval/context assembly dependencies; no separate cache or external store is
introduced.

## Risks / Trade-offs

- [Projection staleness causes missing recent memory] -> Keep live retrieval as
  the on-demand fallback, record source watermarks, and expose a bounded stale
  diagnostic rather than silently treating a projection as complete.
- [Projection rows leak hidden lifecycle data] -> Revalidate lifecycle and
  exact scope at read time, store observed state for audit, and fail closed on
  any mismatch.
- [Large always-visible profile material exhausts budget] -> Enforce byte/
  character and optional token limits before persistence and again during
  assembly; reject or omit oversize items with stable reasons.
- [Concurrent rebuilds produce nondeterministic versions] -> Serialize by
  scope/kind/source watermark using PostgreSQL uniqueness and deterministic
  ordering; a duplicate materialization is idempotent.
- [Schema migration increases startup cost] -> Keep projection migration
  additive and indexed by scope, kind, session, status, and source version;
  materialization remains asynchronous/explicit and bounded.

## Migration Plan

1. Ship the additive projection migration and repository/service code with
   projection reads disabled by default for existing callers.
2. Materialize projections only through an explicit local/admin or bounded
   maintenance path for owned scopes; verify status and source watermarks.
3. Enable projection-backed always-visible/session assembly after the migration
   is current, preserving live retrieval fallback and existing response shape.
4. For rollback, disable projection consumption and materialization. Retain
   projection records and all canonical/raw sources; no schema downgrade or
   source deletion is required.

## Open Questions

None for P2a. Projection policy versioning, reflection input watermarks, and
compaction evidence are intentionally reserved for subsequent proposals.
