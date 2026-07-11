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

### Requirement: Derived insights are inspectable through the admin surface
The service MUST expose admin-only inspection for derived experience insights and their evidence context.

#### Scenario: Operator lists derived insights
- **WHEN** an authorized operator lists derived insights for a scope with optional type or lifecycle filters
- **THEN** the admin surface returns matching insight summaries with type, lifecycle state, confidence, evidence count, and derivation metadata

#### Scenario: Operator reads one derived insight
- **WHEN** an authorized operator reads one derived insight within an authorized scope
- **THEN** the admin surface returns the insight detail, evidence references, provenance, lifecycle history, and lesson output when present

#### Scenario: Operator inspects hidden insight
- **WHEN** an insight is suppressed or otherwise hidden from default context assembly
- **THEN** the admin surface can still expose the insight's lifecycle state and evidence context without making it visible to public retrieval or context assembly

### Requirement: Derived insight lifecycle actions are bounded and auditable
The service SHALL support narrowly scoped admin lifecycle actions for derived insights.

#### Scenario: Operator suppresses a noisy insight
- **WHEN** an authorized operator suppresses an active derived insight with actor and reason attribution
- **THEN** the service records the lifecycle transition and excludes that insight from default context assembly

#### Scenario: Operator action preserves evidence history
- **WHEN** an operator changes a derived insight lifecycle state
- **THEN** the service preserves the linked evidence and records the action in audit history

### Requirement: Derived insight feedback is manageable through the admin surface
The service SHALL expose admin-only operations for recording, listing, reading, and superseding quality feedback for derived insights.

#### Scenario: Operator records insight feedback
- **WHEN** an authorized operator submits feedback for a derived insight with actor and reason attribution
- **THEN** the admin surface creates a scoped feedback record and returns its durable identity

#### Scenario: Operator lists feedback for an insight
- **WHEN** an authorized operator requests feedback for a derived insight within an authorized scope
- **THEN** the admin surface returns the matching feedback records, including supersession state and audit attribution

#### Scenario: Operator supersedes feedback
- **WHEN** an authorized operator supersedes a prior feedback record with a reason
- **THEN** the admin surface records the supersession and excludes that record from active quality summaries

### Requirement: Admin insight inspection includes quality state
The service MUST include effective quality feedback state in admin derived insight inspection without weakening public visibility rules.

#### Scenario: Operator reads one derived insight with feedback
- **WHEN** an authorized operator reads a derived insight that has quality feedback
- **THEN** the admin surface returns the insight detail together with feedback summary, active review signals, and links or identifiers for feedback history

#### Scenario: Hidden insight has feedback history
- **WHEN** an operator inspects a suppressed or hidden insight with feedback
- **THEN** the admin surface can show feedback and lifecycle context without making the insight visible to public retrieval or context assembly

### Requirement: Derived insight replay is controllable from the admin surface
The service SHALL expose admin-only operations for replay dry-run planning, replay apply enqueueing, replay run inspection, and replay report reads.

#### Scenario: Operator requests replay dry-run
- **WHEN** an authorized operator submits a bounded replay dry-run request
- **THEN** the admin surface returns a replay plan and does not schedule mutation work

#### Scenario: Operator requests replay apply
- **WHEN** an authorized operator submits a bounded replay apply request with actor and reason attribution
- **THEN** the admin surface creates or returns a durable replay run identity and exposes where to inspect its status

#### Scenario: Operator reads replay report
- **WHEN** an authorized operator requests a replay run or report within the authorized scope
- **THEN** the admin surface returns status, request bounds, counters, failures, skip reasons, actor attribution, and linked insight identifiers when permitted

### Requirement: Replay admin controls preserve lifecycle and scope safety
The service MUST reject replay admin requests that bypass scope isolation, lifecycle governance, or durable background ownership.

#### Scenario: Replay targets unauthorized scope
- **WHEN** a caller requests replay for a tenant, project, or namespace outside its admin authorization
- **THEN** the admin surface rejects the request without exposing evidence or insight counts from that scope

#### Scenario: Replay tries to mutate hidden content directly
- **WHEN** a replay request attempts to make suppressed, forgotten, deleted, or out-of-scope insight content visible without governed lifecycle evaluation
- **THEN** the admin surface rejects the request or records a skipped decision rather than bypassing lifecycle controls

### Requirement: Scope proof reports are inspectable through the admin surface
The service SHALL expose admin-only inspection of scope proof runs and reports.

#### Scenario: Administrator lists proof runs
- **WHEN** an authorized administrator lists proof runs for a scope
- **THEN** the admin surface returns matching proof runs with status, verdict, timestamps, actor attribution, and bounded summary counts

#### Scenario: Administrator reads proof report
- **WHEN** an authorized administrator reads a proof report within scope
- **THEN** the admin surface returns step evidence, failure categories, linked evaluations, linked repair plans, linked replay runs, and recommended next actions

#### Scenario: Administrator reads out-of-scope proof report
- **WHEN** an administrator requests a proof report outside the authorized scope
- **THEN** the admin surface rejects the request without exposing report existence or content

### Requirement: Memory session reports are inspectable through scoped boundaries
The service SHALL expose scoped inspection of memory session runs and reports.

#### Scenario: Caller reads session report
- **WHEN** an authorized caller reads a session report within scope
- **THEN** the service returns session turns, context evidence summaries, outcome event ids, verification status, and bounded failure reasons

#### Scenario: Caller reads out-of-scope session report
- **WHEN** a caller requests a memory session report outside the authorized scope
- **THEN** the service rejects the request without exposing session content

