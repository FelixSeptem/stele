# Evidence Deduplication And Diversity-Aware Context Packing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add deterministic identity/lineage deduplication and opt-in, scope-safe diversity selection between stable fusion and bounded context packing, with governed rollout, redacted diagnostics, and evaluation gates.

**Architecture:** A pure selector in `internal/retrieval` accepts only lifecycle- and scope-validated fused candidates, performs stable equivalence grouping, and optionally runs bounded deterministic MMR. Existing ranking rollout policy storage carries the versioned diversity configuration and exact-scope activation state. Search and context assembly consume the same selector while preserving existing public response shapes, section boundaries, citation rules, and fallback behavior.

**Tech Stack:** Go, PostgreSQL, pgx v5, pgvector-compatible stored embeddings, OpenSpec, table-driven unit tests, pgxmock and repository integration tests.

---

### Task 1: Pure policy contracts and identity/lineage deduplication

**Files:**
- Create: `internal/retrieval/diversity.go`
- Create: `internal/retrieval/diversity_test.go`
- Reference: `internal/retrieval/fusion.go`
- Reference: `internal/memory/types.go`

- [ ] Write table-driven tests for policy validation bounds, dispositions, canonical/source-event/parent-lineage equivalence, stable representative choice, citation merging, and invalid-candidate exclusion.
- [ ] Run `go test ./internal/retrieval -run 'TestDiversityPolicy|TestDeduplicateEvidence' -count=1` and confirm failures are caused by missing contracts.
- [ ] Add versioned policy, bounded parameters, candidate coverage/embedding metadata, redacted disposition summaries, and deterministic union-based identity/lineage deduplication.
- [ ] Re-run the focused tests and `go test ./internal/retrieval -count=1`.

### Task 2: Bounded semantic clustering and deterministic MMR

**Files:**
- Modify: `internal/retrieval/diversity.go`
- Modify: `internal/retrieval/diversity_test.go`

- [ ] Add failing tests for compatible active embedding revisions, threshold boundaries, unavailable semantic input, class/session/entity/time coverage, stable tie breaks, maximum candidate and pairwise comparison bounds, and deterministic replay.
- [ ] Run the focused tests and confirm the expected behavioral failures.
- [ ] Implement cosine similarity over supplied bounded vectors and deterministic MMR using fused-order relevance, explicit coverage bonuses, and identity-only degradation.
- [ ] Re-run focused and package tests, including repeated runs with `-count=20` for determinism.

### Task 3: Reuse ranking rollout governance and persist diversity parameters

**Files:**
- Modify: `internal/memory/ranking_rollout.go`
- Modify: `internal/memory/ranking_rollout_test.go`
- Modify: `internal/storage/postgres/ranking_rollout_repository.go`
- Modify: `internal/storage/postgres/ranking_rollout_repository_test.go`
- Create: `internal/storage/postgres/migrations/0006_diversity_policy.up.sql`
- Create: `internal/storage/postgres/migrations/0006_diversity_policy.down.sql`
- Modify: `internal/storage/postgres/migrations.go`
- Modify: migration manifest/upgrade tests that enumerate owned migrations

- [ ] Add failing domain tests for complete-or-absent diversity configuration, bounded values, shadow/active/disabled behavior, and activation evidence.
- [ ] Extend `RankingRolloutPolicy` with diversity name/version, lambda, semantic threshold, candidate/comparison bounds, and class/session/entity/time weights; validate configuration atomically.
- [ ] Add failing repository tests for insert/read/list/active resolution, idempotency behavior, exact scope, malformed values, disable, and rollback.
- [ ] Add nullable forward-only columns to the existing rollout table and update all repository scans/writes without creating a parallel rollout system.
- [ ] Run memory, repository, migration manifest, upgrade, and PostgreSQL integration tests available in the environment.

### Task 4: Integrate post-fusion deduplication and section-aware context selection

**Files:**
- Modify: `internal/retrieval/service.go`
- Modify: `internal/retrieval/fusion_service_test.go`
- Modify: `internal/retrieval/service_test.go`
- Modify: `internal/retrieval/projection_assembly_test.go`

- [ ] Add failing service tests proving search always uses identity/lineage deduplication after fusion, active exact-scope policies apply diversity, disabled/foreign/malformed policies retain the deduplicated baseline, and optional semantic data failure degrades safely.
- [ ] Wire candidate metadata enrichment and selector invocation after `FuseCandidates`, retaining canonical fallback and bounded citations.
- [ ] Add failing context tests proving per-section selection occurs after eligibility/summary preference and before budget packing, unknown coverage values stay local, citations remain required, and ordinary responses disclose no policy internals.
- [ ] Apply diversity independently to existing sections and emit policy/disposition summaries only when authorized diagnostics are requested.
- [ ] Run retrieval package and HTTP regression tests.

### Task 5: Extend evaluation metrics, fixtures, and release gates

**Files:**
- Modify: `internal/retrieval/testdata/retrieval-evaluation-fixture-v1.json`
- Modify: `internal/retrieval/evaluation.go`
- Modify: `internal/retrieval/evaluation_metrics.go`
- Modify: `internal/retrieval/evaluation_replay.go`
- Modify: `internal/retrieval/evaluation_report.go`
- Modify: `internal/retrieval/evaluation_comparison.go`
- Modify: corresponding `*_test.go` files
- Modify: `internal/retrieval/evaluation_policy.go`
- Modify: `internal/storage/postgres/evaluation_fixture.go`
- Modify: `internal/storage/postgres/evaluation_fixture_test.go`

- [ ] Add failing fixture/report tests for repeated versions, shared source/parent lineage, near-identical evidence, independent multi-hop evidence, section budgets, and hidden/foreign distractors.
- [ ] Report policy identity and aggregate duplicate/diversity/budget dispositions plus protected recall, coverage, duplicate rate, candidate-pool size, and latency deltas without raw payloads.
- [ ] Add failing release-policy tests for hard isolation/lifecycle failure and protected recall, coverage, budget, and latency regressions.
- [ ] Enforce all hard gates and the explicit owned PostgreSQL+pgvector DSN requirement, retaining the documented stable non-pass skip outcome when absent.
- [ ] Run focused evaluation and PostgreSQL fixture tests.

### Task 6: Documentation, OpenSpec tracking, and final verification

**Files:**
- Modify: `docs/retrieval-quality-baseline.md`
- Modify: `docs/self-hosting.md`
- Modify: `docs/self_hosting_test.go`
- Modify: `openspec/changes/evidence-deduplication-and-diversity-aware-context-packing/tasks.md`

- [ ] Document default identity-deduplicated behavior, exact-scope diversity lifecycle, authorized diagnostics, release evidence, rollback, and real-stack prerequisites.
- [ ] Mark each OpenSpec checkbox immediately after its implementation and verification evidence passes.
- [ ] Run focused retrieval, context, rollout, evaluation, migration, and PostgreSQL tests.
- [ ] Run `go test ./... -count=1 -timeout 15m` and `go test -race ./... -timeout 20m`.
- [ ] Run `openspec validate evidence-deduplication-and-diversity-aware-context-packing --strict` and `git diff --check`.
- [ ] Record any controlled real-stack skip separately from passing evidence and leave unrelated pre-existing worktree changes untouched.
