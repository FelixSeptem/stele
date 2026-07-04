## ADDED Requirements

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
