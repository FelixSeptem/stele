## 1. Contracts and bounded data model

- [x] 1.1 Define redacted trajectory, integrity finding, compatibility identity, verdict, and retention contracts; verify the Go types reject raw queries, scope values, identifiers, scores, provider payloads, credentials, and unbounded plans.
- [x] 1.2 Add deterministic bucket and category normalization for channels, candidate counts, expansion, fallback, freshness, budgets, latency, and integrity findings; verify repeated normalization produces identical output.
- [x] 1.3 Add exact-scope authorization and compatibility checks for fixture, policy, strategy, renderer, provider capability, and source-watermark identities; verify incompatible or foreign inputs fail closed.

## 2. PostgreSQL persistence and retention

- [x] 2.1 Add append-only PostgreSQL migrations for trajectory aggregates, integrity reports/findings, compatibility identities, and retention outcomes with exact-scope and idempotency constraints; verify migration up/down and manifest tests.
- [x] 2.2 Implement repository create/read/list methods with bounded pagination and scope predicates; verify pgxmock or repository tests reject out-of-scope reads and preserve historical versions.
- [x] 2.3 Implement deterministic retention cleanup for expired derived artifacts only; verify cleanup reports bounded outcomes and leaves canonical source records untouched.

## 3. Trajectory collection and integrity evaluation

- [x] 3.1 Add evaluation-side trajectory aggregation that is independent of public search/context response handling; verify collection failure produces a degraded evaluation result without failing ordinary retrieval.
- [x] 3.2 Add memory-organization integrity evaluators for consolidation, merge, reclassification, reflection, and projection changes; verify action success and information-integrity verdicts remain separate.
- [x] 3.3 Add hard failure classification for missing, altered, unexpectedly duplicated, misplaced, foreign-scope, hidden-lifecycle, stale-watermark, nondeterministic, and rollback-incomplete evidence; verify any hard category produces a non-pass verdict.
- [x] 3.4 Add deterministic replay and append-only report comparison; verify identical source/policy/renderer inputs reproduce stable aggregate identities and do not mutate canonical memory or active retrieval.

## 4. Release evidence and admin surface

- [x] 4.1 Extend owned retrieval evidence runs to include optional compatible trajectory and integrity summaries; verify missing, stale, incompatible, or unsafe summaries remain skipped/degraded and cannot authorize rollout.
- [x] 4.2 Expose exact-scope admin inspection for completed trajectory, integrity, replay, and retention reports with redacted fields and bounded pagination; verify public callers receive no diagnostic internals and out-of-scope callers receive no existence signal.
- [x] 4.3 Update OpenAPI schemas, route wiring, authorization checks, and admin documentation for the report surface; verify generated/contract tests cover response redaction and error categories.

## 5. Observability, tests, and documentation

- [x] 5.1 Add low-cardinality metrics and bounded lifecycle logs for collection, integrity checks, replay, retention, and hard safety failures; verify labels contain no scope, query, record, report, provider, credential, or reason-text identifiers.
- [x] 5.2 Add unit and integration fixtures for exact-scope isolation, lifecycle filtering, redaction, compatibility, replay determinism, retention, rollback, and quality-gain-overridden-by-integrity-failure behavior; verify focused package tests pass.
- [x] 5.3 Update self-hosting and retrieval release-gate documentation with the new diagnostics/evaluation-only behavior and operator retention responsibilities; verify docs smoke checks pass.
- [x] 5.4 Run the full verification suite (`go test ./... -count=1 -p 1 -timeout 15m`, focused race tests, `go vet ./...`, `openspec validate --all --strict`, and `git diff --check`) and record the evidence before archive.

## Verification evidence

Completed on 2026-10-01 in the proposal worktree:

- `go test ./... -count=1 -p 1 -timeout 15m` (exit 0)
- `go test -race ./internal/evaluation ./internal/retrieval ./internal/storage/postgres ./internal/app ./internal/telemetry -count=1` (exit 0)
- `go vet ./...` (exit 0)
- `openspec validate --all --strict` (74 passed, 0 failed)
- `pwsh -File scripts/check-self-hosting-smoke-docs.ps1` (exit 0)
- `git diff --check` (exit 0)
