## ADDED Requirements

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
