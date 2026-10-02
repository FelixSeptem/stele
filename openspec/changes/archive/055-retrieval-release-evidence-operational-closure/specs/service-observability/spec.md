## ADDED Requirements

### Requirement: Release-evidence operational observability is bounded

The service SHALL expose redacted operator summaries, low-cardinality metrics,
and bounded structured logs for evaluation preflight, run lifecycle, timeout,
cleanup, freshness/attestation checks, activation disablement, and rollback.
The fields and labels MUST use fixed categories or buckets and MUST exclude
DSNs, credentials, queries, prompts, source content, provider payloads,
tenant/project/namespace values, memory/event/report identifiers, and reason
text.

#### Scenario: Preflight or run completes
- **WHEN** an evaluation is accepted, skipped, degraded, timed out, or fails
- **THEN** telemetry records bounded operation, result, prerequisite category,
  freshness status, and duration bucket without high-cardinality data

#### Scenario: Cleanup or attestation completes
- **WHEN** artifacts are retained, removed, rejected, or an attestation is
  accepted or mismatched
- **THEN** telemetry records bounded cleanup/attestation categories and the
  operator summary identifies whether the evidence is consumable

#### Scenario: Disablement or rollback changes state
- **WHEN** activation is disabled, rollback starts, rollback succeeds, or
  rollback fails
- **THEN** telemetry records bounded surface, operation, result, policy-status,
  and rollback category fields without candidate IDs or scope values

#### Scenario: Sensitive label is supplied
- **WHEN** instrumentation receives a raw DSN, scope, query, identifier, or
  reason string
- **THEN** it rejects, redacts, or buckets the value before emission and
  preserves the low-cardinality contract
