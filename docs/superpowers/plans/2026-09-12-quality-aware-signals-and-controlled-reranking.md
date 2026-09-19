# Quality-Aware Signals And Controlled Reranking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task with verification checkpoints.

**Goal:** Add a disabled-by-default, scope-governed quality-aware reranking stage with an optional OpenAI-compatible provider while preserving the RRF baseline and secret-safe self-hosting.

**Architecture:** Retrieval keeps validated recall and stable fusion as the immutable baseline. A new quality layer computes bounded feature vectors and optional provider scores, then applies clamped adjustments before existing diversity/context packing. Runtime configuration builds a provider resolver consistently for all modes; rollout policy stores only logical identities and uses existing dry-run/activation/rollback gates.

**Tech Stack:** Go, existing internal/retrieval and internal/memory contracts, environment-backed config, net/http, PostgreSQL additive migration/repository patterns, OpenSpec and existing telemetry/evaluation packages.

---

### Task 1: Quality domain and deterministic scorer

**Files:** Create `internal/retrieval/quality_rerank.go`, `internal/retrieval/quality_rerank_test.go`; modify `internal/retrieval/service.go` only after scorer tests pass.

- [ ] Define `QualityFeatureVector` with version, normalized evidence/freshness/reliability/conflict/usefulness/task/verification fields, presence bits, and bounded validation.
- [ ] Implement deterministic `ComputeQualityAdjustment` with per-feature and total caps; neutral missing values and conservative safety handling.
- [ ] Add table-driven tests for complete, missing, superseded, conflicting, NaN/Inf, and cap cases.

### Task 2: Provider-independent reranker and HTTP adapter

**Files:** Create `internal/retrieval/reranker.go`, `internal/retrieval/reranker_test.go`, `internal/retrieval/openai_reranker.go`, `internal/retrieval/openai_reranker_test.go`.

- [ ] Define a `Reranker` interface that receives query plus validated candidate texts/IDs and returns finite scores keyed to known IDs.
- [ ] Add a deterministic fake provider for unit tests and strict response validation (unknown/duplicate IDs, bounds, malformed JSON).
- [ ] Implement bounded `net/http` adapter with configurable endpoint/path/model/timeout and redacted error categories; never log headers or response bodies.

### Task 3: Runtime configuration and registration

**Files:** Modify `internal/config/config.go` and config tests; add provider wiring in the existing runtime construction package; modify `.gitignore` and docs examples only with placeholders.

- [ ] Add disabled-default reranker config fields and environment loading/validation.
- [ ] Register one logical provider resolver shared by api/worker/scheduler; reject incomplete active configuration.
- [ ] Verify local secret files and provider endpoints are not tracked; add placeholder-only examples.

### Task 4: Rollout policy integration

**Files:** Modify `internal/memory/ranking_rollout.go`, related repository/migration files and tests.

- [ ] Add logical quality policy/reranker identity and mode fields with zero-value backward compatibility.
- [ ] Persist fields through additive migration using existing scoped repository/audit methods; no credentials or endpoints.
- [ ] Extend validation and activation matching to require bounded versions and existing evidence gates.

### Task 5: Retrieval pipeline integration

**Files:** Modify `internal/retrieval/service.go` and retrieval tests.

- [ ] Snapshot fused baseline before any new adjustment.
- [ ] Extract visible candidate quality features from existing summaries/findings; apply deterministic bounded adjustments.
- [ ] Invoke provider only in diagnostics/shadow/approved exact-scope active mode; validate outputs against visible candidate IDs.
- [ ] Fall back to baseline on disabled, timeout, malformed, or scope-invalid provider behavior; keep ordinary result shape unchanged.

### Task 6: Evaluation, telemetry, and documentation

**Files:** Modify evaluation/report and telemetry files plus `docs/retrieval-quality-baseline.md`, `docs/self-hosting.md`.

- [ ] Record bounded feature/reranker identities, changed-rank counts, fallback categories, and protected metric deltas with redaction tests.
- [ ] Add low-cardinality rerank counters without query/scope/ID/raw score labels.
- [ ] Document environment variables, Docker secret/local `.env` guidance, shadow-first rollout, activation gates, rollback, and real-stack prerequisites.

### Task 7: Verification and task bookkeeping

- [ ] Run focused retrieval/config/provider/rollout/evaluation tests.
- [ ] Run `go test ./...`, race tests where practical, `openspec validate quality-aware-signals-and-controlled-reranking --strict`, and `git diff --check`.
- [ ] Mark corresponding OpenSpec task checkboxes as complete only after evidence is captured; leave unrelated user changes untouched.
