## Why

Stele already persists leases, retries, checkpoints, duplicate-fire decisions,
and maintenance outcomes, but operators still lack one consistent run-history
contract for understanding scheduler behavior across restarts and retries.
Stable job identity, lease recovery, duplicate dispatch, terminal disposition,
retention, and inspection need to be connected into one bounded operational
record before more scheduled maintenance or provider work is added.

## What Changes

- Define a stable, scope-bound scheduler run identity derived from job class,
  exact tenant/project/namespace, cadence window, and idempotency key.
- Preserve append-only run attempts and terminal dispositions for completed,
  duplicate, skipped, retrying, exhausted, cancelled, and recovered runs.
- Make lease acquisition, renewal, stale-owner reclamation, retry/backoff,
  checkpoint resume, and duplicate-fire handling produce consistent bounded
  state transitions without duplicating durable side effects.
- Add deterministic retention and cleanup for high-volume run history while
  preserving terminal audit evidence and canonical source records.
- Expose authorized, exact-scope, paginated run-history inspection with stable
  filters and redacted failure/recovery categories; public request behavior is
  unchanged.
- Add low-cardinality metrics and bounded lifecycle logs for scheduler runs,
  lease/retry/recovery outcomes, duplicate dispatch, cleanup, and operator
  inspection.
- Reconcile the roadmap so archived change 055 is no longer shown as active and
  this proposal is the sole active post-v1 direction.

## Non-goals

- No new scheduler framework, queue, persistence store, or hosted control plane.
- No change to PostgreSQL as the only system of record, tenant/project/namespace
  isolation, canonical memory lifecycle, or public retrieval behavior.
- No inline execution of maintenance from public API requests and no bypass of
  worker leases by operator actions.
- No new provider, MCP, WebSocket, SDK, UI, or end-user product logic.
- No autonomous policy activation or second authorization system.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `worker-orchestration-and-maintenance-jobs`: strengthen stable scheduler run
  identity, append-only attempt/terminal history, lease-safe recovery,
  duplicate-fire handling, and retry exhaustion semantics.
- `durable-multiscope-maintenance-observability`: define retention-safe run
  history, exact-scope pagination, checkpoint/recovery evidence, and cleanup
  behavior for durable maintenance executions.
- `admin-inspection-surface`: add authorized, exact-scope, paginated scheduler
  run-history inspection with bounded filters and redacted categories.
- `service-observability`: add low-cardinality scheduler lifecycle metrics and
  bounded logs for dispatch, lease, retry, recovery, duplicate, cleanup, and
  inspection outcomes.

## Impact

- Worker/scheduler orchestration and PostgreSQL repositories will gain a
  consistent run-history state model and retention-safe cleanup path.
- Existing admin inspection contracts will gain a bounded read surface for
  operators without exposing scope values, raw errors, credentials, payloads,
  or record contents through metrics/logs.
- Scheduler and maintenance documentation, roadmap reconciliation, and focused
  restart/duplicate/lease/retention tests will be updated.
- Existing runtime modes (`api`, `worker`, `scheduler`) remain intact; no
  migration is allowed to rewrite canonical memory or source records.

## References

- [Worker orchestration and maintenance jobs](../../specs/worker-orchestration-and-maintenance-jobs/spec.md)
- [Durable multi-scope maintenance observability](../../specs/durable-multiscope-maintenance-observability/spec.md)
- [Admin inspection surface](../../specs/admin-inspection-surface/spec.md)
- [Service observability](../../specs/service-observability/spec.md)
- [Stele v1 roadmap](../../../docs/roadmaps/2026-05-28-stele-v1-roadmap.md)
- `openspec validate --all --strict`
