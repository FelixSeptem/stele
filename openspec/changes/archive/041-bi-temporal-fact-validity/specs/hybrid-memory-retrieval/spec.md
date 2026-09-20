## ADDED Requirements

### Requirement: Hybrid channels apply one temporal eligibility predicate
Lexical, semantic, relation, and authorized chunk retrieval MUST apply the same lifecycle and valid-time predicate before fusion. Recorded-time filtering MUST remain distinct from valid-time filtering.

#### Scenario: Channel returns an expired candidate
- **WHEN** one enabled recall channel finds a version outside the requested valid-time constraint
- **THEN** the channel excludes it before fusion and does not allow a stronger similarity score to restore it

#### Scenario: Recorded-time filter is supplied
- **WHEN** a caller supplies only the existing recorded-time window
- **THEN** retrieval preserves that filter's established semantics and still applies default current-valid lifecycle visibility
