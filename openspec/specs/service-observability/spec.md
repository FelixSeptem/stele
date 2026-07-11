# service-observability Specification

## Purpose
Define the baseline observability requirements for API, worker, and scheduler runtimes, including structured logs and operational signals.
## Requirements
### Requirement: Structured runtime logs
The service SHALL emit structured logs across `api`, `worker`, and `scheduler` modes so operators can correlate runtime events.

#### Scenario: Operator inspects a failing worker action
- **WHEN** a worker job fails or is retried
- **THEN** the emitted logs include enough stable identifiers and runtime context to correlate the failure with the affected job and mode

### Requirement: Operational metrics and tracing hooks
The service MUST expose baseline operational signals for API, governance, retrieval, and lifecycle workloads.

#### Scenario: Operator monitors service behavior over time
- **WHEN** the service handles ingest, governance, retrieval, or forgetting work
- **THEN** the runtime exposes metrics or tracing hook points for latency, throughput, error rate, and backlog-oriented inspection

#### Scenario: Operator monitors embedding lifecycle backlogs
- **WHEN** semantic projection work accumulates because embeddings are missing, stale, failed, or provider-mismatched
- **THEN** the runtime exposes backlog and execution telemetry for embedding rebuild eligibility, attempted generation, promotion outcomes, and provider or model drift processing

### Requirement: Actionable readiness and backlog diagnostics
The service MUST surface operational diagnostics that help explain degraded or unhealthy states.

#### Scenario: Backlog grows while dependencies remain reachable
- **WHEN** workers fall behind or maintenance work accumulates
- **THEN** operators can distinguish backlog pressure from simple dependency failure through the emitted diagnostics

### Requirement: Concrete liveness and readiness endpoints
The service SHALL expose concrete runtime liveness and readiness endpoints for self-hosted deployments.

#### Scenario: Runtime liveness is requested
- **WHEN** a caller requests the liveness endpoint
- **THEN** the service returns a result that confirms the process can respond without checking external provider dependencies

#### Scenario: Runtime readiness is requested
- **WHEN** a caller requests the readiness endpoint
- **THEN** the service evaluates readiness according to the active runtime mode and enabled execution dependencies

### Requirement: Embedding admission and rebuild metrics are exported
The service MUST expose concrete metrics for embedding cutover admission and embedding rebuild execution.

#### Scenario: Cutover admission is evaluated
- **WHEN** a cutover preflight or activation admission evaluation completes
- **THEN** the metrics surface records the decision and any blocker or warning codes without high-cardinality identifiers

#### Scenario: Embedding execution health is observed
- **WHEN** embedding rebuild, provider probe, or cutover wave dispatch activity is observed
- **THEN** the metrics surface records backlog, result, and dispatch signals needed to diagnose rollout progress and execution failure

### Requirement: Insight feedback metrics are exported
The service MUST expose operational metrics for derived insight quality feedback using low-cardinality labels.

#### Scenario: Feedback is recorded
- **WHEN** an insight feedback record is created or superseded
- **THEN** the metrics surface records the operation result by feedback type, insight type, and outcome without embedding tenant, project, namespace, insight id, actor, or reason text as metric labels

#### Scenario: Feedback-driven lifecycle decision occurs
- **WHEN** a background job suppresses, reviews, preserves, or prioritizes an insight based on feedback
- **THEN** the metrics surface records the decision category and result without high-cardinality identifiers

### Requirement: Insight feedback diagnostics are operator-visible
The service SHALL expose diagnostics that help operators understand derived insight quality trends.

#### Scenario: Operator inspects insight quality health
- **WHEN** an operator requests operational diagnostics for derived insights
- **THEN** the service can report feedback coverage, noisy insight rate, needs-review count, and feedback-driven suppression counts for an authorized scope

#### Scenario: Diagnostics include hidden insights
- **WHEN** suppressed or hidden insights contribute to quality diagnostics
- **THEN** the service includes aggregate counts without exposing hidden insight content through public metrics or non-admin diagnostics

### Requirement: Replay execution metrics are exported
The service MUST expose low-cardinality metrics for derived insight replay planning and execution.

#### Scenario: Replay dry-run completes
- **WHEN** a replay dry-run finishes
- **THEN** the metrics surface records the result, mode, insight type category, and decision categories without tenant, project, namespace, replay id, insight id, actor, or reason text labels

#### Scenario: Replay apply completes or fails
- **WHEN** replay apply work creates, updates, suppresses, preserves, skips, or fails insight decisions
- **THEN** the metrics surface records counters by outcome and reason category using low-cardinality labels

### Requirement: Smoke loop diagnostics are operator-visible
The service SHALL expose diagnostics that help operators determine which stage of the self-hosting smoke loop failed.

#### Scenario: Smoke loop detects degraded service
- **WHEN** a smoke check reports failure or degradation
- **THEN** operators can inspect readiness, job backlog, replay status, retrieval/context diagnostics, or metrics to identify the failed stage without direct PostgreSQL access

