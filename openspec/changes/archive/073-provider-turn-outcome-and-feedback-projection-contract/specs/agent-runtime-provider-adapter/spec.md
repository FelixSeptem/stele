## ADDED Requirements

### Requirement: Provider operations expose a versioned runtime evidence projection boundary

The provider SHALL advertise the dedicated turn-outcome-feedback projection
capability and enforce its closed DTO, exact runtime binding, compatibility and
bounded error semantics independently from legacy generic turn/outcome routes.

#### Scenario: Projection request uses unsupported capability or schema
- **WHEN** a runtime submits the projection contract with an unsupported version
- **THEN** the provider returns the existing bounded compatibility outcome before
  operation dispatch and does not attempt a legacy route fallback

#### Scenario: Projection reports business-state fields
- **WHEN** a projection includes a task-state, lease, commit, publication or
  delivery override
- **THEN** the provider rejects the unsupported authority field and does not
  invoke memory, feedback or runtime business-state operations

### Requirement: Provider conformance covers durable projection evidence

Provider conformance SHALL validate actual serialized projection operations for
scope safety, equivalent/conflicting replay, correction history, durable
reference states, feedback separation and API/worker restart recovery.

#### Scenario: Projection conformance has a missing durable stage
- **WHEN** a conformance fixture lacks required authoritative storage or restart evidence
- **THEN** its result is ineligible for Provider readiness and does not inherit
  PC1–PC5 readiness conclusions

