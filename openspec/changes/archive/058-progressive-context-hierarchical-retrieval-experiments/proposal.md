## Why

Stele already has versioned context projections, hierarchical chunks, stable
hybrid fusion, diversity packing, and a retrieval release gate, but it does not
yet have one auditable contract for comparing progressive context depth and
parent-first retrieval. The next roadmap step should make those comparisons
replayable and measurable while preserving the stable flat-fusion retrieval
path and keeping experimental strategies offline or shadow-only until they pass
the existing release gates.

## What Changes

- Add a `progressive-context-hierarchical-retrieval` capability covering L0/L1/L2
  derived context levels, parent-first shadow planning, bounded expansion,
  deterministic replay, freshness and lifecycle validation, and redacted
  comparison artifacts.
- Define explicit level identities, source watermarks, freshness categories,
  token/character budgets, citation coverage, and rebuild identities for every
  progressive projection.
- Compare progressive levels and parent-first plans with the stable flat fusion
  baseline using protected recall, evidence integrity, duplicate rate,
  isolation, latency, and rollback gates.
- Keep all experimental plans offline, diagnostic, or shadow-only; shadow output
  must never appear in ordinary public retrieval or context responses.
- Add bounded fallback behavior for stale, hidden, foreign, missing, or
  over-budget projections and child expansion, with low-cardinality redacted
  diagnostics and retention-safe evidence.
- Extend existing projection, chunking, context assembly, release-evidence, and
  observability contracts so the experiment can be implemented without a second
  persistence system or a new public SDK/UI surface.

## Non-goals

- Do not change canonical memory, lifecycle transitions, or source-of-record
  ownership; L0/L1/L2 outputs remain derived artifacts.
- Do not change default public retrieval, stable fusion ordering, response
  sections, or OpenAPI behavior.
- Do not activate parent-first or progressive strategies for production traffic
  in this change.
- Do not add Redis, Kafka, a filesystem-backed memory store, or any other second
  persistence system.
- Do not add SDK, UI, MCP, or end-user product logic.
- Do not use generated summaries as authoritative memory or as a release gate
  without source evidence.

## Capabilities

### New Capabilities

- `progressive-context-hierarchical-retrieval`: Defines progressive L0/L1/L2
  context experiments, parent-first shadow comparisons, bounded replay, and
  fail-closed evidence rules.

### Modified Capabilities

- `retrieval-release-gate-and-progressive-context-evaluation`: Adds the release
  evidence and rollback contract for progressive levels and parent-first plans.
- `retrieval-release-evidence-run`: Adds experiment identity, freshness, and
  redacted artifact requirements to owned evidence runs.
- `context-assembly`: Constrains progressive context to authorized diagnostic or
  shadow envelopes and preserves existing budget, section, and citation rules.
- `versioned-context-projections`: Adds level-specific provenance, watermarks,
  freshness eligibility, and append-only rebuild behavior.
- `hierarchical-memory-chunking`: Adds bounded parent-first grouping and exact
  scope/lifecycle checks for expansion.
- `service-observability`: Adds low-cardinality telemetry for level, strategy,
  fallback, and shadow outcomes.

## Impact

- Affected areas include retrieval evaluation and release tooling, projection
  and chunk derivation metadata, context assembly diagnostics, and service
  metrics/logging.
- Implementation remains within the existing Go service and PostgreSQL+
  pgvector stack; no new runtime dependency or storage service is required.
- Offline fixtures, replay reports, migration/rebuild runbooks, and docs will
  be added or updated. Public API behavior remains unchanged unless an already
  authorized diagnostic surface needs bounded fields.
- Related workflow commands are `openspec apply`, `openspec validate --all
  --strict`, and the existing PostgreSQL/pgvector release-evidence checks.
