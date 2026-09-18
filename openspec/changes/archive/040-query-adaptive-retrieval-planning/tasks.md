## 1. Planner contracts and deterministic classification

- [x] 1.1 Add failing table-driven tests in `internal/retrieval/retrieval_plan_test.go` for all seven query families, deterministic precedence, ambiguous general fallback, stable identity, and rejection of unknown versions or incomplete plans; verify the focused tests fail before planner types exist.
- [x] 1.2 Implement focused policy, input, family, channel-allocation, context-priority, follow-up-rule, plan, disposition, and fallback types in `internal/retrieval/retrieval_plan.go`; verify pure validation and repeated classification tests pass without database or network access.
- [x] 1.3 Add fuzz/property tests for arbitrary validated/invalid analysis categories, constraints, policies, and limits; verify planning terminates without panic, preserves inputs, produces deterministic canonical ordering, and never exceeds hard bounds.
- [x] 1.4 Review mature Go packages for bounded policy validation and immutable configuration modeling, record the standard-library dependency decision in `design.md`, and verify no dependency is added without material risk reduction.

## 2. Shared budget ledger and evidence assessment

- [x] 2.1 Add failing tests in `internal/retrieval/retrieval_budget_test.go` for per-channel allocation, aggregate accounting, headroom redistribution, reserved reranker headroom, elapsed exhaustion, and over-consumption.
- [x] 2.2 Implement request-local envelope and ledger types in `internal/retrieval/retrieval_budget.go`; verify no channel or pass exceeds the shared envelope.
- [x] 2.3 Add failing tests in `internal/retrieval/evidence_assessment_test.go` for sufficient, zero-hit, below-minimum, high-attrition, unavailable-channel, no-headroom, and terminal-incomplete outcomes.
- [x] 2.4 Implement bounded aggregate evidence assessment in `internal/retrieval/evidence_assessment.go`; verify it serializes no content, identifiers, query text, scores, or generated judgments.

## 3. Exact-scope planner rollout persistence

- [x] 3.1 Add failing domain tests for complete planner bundles, exact scope/selectors, compatibility, diagnostics, shadow, active, disabled, expired, malformed, foreign, and rollback behavior.
- [x] 3.2 Extend `internal/memory/ranking_rollout.go` with versioned planner policy, selector, and fail-closed resolution.
- [x] 3.3 Add additive migration `0013_query_adaptive_retrieval_planning` plus manifest and upgrade tests proving existing rows remain baseline-only.
- [x] 3.4 Add repository round-trip and isolation tests, then extend `ranking_rollout_repository.go` for planner policy persistence.
- [x] 3.5 Extend admin/OpenAPI rollout schemas only as needed; verify ordinary search/context schemas remain unchanged.

## 4. Planned retrieval orchestration

- [x] 4.1 Add failing service tests for baseline, diagnostics, shadow, active, disabled, incompatible, and rollback planner resolutions.
- [x] 4.2 Integrate plan construction/validation and declared channel execution into focused `Service.Search` helpers while preserving all visibility checks.
- [x] 4.3 Add fusion tests for planned channel subsets, limits, family parameters, compatibility, and two-pass candidates; preserve canonical identity/citations.
- [x] 4.4 Add tests and adapter for validated query analysis to planner inputs while retaining the mandatory original query.
- [x] 4.5 Add reranker tests proving planner eligibility cannot independently activate it and ledger headroom is required.

## 5. Single bounded follow-up pass and context shaping

- [x] 5.1 Add failing tests for sufficient first pass, eligible follow-up, no headroom, follow-up failure, duplicates, and terminal incompleteness; verify at most two passes.
- [x] 5.2 Implement the follow-up executor and merge both passes through existing validated ranking and packing stages.
- [x] 5.3 Add context-priority/quota/budget/citation/shadow/rollback tests and integrate plan priorities without new response sections.
- [x] 5.4 Add fault-injection tests proving optional planner failures return the approved baseline and baseline failures keep existing semantics.

## 6. Diagnostics, telemetry, and API compatibility

- [x] 6.1 Add allowlist telemetry tests and planner metrics with low-cardinality categories only.
- [x] 6.2 Extend authorized planner diagnostics and redaction tests.
- [x] 6.3 Add search, context, provider, and OpenAPI compatibility tests across rollout stages.

## 7. Evaluation and release gates

- [x] 7.1 Add/version planner fixtures with family, identity, channel, budget, protected-category, and follow-up expectations plus validation tests.
- [x] 7.2 Extend evaluation metadata, compatibility, metrics, replay, and reports with family and per-pass evidence/resource metrics.
- [x] 7.3 Extend release evidence/policy tests so all safety, resource, compatibility, fallback, and rollback failures prevent activation.
- [x] 7.4 Update the evaluation script/runner for explicit owned PostgreSQL+pgvector baseline/planner comparison and stable missing-DSN non-pass.
- [x] 7.5 Run real-stack evidence when an owned DSN exists; otherwise document non-pass and keep rollout at diagnostics/shadow maximum.

## 8. Documentation and verification

- [x] 8.1 Update retrieval quality, release gate/checklist, configuration, and operator docs for planner behavior and activation requirements.
- [x] 8.2 Update the roadmap to actual RQ1 evidence, keep RQ2-RQ4 pending, and replace the stale roadmap copy with a compatibility link.
- [x] 8.3 Run focused uncached retrieval, memory, storage, telemetry, and app tests.
- [x] 8.4 Run full tests, vet, and race tests where supported; record exact environmental limits.
- [x] 8.5 Run strict/all OpenSpec validation, `git diff --check`, and template/sensitive/status scans.
