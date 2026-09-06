## Context

Stele's canonical memories and immutable raw events are the system of record. The
recent retrieval-evaluation baseline provides deterministic quality and isolation
measurements, while versioned context projections provide bounded, auditable
source references. Retrieval still treats many long or mixed-purpose records as
one candidate, which limits evidence coverage and makes later fusion changes hard
to evaluate.

This design adds a derived source-chunk layer. It must work across the existing
Go memory, retrieval, PostgreSQL, context, and evaluation boundaries without
creating a second canonical store. The implementation is self-hosted, exact-scope
and lifecycle-safe, and must remain reversible while the baseline ranking path is
still the production default.

## Goals / Non-Goals

**Goals:**

- Persist versioned chunk metadata and source lineage in PostgreSQL.
- Produce deterministic, bounded chunks from authorized raw-event and
  canonical-memory source records.
- Preserve memory-class-specific granularity and exact scope/lifecycle rules.
- Support parent and limited adjacent evidence lookup with bounded budgets.
- Make chunk materialization, rebuild, and retrieval diagnostics observable and
  idempotent.
- Keep chunk candidates opt-in/shadowed with a canonical retrieval fallback.

**Non-Goals:**

- Changing lexical/semantic/relation score fusion or introducing RRF.
- Semantic deduplication, diversity/MMR packing, query decomposition, or model
  reranking.
- Mutating canonical memory or raw events, or storing a second source of truth.
- Namespace subtree expansion, cross-session retrieval, MCP, SDK, UI, or hosted
  product behavior.

## Decisions

### 1. Derived PostgreSQL chunk records

Add a forward migration for a chunk header/item model keyed by exact
tenant/project/namespace scope and a source identity consisting of source kind,
source ID/version, chunker policy version, and renderer version. Store bounded
content plus ordinal, parent identity, session/user attribution, source ranges,
character/token counts, lifecycle snapshot, and timestamps. Enforce uniqueness for
the same source/version/policy/ordinal so retries converge without overwriting
history.

**Alternatives considered:**

- In-memory chunks: rejected because restart, audit, and deterministic rebuilds
  require durable state.
- Replacing canonical memory rows with chunks: rejected because it breaks
  append-only provenance and canonical lifecycle semantics.
- External search index: rejected because PostgreSQL remains the only system of
  record and the first slice does not require a new operational dependency.

### 2. Boundary-first deterministic chunker

Normalize source text only for deterministic boundary detection, then split at
message, sentence, paragraph, list, and code boundaries before applying configured
maximum character/token limits. Preserve source offsets and ordinal ordering. A
single oversized atomic unit is split with a deterministic hard bound and records
the same parent/source lineage.

**Alternatives considered:**

- Fixed-width-only splitting: rejected because it separates semantic units and
  reduces evidence traceability.
- Model-generated segmentation: rejected for nondeterminism, provider coupling,
  and unavailable offline reproducibility.

### 3. Class-aware chunk policy

Use one policy resolver with explicit rules per memory class. Profile chunks favor
atomic facts; episodic chunks favor event/message units; procedural chunks preserve
rule and step groups; summaries use larger bounded coverage units; relation chunks
remain atomic. The resolver returns policy version and bounded omission reasons so
materialization and evaluation can explain decisions without leaking payloads.

**Alternatives considered:**

- One global size policy: rejected because memory classes have different evidence
  boundaries and context use.
- Per-handler ad hoc branching: rejected because it would make rebuilds and
  diagnostics inconsistent.

### 4. Controlled rollout and retrieval integration

Materialization and chunk candidate consumption are independently gated by a
versioned rollout setting. The default path remains canonical-memory retrieval;
shadow mode may collect bounded candidate diagnostics without changing public
results. When enabled for an exact scope, retrieval may return a chunk hit but must
retain parent canonical/source citations and enforce the same lifecycle and scope
filters before and after parent expansion.

**Alternatives considered:**

- Immediate global default: rejected because the baseline must remain a safe,
  comparable rollback target.
- Separate public chunk API: rejected because this slice changes retrieval
  representation, not the public resource model.

### 5. Rebuild and evidence contract

Rebuild reads only authorized PostgreSQL source records plus policy/renderer
versions, writes a new derived version when source identity changes, and preserves
prior chunk history. Source watermarks and deterministic content identities make
rebuilds replay-safe. Parent/adjacent lookup is capped by count and budget and
never performs scope widening.

### 6. Verification strategy

Add unit tests for segmentation, bounds, class policy, identity, and diagnostics;
repository tests for exact-scope reads, idempotency, lifecycle rejection, and
rebuild determinism; PostgreSQL integration tests gated by an explicit owned DSN;
and retrieval/context evaluation cases comparing chunk-enabled shadow results to
the existing `canonical-v1` / `baseline-v1` report. Public responses must not
expose internal chunk diagnostics unless an authorized evaluation/admin path is
used.

## Risks / Trade-offs

- **[Risk] Chunking increases candidate count and duplicate opportunities.** → Keep
  candidates bounded per source, retain canonical fallback, and measure duplicate
  rate against the retrieval baseline before enabling the path.
- **[Risk] Source offsets become invalid after source normalization.** → Store
  offsets against the original immutable source representation and treat normalized
  text as a segmentation aid only.
- **[Risk] Parent expansion leaks hidden or foreign evidence.** → Re-run exact
  scope, lifecycle, and authorization checks on every parent/adjacent lookup and
  fail closed on uncertainty.
- **[Risk] Rebuild policy changes create stale derived records.** → Include policy
  and renderer versions in source identity, preserve prior versions, and expose
  staleness as a bounded diagnostic rather than silently mixing versions.
- **[Risk] Token counting differs across providers.** → Use a deterministic service
  counter for admission/budget bounds and record the counter version; provider
  tokenizers are not required for correctness in this slice.
- **[Risk] PostgreSQL migration adds write and storage overhead.** → Keep indexes
  scoped and bounded, materialize asynchronously, and retain a disable switch for
  chunk consumption.

## Migration Plan

1. Apply the additive PostgreSQL migration and verify schema/version integrity.
2. Deploy chunk domain/repository code with materialization and consumption
   disabled by default.
3. Backfill selected owned scopes asynchronously using explicit policy and
   renderer versions; verify provenance, lifecycle, and rebuild determinism.
4. Run shadow retrieval evaluation and compare against `canonical-v1` / `baseline-v1`.
5. Enable chunk candidates only for an explicitly approved scope and retain the
   canonical fallback.
6. Roll back by disabling materialization/consumption flags; no destructive
   migration or canonical data rewrite is required.

## Open Questions

- Should the first chunker configuration be service-wide with per-class limits, or
  allow per-scope overrides behind the existing policy governance contract?
- Which deterministic token-counting approximation should be recorded as the first
  `counter_version` for mixed-language content?
- Should parent/adjacent expansion be exposed only through internal retrieval
  diagnostics initially, or also be used by context assembly shadow mode?
