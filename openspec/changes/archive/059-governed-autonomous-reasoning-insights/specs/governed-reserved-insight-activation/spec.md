## ADDED Requirements

### Requirement: Reasoning candidate handoff is an explicit admission boundary

The service SHALL accept reasoning-derived candidates only through an explicit
handoff that carries exact scope proof, evidence digest, replay identity,
provider/schema compatibility, uncertainty bounds, and the target policy
version. Provider output alone MUST NOT constitute an admission decision.

#### Scenario: Eligible reasoning candidate is handed off

- **WHEN** a candidate satisfies the target policy's scope, evidence, freshness, provenance, uncertainty, and idempotency gates
- **THEN** the activation path evaluates it under the existing type-specific policy and records the admission disposition

#### Scenario: Candidate omits handoff proof

- **WHEN** a candidate lacks exact scope proof, evidence digest, replay identity, or compatible policy version
- **THEN** the service rejects or quarantines it without creating an active insight

### Requirement: Reasoning activation remains independently rollbackable

The service SHALL record reasoning-derived admissions with provider and policy
versions so an operator can disable or roll back that policy without changing
other insight types or rewriting prior evidence history.

#### Scenario: Reasoning policy is rolled back

- **WHEN** an operator rolls back the policy used by reasoning-derived admissions
- **THEN** new admissions stop for that policy and prior records remain auditable under their original provenance
