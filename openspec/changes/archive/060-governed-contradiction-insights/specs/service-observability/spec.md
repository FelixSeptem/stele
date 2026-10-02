## ADDED Requirements

### Requirement: Contradiction observability is bounded and redacted

The service SHALL expose low-cardinality contradiction operation, temporal
disposition, evidence eligibility, review, activation, rollback, and freshness
categories through authorized diagnostics and reasoning telemetry. Metrics and
logs MUST exclude fact content, scope values, source or candidate identifiers,
prompts, provider payloads, and raw error text.

#### Scenario: Contradiction run completes

- **WHEN** offline, shadow, replay, review, or activation processing completes, skips, degrades, or fails
- **THEN** telemetry records bounded operation, mode, result, temporal category, review category, freshness, and duration buckets

#### Scenario: Diagnostic includes hidden evidence

- **WHEN** hidden, foreign, stale, or redacted evidence affects contradiction evaluation
- **THEN** authorized diagnostics expose only aggregate counts and stable reason categories
