## ADDED Requirements

### Requirement: Assurance jobs use durable worker orchestration
The service SHALL execute periodic health evaluations, incident refresh, alert candidate generation, and recovery verification through durable scheduler or worker orchestration.

#### Scenario: Scheduler dispatches health evaluation
- **WHEN** the configured assurance cadence is reached for an eligible scope
- **THEN** the scheduler can dispatch a bounded health evaluation job without requiring traffic on the public API

#### Scenario: Scheduler dispatches operational proof checks
- **WHEN** configured capacity/load or backup/restore proof freshness windows require evaluation for an eligible scope
- **THEN** the scheduler can dispatch bounded proof checks or proof freshness evaluation without executing external backup tooling or unbounded load generation

#### Scenario: Worker processes alert delivery
- **WHEN** an alert candidate is eligible for delivery
- **THEN** a worker can claim the delivery attempt with durable ownership, record result, and retry later without duplicating successful deliveries

#### Scenario: Assurance job fails retryably
- **WHEN** health evaluation, alert delivery, incident refresh, or recovery verification fails before completion
- **THEN** the service records attempt count, failure category, next eligibility, and bounded error summary

#### Scenario: Scheduler dispatches assurance cleanup
- **WHEN** configured assurance or conformance history retention windows have elapsed
- **THEN** the scheduler dispatches cleanup work that removes eligible high-volume records while preserving incident records and incident transition audit history

### Requirement: Conformance jobs use durable worker orchestration
The service SHALL execute scheduled conformance runs and stale integration checks through durable scheduler or worker orchestration.

#### Scenario: Scheduler dispatches conformance run
- **WHEN** a conformance profile is active and its cadence window arrives
- **THEN** the scheduler can dispatch a scoped conformance run using the durable job model

#### Scenario: Duplicate conformance dispatch occurs
- **WHEN** the same profile and cadence window are dispatched more than once across retries or restarts
- **THEN** the service resumes or skips idempotently without creating duplicate active diagnostics

#### Scenario: Conformance job reaches bounds
- **WHEN** conformance processing reaches configured evidence, time, or diagnostic limits
- **THEN** the service records bounded degraded or continuation-required status instead of scanning beyond limits
