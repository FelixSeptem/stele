## ADDED Requirements

### Requirement: Temporal plans carry explicit valid-time constraints
The planner SHALL represent temporal query constraints separately from recorded
time filters. A historical family plan MUST contain an authorized `as_of` or
`valid_during` constraint, and a missing or malformed constraint MUST fail closed
to the approved current baseline.

#### Scenario: Temporal plan is replayed
- **WHEN** the same accepted query, temporal selector, scope, and policy versions
  are replayed
- **THEN** the planner produces the same valid-time constraint, plan identity,
  and bounded disposition

#### Scenario: Temporal family lacks explicit selector
- **WHEN** query analysis suggests historical retrieval but no authorized temporal
  selector is present
- **THEN** the planner does not access history and uses the approved current
  retrieval fallback
