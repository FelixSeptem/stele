## ADDED Requirements

### Requirement: Embedding lifecycle state is operator-inspectable
The service MUST expose enough derived embedding lifecycle state for operators to diagnose missing semantic coverage, drift, and rebuild failure without weakening append-only lineage rules.

#### Scenario: Drift is visible before rebuild completion
- **WHEN** the active vector revision no longer matches the currently routed provider, model, or dimensions target for a memory's current canonical projection
- **THEN** operator inspection can report that drift state together with the requested replacement target and current active revision attribution

#### Scenario: Failed rebuild retains diagnostic attribution
- **WHEN** an embedding rebuild attempt fails for the current canonical projection
- **THEN** operator inspection can report the failed requested target, failure reason, attempt timing, and the still-active or missing semantic projection state for that memory
