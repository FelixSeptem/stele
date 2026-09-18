## ADDED Requirements

### Requirement: Approved plans can prioritize existing context sections
The service SHALL allow a compatible active retrieval plan to provide bounded memory-class quotas and an ordered priority over existing context sections while preserving response shape, caller budget, summary preference, lifecycle visibility, exact scope, diversity, and citations.

#### Scenario: Planned quota exceeds caller budget
- **WHEN** plan quotas require more context than the caller-provided or service-default budget permits
- **THEN** context assembly enforces the smaller existing budget and records only a bounded omission category
