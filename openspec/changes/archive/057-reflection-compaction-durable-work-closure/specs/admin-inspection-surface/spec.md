## ADDED Requirements

### Requirement: Administrators can inspect derived queue state

The admin surface SHALL expose exact-scope, bounded inspection of queue mode,
work state, depth/lag buckets, flush status, dropped work, retry/exhaustion, and
watermark freshness without exposing raw payloads, scope values, or worker IDs.

#### Scenario: Administrator inspects durable mode

- **WHEN** an authorized administrator reads derived queue status
- **THEN** the response contains bounded mode, state, lag, retry, loss, freshness, and SLO categories

#### Scenario: Administrator inspects foreign scope

- **WHEN** an administrator requests queue status outside the authorized exact scope
- **THEN** the service rejects the request without revealing queue existence or counts

### Requirement: Queue detail is lease-safe and paginated

Authorized queue detail SHALL provide stable opaque pagination for derived work
outcomes, checkpoints, and bounded failure categories, while active ownership
and recovery actions preserve lease safety.

#### Scenario: Administrator reads work detail

- **WHEN** an authorized administrator reads a derived work item
- **THEN** the response includes bounded identity, state, checkpoint, retry, freshness, and terminal disposition metadata

#### Scenario: Administrator requests recovery for active work

- **WHEN** a work item has an active lease
- **THEN** the recovery request is rejected without seizing worker ownership
