# Durable Multi-Scope Maintenance and Retrieval Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the P6 operational gap with durable exact-scope maintenance execution, projection freshness/SLO evidence, redacted observability, bounded retention, and repeatable assurance conformance.

**Architecture:** Reuse PostgreSQL-backed `job_executions`, workflow/run, projection, and assurance records. Add compare-and-set lease/idempotency state, derived freshness evidence, typed low-cardinality telemetry, and a scope-bounded conformance analyzer without adding a second ledger or public API.

**Tech Stack:** Go, PostgreSQL 18 + pgvector, pgx/v5, existing Stele jobs/scheduler, telemetry, assurance, retrieval, migration, and OpenSpec tooling.

---

## Task 1: Map and lock the existing contracts

**Files:**
- Inspect: `internal/jobs/jobs.go`, `internal/jobs/jobs_test.go`
- Inspect: `internal/storage/postgres/repository.go`, `internal/storage/postgres/repository_test.go`
- Inspect: `internal/memory/context_projection*.go`
- Inspect: `internal/telemetry/*.go`, `internal/assurance/*.go`

- [ ] **Step 1: Record the current execution/projection/assurance interfaces**

  Identify `JobExecution`, `JobExecutionRecord`, `BeginJobExecution`, lease-bearing scope workers, context projection watermark/status fields, assurance conformance evidence, and observer interfaces. Do not change behavior in this task.

- [ ] **Step 2: Run the focused baseline tests**

  Run: `$env:GOCACHE='D:\code\stele\.gocache-p6-baseline'; go test ./internal/jobs ./internal/storage/postgres ./internal/memory ./internal/retrieval ./internal/telemetry ./internal/assurance -count=1`

  Expected: PASS on the clean proposal baseline; any pre-existing environment limitation is recorded before implementation.

## Task 2: Define stable maintenance identity and validation (TDD)

**Files:**
- Modify: `internal/jobs/jobs.go`
- Test: `internal/jobs/jobs_test.go`

- [ ] **Step 1: Write failing identity tests**

  Add tests proving identical job class, exact scope, and cadence window produce the same identity; different namespace/window produces a different identity; empty or oversized scope/key data fails validation; and duplicate terminal execution maps to a bounded disposition.

- [ ] **Step 2: Run the tests and confirm RED**

  Run: `go test ./internal/jobs -run 'Test.*Maintenance.*Identity|Test.*Duplicate.*Disposition' -count=1`

  Expected: FAIL because the stable identity and validation API does not yet exist.

- [ ] **Step 3: Implement the minimal typed identity contract**

  Add a validated maintenance identity/input with fixed job class, exact `memory.Scope`, cadence/idempotency window, stable hash/key, and bounded duplicate disposition. Keep raw scope values out of telemetry labels.

- [ ] **Step 4: Run focused tests and commit**

  Run: `gofmt -w internal/jobs/jobs.go internal/jobs/jobs_test.go; go test ./internal/jobs -run 'Test.*Maintenance.*Identity|Test.*Duplicate.*Disposition' -count=1`

  Expected: PASS. Commit: `git commit -am "feat: define durable maintenance identity"`.

## Task 3: Add durable execution state and migration

**Files:**
- Modify: `internal/jobs/jobs.go`
- Create/modify: `internal/storage/postgres/migrations/0009_durable_maintenance.up.sql`
- Create/modify: `internal/storage/postgres/migrations/0009_durable_maintenance.down.sql`
- Modify: `internal/storage/postgres/migrations.go`
- Test: `internal/storage/postgres/migration_manifest_test.go`, `internal/storage/postgres/migration_apply_test.go`

- [ ] **Step 1: Add failing migration/validation tests**

  Assert the new migration is ordered, repeatable, and adds only nullable/defaulted execution fields and indexes for stable identity, attempt, retry, checkpoint/watermark, and terminal disposition.

- [ ] **Step 2: Run migration tests and confirm RED**

  Run: `go test ./internal/storage/postgres -run 'Test.*Migration|Test.*Manifest' -count=1`

  Expected: FAIL until migration metadata and SQL are present.

- [ ] **Step 3: Implement idempotent forward/down SQL**

  Use `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, fixed enum/check constraints where the repository supports them, and exact-scope/idempotency indexes. Down SQL must remove only this migration's objects.

- [ ] **Step 4: Verify migration behavior**

  Run: `go test ./internal/storage/postgres -run 'Test.*Migration|Test.*Manifest' -count=1`

  Expected: PASS, including repeated application and manifest checks. Commit: `git commit -am "feat: persist durable maintenance execution state"`.

## Task 4: Implement compare-and-set repository transitions

**Files:**
- Modify: `internal/storage/postgres/repository.go`
- Test: `internal/storage/postgres/repository_test.go`

- [ ] **Step 1: Write failing SQL contract tests**

  Add pgxmock tests for acquire, renew, complete, fail/retry, stale reclaim, duplicate suppression, and cursor pagination. Every query must assert tenant/project/namespace predicates and owner/state/expiry compare-and-set conditions.

- [ ] **Step 2: Run repository tests and confirm RED**

  Run: `go test ./internal/storage/postgres -run 'Test.*JobExecution.*(Lease|Retry|Duplicate|Page)' -count=1`

  Expected: FAIL for missing methods/SQL.

- [ ] **Step 3: Implement repository methods**

  Add the smallest interfaces needed by jobs: acquire/renew/complete/fail/reclaim, retry scheduling, and bounded history listing with a stable `(started_at,id)` cursor. Return explicit duplicate, lease-conflict, stale, and exhausted categories.

- [ ] **Step 4: Verify SQL and concurrency semantics**

  Run: `gofmt -w internal/storage/postgres/repository.go internal/storage/postgres/repository_test.go; go test ./internal/storage/postgres -run 'Test.*JobExecution.*(Lease|Retry|Duplicate|Page)' -count=1`

  Expected: PASS. Commit: `git commit -am "feat: add maintenance lease and execution transitions"`.

## Task 5: Wire restart-safe scheduler/worker maintenance

**Files:**
- Modify: `internal/jobs/jobs.go`
- Modify: `internal/app/app.go`
- Modify: `internal/config/config.go`
- Test: `internal/jobs/jobs_test.go`, `internal/app/app_test.go`, `internal/config/config_test.go`

- [ ] **Step 1: Write failing orchestration tests**

  Cover lease renewal, stale reclaim after expiry, bounded retry/backoff, checkpoint resume, duplicate scheduler fire, disabled fallback, and exact-scope dispatch.

- [ ] **Step 2: Run orchestration tests and confirm RED**

  Run: `go test ./internal/jobs ./internal/app ./internal/config -run 'Test.*(Maintenance|Scheduler|Lease|Retry|Resume)' -count=1`

  Expected: FAIL for the new durable path and configuration fields.

- [ ] **Step 3: Implement wiring with default-safe configuration**

  Add bounded env-backed settings for maintenance lease duration/renewal, retry attempts/backoff, freshness/SLO windows, retention, and conformance cadence. Keep the new path disabled by default and preserve the existing scheduler when disabled or when conformance is not ready.

- [ ] **Step 4: Verify restart and fallback behavior**

  Run: `gofmt -w internal/jobs/jobs.go internal/app/app.go internal/config/config.go; go test ./internal/jobs ./internal/app ./internal/config -run 'Test.*(Maintenance|Scheduler|Lease|Retry|Resume)' -count=1`

  Expected: PASS. Commit: `git commit -am "feat: wire restart-safe maintenance scheduling"`.

## Task 6: Implement projection freshness and SLO evidence

**Files:**
- Modify: `internal/memory/context_projection.go` and related projection types
- Modify: `internal/storage/postgres/context_projection_repository.go`
- Modify: `internal/retrieval/progressive_context.go`
- Test: `internal/memory/*projection*_test.go`, `internal/storage/postgres/context_projection_repository_test.go`, `internal/retrieval/progressive_context_test.go`

- [ ] **Step 1: Write failing freshness/fail-closed tests**

  Cover matching/missing/stale/divergent watermarks, policy/renderer mismatch, hidden lifecycle evidence, foreign scope, duration/age buckets, and default retrieval exclusion.

- [ ] **Step 2: Run projection tests and confirm RED**

  Run: `go test ./internal/memory ./internal/storage/postgres ./internal/retrieval -run 'Test.*(Fresh|Watermark|Projection|Stale|Foreign|Hidden)' -count=1`

  Expected: FAIL for missing maintenance evidence/eligibility behavior.

- [ ] **Step 3: Implement derived freshness evidence and eligibility**

  Add validated freshness categories, rebuild/checkpoint state, SLO buckets, and exact-scope persistence/filtering. Keep canonical source records untouched and ensure stale/divergent projections cannot enter ordinary retrieval.

- [ ] **Step 4: Verify projection behavior**

  Run: `gofmt -w internal/memory internal/storage/postgres/context_projection_repository.go internal/retrieval/progressive_context.go; go test ./internal/memory ./internal/storage/postgres ./internal/retrieval -run 'Test.*(Fresh|Watermark|Projection|Stale|Foreign|Hidden)' -count=1`

  Expected: PASS. Commit: `git commit -am "feat: gate projections on maintenance freshness"`.

## Task 7: Add bounded internal observability

**Files:**
- Modify: `internal/telemetry/metrics.go`, `internal/telemetry/telemetry.go`
- Modify: `internal/jobs/jobs.go`, `internal/retrieval/*.go`
- Test: `internal/telemetry/*_test.go`

- [ ] **Step 1: Write failing redaction/cardinality tests**

  Assert allowed fixed categories/buckets and rejection/redaction of query text, scope values, IDs, raw scores, provider payloads, credentials, and unbounded error/plan data.

- [ ] **Step 2: Run telemetry tests and confirm RED**

  Run: `go test ./internal/telemetry -run 'Test.*(Maintenance|Redact|Cardinality|Telemetry)' -count=1`

  Expected: FAIL for missing typed constructors and labels.

- [ ] **Step 3: Implement typed events and emission points**

  Add fixed enums/bucket functions and maintenance/retrieval event methods. Ensure worker, scheduler, projection, and retention paths emit only bounded values; do not add public response fields.

- [ ] **Step 4: Verify telemetry contract**

  Run: `gofmt -w internal/telemetry internal/jobs/jobs.go internal/retrieval; go test ./internal/telemetry -run 'Test.*(Maintenance|Redact|Cardinality|Telemetry)' -count=1`

  Expected: PASS. Commit: `git commit -am "feat: add bounded maintenance observability"`.

## Task 8: Add derived-artifact retention and cleanup

**Files:**
- Modify: `internal/jobs/jobs.go`
- Modify: `internal/storage/postgres/repository.go`
- Modify: `internal/config/config.go`
- Test: `internal/jobs/jobs_test.go`, `internal/storage/postgres/repository_test.go`, `internal/config/config_test.go`

- [ ] **Step 1: Write failing retention tests**

  Prove expired execution diagnostics, freshness evidence, conformance evidence, and redacted trajectories are removable; canonical source and incident audit records are never targets; repeated cleanup is idempotent.

- [ ] **Step 2: Run retention tests and confirm RED**

  Run: `go test ./internal/jobs ./internal/storage/postgres ./internal/config -run 'Test.*(Retention|Cleanup|Canonical|Incident)' -count=1`

  Expected: FAIL until allowlisted derived cleanup exists.

- [ ] **Step 3: Implement allowlisted cleanup**

  Add explicit retention classes/windows, bounded deletion counts/categories, cursor/limit protection, and cleanup wiring through existing scheduler execution. Never issue a delete against canonical source tables.

- [ ] **Step 4: Verify retention safety**

  Run: `go test ./internal/jobs ./internal/storage/postgres ./internal/config -run 'Test.*(Retention|Cleanup|Canonical|Incident)' -count=1`

  Expected: PASS. Commit: `git commit -am "feat: retain and clean derived maintenance evidence"`.

## Task 9: Implement maintenance conformance analyzer

**Files:**
- Modify: `internal/assurance/types.go`, `internal/assurance/service.go`
- Modify: `internal/storage/postgres/assurance_repository.go`
- Test: `internal/assurance/service_test.go`, `internal/assurance/types_test.go`, `internal/storage/postgres/assurance_repository_test.go`

- [ ] **Step 1: Write failing conformance tests**

  Cover durable-scope coverage, lease recovery, projection freshness/rebuild, retention safety, telemetry redaction, evidence completeness, hard failures, and action-success versus integrity separation.

- [ ] **Step 2: Run assurance tests and confirm RED**

  Run: `go test ./internal/assurance ./internal/storage/postgres -run 'Test.*(Maintenance|Conformance|Readiness|Evidence)' -count=1`

  Expected: FAIL for missing maintenance evidence kinds/analyzer.

- [ ] **Step 3: Implement bounded analyzer and persistence**

  Add fixed evidence kinds/categories, scope-bounded analyzer inputs, missing/stale/unsafe diagnostics, readiness impact, and repository serialization using existing assurance records. Keep evidence diagnostic and redact scope values/IDs from telemetry.

- [ ] **Step 4: Verify conformance semantics**

  Run: `gofmt -w internal/assurance internal/storage/postgres/assurance_repository.go; go test ./internal/assurance ./internal/storage/postgres -run 'Test.*(Maintenance|Conformance|Readiness|Evidence)' -count=1`

  Expected: PASS. Commit: `git commit -am "feat: close maintenance assurance conformance"`.

## Task 10: Schedule conformance and update documentation

**Files:**
- Modify: `internal/app/app.go`, `internal/config/config.go`
- Modify: `docs/self-hosting.md`, `docs/retrieval-release-gate.md`
- Modify: `.env.local.example`
- Create/modify: `scripts/check-docs-consistency.ps1` only if required by existing checks
- Test: `internal/app/app_test.go`, `internal/config/config_test.go`, docs checks

- [ ] **Step 1: Write failing scheduler/config/docs tests**

  Assert conformance cadence and retention placeholders parse, the job is disabled by default, failed gates preserve fallback behavior, and docs list the exact operator evidence and rollback commands.

- [ ] **Step 2: Implement scheduler and operator documentation**

  Add conformance dispatch through existing durable scheduler semantics, configuration placeholders without secrets, PostgreSQL 18 + pgvector smoke prerequisites, stale recovery, rollback, and canonical-data safety guidance.

- [ ] **Step 3: Verify integration and docs**

  Run: `go test ./internal/app ./internal/config -run 'Test.*(Conformance|Maintenance|Fallback)' -count=1; pwsh -File scripts/check-docs-consistency.ps1`

  Expected: PASS. Commit: `git commit -am "docs: document maintenance conformance operations"`.

## Task 11: Add deterministic CI and release verification

**Files:**
- Modify: `.github/workflows/product-verification.yml`
- Modify: `scripts/check-quality-gate.ps1` only if an absent check is referenced
- Test: repository verification commands

- [ ] **Step 1: Add deterministic focused coverage**

  Include jobs/storage/projection/telemetry/assurance/redaction tests that do not require provider credentials or ambient production DSNs.

- [ ] **Step 2: Verify CI and local gates**

  Run: `$env:GOCACHE='D:\code\stele\.gocache-p6-full'; go test ./... -count=1 -timeout 15m; openspec validate --all; git diff --check`

  Expected: all Go packages pass, OpenSpec reports zero failures, and diff check is clean.

- [ ] **Step 3: Run optional real-stack smoke**

  When an explicitly owned PostgreSQL 18 + pgvector evaluation DSN is available, run the documented maintenance/conformance smoke and verify redacted evidence, no canonical mutation, and bounded cleanup. If unavailable, record the stable prerequisite skip category without claiming release readiness.

## Task 12: Review, mark complete, and hand off

**Files:**
- Modify: `openspec/changes/durable-multiscope-maintenance-observability/tasks.md`
- Modify: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`

- [ ] **Step 1: Re-read proposal, specs, and design against implementation**

  Verify every requirement has tests/evidence, no public API or canonical-store boundary changed, and any environment-only limitations are documented.

- [ ] **Step 2: Mark only verified tasks complete**

  Change `- [ ]` to `- [x]` only after the corresponding command or evidence has passed; leave unavailable PostgreSQL/race prerequisites explicitly unchecked or annotated.

- [ ] **Step 3: Update roadmap status**

  Record the OpenSpec active/archived state and distinguish implementation evidence from optional real-stack release evidence.

- [ ] **Step 4: Validate the change for archive**

  Run: `openspec validate durable-multiscope-maintenance-observability --strict; openspec status --change durable-multiscope-maintenance-observability --json`

  Expected: strict validation passes and task status accurately reflects implementation evidence.
