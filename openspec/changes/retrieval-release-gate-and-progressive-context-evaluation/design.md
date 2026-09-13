## Context

The repository already provides versioned retrieval fixtures and replay, stable
hybrid fusion, chunk and diversity policies, bounded query understanding,
quality-aware reranking, context projections, benchmark adapters, and redacted
observability. The missing piece is a release-evidence layer that composes these
contracts and proves compatibility, safety, freshness, and reversibility before
an experimental strategy can be promoted. See `proposal.md` for motivation and
the capability spec for observable behavior.

## Goals / Non-Goals

**Goals:**

- Compose existing replay and comparison identities into one release decision.
- Make real-provider evidence explicit, opt-in, redacted, and reproducible.
- Compare progressive context and parent-first retrieval without changing the
  ordinary retrieval path.
- Preserve exact scope, lifecycle visibility, canonical immutability, and
  rebuildability while producing actionable reports and runbooks.
- Make retention, rollback, and policy ownership testable and auditable.

**Non-Goals:**

- Introducing another persistence system or replacing PostgreSQL.
- Choosing or hosting embedding/reranking models.
- Making LLM judging, RAGAS-style scores, LoCoMo, or LongMemEval mandatory for
  deterministic release approval.
- Turning shadow experiments into default behavior in this change.
- Completing every P6 maintenance surface unrelated to release evidence.

## Decisions

### Decision 1: Add a composition layer instead of modifying baseline contracts

The release gate will consume existing fixture, replay, comparison, rollout,
projection, benchmark, and observability identities and add a versioned release
report/checklist around them. This avoids duplicating baseline logic and keeps
the immutable original-query baseline authoritative.

Alternatives considered:

- Modify every existing retrieval spec and report in place: rejected because it
  would create broad compatibility risk and blur archived change boundaries.
- Create a separate benchmark product: rejected because release evidence must
  exercise the real scoped retrieval path and existing safety gates.

### Decision 2: Require an explicitly owned evaluation DSN

Real-provider gates will accept only a dedicated evaluation DSN supplied through
the existing local environment/configuration path. The evaluator will never
fall back to the service DSN. Missing or incompatible prerequisites produce a
stable non-pass result and cannot authorize rollout.

Alternatives considered:

- Reuse `STELE_POSTGRES_DSN`: rejected because it risks testing or mutating a
  production database.
- Make PostgreSQL mandatory for all unit tests: rejected because offline,
  deterministic contributor feedback must remain fast.

### Decision 3: Keep progressive and parent-first paths offline or shadow-only

Progressive levels and parent-first expansion will run against the same fixture
and exact scope as the flat baseline, but their outputs remain evaluation
artifacts until policy approval. The production retrieval path is not branched
on experimental reports.

Alternatives considered:

- Enable parent-first by default after a single positive run: rejected because
  freshness, isolation, and rollback evidence must persist across categories.
- Fork a second canonical hierarchy: rejected because projections and chunks are
  derived, rebuildable artifacts and PostgreSQL remains the only source of truth.

### Decision 4: Use bounded aggregate diagnostics

Trajectory and integrity reports retain category counts, buckets, identities,
  and policy decisions only. Redaction happens before persistence or export;
  public APIs do not expose evaluation internals. Retention cleanup operates on
  derived artifacts and never deletes canonical source records.

Alternatives considered:

- Persist raw traces for later debugging: rejected due to query, scope, content,
  and credential leakage risk.
- Export high-cardinality labels to metrics: rejected because it harms cost,
  privacy, and operational stability.

### Decision 5: Make safety and information integrity hard gates

Isolation, lifecycle visibility, missing/altered evidence, stale projections,
  and failed rollback override aggregate recall or task-quality gains. Advisory
  metrics remain visible but cannot authorize a release.

Alternatives considered:

- Weighted aggregate score: rejected because a high score cannot compensate for
  data leakage or silent information loss.
- LLM judge as the release authority: rejected because deterministic qrels and
  safety checks must remain reproducible and auditable.

## Risks / Trade-offs

- [Real-provider drift] Provider behavior can change between runs → record
  logical model identity, capability mode, dimensions, policy versions, and
  bounded latency; require a fresh owned run for release evidence.
- [Projection staleness] Derived context may lag canonical records → carry source
  watermarks and fail closed on stale or divergent projections.
- [Report leakage] Evaluation artifacts can accidentally contain sensitive data →
  centralize redaction, allowlist fields, test forbidden tokens, and apply bounded
  retention before export.
- [Operational cost] Full real-stack and progressive evaluations are expensive →
  keep them opt-in, scope-isolated, budgeted, and separate from default tests.
- [False confidence from synthetic data] Repository fixtures may not represent
  production workloads → label synthetic/offline runs as non-pass and require
  owned real-provider evidence for activation.

## Migration Plan

1. Add report/schema types and compatibility checks without changing retrieval
   defaults.
2. Wire existing replay, projection, benchmark, and rollout evidence into the
   release checklist and redacted report.
3. Add offline/shadow progressive and parent-first evaluators plus integrity and
   trajectory retention tests.
4. Document explicit local provider/evaluation configuration, rebuild/re-index,
   rollback, and cleanup procedures.
5. Enable the gate in CI for repository-owned fixtures; run real-provider gates
   only in an explicitly configured release job.
6. Roll back by disabling the experimental policy and retaining canonical source
   records; rebuild derived artifacts after code or provider changes.

## Open Questions

- Exact numeric threshold values can remain in the versioned release-policy
  configuration and be tuned from the first owned evidence run without changing
  the capability contract.
