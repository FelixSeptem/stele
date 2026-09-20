## Why

RQ1 can identify entity-relation and multi-hop queries, while RQ2 makes the
validity and source lineage of relation evidence explicit. The current relation
path remains a shallow optional enrichment, so eligible evidence that is one or
two further relation steps away may be missed. RQ3 adds narrowly governed,
request-time graph-distance expansion using existing PostgreSQL relation
projections without making graph state a second source of truth.

## What Changes

- Add a bounded graph-distance retrieval capability for `entity_relation` and
  `multi_hop` plans. It starts from approved relation baseline seeds and expands
  only eligible PostgreSQL `relation_projections` during the request.
- Add deployment-level `GraphTraversalLimits`, with default
  `graph_max_hops=3`, and versioned exact-scope traversal policies that may
  only lower hop and resource limits. A request executes one hop by default;
  a valid scope policy may select zero through three hops.
- Require scope, authorization, lifecycle, valid-time, and source-version
  eligibility checks for every seed and edge at every hop; require immediate
  node/edge cycle truncation and independent per-hop, per-seed, and per-request
  edge, path, candidate, and time budgets.
- Define stable graph-path ordering and an in-memory-only `GraphPathProof`.
  Proofs support internal provenance validation but are neither canonical data
  nor public evidence.
- Make missing projections, policy rejection, authorization failure, traversal
  failure, and budget exhaustion fail closed to the existing RQ1/RQ2 baseline.
- Add exact-scope diagnostics, shadow evaluation, active rollout, and rollback
  rules. Ordinary API responses retain their present shape; graph internals are
  available only as authorized, redacted, low-cardinality diagnostics.
- Add PostgreSQL-only indexing/migration, deterministic fixture, integration,
  evaluation, and release-gate requirements for bounded traversal.

## Non-goals

- Introducing Neo4j, FalkorDB, another graph database, or any second system of
  record.
- Persisting multi-hop paths as canonical memories, facts, or citations.
- Expanding queries outside `entity_relation` and `multi_hop`, running
  unbounded/retrying graph search, or weakening exact scope and lifecycle
  filters.
- Changing ordinary search/context API response fields to expose paths, hops,
  identifiers, internal scores, policy values, hidden data, or diagnostics.

## Capabilities

### New Capabilities

- `bounded-graph-distance-retrieval`: Defines request-time, PostgreSQL-backed,
  policy-governed graph traversal, internal path proofs, diagnostics, rollout,
  fallback, and release safety requirements.

### Modified Capabilities

- `relation-enhanced-retrieval`: Extends optional bounded relation enrichment
  with deterministic multi-hop traversal eligibility, source/temporal checks,
  cycle prevention, and baseline fallback.
- `query-adaptive-retrieval-planning`: Extends immutable entity-relation and
  multi-hop plans with validated traversal-policy identity, bounded graph
  parameters, and reversible exact-scope execution.

## Impact

- **Code:** query-plan policy/configuration, retrieval orchestration and
  candidate fusion integration, relation repository interfaces and PostgreSQL
  queries/indexes, authorized diagnostics, fixtures/benchmarks, and rollout
  controls.
- **Data:** PostgreSQL `relation_projections` remains the single derived graph
  substrate; additive indexes or constraints may be migrated. No canonical
  memory, relation fact, fact-version, provenance, or audit-history rewrite is
  permitted.
- **APIs:** Ordinary public API contracts remain unchanged. Authorized
  diagnostic/evaluation surfaces may add only redacted aggregate traversal
  fields.
- **Dependencies:** Reuses PostgreSQL and existing relation projections; adds
  no graph database or external provider dependency.
- **References:**
  [RQ3 design](../../../docs/superpowers/specs/2026-09-20-bounded-graph-distance-retrieval-design.md);
  `openspec validate --all`; `openspec status --change bounded-graph-distance-retrieval`.
