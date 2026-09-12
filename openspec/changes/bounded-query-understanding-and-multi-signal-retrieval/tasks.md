## 1. Query-analysis contracts and bounds

- [x] 1.1 Add failing `internal/retrieval` tests for immutable original-query retention, versioned analysis identity, explicit absent/unknown dispositions, and deterministic replay; verify the focused tests fail for the missing contracts before implementation and pass afterward.
- [x] 1.2 Define provider-independent query-analysis input, result, hint, signal, disposition, and versioned-limit contracts with validation that rejects unknown versions and unsafe values; verify contract and validation tests pass without database or network access.
- [x] 1.3 Define stable fallback and diagnostic category enums plus bounded counters/durations, including unavailable, malformed, adversarial, duplicate, and over-budget cases; verify table-driven tests reject unknown/unbounded diagnostics and never serialize query or subquery text.
- [x] 1.4 Review mature Go packages relevant to deterministic tokenization, language normalization, and date parsing on `pkg.go.dev`, record the dependency decision in the implementation notes, and verify any adopted dependency has a maintained stable API and passes repository license/dependency checks; prefer the standard library when no package materially reduces risk.

## 2. Deterministic normalization, hints, and decomposition

- [x] 2.1 Add failing table-driven tests for bounded whitespace/Unicode/case normalization and mixed-language aliases or terms, including empty, duplicate, oversized, malformed UTF-8, and adversarial input; implement deterministic normalization and verify stable ordering and configured length/count truncation.
- [x] 2.2 Add failing tests for entity, temporal, memory-class, and intent hint extraction, including ambiguous and unknown cases plus conflicts with explicit request filters; implement non-authoritative bounded hints and verify conflicts are discarded without invented identifiers or widened filters.
- [x] 2.3 Add failing tests for recognizable multi-hop decomposition, duplicate subqueries, stable ordering, maximum subquery/signal counts, and work-budget exhaustion; implement deterministic rule-based decomposition and verify it performs no database, network, or online-model calls.
- [x] 2.4 Add fuzz/property coverage for arbitrary query bytes and limits, asserting termination, no panic, immutable original input, deterministic output, and all configured bounds; verify the focused fuzz seed corpus and ordinary package tests pass.

## 3. Exact-scope rollout governance

- [x] 3.1 Add failing domain tests for diagnostics-only, shadow, active, disabled, rollback, missing, expired, malformed, and foreign-scope query-analysis policies; extend existing ranking-rollout policy validation/resolution and verify every non-approved disposition resolves to original-only behavior.
- [x] 3.2 Inspect the existing rollout persistence representation and either prove with repository round-trip tests that a typed versioned query-analysis payload fits safely or add the smallest forward migration needed; verify unknown fields/versions fail closed and migration manifest/upgrade tests pass.
- [x] 3.3 Add PostgreSQL repository tests for exact tenant/project/namespace and optional session/user isolation of query-analysis rollout state, including same-name foreign policies; implement persistence changes if required and verify no broader-scope fallback is selected.
- [x] 3.4 Add configuration/startup validation for default-disabled query analysis and bounded policy values, without adding a parallel global activation mechanism; verify invalid or absent configuration preserves the existing original-query baseline.

## 4. Bounded retrieval orchestration

- [x] 4.1 Add failing service tests proving the original query is always the first mandatory signal and analysis failure, unavailability, malformed output, rejection, or timeout continues through the existing original-query path; implement analyzer injection and fail-closed orchestration until the focused tests pass.
- [x] 4.2 Add failing tests proving every derived signal reuses the exact resolved scope, lifecycle visibility, memory-class, and explicit time-window constraints; implement signal validation/fan-out and verify foreign, hidden, expired, suppressed, forgotten, deleted, or time-conflicting evidence cannot enter candidates.
- [x] 4.3 Add failing tests for per-signal recall limits, total signal/subquery limits, aggregate candidate limits, elapsed-budget cancellation, and deterministic truncation; implement global bounded fan-out and verify excess derived work is discarded while the original signal remains eligible.
- [x] 4.4 Add failing fusion tests where original and derived signals overlap across lexical, semantic, relation, and chunk channels; route all candidates through existing stable rank fusion, identity/lineage deduplication, diversity, citation, and result budgets and verify one stable canonical identity without cross-signal raw-score addition.
- [x] 4.5 Add fault-injection tests for one or all optional signal/channel failures and for an independent original retrieval failure; verify optional failures preserve original-only results while original-path failures preserve existing public error semantics.

## 5. Rollout effects, diagnostics, and API compatibility

- [x] 5.1 Add diagnostics-only and shadow integration tests proving bounded analysis/comparison may run but ordinary ranking remains original-only; implement rollout-stage effects and verify active is the only approved stage in which derived signals can affect the common pipeline.
- [x] 5.2 Add active, disabled, and rollback integration tests for one exact scope alongside adjacent foreign scopes; implement reversible resolution and verify rollback restores original-only behavior without canonical-memory rewrites.
- [x] 5.3 Add authorized diagnostic tests for policy/version identity, original-retained and normalization status, hint/subquery/signal counts, time status, fallback category, rollout disposition, candidate counts, and elapsed budget; implement allowlisted aggregate diagnostics and verify raw query plans, normalized text, subqueries, candidates, scores, hidden/foreign IDs, and scope values are absent.
- [x] 5.4 Add ordinary search/context and OpenAPI compatibility tests around all rollout stages; verify existing public request/response schemas and result identity/citation shapes remain unchanged and no analysis or shadow internals are exposed.

## 6. Evaluation fixtures, reports, and release gates

- [ ] 6.1 Version the repository fixture schema and add protected simple-fact, temporal, entity-centric, mixed-language, ambiguous, multi-hop, malformed, adversarial, and analyzer-unavailable cases; verify fixture validation rejects unsafe scopes, duplicate aliases, missing expectations, invalid analysis categories, and unbounded inputs before database access.
- [ ] 6.2 Extend deterministic replay and report compatibility with analysis/limit version, rollout disposition, original-retained status, bounded fallback/signal/subquery/candidate counts, temporal and multi-hop coverage, protected recall, duplicate rate, and latency; verify repeated compatible replay is byte-stable where promised and reports contain no raw queries, plans, subqueries, credentials, DSNs, or unsafe evidence.
- [ ] 6.3 Extend candidate comparison to use the immutable original-query baseline and reject incompatible fixture, representation, fusion, ranking, analysis, or release-policy versions; verify per-category quality regressions and any isolation/lifecycle failure override aggregate gains.
- [ ] 6.4 Encode the Phase 6.4 real-stack duplicate-rate, protected-coverage, candidate-budget, and latency evidence as a prerequisite to query-analysis eligibility; verify absent, skipped, incompatible, stale, or failing prerequisite evidence makes active decomposition ineligible.
- [ ] 6.5 Update the evaluation entrypoint so only an explicitly supplied owned evaluation DSN can run PostgreSQL+pgvector gates; verify absence yields `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED`, never falls back to `STELE_POSTGRES_DSN`, and cannot authorize active rollout.
- [ ] 6.6 When an explicitly owned evaluation DSN is available, run the compatible Phase 6.4 prerequisite and original-versus-analyzed Phase 6.5 evaluations and retain redacted reports; otherwise verify the stable non-pass skip and document that active rollout remains prohibited without claiming real-stack passage.

## 7. Documentation and final verification

- [x] 7.1 Update retrieval quality and operator documentation with deterministic policy/version semantics, bounds, exact-scope rollout stages, redacted diagnostics, original-only fallback/rollback, and the owned-DSN activation gate; verify documentation consistency checks and examples match executable configuration.
- [ ] 7.2 Update the Phase 6 roadmap status only to the implementation state actually evidenced, keeping Task 6.6 separate; verify roadmap/OpenSpec consistency checks pass and do not mark active decomposition complete when real-stack gates are skipped.
- [x] 7.3 Run focused analyzer, rollout, retrieval, evaluation, PostgreSQL repository, migration, and OpenAPI tests with uncached execution; verify all targeted packages pass and no test depends on ambient runtime database credentials.
- [ ] 7.4 Run `go test ./... -count=1 -timeout 15m` and the repository quality gate; verify both exit successfully with zero failures.
- [ ] 7.5 Run `go test -race ./... -count=1 -timeout 20m`; verify it passes, or if the host lacks the required C toolchain, record that environmental blocker verbatim and run the same command in a supported CI/toolchain before completion is claimed.
- [x] 7.6 Run `openspec validate bounded-query-understanding-and-multi-signal-retrieval --strict`, `openspec validate --all`, `git diff --check`, and a template-marker scan; verify zero validation failures, whitespace errors, or unfinished template markers in this change.
