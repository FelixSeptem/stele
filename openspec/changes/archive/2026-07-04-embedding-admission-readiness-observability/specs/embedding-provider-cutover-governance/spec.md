## ADDED Requirements

### Requirement: Draft cutover plans support preflight admission
The service SHALL support immediate preflight evaluation for draft embedding provider cutover plans before activation.

#### Scenario: Operator preflights a draft plan
- **WHEN** an authorized operator requests preflight for a draft cutover plan
- **THEN** the service returns an admission report with decision, blockers, warnings, immutable target snapshot, scope, eligible memory total, class breakdown, conflicting plan context when present, and observed time

#### Scenario: Preflight report is not persisted
- **WHEN** the service returns a preflight report
- **THEN** the report reflects the current evaluation result without mutating the cutover plan or storing the report as durable plan state

### Requirement: Cutover activation enforces scoped concurrency
The service MUST prevent more than one active or paused cutover plan from existing for the same tenant, project, and namespace scope.

#### Scenario: Same scope has an active plan
- **WHEN** an authorized operator activates a draft plan for a scope that already has an active or paused cutover plan
- **THEN** activation is rejected with a blocker that identifies the scoped plan conflict

#### Scenario: Same scope has multiple draft plans
- **WHEN** multiple draft cutover plans exist for the same scope
- **THEN** the service allows them to remain draft until one is activated and passes admission

## MODIFIED Requirements

### Requirement: Cutover activation validates runtime support before rollout begins
The service MUST reject cutover activation if admission detects hard blockers before rollout begins.

#### Scenario: Operator activates a plan with a valid target
- **WHEN** an authorized operator activates a cutover plan whose target can be resolved by the current runtime provider registry and whose admission report has no blockers
- **THEN** the plan becomes active and the service registers eligible memory membership for later bounded rollout through the ordinary embedding rebuild path

#### Scenario: Operator activates a plan with an unavailable target
- **WHEN** an authorized operator activates a cutover plan whose provider, model, dimensions, or target construction is not available in the current runtime
- **THEN** the service rejects activation before any rollout wave is scheduled

#### Scenario: Operator activates an empty plan
- **WHEN** an authorized operator activates a cutover plan whose current eligible memory total is zero
- **THEN** the service rejects activation before changing the plan status or scheduling work

#### Scenario: Provider network is temporarily unreachable during activation
- **WHEN** an authorized operator activates a plan whose target is statically supported but whose provider network probe is currently failing
- **THEN** the service does not treat the transient probe failure as an activation blocker and exposes the dependency issue through readiness or metrics instead
