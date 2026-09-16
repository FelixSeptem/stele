# Retrieval Quality Roadmap Calibration Design

Date: 2026-09-16

## Purpose

Reconcile the Stele v1 roadmap with the code and archived OpenSpec state, then
extend the same roadmap with the next retrieval-quality work without re-planning
capabilities that are already implemented.

This is a roadmap-documentation change. It does not change runtime behavior,
public APIs, schemas, or rollout policy.

## Evidence Baseline

The roadmap update must use the repository state on `main` as its source of
truth:

- OpenSpec has no active changes and `openspec validate --all` reports 63 passed
  specifications and no failures.
- The archived baseline extends through change 039.
- Changes 032 through 039 cover evidence deduplication and diversity, stable
  fusion, bounded query understanding, retrieval release gates, quality-aware
  reranking, durable multi-scope maintenance, the runtime memory-provider
  contract, and retrieval release evidence.
- Retrieval already supports lexical, semantic, relation, and chunk recall;
  RRF and normalized weighted fusion; bounded query analysis; lineage-aware
  deduplication; MMR and coverage diversity; quality signals; an optional
  controlled reranker; progressive context evaluation; and deterministic
  retrieval metrics.
- Governed experience insights are an archived baseline from change 013 rather
  than wholly new P8 work.

The calibrated roadmap must correct the current stale claims that P4 requires
archive reconciliation, Task 6.7 is pending, P6 lacks an archive, and P7 is new
work.

## Document Authority

`docs/roadmaps/2026-05-28-stele-v1-roadmap.md` remains the authoritative v1
roadmap. The tracked file under `docs/docs/roadmaps/` currently contains an old
Phase 1-5 copy and must no longer present conflicting product status.

The legacy path will become a short compatibility notice linking to the
authoritative roadmap rather than a second full copy. This keeps existing links
usable while establishing one maintained source of truth.

## Roadmap Structure

The roadmap will preserve P0 through P7 as the historical, implemented baseline.
It will separate the implemented governed-insight portion of P8 from genuinely
optional adapter and convention work.

The next approved planning frontier will be represented as four sequential,
Stele-native retrieval-quality workstreams:

```text
Archived baseline P0-P7
        |
        v
RQ1 Query-adaptive retrieval planning
        |
        v
RQ2 Bi-temporal fact validity
        |
        v
RQ3 Bounded graph-distance retrieval and evidence paths
        |
        v
RQ4 Context efficiency and feedback calibration
```

These identifiers distinguish future work from the already archived P0-P8
history and avoid rewriting completed phases.

## Status Reconciliation

The global snapshot, priority table, critical path, stage descriptions,
execution order, review gates, and immediate next step must agree on the
following state:

| Area | Calibrated state |
| --- | --- |
| P0 | Implemented and archived through changes 023 and 029. |
| P1 | Implemented and archived through changes 024 and 028. |
| P2 | Implemented and archived through changes 030 and 031. |
| P3 | Implemented and archived through changes 027, 032, and 033. |
| P4 | Implemented and archived through changes 034 and 036. |
| P5 | Implemented and archived through changes 025, 026, 035, and 039. |
| P6 | Implemented and archived through change 037. |
| P7 | Implemented and archived through change 038. |
| P8 | Governed insights are implemented through changes 013 and 014; MCP and shared-memory conventions remain optional candidates. |

The roadmap will retain historical descriptions where they explain why the
architecture exists, but future-oriented wording will not imply that archived
work is still pending.

## RQ1: Query-Adaptive Retrieval Planning

### Goal

Convert existing query-analysis hints and rollout controls into a versioned,
deterministic retrieval plan that chooses a bounded strategy for each query
family.

### Planned capabilities

- Classify exact lookup, semantic, temporal, entity-relation, multi-hop, and
  procedural query families using existing bounded signals.
- Select enabled recall channels, per-channel candidate budgets, fusion
  parameters, memory-class quotas, relation depth, reranker eligibility, and
  context priorities.
- Adapt candidate budgets to query complexity, post-filter attrition, and
  reranker headroom while retaining hard candidate and latency caps.
- Allow at most one bounded second retrieval pass when evidence completeness is
  below policy; the second pass must use an explicit follow-up plan and the same
  isolation and lifecycle filters.
- Roll out through `diagnostics_only`, `shadow`, and `active_for_scope` states.

### Acceptance gates

- Plans have stable version and policy identities and deterministic replay.
- Query-family parameters are derived from Stele evaluation evidence, not copied
  constants from external frameworks.
- Protected recall, isolation, lifecycle, latency, and rollback gates remain
  green.
- The service returns memory evidence only and never runs an unbounded agentic
  retrieval loop or generates the caller's final answer.

## RQ2: Bi-Temporal Fact Validity

### Goal

Distinguish when a fact was recorded from when it was true so current and
historical questions retrieve the right canonical version.

### Planned capabilities

- Represent `ingested_at`, `valid_from`, `valid_to`, and `superseded_at` with
  append-only versions and provenance.
- Make current-fact retrieval suppress no-longer-valid evidence by default.
- Allow historical validity intervals only through an explicit temporal query
  plan.
- Propagate temporal validity through relation projections, derived chunks,
  citations, rebuilds, and evaluation fixtures.

### Acceptance gates

- Canonical memory is never overwritten in place.
- Temporal boundaries are scope-safe, auditable, replayable, and deterministic.
- Current and historical benchmark families improve without simple-fact or
  lifecycle regressions.
- Stale facts cannot win solely through lexical or semantic similarity.

## RQ3: Bounded Graph-Distance Retrieval And Evidence Paths

### Goal

Improve entity-centric and multi-hop completeness using PostgreSQL relation
projections while keeping graph memory subordinate to canonical persistence.

### Planned capabilities

- Seed expansion from authorized semantic or entity hits.
- Traverse PostgreSQL relation projections with recursive CTEs at depth one or
  two only.
- Rank path candidates using bounded graph-distance decay, edge confidence,
  temporal validity, and source reliability.
- Preserve a bounded evidence path for multi-hop citations and evaluation.
- Apply exact scope, lifecycle, candidate, time, and context-budget limits at
  every expansion step.

### Acceptance gates

- PostgreSQL remains the only system of record; no Neo4j, FalkorDB, or other
  graph database is introduced.
- Relation and path state remains derived and rebuildable from durable records.
- Multi-hop evidence coverage improves without leakage, candidate explosion, or
  unacceptable p95 latency.
- The existing lexical, semantic, relation, and chunk fusion path remains the
  reversible baseline.

## RQ4: Context Efficiency And Feedback Calibration

### Goal

Measure retrieval quality per unit of context and prevent weak behavioral
signals from producing self-reinforcing popularity bias.

### Planned capabilities

- Measure retrieved context tokens, relevant-token ratio, evidence density,
  duplicate-token rate, stale-token rate, quality per 1,000 context tokens, and
  reranker or model cost per query family.
- Attribute quality and cost separately to the first and optional second pass.
- Calibrate packing and planner policy against context efficiency as well as
  traditional rank metrics.
- Keep access frequency or reinforcement as an optional, weak, bounded,
  asynchronous experiment.
- Prefer existing usefulness, task-success, verification, evidence coverage,
  reliability, freshness, and conflict signals.

### Acceptance gates

- Efficiency gains cannot waive isolation, lifecycle, provenance, or protected
  recall gates.
- Feedback contributions have caps, evidence minima, decay or expiry policy,
  audit history, and a tested disable path.
- A popularity-only signal cannot promote evidence or dominate ranking.
- Reports remain bounded and redact query text, scope values, identifiers,
  content, credentials, provider payloads, and raw scores.

## Shared Release Policy

Every RQ workstream must define:

- a versioned strategy identity and owner;
- exact tenant, project, and namespace boundaries;
- candidate, depth, time, token, and cost limits;
- deterministic offline and replay evidence;
- diagnostics or shadow evidence before scoped activation;
- a stop condition and tested rollback path;
- rebuildability of all derived state from PostgreSQL;
- zero-tolerance isolation and hidden-lifecycle leakage gates before quality
  comparisons.

LLM judges and vendor benchmark claims may supplement release evidence but may
not replace deterministic qrels, safety, latency, provenance, or rollback gates.

## Explicit Non-Goals

- No second canonical store or polyglot persistence architecture.
- No graph database dependency.
- No in-place profile or canonical-memory overwrite.
- No global recency decay applied uniformly across memory classes.
- No free-running query-rewrite or retrieval agent loop.
- No service-generated final answer.
- No SDK, UI, or end-user product work.
- No adoption of external implementation code with incompatible licensing.

## Documentation Changes

Implementation of this design will update only roadmap documentation:

1. Update the authoritative roadmap snapshot date and reconciliation table.
2. Correct the P3-P8 archive mappings and completed-status language.
3. Replace the stale provider-readiness critical path with an archived-baseline
   summary followed by RQ1-RQ4.
4. Mark Phase 6 Tasks 6.5 through 6.7 and Stages 6 through 9 as completed
   baselines, retaining their historical acceptance criteria.
5. Split implemented governed insights from optional MCP and convention work.
6. Add RQ1-RQ4 goals, dependencies, outputs, acceptance gates, rollback, and
   non-goals.
7. Align execution order, review gates, and the immediate next step with RQ1.
8. Replace the stale tracked roadmap copy with a compatibility link to the
   authoritative document.

## Verification

Because implementation changes documentation only, verification consists of:

- checking every referenced archive number against
  `openspec/changes/archive/INDEX.md`;
- running `openspec validate --all`;
- scanning the updated roadmap for contradictory pending/completed claims;
- checking local Markdown links introduced by the change;
- running `git diff --check`;
- reviewing the final diff to ensure existing untracked caches, backups, logs,
  and plan files are untouched.

The repository does not currently contain the previously anticipated roadmap or
documentation consistency scripts, so they are not listed as executable gates.
