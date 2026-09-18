## ADDED Requirements

### Requirement: Evaluation measures adaptive plans by query family and pass
The evaluator SHALL support versioned planner fixtures and compare baseline with the planned candidate using protected quality, safety, candidate, latency, fallback, reranker-use, and first- versus second-pass evidence metrics.

#### Scenario: Query-family regression is hidden by aggregate gain
- **WHEN** aggregate quality improves but a protected query family regresses beyond policy
- **THEN** the evaluator records a protected-family failure and does not mark the planner candidate eligible
