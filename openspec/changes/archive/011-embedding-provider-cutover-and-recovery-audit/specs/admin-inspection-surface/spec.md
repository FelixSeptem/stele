## ADDED Requirements

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
