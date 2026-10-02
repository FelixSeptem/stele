## ADDED Requirements

### Requirement: Intent telemetry is bounded and redacted
Intent logs, metrics, and diagnostics MUST use fixed low-cardinality categories for type, lifecycle outcome, replay, queue, freshness, authorization, and rollback, and MUST exclude payloads, claims, scopes, identifiers, prompts, credentials, and raw errors.

#### Scenario: Intent lifecycle is emitted
- **WHEN** an intent is submitted, replayed, accepted, suppressed, failed, or rolled back
- **THEN** telemetry records only the bounded operation and outcome categories

#### Scenario: Sensitive intent data is supplied
- **WHEN** an intent contains content, evidence IDs, scope values, or raw provider error text
- **THEN** none of those values appear in exported telemetry or aggregate diagnostics
