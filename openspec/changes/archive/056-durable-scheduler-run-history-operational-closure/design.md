## Context

See `proposal.md` for the motivation. The existing worker/scheduler baseline
already persists leases, retries, checkpoints, duplicate-fire decisions, and
maintenance outcomes, but these signals are spread across execution paths and
do not form one operator-readable history contract. PostgreSQL remains the
only system of record; all run details are derived operational evidence and
must preserve exact tenant/project/namespace isolation.

## Goals / Non-Goals

**Goals:**

- Define one stable scheduler run identity and append-only attempt lifecycle.
- Make duplicate dispatch, lease recovery, retry exhaustion, cancellation, and
  checkpoint resume deterministic across process restarts.
- Preserve a terminal summary while applying bounded retention to high-volume
  attempt/detail records.
- Add authorized exact-scope pagination for operator inspection.
- Reuse existing low-cardinality telemetry and admin authorization boundaries.

**Non-Goals:**

- Replacing the current worker, scheduler, queue, or PostgreSQL repository
  architecture.
- Adding a second job store, event bus, public request execution path, or
  external scheduler dependency.
- Changing canonical memory, retrieval, provider contracts, or lifecycle rules.

## Decisions

### 1. Model a run as an append-only identity plus bounded attempts

Use a deterministic identity from job class, normalized exact scope, cadence or
idempotency window, and bounded execution parameters. Attempts and state
transitions append records keyed by that identity; a compact terminal summary
supports fast inspection and retention handoff.

This is preferred over one mutable status row because restart/retry history and
duplicate-fire decisions must remain auditable. It also avoids using process
memory as the source of truth.

### 2. Keep lease state and checkpoint state in the same ownership contract

Lease acquisition, renewal, stale reclaim, checkpoint update, and terminal
completion use compare-and-set ownership rules. A worker that loses ownership
cannot write a checkpoint or completion transition. Reclaim resumes from the
latest committed checkpoint and never rewrites canonical source records.

This is preferred over a separate recovery queue, which would create a second
ownership path and make duplicate execution harder to reason about.

### 3. Retain terminal summaries, prune attempt details

Retention deletes only derived high-volume attempt/detail rows after their
window expires. The latest terminal summary, duplicate/recovery disposition,
freshness identity, and required audit transition remain retained for the
configured audit horizon. Cleanup is idempotent and itself observable.

This balances operator diagnosis with bounded storage and avoids deleting
canonical memory or source evidence.

### 4. Reuse the exact-scope admin boundary with cursor pagination

Run-history inspection is exposed through the existing admin authorization
surface. The repository filters by normalized exact scope before resolving
existence, orders by stable terminal/observed time plus opaque tie-breaker,
and returns bounded cursors and page sizes.

A new public endpoint or client-held scope model is not adopted because it
would broaden the authorization boundary for an operational read-only need.

### 5. Use one fixed telemetry vocabulary

Metrics and logs share bounded values for operation, result, run state, lease,
retry, recovery, cleanup, freshness, SLO, and duration buckets. Raw scope,
identifiers, errors, payloads, and reasons are rejected or bucketed before
emission.

This makes dashboards stable and prevents scheduler volume from creating
high-cardinality telemetry.

### 6. Roadmap reconciliation is documentation-only

The roadmap will mark change 055 as archived and this proposal as the active
bounded post-v1 direction. It does not alter archive numbering, branch policy,
or implementation behavior.

## Risks / Trade-offs

- **[Risk] Append-only attempts increase storage volume.** → Apply bounded
  detail retention, preserve compact terminal summaries, and make cleanup
  idempotent and measurable.
- **[Risk] Cursor pagination can miss records during concurrent writes.** → Use
  stable ordering with an opaque cursor and document snapshot/continuation
  semantics; never expose raw database offsets.
- **[Risk] Lease recovery can replay non-idempotent external effects.** → Keep
  durable side effects behind existing idempotency keys and require checkpoint
  commits before progress; no new external side-effect path is introduced.
- **[Risk] Category drift breaks dashboards and runbooks.** → Centralize allowed
  values and add contract tests for metrics, logs, summaries, and admin DTOs.
- **[Risk] Operators mistake a terminal summary for current readiness.** →
  Include freshness/SLO and terminal disposition, and fail conformance/readiness
  claims closed when evidence is stale or divergent.

## Migration Plan

1. Add the stable run/attempt representation and compatibility mapping for
   existing durable execution records without rewriting canonical data.
2. Emit new lifecycle categories alongside existing maintenance telemetry and
   populate terminal summaries for new and recovered runs.
3. Enable idempotent detail retention cleanup and verify that the terminal
   summary/audit horizon is preserved.
4. Add exact-scope admin pagination and authorization tests, then exercise
   restart, duplicate-fire, lease reclaim, retry exhaustion, and cancellation
   scenarios.
5. On rollback, disable the new inspection/cleanup consumer and continue using
   the existing worker lease path; retain already-written history and do not
   delete source records.

## Open Questions

None. Retention durations, page-size ceilings, and exact category constant names
can be selected during implementation as long as they remain bounded, stable,
redacted, and compatible with these specs.
