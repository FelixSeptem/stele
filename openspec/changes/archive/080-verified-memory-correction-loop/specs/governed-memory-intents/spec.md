## ADDED Requirements

### Requirement: Verification-attributed procedural updates are governed and idempotent
An enabled verification-attributed procedural `update` intent SHALL contain a
server-resolved current refuted verification, target memory/version, corrected
payload, exact scope, policy identity, and idempotency identity. Admission MUST
validate all of those facts before durable handoff. Equivalent replay MUST return
the original outcome; conflicting reuse MUST fail closed. Caller-supplied
verification/evidence references MUST NOT substitute for the server-resolved
association.

#### Scenario: Equivalent verification correction is replayed
- **WHEN** an authorized administrator repeats the same eligible verification
  correction after API or worker restart
- **THEN** the service returns the original intent outcome without a duplicate
  intent, candidate, canonical successor, or verification association

#### Scenario: Correction association is forged or stale
- **WHEN** a request supplies a foreign, unsupported, nonrefuted, stale, hidden,
  or mismatched verification/target association
- **THEN** the service rejects it before idempotency persistence, queue handoff,
  candidate creation, or canonical mutation without revealing the cause

### Requirement: Intent workers recheck verification attribution before promotion
The governed intent worker MUST recheck verification disposition, source
watermark eligibility, target version, lifecycle, exact scope, and current
policy before routing a verification-attributed update. A failed recheck SHALL
append a bounded nonactive transition and complete the durable job without a
candidate or canonical successor.

#### Scenario: Source drift occurs after admission
- **WHEN** an eligible verification correction is admitted and a cited source
  changes before worker execution
- **THEN** the worker records a bounded suppressed or rejected outcome and
  retains the intent/verification audit chain without promoting a successor
