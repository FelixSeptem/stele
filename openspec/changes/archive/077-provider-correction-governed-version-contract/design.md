## Context

See `proposal.md` for motivation. The Provider projection ledger already owns
immutable runtime-binding identities and ordered source revisions. The governed
intent ledger already owns exact-scope idempotency, policy decisions, durable
work and transition history. Canonical promotion already calculates the next
version under transaction and preserves prior versions. The missing connection
is a trusted, durable association from a correction revision to an update
intent's target lineage. Memory-intent processing also needs a transactionally
coupled queue handoff and completion boundary so a failed worker cannot leave
governance effects committed while intent and queue state disagree.

## Goals / Non-Goals

**Goals:**

- Add one opt-in, closed Provider correction-update operation and capability.
- Verify its source from PostgreSQL rather than caller-provided generic
  evidence, preserving exact scope, binding and lifecycle behavior.
- Persist enough immutable attribution for history, replay, worker rechecks and
  provenance without exposing correction payloads in public output.
- Exercise the complete correction-to-successor path on fresh PostgreSQL plus
  pgvector and across API/worker restart.
- Use Riverqueue for governed memory-intent scheduling and retries, while
  keeping the intent ledger authoritative and limiting the existing derived
  work queue to its other work kinds.

**Non-Goals:**

- Do not change generic Provider intent compatibility or make arbitrary
  `MemoryIntentEvidence` a privileged source type.
- Do not make a projection write a candidate, a canonical version or business
  state directly.
- Do not revoke or rewrite an already-promoted canonical successor merely
  because a later runtime correction arrives; a later governed correction is a
  new append-only request.

## Decisions

### Separate correction endpoint and capability

Add `POST /v1/provider/corrections` under
`provider-correction-governed-version-v1`, instead of extending the generic
Provider intent envelope. Its request contains canonical Provider metadata, a
single prior `projection_id`, target memory/version, bounded content and reason.
The response is a bounded normal intent acknowledgment plus a fixed attribution
state, never a projection payload or canonical receipt.

This keeps the new security condition explicit and lets strict consumers
negotiate it. Reusing generic `evidence.kind/id` would preserve backward
compatibility but cannot prove source identity. Automatically submitting an
update from a projection would violate PC6's evidence-only boundary.

### Resolver validates a current correction source and target lineage

The PostgreSQL resolver joins a correction revision to its logical identity and
the current runtime binding, requiring the exact binding identity, source
revision greater than one, predecessor linkage and no newer revision. It
re-evaluates the projection's durable reference selectors using existing PC5
reference rules, then requires one completed eligible lineage to name the
requested target memory at its current expected version. Foreign, missing,
wrong-binding, first-revision, superseded, hidden and mismatched records collapse
to bounded unavailable/stale results.

The target must also pass normal intent target validation. This prevents a
correction from widening retrieval or directing an update to an unrelated
memory. It deliberately does not expose raw correction payload, event IDs,
candidate IDs, version IDs or ranking data in public output.

### Immutable correction association on the intent ledger

Add an additive migration storing the verified projection ID, source revision
and bounded association state on `memory_intents`, with scope-aware indexes and
an append-only constraint/trigger consistent with existing intent history. The
association is included in the intent fingerprint and returned only to
authorized history/admin inspection. Candidate and promotion provenance carry
the internal association reference so the successor can be audited without
copying the correction payload.

Columns on the existing ledger are preferable to a second workflow table: the
intent remains the sole durable mutation request and existing worker/replay
semantics continue to apply.

### Recheck at worker execution

The worker invokes the same resolver before routing a correction-linked update.
If a newer correction revision, binding/grant revocation, hidden reference or
target version change occurs after admission, it records a bounded
suppressed/rejected transition rather than creating a candidate. This closes the
admission-to-worker gap while retaining an auditable request.

An already promoted successor is not automatically rewritten or hidden by a
newer source revision. Current feedback summaries already exclude superseded
projection-derived signals. A later content correction requires its own normal
correction-update request and successor version.

### River-backed memory-intent execution

Use `github.com/riverqueue/river v0.44.1` and the matching pgx v5 driver. This
release supports the repository's Go 1.25 baseline; newer inspected releases
require Go 1.26. The bounded River args contain only the `memory_intents.id`.
The memory-intent repository owns the acceptance transaction: it persists the
intent, immutable correction association and initial transition, then inserts
the River job through `InsertTx` before commit. Idempotent replay returns the
existing intent and must not create a second execution job.

The River worker loads and validates the intent under exact scope. Its
execution boundary holds one PostgreSQL transaction for source/target
revalidation, candidate and promotion/version/provenance effects, intent
transition and River `JobCompleteTx`. The app layer does not receive a raw
`pgx.Tx`; transaction ownership stays in the persistence adapter. Retryable
execution errors roll back all work in that transaction and are returned to
River so its attempt/retry policy remains authoritative. A source that is
stale at execution is a successful, bounded suppressed/rejected terminal
transition and completes in that same transaction. A committed
`JobCompleteTx` takes precedence if the worker subsequently returns an error.

Existing `derived_work_items` rows remain the durable record for other work
kinds and their public status APIs. The migration/handoff MUST enqueue each
eligible pending or recoverable `memory_intent` item into River once, preserve
its reference to the authoritative intent, and prevent the old claim worker
from processing that kind after cutover. Completed historical rows remain for
audit; no item may be executed by both workers. Do not backfill terminally
completed/exhausted work as runnable.

### Capability, errors and compatibility

Capability discovery advertises the correction contract and operation only when
the server is configured with the resolver. The existing projection and intent
operations retain their schemas and semantics. The new route uses the closed
Provider error envelope: malformed input is validation, unavailable source is
non-disclosing lifecycle/scope behavior, replay conflict is conflict, stale
source is stale, and unavailable storage/grant dependencies are retryable
dependency errors.

## Risks / Trade-offs

- [Resolver duplicates parts of PC5 eligibility] → Reuse existing reference
  status/lineage helpers and pin fresh integration fixtures to prevent drift.
- [A later correction arrives during processing] → Recheck source and target in
  the worker before routing; require explicit successor requests after promotion.
- [Two queue systems during cutover] → Route only memory intents to River,
  transactionally hand off recoverable legacy work, and retain the existing
  derived queue for unrelated kinds; enforce a single active consumer per
  intent.
- [More provider surface complexity] → Keep it one additive capability/route
  with strict request/response bounds and no fallback.
- [History could reveal sensitive identifiers] → Keep public responses to
  fixed states and IDs already owned by the caller; redact source lineage from
  non-admin reports and verifier artifacts.

## Migration Plan

1. Apply the additive Stele migration and the pinned River migrations in a
   dedicated River schema through its pgx v5 migration API.
2. Deploy API and worker replicas that understand the new association. During
   cutover, transactionally backfill recoverable legacy memory-intent work and
   ensure only the River consumer can claim that kind. Keep other derived-work
   consumers unchanged.
3. Keep the correction capability disabled/absent until fresh PostgreSQL plus
   pgvector conformance passes, including queue rollback and retry evidence.
4. Roll back by disabling the route/capability and River worker handling while
   preserving accepted intents, correction links, successors and audit
   history. Do not delete River or legacy queue history, or destroy correction
   evidence, as part of a production rollback.
