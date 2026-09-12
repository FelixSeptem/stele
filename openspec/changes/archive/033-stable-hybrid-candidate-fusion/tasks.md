## 1. Fusion Domain Contracts

- [x] 1.1 Define versioned fusion strategy, channel, parameter, candidate-rank, and bounded diagnostic types in `internal/retrieval` (or the existing ranking domain package), including RRF and normalized-weighted strategy identifiers.
- [x] 1.2 Implement validation for strategy versions, rank constants, channel weights, per-channel limits, total candidate bounds, and deterministic tie-break configuration; add unit tests for valid and invalid definitions.
- [x] 1.3 Implement deterministic RRF scoring and canonical-candidate aggregation from ranked channel lists, with tests for single-channel, multi-channel, missing-channel, duplicate-parent, and tie-break cases.

## 2. Retrieval Service Integration

- [x] 2.1 Refactor lexical, semantic, relation, and authorized chunk collection into bounded channel candidate lists that retain one-based channel ranks and pass existing scope, class, lifecycle, lineage, and citation validation before fusion.
- [x] 2.2 Integrate the selected fusion strategy into search and context retrieval while preserving canonical public hits, parent/source citations, existing top-k and character/token budgets, and deterministic final ordering.
- [x] 2.3 Implement safe optional-channel degradation and fail-closed handling for malformed strategies or invalid candidates; add regression tests proving hidden, foreign, and unbounded candidates never influence fused results.
- [x] 2.4 Implement the explicit normalized-weighted experimental strategy behind the same interface, with bounded score normalization and tests proving it is never selected implicitly as the default.

## 3. Scoped Rollout And Persistence

- [x] 3.1 Extend the existing scoped ranking rollout contract with an explicit fusion strategy name/version and bounded parameters, preserving activation gates, audit reasons, diagnostics-only, dry-run, active, disabled, and rolled-back states.
- [x] 3.2 Add or update PostgreSQL migration, repository queries, validation, and compatibility tests for persisted fusion strategy selections without rewriting canonical memory, chunks, or provenance records.
- [x] 3.3 Wire search/context resolution to the effective scope policy and prove exact tenant/project/namespace matching, default baseline fallback, and rollback restoration with repository/service tests.
- [x] 3.4 Update admin/OpenAPI rollout inspection and mutation contracts only where required, keeping strategy internals, raw scores, candidate pools, and hidden identifiers out of ordinary public responses.

## 4. Evaluation And Diagnostics

- [x] 4.1 Extend retrieval evaluation replay inputs and reports with the effective fusion strategy identity, bounded channel ranks, optional-channel status, and final disposition categories.
- [x] 4.2 Add deterministic fixture cases and comparison tests covering lexical/semantic overlap, relation and chunk channels, unavailable optional channels, candidate-pool bounds, and stable tie ordering.
- [x] 4.3 Enforce redaction and hard safety gates for fusion diagnostics, proving hidden or foreign candidates cannot expose IDs, content, raw scores, or scope values and cannot affect quality metrics.
- [x] 4.4 Add low-cardinality telemetry and regression assertions for fusion strategy, channel availability, candidate counts, and fallback/rollback outcomes without high-cardinality scope or query labels.

## 5. Documentation And Release Readiness

- [x] 5.1 Document the fusion contract, RRF formula, default parameters, channel bounds, deterministic ordering, rollout lifecycle, and rollback procedure in retrieval-quality and self-hosting documentation.
- [x] 5.2 Add an operator/evaluator runbook for comparing RRF with normalized-weighted fusion through diagnostics-only or dry-run modes and activating a strategy for one exact scope.
- [x] 5.3 Run the full Go test suite, OpenSpec strict validation, retrieval evaluation checks, and PostgreSQL integration tests with the explicit DSN behavior documented; record the evidence needed for the next diversity-packing proposal.
