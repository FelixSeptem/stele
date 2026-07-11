## ADDED Requirements

### Requirement: Bootstrap guidance covers provider cutover operations
The service MUST document the operator workflow for creating, activating, pausing, cancelling, and rolling back embedding provider cutovers.

#### Scenario: Operator prepares a provider cutover rollout
- **WHEN** an operator reads the bootstrap documentation before migrating a scope to a new embedding target
- **THEN** the documentation describes the required admin routes, rollout sequencing, runtime validation expectations, and progress inspection workflow

#### Scenario: Operator plans rollback after a failed cutover
- **WHEN** an operator needs to reverse a provider migration
- **THEN** the documentation explains that rollback is modeled as a new forward cutover plan toward the prior target rather than as direct vector history mutation

### Requirement: Smoke checks cover cutover progress and recovery audit
The service MUST provide operator guidance for verifying cutover progress and recovery history during rollout incidents.

#### Scenario: Operator monitors an active cutover
- **WHEN** an operator runs the documented smoke check for an active provider cutover
- **THEN** the documented workflow confirms how to inspect plan progress, backlog pressure, and memory-level cutover context in addition to baseline readiness

#### Scenario: Operator investigates remediation during rollout
- **WHEN** an operator follows the documented incident workflow for a failed provider cutover
- **THEN** the documentation shows how to query embedding recovery history at both scope and memory level to explain retry or requeue activity
