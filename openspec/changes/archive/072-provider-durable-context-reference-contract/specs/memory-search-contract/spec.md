## ADDED Requirements

### Requirement: Exact reference lookup is distinct from ranked search

The public memory contract SHALL define deterministic exact-path reference
resolution separately from ranked top-k search, including its selector, result
identity and bounded unavailable outcomes.

#### Scenario: Exact path resolves one current reference
- **WHEN** an authorized caller supplies an exact scope, normalized path and
  reference selector
- **THEN** the service returns the current eligible reference identity/value and
  does not apply ranking as the identity decision

#### Scenario: Exact path has no current eligible reference
- **WHEN** no active canonical reference is eligible at the exact path
- **THEN** the service returns a bounded not-found or unavailable category and
  does not expose hidden or foreign candidates
