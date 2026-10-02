## ADDED Requirements

### Requirement: Intent processing preserves append-only canonical lifecycle
Processing a `remember`, `update`, `forget`, or `contradiction` intent MUST create candidates or append lifecycle/version transitions through canonical governance rules and MUST NOT mutate a prior canonical version in place.

#### Scenario: Update intent targets a prior version
- **WHEN** an update intent names an existing memory and target version
- **THEN** governance verifies the target version and writes a new candidate or canonical version while retaining the targeted history

#### Scenario: Forget intent is applied after review
- **WHEN** a reviewed forget intent is accepted
- **THEN** the service records the lifecycle transition and audit attribution while preserving the prior version and provenance
