## Context

See `proposal.md` for motivation. Stele already has scope-aware scheduler jobs,
durable worker leases, `job_executions`, projection records, assurance records,
and low-cardinality telemetry. The missing part is a unified durable contract
that proves every durable scope has safe maintenance, fresh projections,
restart recovery, retained run evidence, and redacted observability.

PostgreSQL remains the only system of record. Public APIs are stable, and the
change must not expose internal trajectories or mutate canonical memory.

## Goals / Non-Goals

**Goals:**

- Reuse existing execution, workflow, projection, and assurance persistence.
- Make maintenance identity, leases, retries, checkpoints, and terminal outcomes
  durable and exact-scope.
- Gate derived projection eligibility on watermark freshness, scope, lifecycle,
  rebuild, and SLO evidence.
- Emit bounded internal observability and produce repeatable conformance proof.
- Apply deterministic retention only to derived operational artifacts.

**Non-Goals:**

- No new public endpoint or public response fields.
- No separate maintenance ledger, queue, or storage engine.
- No default-ranking change, provider adapter, MCP surface, SDK, or UI.
- No canonical-memory, raw-event, or memory-version overwrite.

## Decisions

### Reuse job executions as the durable scheduling boundary

Extend the existing job execution model with a deterministic identity derived
from job class, exact scope, and cadence/idempotency window. Store lease owner,
lease expiry, attempt, retry category, checkpoint/source watermark, and terminal
disposition there or in an existing execution-linked evidence field. Enforce one
active or successful execution per stable identity with PostgreSQL constraints
and conditional updates.

This is preferred over new `maintenance_runs` and `maintenance_leases` tables
because Stele already has execution history, cleanup, scope indexes, and
operator semantics around `job_executions`. A separate ledger would duplicate
state transitions and introduce reconciliation failure modes.

### Use PostgreSQL compare-and-set lease transitions

Acquire, renew, complete, fail, and reclaim operations use exact identity,
scope, owner, current state, and expiry predicates. Reclaim is allowed only
after expiry. Duplicate completed identities return a bounded duplicate
disposition. Retry scheduling uses a bounded attempt count and `next_attempt_at`
rather than process timers as durable state.

An external queue was rejected because maintenance is already scheduler/worker
owned and PostgreSQL provides the required transactional claim semantics.

### Keep projection eligibility separate from canonical lifecycle

Maintenance computes a freshness evidence record from source watermark,
projection watermark, policy/renderer identity, lifecycle visibility, rebuild
checkpoint, and bounded SLO result. Retrieval consults only the resulting
eligible/ineligible derived state. Failure never rewrites source records.

Embedding freshness directly into canonical memory was rejected because it
would mix derived operational state with append-only memory history.

### Model observability through fixed enums and buckets

Instrumentation accepts typed categories for job class, execution outcome,
lease/retry/recovery result, freshness, retrieval channel availability,
candidate/expansion count, latency, and SLO result. Values outside the allowed
sets are rejected or converted to an `unknown` category. Raw scope values,
queries, IDs, scores, errors, provider payloads, and credentials never enter
labels or retained diagnostic payloads.

This is preferred over dynamically labeled metrics because arbitrary values
create cardinality, privacy, and self-hosting cost risks.

### Extend assurance with a maintenance closure analyzer

The analyzer consumes bounded execution, projection, retention, and telemetry
evidence for a requested exact scope. It reports coverage and stable missing,
stale, unsafe, or incomplete categories. Safety findings are hard failures and
cannot be averaged away by successful jobs or latency improvements.

Assurance remains diagnostic: it does not run maintenance, perform cleanup, or
change retrieval eligibility itself.

### Retain only derived artifacts under bounded policy

Execution diagnostics, freshness evidence, conformance evidence, and redacted
trajectories use explicit retention classes and deterministic cleanup windows.
Cleanup uses allowlisted derived categories and idempotent deletion. Canonical
source tables and incident audit history are outside the cleanup target set.

## Risks / Trade-offs

- **Existing `job_executions` may lack fields needed for safe claim state.**
  → Add the smallest versioned migration and indexes needed; do not create a
  parallel scheduling model.
- **A broad conformance analyzer could become expensive across many scopes.**
  → Require exact-scope evaluation, bounded batch/cursor pagination, fixed
  evidence windows, and explicit SLO buckets.
- **Stale reclaim can cause concurrent work if clocks or lease predicates are
  wrong.** → Use database time and conditional owner/state/expiry updates;
  exercise concurrent claim and renewal tests against PostgreSQL.
- **Telemetry redaction can regress as new fields are added.** → Expose typed
  bounded constructors and tests that reject sensitive/high-cardinality values.
- **Fail-closed projection eligibility may reduce recall during maintenance
  incidents.** → Fall back to the previously approved canonical/flat retrieval
  path and record the freshness failure; never serve stale derived evidence
  silently.

## Migration Plan

1. Add backward-compatible nullable/defaulted execution fields and indexes with
   an idempotent forward migration and an explicit down migration.
2. Deploy repository support while the new behavior remains disabled; existing
   scheduler execution continues unchanged.
3. Enable durable identity/lease handling for a bounded owned scope and verify
   duplicate, restart, retention, freshness, and conformance evidence.
4. Expand by scope only after the conformance report is green.
5. Roll back by disabling the new scheduler path. Preserve execution/evidence
   history until normal derived-artifact retention removes it; do not rewrite
   canonical data or automatically down-migrate a live database.
