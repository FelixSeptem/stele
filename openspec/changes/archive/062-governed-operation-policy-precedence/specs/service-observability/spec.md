## MODIFIED Requirements

### Requirement: Intent telemetry is bounded and redacted

Intent logs, metrics, and diagnostics MUST use fixed low-cardinality categories for precedence stage, type, lifecycle outcome, replay, queue, freshness, authorization, policy enablement, and rollback, and MUST exclude payloads, claims, scopes, identifiers, prompts, credentials, and raw errors. The same bounded categories MUST be usable by provider, insight, lifecycle, and manual mutation conformance reports without exposing hidden-record existence.

#### Scenario: Intent lifecycle is emitted

- **WHEN** an intent is submitted, replayed, accepted, suppressed, failed, rolled back, or stopped at a precedence stage
- **THEN** telemetry records only the bounded type, precedence stage, lifecycle outcome, replay, authorization, policy, queue, freshness, and rollback categories

#### Scenario: Sensitive intent data is supplied

- **WHEN** an intent contains content, evidence IDs, scope values, target identifiers, or raw provider error text
- **THEN** none of those values appear in exported telemetry or aggregate diagnostics

#### Scenario: Precedence denial is observable without disclosure

- **WHEN** an operation stops at scope, lifecycle, grant, policy, replay, or handoff evaluation
- **THEN** telemetry records the fixed stage and bounded outcome category without tenant/project/namespace values, payloads, target identifiers, or raw errors

#### Scenario: Authorized diagnostics summarize conformance

- **WHEN** an authorized operator inspects a precedence conformance run for one scope
- **THEN** the response contains bounded stage counters, replay/conflict counts, rollback state, and contract versions without hidden content or foreign-scope identifiers
