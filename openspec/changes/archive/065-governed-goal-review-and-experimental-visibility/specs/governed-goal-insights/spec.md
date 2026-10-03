## ADDED Requirements

### Requirement: Goal review handoff distinguishes visibility eligibility

Goal review handoff MUST record review attribution and a bounded visibility eligibility result separately from goal lifecycle state. A review decision MUST NOT by itself activate a goal or make it visible to ordinary retrieval or context assembly.

#### Scenario: Reviewer approves an eligible candidate

- **WHEN** an authorized reviewer approves a fixed goal candidate under its exact-scope policy
- **THEN** the service records the review attribution and separately evaluates experimental visibility without changing ordinary retrieval behavior

#### Scenario: Reviewer approves an ineligible candidate

- **WHEN** review succeeds but freshness, evidence, grant, or visibility policy gates fail
- **THEN** the service preserves the review record and records a bounded visibility-ineligible disposition
