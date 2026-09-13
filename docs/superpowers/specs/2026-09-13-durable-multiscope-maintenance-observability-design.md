# Durable Multi-Scope Maintenance and Retrieval Observability Design

## Goal

Close the P6 operational gap by making scope-bound maintenance durable,
restart-safe, freshness-aware, observable, and conformance-testable while
preserving PostgreSQL as Stele's only system of record and keeping retrieval
diagnostics internal.

## Scope and non-goals

This change reuses the existing `job_executions`, workflow/run, projection,
and assurance records. It adds only the fields, indexes, repository behavior,
and evaluator logic needed to represent stable maintenance execution and its
evidence. A separate `maintenance_*` ledger is not introduced.

The change does not add public OpenAPI endpoints, MCP adapters, a second
canonical store, autonomous reasoning, or changes to default retrieval ranking.
Canonical memories, raw events, and memory versions remain append-only and are
never overwritten by maintenance.

## Architecture

The scheduler discovers eligible scopes and derives a stable job identity from
the job name, exact tenant/project/namespace scope, and an idempotency window.
The existing job execution record is used to acquire, renew, reclaim, and
complete a lease. Retry and stale-recovery outcomes are represented as bounded
status/category fields and append-only execution history. A completed execution
is not repeated when its idempotency key has already succeeded.

Projection maintenance records source and projection watermarks, freshness
categories, rebuild/checkpoint state, and bounded duration/SLO outcomes. A
missing or stale watermark, foreign scope, hidden lifecycle record, or unsafe
rebuild fails closed for the derived projection and cannot influence default
retrieval.

Observability emits only low-cardinality categories and buckets: maintenance
job class, lease/retry/recovery outcome, projection freshness, retrieval
channel availability, candidate/expansion count, latency, and SLO result.
Queries, scope values, identifiers, hidden candidates, raw scores, provider
payloads, credentials, and unbounded plans are excluded. Internal diagnostic
records are retained as bounded derived artifacts and are not added to public
responses.

## Data flow

```text
scheduler tick
  -> discover eligible exact scopes
  -> derive stable job key and idempotency window
  -> acquire, renew, or reclaim lease
  -> run scope-bounded maintenance
       -> projection freshness/rebuild check
       -> retention and derived-artifact cleanup
       -> bounded maintenance/retrieval diagnostics
  -> record job execution and evidence
  -> emit low-cardinality telemetry
  -> run conformance evaluation over recorded evidence
```

On restart, an unexpired lease remains owned by its worker, an expired lease
may be reclaimed only after the stale check, and an interrupted job resumes
from its checkpoint or source watermark. Canonical source records remain
untouched when a run fails. Retention removes expired derived artifacts only
and records a bounded deletion result.

## Failure handling and safety gates

The following conditions produce stable, fail-closed categories:

- lease owner conflict, unsafe stale reclaim, or duplicate-fire ambiguity;
- retry budget exhaustion or an incomplete restart checkpoint;
- missing or stale projection watermark;
- tenant, project, namespace, or lifecycle leakage;
- retention targeting canonical source records;
- high-cardinality or sensitive observability fields;
- incomplete conformance evidence.

An individual successful job, provider transient, or aggregate latency result
cannot override a safety failure. A scope's derived projection remains
ineligible for default retrieval until freshness, isolation, lifecycle, and
conformance gates pass.

## Implementation units

1. **Durable execution:** stable job identity, duplicate-fire prevention,
   lease acquire/renew/reclaim, bounded retry/backoff, restart-safe resume, and
   paginated execution history.
2. **Projection freshness and SLO:** source/projection watermark checks,
   freshness and rebuild outcomes, latency/SLO buckets, and fail-closed
   projection eligibility.
3. **Internal observability:** redacted low-cardinality maintenance and
   retrieval telemetry plus bounded diagnostic retention.
4. **Assurance closure:** a repeatable conformance evaluator covering scope
   coverage, lease recovery, projection rebuild, retention safety, telemetry
   redaction, and evidence completeness.

## Testing strategy

Use TDD at each unit. Add job tests for stable keys, duplicate fire, lease
renew/reclaim, retry/backoff, and restart resume; storage tests for SQL scope
predicates, cursor pagination, concurrent claims, and idempotent migrations;
projection tests for watermark/freshness/rebuild and hidden/foreign fail-close;
telemetry tests for redaction and bounded cardinality; retention tests proving
canonical records survive; and assurance tests for coverage, recovery, SLO,
and evidence integrity. Run the full Go suite, strict OpenSpec validation,
diff checks, and (when available) a PostgreSQL 18 + pgvector maintenance smoke.

## Rollout and rollback

The new maintenance behavior is opt-in behind existing scheduler configuration
until its conformance report is green. If any required gate fails, the
operator disables the new path and the scheduler continues with the previously
approved maintenance implementation. Rollback does not rewrite canonical
records; it only stops new derived maintenance artifacts and leaves bounded
history for inspection and cleanup.
