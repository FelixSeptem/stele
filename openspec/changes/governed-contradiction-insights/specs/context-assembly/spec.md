## ADDED Requirements

### Requirement: Contradiction insights require explicit context authorization

The context assembler MUST exclude contradiction candidates, unresolved
contradictions, temporal-coexistence records, and non-authoritative replay
results from default retrieval and ordinary context. An authorized
contradiction section MUST specify exact scope, lifecycle visibility, evidence
freshness, review status, and a bounded output budget.

#### Scenario: Default context contains conflicting facts

- **WHEN** ordinary context assembly encounters an active contradiction insight or a shadow candidate
- **THEN** it excludes the contradiction surface unless an explicit section policy authorizes it

#### Scenario: Authorized contradiction section is requested

- **WHEN** a request carries a compatible exact-scope policy and a fresh reviewed contradiction section budget
- **THEN** context includes only eligible cited contradiction insights and preserves both evidence references

#### Scenario: Unresolved or stale contradiction is present

- **WHEN** a contradiction is unresolved, temporal-only, stale, quarantined, or hidden
- **THEN** the assembler excludes it even when the section is authorized
