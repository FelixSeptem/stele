## ADDED Requirements

### Requirement: Evidence handoffs expose immutable reconciliation identities

Release evidence handoffs SHALL persist the exact scope, policy version, source watermark, fixture identity, representation identity, freshness deadline, attestation identity, and rollback proof identity required for later reconciliation. Once accepted, those identities MUST NOT be rewritten in place.

#### Scenario: Handoff is accepted

- **WHEN** an owned release run produces a complete handoff
- **THEN** the handoff stores all reconciliation identities and can be referenced by a stable opaque handoff identifier

#### Scenario: Handoff omits a required identity

- **WHEN** a release run attempts to submit an incomplete handoff
- **THEN** submission is rejected or marked unusable for activation and the missing category is recorded without accepting partial eligibility

