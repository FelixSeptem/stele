## ADDED Requirements

### Requirement: Validated query analysis can inform retrieval planning
The service SHALL make only validated query-analysis identity, disposition, bounded hint categories, signal kinds, and count categories available to the retrieval planner. Planning MUST preserve the accepted original query as the mandatory signal and MUST NOT treat planner classification as new query facts, scope, or caller constraints.

#### Scenario: Analysis is unavailable or incompatible
- **WHEN** query analysis fails, is disabled, or has an identity incompatible with the selected planner policy
- **THEN** planning uses the general original-query baseline and does not infer replacement hints or derived signals
