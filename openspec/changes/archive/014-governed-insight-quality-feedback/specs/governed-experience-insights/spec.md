## ADDED Requirements

### Requirement: Derived insight governance accounts for quality feedback
The service SHALL use governed quality feedback as an input to derived insight lifecycle, confidence, and derivation decisions without rewriting insight evidence in place.

#### Scenario: Insight has strong negative feedback
- **WHEN** insight derivation evaluates an existing insight with active `noisy`, `incorrect`, or `stale` feedback
- **THEN** the service can lower the insight's effective quality, avoid reactivating it automatically, or create an auditable lifecycle transition according to policy without deleting evidence

#### Scenario: Insight has positive feedback
- **WHEN** insight derivation evaluates an existing insight with active `useful` feedback
- **THEN** the service can preserve or prioritize that insight as long as the evidence and scope remain valid

#### Scenario: Conflicting feedback exists
- **WHEN** an insight has both positive and negative active feedback signals
- **THEN** the service preserves the conflict in effective quality state rather than silently discarding either signal

### Requirement: Feedback-driven lifecycle changes remain auditable
The service MUST record any feedback-driven derived insight lifecycle transition with attribution to the policy or job that consumed the feedback.

#### Scenario: Feedback causes insight suppression
- **WHEN** a derivation or maintenance policy suppresses an insight because of effective feedback state
- **THEN** the service records a lifecycle transition that identifies the feedback-driven reason and preserves the feedback and evidence history

#### Scenario: Feedback does not justify lifecycle mutation
- **WHEN** feedback affects ranking or review status but does not meet the policy threshold for lifecycle mutation
- **THEN** the service leaves the insight lifecycle unchanged and exposes the quality state through inspection or ranking surfaces
