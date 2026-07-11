## MODIFIED Requirements

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
