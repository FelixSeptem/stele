## ADDED Requirements

### Requirement: Replay apply uses durable worker execution
The service SHALL execute derived insight replay apply and backfill work through the durable worker or scheduler model with leases, retry state, idempotency, and failure summaries.

#### Scenario: Worker claims replay work
- **WHEN** a replay apply run is pending and eligible
- **THEN** a worker can claim it with durable ownership and process it without requiring the admin request to remain open

#### Scenario: Replay worker fails
- **WHEN** replay execution fails before completion
- **THEN** the service records attempt count, failure summary, next eligibility, and partial report state so the run can be retried or inspected

### Requirement: Replay scheduling remains scope-bound and bounded
The service MUST keep scheduled or operator-triggered replay execution limited to the requested scope, evidence window, insight types, and configured execution limits.

#### Scenario: Replay backfill is scheduled
- **WHEN** a bounded replay backfill is queued for one authorized scope
- **THEN** the scheduler or worker processes only that scope and window rather than enumerating unrelated scopes

#### Scenario: Replay reaches execution limit
- **WHEN** replay processing reaches the configured evidence or decision limit before the window is exhausted
- **THEN** the service records a bounded completion or continuation-required status instead of silently scanning beyond the limit
