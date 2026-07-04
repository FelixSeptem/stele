## ADDED Requirements

### Requirement: Embedding cutover preflight is available through the admin surface
The service MUST expose cutover preflight evaluation through the existing admin-only boundary.

#### Scenario: Operator requests cutover preflight
- **WHEN** an authorized operator requests preflight for a cutover plan within an authorized scope
- **THEN** the admin surface returns the structured admission report without activating the plan or scheduling rollout work

#### Scenario: Activation is rejected by admission
- **WHEN** an authorized operator activates a cutover plan and admission denies the request
- **THEN** the admin surface returns the blocker report using the same response shape as preflight

### Requirement: Runtime health endpoints are inspectable without weakening admin boundaries
The service SHALL expose liveness and readiness endpoints for runtime orchestration while keeping privileged memory and cutover inspection under admin routes.

#### Scenario: Orchestrator reads liveness
- **WHEN** a runtime orchestrator requests liveness
- **THEN** the service returns whether the process is alive without exposing privileged memory or cutover details

#### Scenario: Orchestrator reads readiness
- **WHEN** a runtime orchestrator requests readiness
- **THEN** the service returns readiness according to runtime mode without exposing privileged memory or cutover details

### Requirement: Runtime metrics are exposed for embedding rollout operation
The service MUST expose metrics suitable for scraping embedding cutover admission and rebuild execution health.

#### Scenario: Operator scrapes metrics
- **WHEN** an operator or monitoring system requests metrics
- **THEN** the response includes embedding admission, cutover state, rebuild backlog, provider readiness, and scheduler dispatch signals using low-cardinality labels
