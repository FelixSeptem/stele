## ADDED Requirements

### Requirement: Scheduler run lifecycle telemetry is bounded

The service SHALL emit low-cardinality metrics and bounded structured logs for
scheduler dispatch, lease acquisition/renewal/reclaim, retry/backoff,
duplicate-fire, checkpoint resume, terminal disposition, retention cleanup, and
admin inspection. Labels and fields MUST use fixed categories or buckets and
MUST exclude scope values, run/attempt identifiers, raw errors, credentials,
source content, and reason text.

#### Scenario: Scheduler run changes state
- **WHEN** a run is dispatched, leased, renewed, reclaimed, retried, skipped,
  exhausted, cancelled, completed, or marked duplicate
- **THEN** telemetry records bounded operation, result, state, lease/retry,
  recovery, and duration categories without high-cardinality data

#### Scenario: Run-history cleanup completes
- **WHEN** derived attempt details are retained, deleted, skipped, or fail
  cleanup
- **THEN** telemetry records bounded record, result, expiration, and deletion
  categories while preserving terminal audit evidence

#### Scenario: Sensitive telemetry input is supplied
- **WHEN** instrumentation receives a scope value, raw identifier, error text,
  or reason string
- **THEN** it rejects, redacts, or buckets the value before emission
