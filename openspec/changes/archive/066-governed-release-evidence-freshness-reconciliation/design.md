## Context

The proposal closes the gap between a release evidence handoff that was valid at submission time and the conditions required to keep a governed activation eligible later. Existing release runs, attestations, rollback proofs, durable maintenance jobs, policy precedence, redacted diagnostics, and admin inspection are the source contracts. See `proposal.md` and the delta specs for the behavioral boundary.

The design must preserve PostgreSQL as the system of record, exact project/tenant/namespace isolation, append-only evidence and audit history, fail-closed activation, and unchanged ordinary retrieval behavior.

## Goals / Non-Goals

**Goals:**

- Make current activation eligibility an explicit, queryable projection derived from immutable handoffs and append-only reconciliation verdicts.
- Reconcile freshness, source watermark, policy, fixture, representation, attestation, and rollback proof compatibility with deterministic, bounded work.
- Reuse the existing scheduler/worker lease, retry, checkpoint, redaction, and admin authorization boundaries.
- Make revocation, restoration, lag, and failure observable and reviewable without exposing sensitive evidence.
- Support safe replay and migration from existing handoffs that do not yet have reconciliation rows.

**Non-Goals:**

- Re-running benchmark suites or provider evaluations automatically.
- Introducing a feature-flag or rollout system parallel to the current release policy.
- Mutating canonical memory, projections, raw events, or historical evidence.
- Changing public retrieval or context assembly defaults.

## Decisions

### 1. Store append-only verdicts plus a current eligibility projection

Create a PostgreSQL reconciliation history keyed by exact scope, handoff identity, policy identity, and reconciliation window. Each record contains a deterministic replay key, input watermark, observed freshness category, bounded verdict/reason, actor or job attribution, and timestamps. Maintain a separate current eligibility row for fast release-gate reads; update it transactionally with the effective transition and a pointer to the immutable verdict.

This separates historical evidence from current eligibility and avoids rewriting accepted handoffs. A materialized projection is preferred over calculating every gate synchronously during activation because it gives operators a stable state and lets the scheduler bound work. The release gate still fails closed when the projection is missing or stale.

### 2. Resolve compatibility through one ordered gate evaluator

Evaluate gates in a deterministic order: exact scope, handoff completeness, source watermark, policy version/precedence, fixture identity, representation identity, freshness deadline, attestation, and rollback proof. Record only the first bounded failure category plus aggregate counters; preserve enough logical identities to replay the decision. Any lookup error, missing dependency, or foreign row is ineligible.

The evaluator reads current policy and source watermarks under the same scope boundary as the handoff. It never changes those sources and never interprets a newer handoff as an implicit promotion for another policy.

### 3. Use a shared durable job for scheduler and manual triggers

Add a reconciliation job class to the existing maintenance work model. The scheduler creates one run per scope and bounded time window; the admin endpoint requests the same job with actor and reason metadata. A deterministic idempotency key `(scope, policy, window, handoff set watermark)` coalesces duplicate fires. Worker leases, retry budgets, monotonic checkpoints, and terminal history follow existing orchestration contracts.

Manual requests return the durable run identity and never perform direct activation or unbounded synchronous scans. A run that is already leased is returned or deduplicated rather than seized.

### 4. Apply fail-closed transitions transactionally

For each effective verdict, write the append-only history row and update current eligibility in one transaction guarded by the exact scope and expected projection version. Eligible remains eligible only when all required gates pass. A revoked row can be restored only when a new compatible handoff is linked; replaying the same stale handoff cannot restore it. Concurrent policy or watermark changes cause a retry with a new input identity rather than an in-place overwrite.

### 5. Keep diagnostics redacted and low cardinality

Expose opaque run and handoff references, policy and watermark version identities, freshness buckets, reason categories, counts, and transition timestamps. Do not return raw evidence, query text, provider output, scope strings outside the authorized route, credentials, or unbounded identifiers. Metrics use bounded labels such as `outcome`, `reason`, `job_class`, and `freshness_bucket`.

### 6. Migrate conservatively and roll back by disabling eligibility reads

Deploy additive tables/indexes and a backfill job that creates reconciliation records for existing handoffs without granting new activation rights. Until a handoff has a current compatible verdict, the release gate treats it as ineligible. Rollback of the implementation disables reconciliation dispatch and leaves history intact; the release gate continues to fail closed for missing current eligibility. A later deployment can resume from checkpoints.

## Risks / Trade-offs

- **[Risk]** A stale source watermark may revoke an activation during a transient dependency outage. → Treat dependency errors as bounded ineligible states, expose the reason and lag, and require a new compatible handoff for restoration rather than silently keeping activation alive.
- **[Risk]** A large handoff population can make scheduled scans expensive. → Use scope and watermark indexes, bounded batches, monotonic checkpoints, and lag metrics; never exceed the existing maintenance envelope.
- **[Risk]** Projection and history can diverge after a crash. → Commit both in one transaction and reconcile the projection from append-only history during recovery.
- **[Risk]** Existing handoffs lack one of the new identities. → Backfill them as incomplete/ineligible and retain the original report; do not infer identities from mutable current state.
- **[Risk]** Operators may confuse historical pass evidence with current eligibility. → Admin responses and telemetry label both explicitly and expose transition history with bounded reason categories.

## Migration Plan

1. Add additive schema objects, indexes, reason enums, and retention metadata through the existing migration manager.
2. Deploy read-path support that treats absent or stale current eligibility as ineligible while leaving ordinary retrieval unchanged.
3. Run a bounded backfill over existing release handoffs, recording incomplete verdicts where identities are unavailable.
4. Enable scheduler dispatch and the admin trigger after focused migration and scope-isolation checks pass.
5. Observe reconciliation lag, revocation, restoration, and failure metrics; keep activation disabled for any handoff without a current compatible verdict.
6. To roll back, disable dispatch and manual triggers, preserve all history, and continue fail-closed eligibility reads until a compatible implementation is restored.

## Open Questions

None. Retention intervals and batch-size defaults can follow existing maintenance configuration without changing the contract or task breakdown.

