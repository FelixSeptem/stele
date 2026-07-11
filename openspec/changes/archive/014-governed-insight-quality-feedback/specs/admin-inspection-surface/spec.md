## ADDED Requirements

### Requirement: Derived insight feedback is manageable through the admin surface
The service SHALL expose admin-only operations for recording, listing, reading, and superseding quality feedback for derived insights.

#### Scenario: Operator records insight feedback
- **WHEN** an authorized operator submits feedback for a derived insight with actor and reason attribution
- **THEN** the admin surface creates a scoped feedback record and returns its durable identity

#### Scenario: Operator lists feedback for an insight
- **WHEN** an authorized operator requests feedback for a derived insight within an authorized scope
- **THEN** the admin surface returns the matching feedback records, including supersession state and audit attribution

#### Scenario: Operator supersedes feedback
- **WHEN** an authorized operator supersedes a prior feedback record with a reason
- **THEN** the admin surface records the supersession and excludes that record from active quality summaries

### Requirement: Admin insight inspection includes quality state
The service MUST include effective quality feedback state in admin derived insight inspection without weakening public visibility rules.

#### Scenario: Operator reads one derived insight with feedback
- **WHEN** an authorized operator reads a derived insight that has quality feedback
- **THEN** the admin surface returns the insight detail together with feedback summary, active review signals, and links or identifiers for feedback history

#### Scenario: Hidden insight has feedback history
- **WHEN** an operator inspects a suppressed or hidden insight with feedback
- **THEN** the admin surface can show feedback and lifecycle context without making the insight visible to public retrieval or context assembly
