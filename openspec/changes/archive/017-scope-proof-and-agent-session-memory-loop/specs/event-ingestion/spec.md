## ADDED Requirements

### Requirement: Ingestion accepts proof and session attribution
The event ingestion contract SHALL support optional proof and memory-session attribution metadata without weakening validation or scope isolation.

#### Scenario: Proof run writes fixture event
- **WHEN** proof execution ingests a smoke fixture event for a scope
- **THEN** the event is persisted through the ordinary ingestion contract with proof attribution and admission metadata

#### Scenario: Session records turn outcome event
- **WHEN** a memory session records a turn outcome that should enter the memory pipeline
- **THEN** the service writes the outcome through ordinary event ingestion with session and turn attribution

#### Scenario: Attribution scope mismatches event scope
- **WHEN** proof or session attribution references a run outside the resolved event scope
- **THEN** ingestion rejects the request before writing the event
