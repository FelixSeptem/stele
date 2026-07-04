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

