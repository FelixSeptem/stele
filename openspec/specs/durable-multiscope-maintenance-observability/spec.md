# durable-multiscope-maintenance-observability Specification

## Purpose
This capability makes multi-scope maintenance durable, restart-safe, freshness-aware,
and auditable while keeping diagnostics internal and derived from PostgreSQL records.

## Requirements

### Requirement: Maintenance execution is durable and exactly scoped

The service SHALL derive a stable maintenance identity from job class, exact
tenant, project, namespace, and cadence/idempotency window, and SHALL execute
the job only within that scope. A successful execution with the same identity
MUST suppress duplicate scheduler fires while preserving an inspectable bounded
run-history record.

#### Scenario: Duplicate scheduler fire

- **WHEN** two scheduler ticks submit the same maintenance identity for the same scope and cadence window
- **THEN** only one execution acquires the lease and the other records a duplicate-fire disposition without running the job

#### Scenario: Foreign scope is discovered

- **WHEN** a maintenance request contains a scope that differs from the discovered exact scope
- **THEN** the request fails closed, records a scope-isolation category, and does not read or write records outside the discovered scope

### Requirement: Lease recovery and retry are restart-safe

The service SHALL support lease acquisition, renewal, stale-owner reclamation,
bounded retry/backoff, and checkpoint or source-watermark resume. A stale lease
MUST NOT be reclaimed before its expiry, and a retry-exhausted job MUST remain
inspectable without being silently re-enqueued.

#### Scenario: Worker restarts during maintenance

- **WHEN** a worker stops after recording a checkpoint and its lease expires
- **THEN** a later worker safely reclaims the stale lease and resumes from the checkpoint without rewriting canonical records

#### Scenario: Lease renewal conflicts

- **WHEN** a worker attempts to renew a lease owned by another worker or already completed
- **THEN** renewal fails with a bounded lease-conflict category and the worker stops mutating derived records

### Requirement: Projection freshness and SLO eligibility fail closed

The service SHALL record source watermark, projection watermark, freshness
category, rebuild/checkpoint state, bounded duration/latency SLO buckets, and
scope/lifecycle validation for each maintained projection. Missing, stale,
divergent, foreign, or lifecycle-hidden evidence MUST make that derived
projection ineligible for default retrieval.

#### Scenario: Fresh projection passes eligibility

- **WHEN** source and projection watermarks match within the configured freshness window and all evidence is exact-scope and lifecycle-visible
- **THEN** the service records a fresh SLO outcome and allows the projection to remain eligible for default retrieval

#### Scenario: Projection is stale or divergent

- **WHEN** a projection watermark is missing, stale, or does not match the source watermark
- **THEN** the service records a stable freshness failure and excludes the projection from default retrieval until rebuilt and revalidated

### Requirement: Observability is redacted and low cardinality

The service SHALL emit internal maintenance and retrieval diagnostics using
bounded categories and buckets for job class, lease/retry/recovery outcome,
freshness, channel availability, candidate/expansion count, latency, and SLO.
Diagnostics MUST exclude query text, scope values, memory/event identifiers,
hidden candidates, raw scores, provider payloads, credentials, and unbounded
plans.

#### Scenario: Diagnostic event is emitted

- **WHEN** a maintenance or retrieval operation completes
- **THEN** the emitted diagnostic contains only the allowed bounded categories and bucket values

#### Scenario: Sensitive diagnostic field is attempted

- **WHEN** an operation would emit query text, a scope value, an identifier, a raw score, or provider payload
- **THEN** the field is rejected or redacted before persistence and the event remains bounded

### Requirement: Conformance evidence covers maintenance closure

The service SHALL run a repeatable conformance evaluation that verifies scope
coverage, duplicate-fire behavior, lease recovery, projection rebuild and
freshness, retention safety, telemetry redaction/cardinality, and evidence
completeness. Any hard safety failure MUST fail the conformance result even if
aggregate execution success is positive.

#### Scenario: Conformance run passes

- **WHEN** every discovered durable scope has fresh projection evidence, recoverable leases, safe retention, and compliant diagnostics
- **THEN** the service records a passing conformance result with bounded evidence identities and timestamps

#### Scenario: Conformance run has a safety failure

- **WHEN** any scope leaks, canonical record is targeted by retention, evidence is incomplete, or telemetry is sensitive/high-cardinality
- **THEN** the service records a stable failure category and does not report the maintenance closure as ready

### Requirement: Derived maintenance artifacts have bounded retention

The service SHALL apply deterministic retention to maintenance diagnostics,
execution evidence, and freshness artifacts while preserving canonical source
records and incident audit history. Cleanup MUST record bounded deletion
outcomes and MUST be idempotent.

#### Scenario: Derived artifact expires

- **WHEN** a derived diagnostic or maintenance evidence record is outside its configured retention window
- **THEN** cleanup removes only that derived artifact and records the deletion outcome without deleting canonical source data

#### Scenario: Cleanup repeats after restart

- **WHEN** the same cleanup window is processed more than once
- **THEN** the second run is a no-op or bounded duplicate disposition and leaves surviving records unchanged

### Requirement: Maintenance run history has bounded retention semantics

The service SHALL retain terminal run summaries, attempt counts, duplicate and
recovery dispositions, checkpoint/watermark identities, and cleanup outcomes
for a configured bounded history window. Retention MUST remove only derived
attempt/detail records that are outside policy while preserving the latest
terminal summary and required audit transitions.

#### Scenario: Old attempt detail expires
- **WHEN** an attempt detail record exceeds its configured retention window
- **THEN** cleanup removes only the eligible derived detail, records a bounded
  deletion outcome, and retains the terminal run summary

#### Scenario: Cleanup repeats after restart
- **WHEN** the same run-history retention window is processed more than once
- **THEN** subsequent cleanup is a no-op or duplicate disposition and leaves
  surviving summaries unchanged

### Requirement: Maintenance run history is exact-scope and paginated

Authorized inspection SHALL resolve run history within one exact
tenant/project/namespace scope and SHALL provide bounded pagination and stable
filters for job class, terminal state, recovery disposition, cadence window,
and observed time. Missing or unauthorized scope proof MUST fail before any
run existence or count is disclosed.

#### Scenario: Operator pages run history
- **WHEN** an authorized operator requests a bounded page for one exact scope
- **THEN** the service returns stable ordering, an opaque continuation cursor,
  bounded run summaries, and no records from another scope

#### Scenario: Operator requests a foreign scope
- **WHEN** the caller lacks a grant for the requested tenant, project, or
  namespace
- **THEN** the service rejects the request without revealing run existence,
  counts, identifiers, or failure details

### Requirement: Recovery evidence remains freshness-aware

Each retained terminal summary SHALL expose bounded checkpoint/source-watermark
identity, freshness category, retry/recovery result, and SLO bucket. Missing,
stale, divergent, foreign, or lifecycle-hidden evidence MUST make the summary
non-eligible for conformance claims until a new run validates it.

#### Scenario: Recovery summary is fresh
- **WHEN** a terminal run has a matching source watermark and valid checkpoint
  recovery within the configured freshness window
- **THEN** the summary records a fresh recovery/SLO outcome suitable for
  authorized conformance inspection

#### Scenario: Recovery summary is stale
- **WHEN** a retained summary is outside its freshness window or its watermark
  diverges from the source
- **THEN** the service marks it stale/divergent and excludes it from readiness
  or conformance success

### Requirement: Derived queue health is freshness-aware

Maintenance observability SHALL include bounded queue mode, depth, lag, flush,
drop, retry, exhaustion, watermark freshness, and SLO categories for each exact
scope without exposing scope values or payloads.

#### Scenario: Durable queue falls behind

- **WHEN** queue lag exceeds the configured freshness budget
- **THEN** the service reports a bounded degraded SLO and affected derived evidence cannot satisfy readiness claims

#### Scenario: Memory buffer loses work

- **WHEN** buffered derived work is dropped before PostgreSQL flush
- **THEN** the service records bounded loss evidence and marks affected freshness claims ineligible

### Requirement: Derived work cleanup is restart-safe

Queue retention and audit cleanup SHALL be idempotent across worker and scheduler
restart and SHALL preserve terminal evidence required for recovery review.

#### Scenario: Cleanup runs twice

- **WHEN** the same derived queue retention window is processed repeatedly
- **THEN** later runs are no-op or duplicate dispositions with stable surviving summaries
