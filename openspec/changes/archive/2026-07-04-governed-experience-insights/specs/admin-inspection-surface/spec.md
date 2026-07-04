## ADDED Requirements

### Requirement: Derived insights are inspectable through the admin surface
The service MUST expose admin-only inspection for derived experience insights and their evidence context.

#### Scenario: Operator lists derived insights
- **WHEN** an authorized operator lists derived insights for a scope with optional type or lifecycle filters
- **THEN** the admin surface returns matching insight summaries with type, lifecycle state, confidence, evidence count, and derivation metadata

#### Scenario: Operator reads one derived insight
- **WHEN** an authorized operator reads one derived insight within an authorized scope
- **THEN** the admin surface returns the insight detail, evidence references, provenance, lifecycle history, and lesson output when present

#### Scenario: Operator inspects hidden insight
- **WHEN** an insight is suppressed or otherwise hidden from default context assembly
- **THEN** the admin surface can still expose the insight's lifecycle state and evidence context without making it visible to public retrieval or context assembly

### Requirement: Derived insight lifecycle actions are bounded and auditable
The service SHALL support narrowly scoped admin lifecycle actions for derived insights.

#### Scenario: Operator suppresses a noisy insight
- **WHEN** an authorized operator suppresses an active derived insight with actor and reason attribution
- **THEN** the service records the lifecycle transition and excludes that insight from default context assembly

#### Scenario: Operator action preserves evidence history
- **WHEN** an operator changes a derived insight lifecycle state
- **THEN** the service preserves the linked evidence and records the action in audit history
