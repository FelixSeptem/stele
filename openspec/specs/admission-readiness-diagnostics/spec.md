# admission-readiness-diagnostics Specification

## Purpose
Define reusable, structured readiness decisions for safely admitting governed operations.
## Requirements
### Requirement: Admission evaluations produce structured decisions
The service SHALL define a reusable admission evaluation contract that returns a decision, blocker findings, warning findings, observed time, and optional component-specific summary data.

#### Scenario: Evaluation blocks an unsafe operation
- **WHEN** an admission evaluator detects one or more hard blockers
- **THEN** the resulting decision is `deny` and the response includes stable blocker codes suitable for API responses, logs, and metrics

#### Scenario: Evaluation allows an operation with warnings
- **WHEN** an admission evaluator detects no hard blockers but does detect non-blocking risk
- **THEN** the resulting decision is `allow` and the response includes warning codes without preventing the requested operation

### Requirement: Readiness checks are mode-aware
The service MUST evaluate runtime readiness according to the active runtime mode and the dependencies needed by that mode.

#### Scenario: API mode readiness omits embedding provider reachability
- **WHEN** the service runs in `api` mode and readiness is requested
- **THEN** the readiness result checks service runtime and PostgreSQL dependencies without requiring embedding provider reachability

#### Scenario: Worker or scheduler readiness includes enabled embedding execution dependencies
- **WHEN** the service runs in `worker` or `scheduler` mode with embedding rebuild or cutover execution enabled
- **THEN** the readiness result includes embedding provider reachability in addition to service runtime and PostgreSQL dependencies

### Requirement: Diagnostic findings are metrics-safe
The service MUST represent diagnostic findings with stable low-cardinality codes and component labels.

#### Scenario: Finding is exported to metrics
- **WHEN** an admission, readiness, or embedding execution finding is recorded for metrics
- **THEN** the exported labels avoid high-cardinality values such as memory ids, raw event ids, cutover plan ids, or free-form error messages

### Requirement: Admission pressure decisions cover ingestion and repair work
The service SHALL extend admission evaluation diagnostics to classify ingestion and repair work pressure with stable decisions and finding codes.

#### Scenario: Ingestion is accepted under normal pressure
- **WHEN** ingestion dependencies and scoped backlog are within configured limits
- **THEN** the admission evaluation returns `accept` with no blocker findings

#### Scenario: Ingestion is accepted with degradation
- **WHEN** the service can durably persist the event but semantic projection, governance processing, or downstream maintenance is degraded
- **THEN** the admission evaluation returns `accept_degraded` with warning finding codes that explain the degraded component

#### Scenario: Ingestion should be queued
- **WHEN** the service can durably preserve intent but immediate downstream work should be delayed because scoped pressure exceeds configured limits
- **THEN** the admission evaluation returns `queue` with stable pressure finding codes

#### Scenario: Ingestion or repair is rejected
- **WHEN** the service cannot safely persist intent, cannot resolve scope, or would violate configured safety limits
- **THEN** the admission evaluation returns `reject` with blocker finding codes before creating new work

### Requirement: Repair admission uses the same diagnostics contract
The service MUST evaluate repair plan creation and repair action dispatch with the same structured admission contract used for ingestion pressure.

#### Scenario: Repair plan is accepted
- **WHEN** a repair plan can be created within scope and within configured action limits
- **THEN** the admission result records an `accept` or `accept_degraded` decision and stable finding codes

#### Scenario: Repair dispatch is delayed by worker pressure
- **WHEN** repair execution would exceed scoped worker backlog, lease, or dependency limits
- **THEN** the admission result records `queue` rather than bypassing durable worker execution

#### Scenario: Repair dispatch exceeds safety limits
- **WHEN** a repair action would exceed configured scope, cardinality, action category, or dependency safety limits
- **THEN** the admission result records `reject` and the action is not dispatched
