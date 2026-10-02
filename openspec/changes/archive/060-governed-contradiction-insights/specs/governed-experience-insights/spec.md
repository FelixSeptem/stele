## ADDED Requirements

### Requirement: Contradiction insights preserve both sides and temporal review

The service SHALL represent a `contradiction` insight with a stable
contradiction key, both source-version citations, valid-time relationship,
uncertainty, review state, derivation provenance, and the policy decision that
created or changed it.

#### Scenario: Contradiction insight is admitted

- **WHEN** a contradiction candidate passes its type policy and review gate
- **THEN** the active derived insight retains both evidence sides, overlap or temporal disposition, uncertainty, and audit attribution

#### Scenario: Contradiction lacks a complete side

- **WHEN** a candidate lacks either source-version citation, temporal identity, or provenance
- **THEN** the service keeps it non-active and records an incomplete-evidence disposition

### Requirement: Contradiction feedback is independently auditable

The service SHALL allow contradiction-specific feedback such as `confirmed`,
`coexists`, `incorrect`, or `stale` to affect later review or lifecycle
decisions only through a versioned policy, preserving prior evidence and
feedback history.

#### Scenario: Operator marks facts as coexisting

- **WHEN** authorized feedback says two facts are temporally compatible
- **THEN** the service records the feedback and can suppress or resolve the contradiction through an auditable transition without deleting source facts
