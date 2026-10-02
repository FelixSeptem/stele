## 1. Run Identity And State Model

- [x] 1.1 Define stable scheduler run, attempt, terminal-disposition, lease, retry, recovery, checkpoint, and cleanup categories; verify serialization and allowed transitions with focused unit tests.
- [x] 1.2 Derive deterministic run identity from job class, normalized exact scope, cadence/idempotency window, and bounded execution parameters; verify duplicate submissions produce the same identity and do not include raw scope in telemetry.
- [x] 1.3 Add append-only attempt history and compact terminal summary persistence while preserving compatibility with existing durable execution records; verify successful, duplicate, skipped, retrying, exhausted, cancelled, and completed outcomes.

## 2. Lease Recovery And Retry Closure

- [x] 2.1 Integrate lease acquisition, renewal, stale-owner reclamation, and checkpoint resume with compare-and-set ownership rules; verify a lost owner cannot write subsequent checkpoints or completion state.
- [x] 2.2 Implement retry/backoff and retry-exhaustion terminal handling with explicit manual-review/recovery disposition; verify exhausted runs are not automatically re-enqueued.
- [x] 2.3 Add cancellation and governed recovery transitions that preserve prior terminal history and re-enter only through the ordinary lease-safe claim path; verify active leases cannot be seized.
- [x] 2.4 Add restart and duplicate-fire integration tests across worker and scheduler modes; verify canonical source records are not rewritten and duplicate durable side effects are not created.

## 3. Retention And Freshness

- [x] 3.1 Add bounded retention for high-volume attempt/detail records while preserving terminal summaries, audit transitions, and required freshness/watermark identities; verify cleanup never deletes canonical source records.
- [x] 3.2 Make run-history cleanup idempotent and observable across restart/retry; verify repeated cleanup yields no-op or duplicate disposition with stable surviving summaries.
- [x] 3.3 Apply freshness/SLO eligibility checks to retained recovery summaries; verify stale, divergent, foreign, or lifecycle-hidden evidence cannot satisfy conformance/readiness.

## 4. Admin Inspection And Telemetry

- [x] 4.1 Add exact-scope, cursor-paginated admin inspection for run summaries and attempts with bounded job/state/time/recovery filters; verify unauthorized scope requests reveal no existence or counts.
- [x] 4.2 Emit bounded metrics and structured logs for dispatch, lease, retry, recovery, duplicate, terminal, cleanup, and inspection lifecycle events; verify sensitive fields are rejected, redacted, or bucketed.
- [x] 4.3 Add admin and telemetry contract tests for stable ordering, opaque cursors, redacted failure categories, and low-cardinality labels.

## 5. Documentation And Verification

- [x] 5.1 Update scheduler/maintenance operator documentation with run identity, lease recovery, retry exhaustion, retention, pagination, and cancellation/recovery semantics; verify docs consistency checks available in the repository.
- [x] 5.2 Reconcile `docs/roadmaps/2026-05-28-stele-v1-roadmap.md` so change 055 is archived and this proposal is the only active post-v1 direction; verify OpenSpec/roadmap status agrees.
- [x] 5.3 Run focused worker, scheduler, storage, admin, telemetry, isolation, race, vet, OpenSpec strict validation, and `git diff --check`; verify no public retrieval or canonical-memory behavior changes.
