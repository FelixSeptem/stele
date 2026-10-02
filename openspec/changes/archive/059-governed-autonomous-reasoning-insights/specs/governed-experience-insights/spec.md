## ADDED Requirements

### Requirement: Reasoning-derived candidates preserve derivation provenance

The service SHALL preserve provider-neutral derivation metadata for a
reasoning-derived candidate or activated insight, including operation mode,
provider and schema identity, normalized input digest, source watermark,
uncertainty bounds, and the policy decision that consumed it.

#### Scenario: Reasoning candidate is retained for review

- **WHEN** an offline or shadow run produces a structurally valid candidate
- **THEN** the retained candidate includes provenance and uncertainty metadata sufficient to reproduce and audit the derivation

#### Scenario: Candidate provenance is incomplete

- **WHEN** a reasoning candidate lacks provider identity, source watermark, or normalized input identity
- **THEN** the service does not expose it as an active insight and records an incomplete-provenance disposition
