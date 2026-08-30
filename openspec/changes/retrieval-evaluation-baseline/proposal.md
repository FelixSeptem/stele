## Why

Stele has lifecycle-safe lexical, semantic, and relation retrieval, but it has no
versioned, repeatable way to determine whether a change to representation, embedding,
fusion, context packing, or ranking improves retrieval quality. Future work on chunks,
hybrid fusion, diversity, query understanding, and quality-aware ranking must be
measured against a stable baseline rather than inferred from ad hoc examples.

This change establishes the quality and safety measurement foundation before changing
the default retrieval behavior. It protects the product's mandatory scope and lifecycle
guarantees while making later retrieval changes evidence-driven and reversible.

## What Changes

- Add a versioned internal retrieval-evaluation fixture format that declares scoped
  source data, queries, expected evidence, acceptable evidence groups, and safety
  assertions without storing generated answers as memory.
- Add deterministic replay and reporting for selected fixture and ranking versions,
  with machine-readable and human-readable outputs suitable for local use and CI.
- Measure recall, ranking quality, multi-hop evidence coverage, duplicate rate,
  candidate-pool size, and bounded latency.
- Make cross-scope disclosure and lifecycle-visible retrieval violations hard failures
  rather than aggregate quality metrics.
- Add bounded retrieval diagnostics that explain candidate channels and final result
  disposition without exposing hidden content, cross-scope identifiers, credentials,
  DSNs, or raw evaluation payloads.
- Define release-gate thresholds and regression comparison behavior for future
  retrieval representation and ranking changes.

## Non-goals

- Do not change default lexical, semantic, relation, feedback-aware, or context ranking.
- Do not introduce chunk storage, parent-child memory, RRF, MMR, query decomposition,
  online rerankers, or an external search system.
- Do not add benchmark-specific Add/Search APIs, public leaderboard integration, or
  external benchmark data.
- Do not weaken PostgreSQL system-of-record ownership, append-only raw-event storage,
  provenance, scope isolation, or lifecycle visibility defaults.

## Capabilities

### New Capabilities

- `retrieval-evaluation-baseline`: Versioned, deterministic retrieval-quality replay,
  reporting, safety assertions, and release-gate behavior for internal fixtures.

### Modified Capabilities

- `hybrid-memory-retrieval`: Retrieval candidates and final results gain bounded,
  authorized evaluation diagnostics without changing ordinary retrieval ranking.
- `service-observability`: Retrieval-quality replay gains low-cardinality, redacted
  operation metrics and release-evidence reporting.

## Impact

- Affected code: `internal/retrieval`, `internal/storage/postgres`, `internal/app`,
  `internal/telemetry`, command wiring, and focused test fixtures.
- Affected APIs: no required change to ordinary public search or context contracts;
  any evaluation entrypoint is local/administrative and must remain redacted.
- Affected systems: PostgreSQL-backed fixtures, CI quality gates, self-hosting
  documentation, and future ranking rollout evidence.
- Related roadmap: `D:\code\stele\docs\roadmaps\2026-05-28-stele-v1-roadmap.md`,
  Phase 6 Task 6.1.
- Related commands: `openspec validate retrieval-evaluation-baseline --strict` and,
  after completion, `pwsh -File scripts/openspec-archive-seq.ps1 -ChangeName
  "retrieval-evaluation-baseline"`.
