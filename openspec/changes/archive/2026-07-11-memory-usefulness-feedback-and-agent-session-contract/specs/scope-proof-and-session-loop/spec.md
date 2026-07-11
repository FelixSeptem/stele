## ADDED Requirements

### Requirement: Memory session turns are idempotent
The service SHALL support idempotent memory session turn and outcome writes for external agent integrations.

#### Scenario: Caller repeats a turn creation request
- **WHEN** a caller creates a session turn with the same scope, session id, and idempotency key
- **THEN** the service returns the existing turn result rather than creating duplicate turn evidence

#### Scenario: Caller repeats an outcome request
- **WHEN** a caller records a turn outcome with the same scope, session id, turn id, and idempotency key
- **THEN** the service preserves idempotent behavior and avoids duplicate outcome events, expected recall records, or feedback attribution

### Requirement: Session outcomes can ingest bounded event payloads
The service SHALL allow memory session outcomes to include bounded memory-relevant event payloads that are written through the existing event ingestion contract.

#### Scenario: Caller records outcome payloads
- **WHEN** an external agent records a turn outcome with bounded event payloads
- **THEN** the service ingests those payloads through the normal event ingestion path and attaches session id, turn id, actor, reason, and outcome attribution metadata

#### Scenario: Outcome payload is invalid
- **WHEN** an outcome payload violates event validation, admission, size, or scope rules
- **THEN** the service rejects or reports that payload without bypassing event ingestion governance

### Requirement: Session reports include verification and feedback history
The service SHALL preserve session verification history and feedback summaries in scoped session reports.

#### Scenario: Session verification is requested multiple times
- **WHEN** a caller reruns verification for a session or turn
- **THEN** the service creates fresh verification evidence while preserving prior verification attempts and verdicts

#### Scenario: Session feedback exists
- **WHEN** feedback has been recorded for session context, citations, outcomes, expected recall, or turns
- **THEN** the session report includes bounded feedback summaries and next actions without exposing hidden or out-of-scope memory content

### Requirement: Memory sessions remain service-side contracts
The service MUST NOT execute the external agent while strengthening session contracts.

#### Scenario: Caller uses session APIs
- **WHEN** a caller creates turns, records outcomes, requests verification, or records feedback
- **THEN** Stele provides memory context, ingestion, verification, feedback evidence, and reports without invoking models, building prompts, or generating final answers
