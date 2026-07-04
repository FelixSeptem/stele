# admin-inspection-surface Specification

## Purpose
Define the privileged inspection APIs that let operators review governed memory internals, history, and maintenance state without direct database access.
## Requirements
### Requirement: Admin inspection remains separate from public APIs
The service SHALL expose operational inspection surfaces through an admin-only route namespace and auth boundary separate from public product APIs.

#### Scenario: Operator accesses runtime diagnostics
- **WHEN** a caller requests admin inspection endpoints
- **THEN** the request is handled through an admin-specific surface rather than the standard public API contract

### Requirement: Job and backlog inspection
The service MUST support inspection of worker and scheduler execution state without requiring direct database access.

#### Scenario: Operator checks maintenance health
- **WHEN** an operator requests job or backlog state
- **THEN** the service can return current or recent status for job execution, retry state, queue or backlog pressure, and maintenance cadence health

#### Scenario: Operator filters governance raw events by recovery state
- **WHEN** an operator requests governance raw event inspection with filters such as scope, state, attempt range, failed time window, or next-attempt window
- **THEN** the admin surface returns only the matching raw events together with enough derived governance state to support remediation decisions

#### Scenario: Operator reads one governance raw event detail
- **WHEN** an operator requests a specific governance raw event within an authorized scope
- **THEN** the admin surface returns the raw event identity, derived governance state, attempt count, lease window, failure summary, next-attempt timing, and exhausted or processed timestamps when present

#### Scenario: Operator reads governance recovery history
- **WHEN** an operator requests recovery history for a specific governance raw event
- **THEN** the admin surface returns the recorded recovery actions, actor attribution, reason, and before or after recovery summaries without requiring direct database access

### Requirement: Memory history and lifecycle diagnostics
The service MUST support operator inspection of governed memory history and hidden lifecycle states without weakening public retrieval safety defaults.

#### Scenario: Operator investigates a hidden memory
- **WHEN** a memory was suppressed, forgotten, expired, or deleted
- **THEN** the admin surface can expose the relevant history, lifecycle state transitions, and provenance diagnostics while public retrieval remains lifecycle-safe by default

### Requirement: Embedding rebuild and vector lineage inspection
The service MUST support admin-only inspection of embedding rebuild state and vector revision lineage without requiring direct database access.

#### Scenario: Operator inspects one memory's semantic lineage
- **WHEN** an operator requests embedding inspection for a specific memory within an authorized scope
- **THEN** the admin surface returns the current rebuild state, requested target, active vector revision identity, and append-only revision history needed to diagnose semantic drift or failure

#### Scenario: Operator inspects rebuild backlog for a scope
- **WHEN** an operator requests embedding backlog inspection for an authorized scope
- **THEN** the admin surface returns rebuild records filtered by status, requested provider or model target, and failure or drift indicators so remediation decisions can be made without querying PostgreSQL directly

### Requirement: Embedding remediation actions remain bounded and auditable
The service MUST support narrowly scoped operator actions for retrying or requeueing eligible embedding rebuild work while preserving audit attribution and durable worker ownership rules.

#### Scenario: Operator retries a failed embedding rebuild
- **WHEN** an operator targets a failed and unleased embedding rebuild record with a retry action
- **THEN** the admin surface records actor and reason attribution, restores that record to ordinary rebuild eligibility, and does not mutate vector revision history directly

#### Scenario: Operator action is rejected for an actively leased rebuild
- **WHEN** an operator targets embedding rebuild work that is already under an active worker lease
- **THEN** the admin surface rejects the action rather than bypassing the durable background ownership contract

### Requirement: Embedding recovery history is queryable without direct database access
The service MUST support admin-only reads of embedding recovery history at both scope and memory granularity.

#### Scenario: Operator lists scope-level embedding recovery history
- **WHEN** an authorized operator requests embedding recovery history for a scope with optional filters such as action, actor, time window, or cutover plan id
- **THEN** the admin surface returns the matching recovery records with attribution and before or after snapshots without requiring direct PostgreSQL access

#### Scenario: Operator reads one memory's embedding recovery timeline
- **WHEN** an authorized operator requests embedding recovery history for a specific memory within an authorized scope
- **THEN** the admin surface returns the ordered retry and requeue history for that memory together with any linked cutover context

### Requirement: Embedding cutover plans are inspectable and controllable from the admin surface
The service MUST expose cutover plan inspection and bounded plan controls through the existing admin boundary.

#### Scenario: Operator lists active and recent cutover plans
- **WHEN** an authorized operator requests embedding cutover plans for a scope
- **THEN** the admin surface returns plan identity, target snapshot, rollout status, and aggregate progress needed to detect stalled or failed cutovers

#### Scenario: Operator pauses a cutover through the admin surface
- **WHEN** an authorized operator requests a pause or cancel action for an eligible cutover plan
- **THEN** the admin surface records actor and reason attribution and applies the bounded plan-state transition without taking over already rebuilding work

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
