## ADDED Requirements

### Requirement: Event ingestion surfaces admission pressure metadata
Successful event ingestion responses SHALL include pressure-aware admission metadata without removing the stable event identifier contract.

#### Scenario: Event is accepted normally
- **WHEN** a client submits a valid event and admission returns `accept`
- **THEN** the service persists the raw event, returns the stable event identifier, and includes admission metadata with decision `accept`

#### Scenario: Event is accepted while downstream work is degraded
- **WHEN** a client submits a valid event and admission returns `accept_degraded`
- **THEN** the service persists the raw event, returns the stable event identifier, and includes warning finding codes that describe downstream degradation

#### Scenario: Event is queued under pressure
- **WHEN** a client submits a valid event and admission returns `queue`
- **THEN** the service durably records the event or write intent according to the ingestion contract and returns admission metadata that identifies queued processing state

#### Scenario: Event is rejected before write
- **WHEN** a client submits an event and admission returns `reject`
- **THEN** the service rejects the request before writing a raw event and returns stable blocker finding codes
