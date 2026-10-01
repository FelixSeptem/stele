## 1. Release Evidence Identity And Validation

- [x] 1.1 Add a stable redacted release-evidence identity and source-watermark/freshness fields to the existing retrieval evidence domain, derive them from exact scope and compatible policy/strategy inputs, and verify deterministic identity plus secret/content exclusion with focused unit tests.
- [x] 1.2 Implement shared fail-closed compatibility validation for verdict, evidence age, scope hash, fixture/provider/analysis/fusion/ranking/renderer identities, semantic-hit proof, integrity/trajectory summaries, resource budgets, replay, and rollback; verify stable failure categories for each missing, stale, incompatible, and unsafe prerequisite.
- [x] 1.3 Extend release report serialization and human-readable summaries with only bounded run identity, freshness, replay, rollback, and aggregate fields; verify DSNs, scope values, queries, content, identifiers, raw scores, prompts, credentials, and provider payloads cannot appear.

## 2. Scoped Activation Integration

- [x] 2.1 Extend the existing ranking rollout activation gate to require a matching passed evidence attestation for the exact scope, policy, strategy, and dependency identities; verify diagnostics-only, shadow, skipped, degraded, stale, rejected, and mismatched evidence cannot activate.
- [x] 2.2 Persist only the minimal forward-only attestation reference/digest and bounded compatibility metadata in the existing rollout audit path, or prove existing fields are sufficient; verify append-only history, tenant/project/namespace isolation, and no raw evidence persistence.
- [x] 2.3 Revalidate evidence freshness and compatibility during runtime rollout resolution and return the approved baseline after expiry, policy change, dependency mismatch, disablement, or rollback; verify exact-scope active behavior and baseline fallback with domain tests.
- [x] 2.4 Add admin activation, disablement, and rollback contract tests for actor/reason attribution, evidence binding, conflict handling, and redacted audit output; verify ordinary OpenAPI search/context behavior remains unchanged when no exact-scope activation is valid.

## 3. Owned Real-Stack Workflow

- [x] 3.1 Extend the repository-owned PostgreSQL + pgvector fixture runner to cover baseline equivalence, progressive context, parent-first shadow, adaptive planning, temporal cases, semantic-hit proof, trajectory/integrity summaries, deterministic replay, and rollback evidence; verify exact fixture cleanup and isolated scope behavior.
- [x] 3.2 Update the opt-in retrieval evaluation wrapper with explicit ownership validation, bounded timeout, isolated report directory, stable skip/fail exit codes, and no runtime-DSN fallback; verify missing DSN returns `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` and rejected reports never authorize activation.
- [x] 3.3 Add redaction and report-retention tests around the real-stack command, including failure-path cleanup and rerun behavior; verify only allow-listed categories and logical identities are emitted.

## 4. Documentation And Roadmap Calibration

- [x] 4.1 Update `docs/retrieval-release-checklist.md` and `docs/retrieval-release-gate.md` with evidence freshness, stable identity, activation attestation, exact-scope rollout, disablement, and rollback requirements; verify documentation checks and command examples remain valid.
- [x] 4.2 Update `docs/self-hosting.md` with the refresh command, prerequisites, skip/non-pass semantics, report redaction, operator activation boundary, and baseline rollback path; verify self-hosting documentation tests pass.
- [x] 4.3 Mark change 052/P8.7 archived and this proposal as P8.8 in the v1 roadmap and status contract tests; verify roadmap/archive consistency against `openspec list --json` and archive index.

## 5. Verification

- [x] 5.1 Run focused retrieval, rollout, storage, app-contract, and documentation tests with a task-specific Go cache and verify all pass.
- [x] 5.2 Run the owned real-stack PostgreSQL + pgvector evidence command when an explicit disposable DSN is available, verify the redacted artifacts and cleanup, and otherwise verify the controlled skip result without claiming release readiness.
- [x] 5.3 Run `openspec validate --all`, `git diff --check`, and the full relevant Go test/race/vet suite; verify no active evidence can authorize rollout without the required fresh exact-scope gates.
