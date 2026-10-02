## ADDED Requirements

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
