## ADDED Requirements

### Requirement: Embedding operator recovery reuses durable rebuild execution
The service SHALL treat operator-triggered embedding retry or requeue actions as state transitions back into the ordinary durable rebuild path rather than as ad hoc execution shortcuts.

#### Scenario: Requeue returns work to normal claim flow
- **WHEN** an authorized operator requeues eligible embedding rebuild work
- **THEN** the rebuild record becomes claimable by the existing background rebuild job instead of being executed inline by the admin request

#### Scenario: Retry preserves idempotent execution semantics
- **WHEN** operator remediation restores failed embedding work to pending eligibility
- **THEN** later background execution still uses the same append, compare, and promote safeguards as scheduler-discovered rebuild work

### Requirement: Embedding recovery actions do not seize active leases
The service MUST reject embedding remediation actions that would bypass active background ownership of a rebuild record.

#### Scenario: Operator targets rebuilding work
- **WHEN** an operator attempts to retry or requeue a rebuild record that is currently rebuilding under an active lease
- **THEN** the action is rejected and the existing worker ownership remains unchanged
