## ADDED Requirements

### Requirement: Admission pressure decisions cover ingestion and repair work
The service SHALL extend admission evaluation diagnostics to classify ingestion and repair work pressure with stable decisions and finding codes.

#### Scenario: Ingestion is accepted under normal pressure
- **WHEN** ingestion dependencies and scoped backlog are within configured limits
- **THEN** the admission evaluation returns `accept` with no blocker findings

#### Scenario: Ingestion is accepted with degradation
- **WHEN** the service can durably persist the event but semantic projection, governance processing, or downstream maintenance is degraded
- **THEN** the admission evaluation returns `accept_degraded` with warning finding codes that explain the degraded component

#### Scenario: Ingestion should be queued
- **WHEN** the service can durably preserve intent but immediate downstream work should be delayed because scoped pressure exceeds configured limits
- **THEN** the admission evaluation returns `queue` with stable pressure finding codes

#### Scenario: Ingestion or repair is rejected
- **WHEN** the service cannot safely persist intent, cannot resolve scope, or would violate configured safety limits
- **THEN** the admission evaluation returns `reject` with blocker finding codes before creating new work

### Requirement: Repair admission uses the same diagnostics contract
The service MUST evaluate repair plan creation and repair action dispatch with the same structured admission contract used for ingestion pressure.

#### Scenario: Repair plan is accepted
- **WHEN** a repair plan can be created within scope and within configured action limits
- **THEN** the admission result records an `accept` or `accept_degraded` decision and stable finding codes

#### Scenario: Repair dispatch is delayed by worker pressure
- **WHEN** repair execution would exceed scoped worker backlog, lease, or dependency limits
- **THEN** the admission result records `queue` rather than bypassing durable worker execution

#### Scenario: Repair dispatch exceeds safety limits
- **WHEN** a repair action would exceed configured scope, cardinality, action category, or dependency safety limits
- **THEN** the admission result records `reject` and the action is not dispatched
