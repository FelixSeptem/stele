## Why

Stele's maintenance foundation now has scope-aware jobs, leases, projections,
and assurance records, but it does not yet provide one durable, restart-safe,
and auditable contract proving that every durable scope is maintained. Without
that closure, stale projections, duplicate scheduler fires, lease recovery,
and observability regressions can remain invisible until they affect retrieval.
P6 closes this operational gap before the agent-runtime provider adapter is
introduced.

## What Changes

- Add stable, scope-bound maintenance job identity and idempotency windows on
  existing job execution records.
- Extend lease renewal, stale reclaim, bounded retry/backoff, checkpoint resume,
  and paginated run-history behavior for scheduler and worker maintenance.
- Record projection source/projection watermarks, freshness categories,
  rebuild state, latency/SLO buckets, and fail-closed eligibility outcomes.
- Emit internal maintenance and retrieval diagnostics using only low-cardinality
  categories and bounded buckets; exclude queries, scope values, identifiers,
  raw scores, provider payloads, and credentials.
- Add a repeatable assurance/conformance evaluation for maintenance coverage,
  recovery, projection rebuild, retention safety, telemetry redaction, and
  evidence completeness.
- Add bounded retention and cleanup handling for derived maintenance and
  diagnostic artifacts while preserving canonical source and incident audit
  history.
- Document operator evidence, rollback, and PostgreSQL 18 + pgvector smoke
  prerequisites.

## Non-goals

- No second maintenance ledger or non-PostgreSQL system of record.
- No public OpenAPI endpoint, MCP adapter, UI, SDK, or provider-runtime work.
- No change to default retrieval ranking or canonical-memory mutation rules.
- No autonomous reasoning, new memory classes, or cross-scope namespace
  expansion.

## Capabilities

### New Capabilities

- `durable-multiscope-maintenance-observability`: durable maintenance execution,
  projection freshness/SLO evidence, internal redacted observability, and
  conformance closure.

### Modified Capabilities

- `worker-orchestration-and-maintenance-jobs`: add durable maintenance identity,
  duplicate-fire protection, stale lease recovery, bounded retry, restart
  resume, and run-history guarantees.
- `service-observability`: add bounded maintenance/projection freshness/SLO
  metrics and redaction/cardinality guarantees.
- `self-hosted-assurance-and-conformance`: add maintenance coverage and recovery
  evidence to conformance and readiness outcomes.
- `versioned-context-projections`: add maintenance-owned freshness/rebuild
  eligibility evidence and retention safety expectations.

## Impact

Affected areas are `internal/jobs`, `internal/storage/postgres`,
`internal/workflow`, `internal/retrieval`, `internal/telemetry`,
`internal/assurance`, migration SQL, scheduler configuration, and operator
documentation. Existing public API contracts remain unchanged. The change is
backward-compatible when the new maintenance path is disabled; failed gates
fall back to the previously approved scheduler behavior.
