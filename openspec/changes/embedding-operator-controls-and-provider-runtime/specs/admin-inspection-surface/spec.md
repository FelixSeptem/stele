## ADDED Requirements

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
