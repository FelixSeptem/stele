# Design: Governed Memory Intents, Reflection Runs, and Compaction Evidence

## Goals

1. Make every externally requested memory mutation explicit, scoped, idempotent, attributable, and auditable.
2. Make reflection a durable, restart-safe, replay-safe derived computation.
3. Make compaction a traceable state transition retaining enough evidence to rebuild and explain projections.
4. Feed only policy-eligible, lifecycle-safe derived data into existing context projections.

## Architecture

The flow is:

```text
request/session event
  -> intent ledger
  -> candidate/governance workflow
  -> durable reflection run
  -> candidate + evidence
  -> review decision
  -> canonical version (existing governed path)
  -> compaction evidence
  -> context projection
```

PostgreSQL remains the sole system of record. New records are append-only except for explicitly modeled status/lease transitions. Canonical memory is never overwritten in place.

### Memory intents

Each intent contains a stable intent ID, intent type, project/tenant/namespace, actor, reason, provenance, request ID, operation ID, idempotency key, target references, payload, and validation result. The unique scope of `(scope, idempotency_key)` returns the original result for retries. Validation of scope, lifecycle, and target version precedes business processing. Accepted intents enqueue governance work; they do not mutate canonical memory directly.

### Reflection runs

Runs are created by session completion, event/turn thresholds, compaction pressure, schedules, or operator request. A run stores trigger metadata, input watermark, transcript schema version, processed offset, lease owner/expiry, attempt count, retry budget, status, failure category, output candidate IDs, and evidence references. Workers claim leases transactionally. Checkpoints advance only after durable output writes. Replays use the same input watermark and schema version and are deduplicated by stable run identity.

### Review

Review decisions are append-only and reference the candidate, run, reviewer actor, decision (`accept`, `suppress`, `reject`, `request_more_evidence`), reason, policy version, and timestamp. Accepting a candidate invokes the existing versioned canonical-memory path; all other outcomes preserve the candidate and audit trail without making it retrievable by default.

### Compaction evidence

Every compaction record binds a source session/conversation to source watermark and raw-event range, canonical memory version references, derivation/summarizer version, input/output token estimates, evidence coverage, bounded recent-tail references, summary version, state (`active`, `superseded`, `stale`, `failed`), and any follow-up reflection run. Rebuilds create new derived records and never alter source canonical records.

### Projection integration

Existing projection policy remains authoritative. Derived summaries/candidates are eligible only when scope, lifecycle, review state, and policy checks pass. Projection results may include bounded, redacted evidence references and diagnostics. Missing, stale, foreign-scope, suppressed, forgotten, or deleted sources fail closed.

## Error Handling and Recovery

- Duplicate idempotency requests return the first durable result.
- Lease expiry permits another worker to resume from the last committed offset.
- Retryable failures use bounded categories and retry budgets; terminal failures remain queryable for operations.
- Scope or lifecycle violations are rejected before side effects.
- Incomplete evidence prevents default projection and emits a bounded diagnostic code.

## API and Operational Contracts

Expose OpenAPI-first internal/admin endpoints for intent submission/status, reflection run status/replay, candidate review, and compaction evidence inspection. Endpoints require explicit scope and authorization context. List endpoints are paginated and redact payloads according to existing diagnostics policy.

## Testing Strategy

- Unit tests for intent validation, idempotency, state transitions, and evidence coverage.
- Repository tests for scope predicates, version references, leases, and append-only audit rows.
- Worker tests for checkpoint/restart, duplicate fire, retry budget, and replay determinism.
- Integration tests for review-to-canonical flow and projection eligibility.
- Conformance tests for isolation, lifecycle filtering, bounded assembly, recovery, and rebuild without canonical mutation.
