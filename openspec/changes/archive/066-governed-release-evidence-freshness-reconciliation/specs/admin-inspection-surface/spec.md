## ADDED Requirements

### Requirement: Reconciliation runs and eligibility are admin-inspectable

The admin surface SHALL provide exact-scope, paginated inspection of reconciliation run status, attempt and checkpoint summaries, current eligibility, bounded reason categories, policy and watermark identities, and linked opaque handoff references. Responses MUST be redacted and MUST not expose raw evidence payloads.

#### Scenario: Administrator reads reconciliation status

- **WHEN** an authorized administrator inspects one exact scope
- **THEN** the response includes current eligibility, latest run status, freshness bucket, reason category, and bounded transition counts

#### Scenario: Administrator requests a foreign scope

- **WHEN** an administrator lacks an exact-scope grant for the requested reconciliation record
- **THEN** the service rejects the request without revealing run existence or eligibility state

### Requirement: Reconciliation can be triggered with bounded attribution

The admin surface SHALL allow an authorized administrator to request a bounded manual reconciliation with actor, reason, scope, and idempotency attribution. The request MUST enqueue the durable job and MUST not perform direct activation or evidence mutation.

#### Scenario: Administrator submits a manual trigger

- **WHEN** an authorized administrator submits a valid bounded trigger
- **THEN** the service returns a stable run identity and records actor and reason attribution for later inspection

#### Scenario: Administrator attempts direct activation

- **WHEN** an admin request tries to promote or restore activation without a new compatible handoff
- **THEN** the service rejects the request and records a bounded governance denial

