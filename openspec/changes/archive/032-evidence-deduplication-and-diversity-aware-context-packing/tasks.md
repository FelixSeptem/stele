## 1. Policy Contracts And Deterministic Selection

- [x] 1.1 Inspect the existing stable-fusion, context-packing, rollout-policy, and evaluation contracts; define versioned diversity-policy, disposition, and bounded parameter domain types that reuse their naming and scope conventions.
- [x] 1.2 Implement deterministic canonical/source-event/parent-lineage candidate identity deduplication with stable representative and citation retention rules; add table-driven tests for overlapping lexical, semantic, relation, and chunk evidence.
- [x] 1.3 Implement bounded semantic-similarity clustering and MMR/equivalent diversity selection over already validated candidates, including compatible embedding-revision checks, unknown coverage categories, stable tie breaking, and identity-only degradation when semantic input is unavailable.
- [x] 1.4 Add pure unit tests for deterministic replay, memory-class/session/entity/time coverage, threshold boundaries, candidate and pairwise-comparison bounds, and protection against an invalid candidate suppressing valid evidence.

## 2. Scoped Rollout And Durable Policy Governance

- [x] 2.1 Extend the existing scoped ranking rollout contract to represent diversity policy identity, parameters, diagnostics-only/shadow/active/disabled state, activation evidence, and rollback without adding a parallel rollout mechanism.
- [x] 2.2 Add any required forward PostgreSQL migration, repository methods, and audit-safe persistence for diversity rollout selection; cover idempotent writes, exact tenant/project/namespace matching, invalid parameter rejection, and rollback in PostgreSQL integration tests.
- [x] 2.3 Wire rollout resolution so diversity selection is default-safe and exact-scope only; verify that absent, disabled, malformed, foreign-scope, or hidden-policy inputs retain the deterministic identity-deduplicated baseline.

## 3. Retrieval And Context Assembly Integration

- [x] 3.1 Integrate identity/lineage deduplication after lifecycle-safe stable fusion and before final evidence handoff, preserving canonical fallback, bounded citations, public result identity, and optional-channel degradation.
- [x] 3.2 Integrate active diversity selection into applicable existing context sections after eligibility and summary preference but before existing token/character packing; preserve section names, caller budgets, citations, projections, and fail-closed scope/lifecycle behavior.
- [x] 3.3 Add authorized bounded diagnostics for policy version and duplicate/diversity/budget dispositions, and regression tests confirming that ordinary search/context responses disclose none of the policy, similarity, cluster, hidden-candidate, or foreign-scope internals.

## 4. Evaluation And Safety Gates

- [x] 4.1 Extend repository-owned retrieval fixtures with repeated versions, shared source/parent lineage, near-identical visible evidence, independent multi-hop evidence, constrained section budgets, and matching hidden/foreign-scope distractors.
- [x] 4.2 Extend deterministic evaluation reports and baseline comparison to identify diversity-policy version and aggregate dispositions and to compare duplicate rate, protected recall, evidence coverage, candidate-pool size, and latency against the identity-deduplicated baseline.
- [x] 4.3 Add release-policy tests proving that scope/lifecycle leakage is always a hard failure and that protected recall, multi-hop coverage, budget, or latency regressions prevent scoped diversity activation.
- [x] 4.4 Run the real-stack evaluation only with an explicitly owned PostgreSQL + pgvector test DSN; verify the documented stable non-pass skip category when that prerequisite is absent and never use an ambient service DSN.

## 5. Documentation And Verification

- [x] 5.1 Update retrieval/context governance and operational documentation with diversity-policy lifecycle, exact-scope rollout, diagnostic redaction, evaluation evidence, and rollback instructions; keep public API documentation unchanged unless an existing authorized diagnostic contract requires publication.
- [x] 5.2 Run focused retrieval, context-assembly, rollout-policy, evaluation, and PostgreSQL integration tests, then `go test ./...`, `go test -race ./...`, `openspec validate evidence-deduplication-and-diversity-aware-context-packing --strict`, and `git diff --check`; record controlled real-stack skips separately from passing evidence.
