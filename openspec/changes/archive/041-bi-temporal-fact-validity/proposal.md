## Why

Stele currently records when canonical rows and versions were written, but it
cannot distinguish that recorded time from the interval in which a fact was
true. As a result, a stale fact can remain retrieval-eligible when lexical or
semantic similarity is strong, while historical questions cannot deterministically
select the version that was valid at the requested time. RQ1 now provides the
explicit temporal planning and evaluation boundary needed to add this model
without changing ordinary retrieval into an implicit history query.

## What Changes

- Introduce a `bi-temporal-fact-validity` capability for factual canonical
  versions using separate recorded/ingested time and valid-time intervals.
- Add a stable temporal fact identity and append-only validity corrections so
  mutually exclusive versions are selected deterministically without overwriting
  canonical history.
- Represent validity as a half-open interval `[valid_from, valid_to)` with an
  open-ended current interval; reject malformed bounds and make overlap policy
  explicit and auditable.
- Preserve legacy rows through a documented current-compatible migration rule;
  do not make existing memories disappear solely because they predate validity
  columns.
- Make default retrieval select only lifecycle-visible versions valid at the
  request's current time. Historical `as_of` and interval queries require an
  explicit temporal plan, authorized scope, and deterministic time constraint.
- Preserve the existing meaning of `time_from`/`time_to` as recorded/update-time
  filters unless a caller opts into the new valid-time fields; do not silently
  reinterpret an existing public filter.
- Propagate source-version and validity identity through relation projections,
  chunks, embeddings/rebuilds, citations, context projections, and evaluation
  fixtures. Derived artifacts remain rebuildable and never become canonical.
- Extend manual correction and governance paths so retroactive fact corrections
  append a new version or temporal correction record, retain provenance, and
  cannot erase prior valid intervals.
- Extend deterministic current/historical benchmark families, stale-fact
  suppression metrics, isolation/lifecycle assertions, and release gates. A
  skipped or missing owned PostgreSQL + pgvector run remains a non-pass.
- Add bounded authorized diagnostics for temporal disposition categories without
  exposing hidden content, raw identifiers, query text, or provider payloads.

## Non-goals

- No graph database, recursive graph traversal, or RQ3 evidence-path ranking.
- No online learning, popularity ranking, autonomous validity inference, or LLM
  judge as a release gate.
- No destructive rewrite of canonical history, raw events, provenance, chunks,
  or embeddings.
- No change to the default lifecycle visibility rules or exact
  tenant/project/namespace isolation boundaries.
- No reinterpretation of existing recorded-time filters, and no requirement that
  every non-factual derived class gain temporal semantics in this change.
- No SDK, UI, answer generation, or end-user product logic.

## Capabilities

### New Capabilities

- `bi-temporal-fact-validity`: Separate recorded time and fact-valid time,
  define interval semantics, temporal identity, correction rules, and current vs
  historical selection.

### Modified Capabilities

- `canonical-memory-lifecycle`: Canonical factual versions carry append-only
  validity intervals and current visibility must account for valid time.
- `memory-history-and-provenance`: History/provenance exposes temporal identity,
  recorded time, validity bounds, and correction lineage through privileged paths.
- `manual-memory-mutation-surface`: Manual factual corrections append temporal
  versions and validate interval/identity constraints.
- `memory-search-contract`: Add explicit valid-time selectors and preserve the
  existing recorded-time filter semantics and public response compatibility.
- `hybrid-memory-retrieval`: Lexical, semantic, relation, and chunk retrieval
  apply the same valid-time and lifecycle predicates before fusion.
- `relation-enhanced-retrieval`: Relation projections retain source-version and
  validity identity and cannot bridge expired or unauthorized facts.
- `hierarchical-memory-chunking`: Chunks retain the source version's validity
  snapshot and are excluded when that source is not valid for the query.
- `query-adaptive-retrieval-planning`: Temporal plans carry explicit valid-time
  constraints and fail closed when historical access is not authorized.
- `retrieval-evaluation-baseline`: Fixtures/replay/reporting cover current,
  historical, retroactive-correction, and stale-fact cases.
- `retrieval-release-gate-and-progressive-context-evaluation`: Release gates
  treat stale-fact wins, temporal ambiguity, provenance mismatch, and validity
  leakage as hard failures.
- `runtime-api-contract-publication`: Publish additive temporal search and
  history fields with OpenAPI-first schemas and compatibility rules.

## Impact

- **Storage and domain:** additive PostgreSQL migration; canonical/version
  domain types, temporal identity, interval validation, provenance records, and
  scope-safe indexes. PostgreSQL remains the only system of record.
- **Retrieval:** repository predicates and planner/executor contracts gain
  explicit valid-time constraints while preserving current lexical/semantic/
  relation/chunk channels, fusion, citations, and bounded budgets.
- **Derived data:** relation projections, chunks, embeddings, citations, and
  context projections bind to immutable source-version/validity identities and
  rebuild deterministically after temporal corrections.
- **APIs:** additive OpenAPI request/response fields for valid-time selection;
  ordinary callers retain lifecycle-safe current behavior and recorded-time
  filters.
- **Evaluation and operations:** new temporal fixtures, metrics, diagnostics,
  release-policy identities, migration/backfill evidence, and rollback that
  disables temporal-aware retrieval without deleting history.
- **Dependencies:** no new runtime dependency is expected; standard-library
  time/interval validation and existing PostgreSQL/pgvector infrastructure are
  sufficient. Implementation follows [`openspec-apply-change`](../../.codex/skills/openspec-apply-change/SKILL.md),
  with [`verification-before-completion`](../../.agents/skills/superpowers/verification-before-completion/SKILL.md)
  before merge and the repository archive command after completion.
