## ADDED Requirements

### Requirement: Assurance and conformance are inspectable through admin surfaces
The service SHALL expose admin-only inspection and bounded controls for health evaluations, incidents, alert candidates, conformance profiles, conformance runs, readiness reports, and recovery verification.

#### Scenario: Administrator lists health evaluations
- **WHEN** an authorized administrator lists health evaluations for a scope with optional status, component, severity, or time filters
- **THEN** the admin surface returns bounded evaluation summaries without requiring direct PostgreSQL access

#### Scenario: Administrator controls incident lifecycle
- **WHEN** an authorized administrator acknowledges, suppresses, or resolves an incident with actor and reason attribution
- **THEN** the admin surface records an auditable transition without mutating the underlying proof, session, feedback, task, ranking, repair, or memory evidence

#### Scenario: Administrator manages conformance profiles
- **WHEN** an authorized administrator creates, updates, disables, or reads a conformance profile within scope
- **THEN** the admin surface validates bounded evidence requirements and returns the profile state through the admin boundary

#### Scenario: Administrator requests out-of-scope assurance record
- **WHEN** an administrator requests an assurance, incident, alert, conformance, readiness, or recovery record outside an authorized scope
- **THEN** the admin surface rejects the request without exposing record existence or evidence counts

### Requirement: Alert delivery inspection preserves sensitive configuration
The service SHALL allow administrators to inspect alert candidates and delivery attempts without exposing secret delivery targets.

#### Scenario: Administrator reads alert candidate detail
- **WHEN** an authorized administrator reads an alert candidate
- **THEN** the admin surface returns severity, component, status, reason category, delivery policy, delivery attempt summaries, and recommended actions

#### Scenario: Alert uses webhook adapter
- **WHEN** an alert candidate or delivery attempt used a webhook adapter
- **THEN** the admin surface redacts webhook URL, headers, tokens, and recipient secrets while preserving delivery result and failure category

### Requirement: Readiness and recovery reports are admin-inspectable
The service SHALL expose scope readiness and recovery verification reports through authorized admin routes.

#### Scenario: Administrator reads scope readiness
- **WHEN** an authorized administrator requests readiness for a scope
- **THEN** the admin surface returns current or latest readiness status, component summaries, conformance status, incident counters, alert counters, and recommended admin surfaces

#### Scenario: Administrator reads recovery verification
- **WHEN** an authorized administrator reads recovery verification for an incident or conformance failure
- **THEN** the admin surface returns checked surfaces, bounded result categories, linked evidence references, status, and next actions
