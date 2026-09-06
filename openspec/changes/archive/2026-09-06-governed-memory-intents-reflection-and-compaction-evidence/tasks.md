# Tasks

## Schema and repositories

- [x] Define PostgreSQL migrations for intent ledger, reflection runs/checkpoints, review decisions, and compaction evidence.
- [x] Add repository interfaces and scope-safe queries with append-only audit semantics.
- [x] Add unique idempotency and stable replay/duplicate-fire constraints.

## Governance and APIs

- [x] Implement intent validation, lifecycle/version checks, and candidate enqueue behavior.
- [x] Add OpenAPI contracts and handlers for intent submission/status and admin review/run/evidence inspection.
- [x] Implement review decisions and canonical version integration.

## Worker and scheduler

- [x] Implement durable reflection run creation triggers and transactional lease/checkpoint handling.
- [x] Add retry budgets, bounded failure categories, replay, and recovery behavior.
- [x] Connect compaction pressure and scheduled/operator triggers without bypassing scope or policy.

## Compaction and projection

- [x] Persist evidence-backed compaction state and recent-tail/source references.
- [x] Integrate approved derived outputs with existing projection policy and bounded diagnostics.
- [x] Ensure stale/suppressed/forgotten/deleted/foreign-scope inputs fail closed.

## Verification and documentation

- [x] Add unit, repository, worker, integration, isolation, lifecycle, replay, and rebuild tests.
- [x] Add recovery/conformance coverage proving canonical memory is not mutated by rebuilds.
- [x] Update OpenAPI, lifecycle, governance, and operational documentation.
- [x] Run validation, quality gates, and full relevant Go test suites.
