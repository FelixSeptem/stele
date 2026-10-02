## ADDED Requirements

### Requirement: Contradiction activation requires type-specific temporal gates

An enabled contradiction policy SHALL require two exact-scope evidence sides,
a mutually exclusive contradiction key, a valid-time overlap or explicit
review override, bounded uncertainty, compatible provenance, and an
idempotency identity. The policy MUST define whether operator review is
required before activation.

#### Scenario: Overlapping contradiction is eligible

- **WHEN** both fact versions are visible, mutually exclusive, temporally overlapping, policy-compatible, and reviewed as required
- **THEN** activation admits one append-only contradiction insight through the ordinary reserved-insight lifecycle

#### Scenario: Temporal coexistence is submitted for activation

- **WHEN** a candidate has disjoint valid-time intervals and no explicit review override
- **THEN** activation records temporal-coexistence and does not create an active contradiction insight

#### Scenario: Contradiction policy requires review

- **WHEN** an otherwise eligible candidate has not reached the policy's required review state
- **THEN** activation records a review-required disposition and creates no active insight
