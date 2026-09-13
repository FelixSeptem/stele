## 1. Durable execution model

- [x] 1.1 Add failing unit tests for stable maintenance identity, exact-scope validation, cadence-window idempotency, and duplicate-fire dispositions; verify tests fail before implementation.
- [x] 1.2 Extend maintenance execution domain types with stable identity, attempt/retry, lease, checkpoint/watermark, terminal disposition, and bounded category validation; verify focused jobs tests pass.
- [ ] 1.3 Add failing repository tests for conditional lease acquire, renew, complete, stale reclaim, retry scheduling, and cursor-paginated run history; verify SQL expectations include exact scope and owner predicates.
- [ ] 1.4 Implement idempotent PostgreSQL migration fields, constraints, and indexes on existing job execution records; verify migration manifest, up/down tests, and repeat application pass.
- [ ] 1.5 Implement repository compare-and-set transitions and bounded history pagination; verify concurrent claim, owner conflict, stale reclaim, and duplicate identity tests pass.
- [ ] 1.6 Wire scheduler and worker maintenance dispatch to durable identity, lease renewal, retry/backoff, and checkpoint resume; verify restart and duplicate-fire orchestration tests pass.

## 2. Projection freshness and SLO

- [ ] 2.1 Add failing tests for source/projection watermark matching, freshness windows, policy/renderer identity, lifecycle visibility, exact scope, and SLO bucket classification.
- [ ] 2.2 Implement projection maintenance evidence calculation and fail-closed eligibility transitions without mutating canonical records; verify focused retrieval/projection tests pass.
- [ ] 2.3 Add repository persistence and retrieval filtering for freshness/rebuild evidence; verify stale, divergent, foreign, and hidden projections are excluded from ordinary retrieval.
- [ ] 2.4 Add bounded maintenance SLO configuration and validation for age, duration, retry, and rebuild limits; verify invalid and over-limit configuration tests pass.
- [ ] 2.5 Integrate projection freshness/rebuild checks into scope maintenance dispatch and checkpoint resume; verify exact-scope rebuild and interrupted-run recovery tests pass.

## 3. Internal observability and retention

- [ ] 3.1 Add failing telemetry tests for fixed maintenance job, lease/retry/recovery, freshness, channel, candidate/expansion, latency, and SLO categories plus sensitive-field rejection.
- [ ] 3.2 Implement typed low-cardinality maintenance and retrieval telemetry constructors and bounded structured logs; verify no query, scope value, identifier, raw score, provider payload, or credential is emitted.
- [ ] 3.3 Add bounded derived-artifact retention policy and cleanup for execution diagnostics, freshness evidence, conformance evidence, and redacted trajectories; verify cleanup is idempotent and canonical/incident records survive.
- [ ] 3.4 Wire scheduler, worker, projection, and retention outcomes to telemetry; verify integration metrics/log tests expose only fixed categories and bucket values.

## 4. Assurance and conformance closure

- [ ] 4.1 Add failing assurance tests for durable-scope discovery, maintenance coverage, lease recovery, projection freshness/rebuild, retention safety, telemetry redaction, and evidence completeness.
- [ ] 4.2 Implement the scope-bounded maintenance conformance analyzer with stable failure categories and fail-closed readiness outcome; verify unit tests separate action success from safety/integrity failure.
- [ ] 4.3 Persist bounded conformance evidence through existing assurance records and include it in readiness summaries without exposing sensitive scope details; verify repository and serialization tests pass.
- [ ] 4.4 Add scheduler-driven conformance execution with leases, retries, duplicate-fire suppression, and bounded retention; verify restart, stale reclaim, and cleanup tests pass.

## 5. Documentation and rollout controls

- [ ] 5.1 Add configuration reference for maintenance identity, lease, retry, freshness, SLO, telemetry, retention, and conformance limits; verify `.env.local.example` contains placeholders only.
- [ ] 5.2 Document operator evidence collection, PostgreSQL 18 + pgvector maintenance smoke, rollback, stale recovery, and canonical-data safety; verify docs consistency checks pass.
- [ ] 5.3 Add disabled-by-default rollout and fallback wiring; verify failed conformance gates preserve the previously approved scheduler and retrieval behavior.
- [ ] 5.4 Add deterministic CI coverage for focused maintenance/conformance tests and redaction checks; verify CI commands run without external provider credentials.

## 6. Verification and release evidence

- [ ] 6.1 Run focused jobs, storage, workflow, projection, telemetry, retrieval, and assurance tests with a repository-local Go cache; verify zero failures.
- [ ] 6.2 Run `go test ./... -count=1 -timeout 15m`, `openspec validate --all`, and `git diff --check`; record command output and any environment-only limitations.
- [ ] 6.3 When available, run the PostgreSQL 18 + pgvector maintenance/conformance smoke with an explicitly owned evaluation DSN; verify redacted evidence and no canonical-data mutation.
- [ ] 6.4 Review all task evidence, mark completed tasks, and prepare the change for archive using `scripts/openspec-archive-seq.ps1` only after implementation and release gates are complete.
