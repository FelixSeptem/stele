## Why

Stele now has a provider-neutral reasoning envelope, reserved-insight
activation, temporal validity, and durable replay, but `contradiction` remains
reserved without a concrete, testable insight contract. Contradictory facts are
the safest first reasoning type because their evidence can be tied to existing
version, validity, provenance, and scope records. A bounded contradiction path
will make that value measurable without allowing model output to become
canonical truth or ordinary context.

## What Changes

- Add a `governed-contradiction-insights` capability for detecting and
  evaluating evidence-backed contradictions within one exact scope.
- Define contradiction pairs and groups over lifecycle-visible canonical
  versions, with explicit temporal coexistence rules so facts valid in
  different intervals are not falsely classified as conflicts.
- Require both sides of a contradiction to retain source-version identity,
  provenance, validity bounds, evidence digests, uncertainty, and a stable
  replay identity.
- Keep detection offline or shadow-only by default; neither mode may alter
  canonical memory, active insight state, default retrieval, or ordinary
  context assembly.
- Route any apply operation through the existing reserved-insight activation
  policy with exact-scope checks, idempotency, append-only lifecycle history,
  audit attribution, and rollback.
- Add bounded replay outcomes, diagnostics, and telemetry for contradiction
  detection, temporal coexistence, stale evidence, policy eligibility,
  suppression, and false-positive review without exposing source content or
  identifiers.

## Non-goals

- Do not enable `contradiction` activation by default or infer contradictions
  from semantic similarity alone.
- Do not treat two temporally disjoint, explicitly scoped facts as a conflict
  unless a policy says they are mutually exclusive.
- Do not implement `hypothesis`, `goal`, or `causal_link` activation in this
  change.
- Do not overwrite, merge, delete, or rewrite canonical memory, source
  evidence, prior insight versions, or temporal history.
- Do not change default retrieval ranking or inject contradiction candidates into
  ordinary context.
- Do not add a new storage system, graph database, SDK, UI, MCP surface, or
  provider-specific reasoning dependency.

## Capabilities

### New Capabilities

- `governed-contradiction-insights`: Define bounded contradiction detection,
  evidence binding, temporal coexistence, replay, review, and governed
  activation for one exact scope.

### Modified Capabilities

- `governed-experience-insights`: Add type-specific contradiction provenance,
  lifecycle, confidence, review, and feedback requirements.
- `governed-reserved-insight-activation`: Add contradiction-specific policy
  gates for mutually exclusive evidence, temporal validity, and review state.
- `derived-insight-replay`: Add deterministic contradiction-pair/group replay
  and stable temporal-conflict dispositions.
- `context-assembly`: Keep contradiction candidates and non-authoritative
  results out of default context and define authorized contradiction sections.
- `service-observability`: Add bounded contradiction operation, temporal
  coexistence, eligibility, and review categories to reasoning diagnostics.

## Impact

- Affected implementation areas include derived-insight orchestration,
  temporal evidence selection, reserved activation policy, replay reports,
  context authorization, and low-cardinality telemetry.
- PostgreSQL remains the only system of record. Contradiction records are
  append-only, scoped, provenance-linked, and rebuildable from canonical
  versions and source evidence.
- Public behavior remains unchanged until an independently versioned,
  exact-scope contradiction policy is explicitly enabled.
- The proposal follows the repository's `openspec-propose`, `openspec-apply`,
  and `openspec-archive-seq.ps1` workflow; implementation must include Go and
  PostgreSQL + pgvector verification before activation is considered.
