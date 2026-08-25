## ADDED Requirements

### Requirement: Public event ingestion is idempotent per principal and scope
The service SHALL require a bounded idempotency key for `POST /v1/events` and
SHALL persist a durable idempotency decision scoped to the authenticated
principal and authorized tenant, project, and namespace.

#### Scenario: Exact event retry returns original result
- **WHEN** the same authenticated principal retries an event request in the same scope with the same idempotency key and equivalent normalized payload
- **THEN** the service returns the original durable event identifier and admission result without creating another raw event or provenance record

#### Scenario: Key is reused for another payload
- **WHEN** the same authenticated principal and scope reuse an idempotency key with a different normalized event payload
- **THEN** the service returns a conflict response and does not create another raw event

#### Scenario: Same key is used by another principal or scope
- **WHEN** a different authorized principal or a different authorized scope uses an idempotency key already used elsewhere
- **THEN** the service treats it as an independent idempotency domain and does not disclose the prior event or caller

#### Scenario: Retry follows interrupted write
- **WHEN** an event write is interrupted after idempotency claim acquisition but before completion
- **THEN** a later retry safely resumes after bounded durable lease recovery or receives a bounded retryable response without duplicate event creation

### Requirement: Ingestion idempotency preserves governance lifecycle
The service SHALL apply idempotency before creating a raw event and SHALL not
allow a retry to bypass admission, provenance, or the ordinary
event-to-candidate-to-active lifecycle.

#### Scenario: First event write is accepted
- **WHEN** an authorized event request with a new idempotency key passes admission validation
- **THEN** the service persists one raw event, one provenance chain, and one completed idempotency result transactionally

#### Scenario: Admission rejection is retried
- **WHEN** an event request is rejected before a raw event is written because admission returns a blocking decision
- **THEN** the service returns the stable rejection category and does not persist a completed event idempotency result
