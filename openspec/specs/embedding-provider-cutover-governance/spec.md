# embedding-provider-cutover-governance Specification

## Purpose
Define the durable provider cutover plan model, rollout controls, and audit expectations for migrating one scope toward a new embedding target without rewriting vector lineage.
## Requirements
### Requirement: Provider cutover plans are durable and target-immutable
The service MUST persist provider cutover plans as durable operator-owned records that capture immutable target provider, model, and dimensions snapshots together with scope, rollout pacing, and attribution.

#### Scenario: Operator creates a cutover plan
- **WHEN** an authorized operator creates an embedding provider cutover plan for an authorized scope
- **THEN** the service stores the requested target snapshot, rollout settings, and operator attribution as a durable plan record rather than deriving the rollout only from current drift state

#### Scenario: Activated plan preserves its original target
- **WHEN** runtime embedding defaults or class routes change after a cutover plan has been created
- **THEN** the stored cutover plan still reflects the exact provider, model, and dimensions target it originally declared

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

### Requirement: Cutover progression is batch-oriented and operator-visible
The service SHALL advance active provider cutovers in bounded waves and expose progress at both aggregate and per-memory levels.

#### Scenario: Scheduler advances the next cutover wave
- **WHEN** an active cutover plan still has eligible memory items remaining and rollout capacity is available
- **THEN** the scheduler advances only the next bounded wave of items back into the ordinary embedding rebuild path instead of scheduling the entire scope at once

#### Scenario: Operator reads cutover plan detail
- **WHEN** an authorized operator reads one cutover plan
- **THEN** the service returns plan status, target snapshot, aggregate progress counters, and item-level state needed to diagnose rollout health

### Requirement: Cutover controls stop future rollout without rewriting history
The service MUST support pausing or cancelling a cutover plan without seizing active leases or mutating already recorded vector lineage.

#### Scenario: Operator pauses an active cutover plan
- **WHEN** an authorized operator pauses an active cutover plan
- **THEN** the service stops scheduling future rollout waves while leaving already rebuilding work under its current worker ownership

#### Scenario: Operator cancels a cutover plan
- **WHEN** an authorized operator cancels a draft, active, or paused cutover plan
- **THEN** unscheduled remaining items stop advancing, while recorded plan history and already completed or rebuilding work remain auditable

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
