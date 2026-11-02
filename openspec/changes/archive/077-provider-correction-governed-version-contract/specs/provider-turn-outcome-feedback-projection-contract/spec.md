## ADDED Requirements

### Requirement: Corrected projection revisions can be verified correction sources

The service SHALL expose a corrected Provider projection revision only as a
server-resolved source for a separately negotiated governed correction-update
operation. Eligibility MUST require the current runtime binding and exact
scope, a strictly later revision linked to its predecessor, current-head status,
and currently eligible durable-reference attribution. A projection correction
MUST NOT itself mutate canonical memory or authorize any business state.

#### Scenario: Current bound correction is used as a source
- **WHEN** the same runtime binding presents a current corrected projection to
  the correction-update operation
- **THEN** the service may resolve only its bounded source identity and current
  eligible reference lineage for governed intent validation

#### Scenario: Historical or foreign correction is requested as a source
- **WHEN** a caller presents a first, superseded, foreign, expired, revoked or
  lifecycle-ineligible projection revision
- **THEN** the source is unavailable to correction update processing without
  revealing its identity, payload, history or references
