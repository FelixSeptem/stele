## 1. Contract and schema

- [x] 1.1 Add the governed-release-evidence-reconciliation capability spec and verify `openspec validate governed-release-evidence-freshness-reconciliation --type change` accepts all new requirements and scenarios.
- [x] 1.2 Define additive PostgreSQL tables/indexes for immutable reconciliation verdicts, current eligibility, replay keys, checkpoints, and retention metadata; verify migration up/down or recovery behavior with the repository migration checks.
- [x] 1.3 Extend evidence handoff persistence with immutable scope, policy, watermark, fixture, representation, freshness, attestation, and rollback identities; verify incomplete handoffs cannot become eligible.

## 2. Reconciliation domain behavior

- [x] 2.1 Implement the deterministic ordered gate evaluator with exact-scope and fail-closed handling; verify unit tests cover freshness, watermark, policy, fixture, representation, attestation, rollback, missing dependency, and foreign-row failures.
- [x] 2.2 Implement append-only verdict recording and transactional current-eligibility transitions; verify stale replay cannot restore eligibility and a new compatible handoff can restore it.
- [x] 2.3 Add deterministic idempotency and replay-key handling; verify duplicate scheduler fires and worker retries converge on one effective transition with preserved attempt history.

## 3. Durable maintenance execution

- [x] 3.1 Add the scope-bound reconciliation job class to scheduler dispatch and worker claim paths; verify lease, retry, checkpoint, cancellation, and crash recovery behavior with focused worker tests.
- [x] 3.2 Add bounded batch/time/retry limits and monotonic checkpoints; verify unprocessed evidence never receives an eligible verdict after a run reaches its bounds.
- [x] 3.3 Add the authorized manual trigger through the same durable job path; verify duplicate active runs are reused or deduplicated without lease seizure.

## 4. Release gate integration

- [x] 4.1 Make governed activation read current reconciliation eligibility and fail closed on absent, stale, revoked, or incompatible state; verify existing protected gates and rollback behavior remain enforced.
- [x] 4.2 Verify reconciliation state does not affect ordinary retrieval or context assembly when the governed release policy is inactive, shadow-only, or unauthorized.

## 5. Operations and admin surfaces

- [x] 5.1 Add bounded, redacted reconciliation metrics and structured logs; verify labels exclude raw scope, evidence IDs, query text, provider payloads, and credentials.
- [x] 5.2 Add exact-scope admin inspection for runs, current eligibility, checkpoints, freshness buckets, reason categories, and transition counts; verify foreign-scope requests reveal neither existence nor counts.
- [x] 5.3 Add admin attribution and audit records for manual triggers and governance denials; verify direct activation or restoration without a new handoff is rejected.

## 6. Backfill, documentation, and verification

- [x] 6.1 Implement bounded backfill for existing handoffs and verify missing identities produce incomplete/ineligible verdicts without mutating historical evidence.
- [x] 6.2 Update release-evidence, release-gate, worker, observability, admin, and roadmap documentation; verify links and capability names match the OpenSpec inventory.
- [x] 6.3 Run focused reconciliation, release-gate, worker, admin, and isolation tests; verify `go test ./...` and required race/vet gates pass before implementation is considered complete.
- [x] 6.4 Run `openspec validate --all --strict`, `git diff --check`, and the repository quality/documentation gates; record outputs for review.
