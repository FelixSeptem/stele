## ADDED Requirements

### Requirement: Runtime projection feedback retains source attribution and governed effect boundaries

The service SHALL allow scoped usefulness feedback to cite a compatible runtime
projection, source-outcome revision and eligible durable-reference attribution
while preserving the feedback log as source of truth, existing idempotency and
supersession semantics. Pending, failed, unavailable, foreign or lifecycle-hidden
references MUST NOT become active ranking evidence merely through projection
feedback.

#### Scenario: Projected feedback cites a pending reference
- **WHEN** admitted feedback cites a same-scope pending durable reference
- **THEN** the service records the bounded source attribution but excludes it
  from eligible feedback summaries and ranking activation until normal gates pass

