## ADDED Requirements

### Requirement: Goal context is an opt-in isolated section

Context assembly MUST treat `goal_context` as an optional experimental section that requires an explicit caller opt-in and a successful exact-scope visibility-policy decision. The section MUST be omitted for ordinary requests and MUST NOT influence ordinary section ranking, budget allocation, counts, or fallback behavior.

#### Scenario: Caller does not opt in

- **WHEN** an ordinary context request is assembled while eligible goal records exist
- **THEN** the response omits `goal_context` and exposes no goal-derived ranking, count, or error detail

#### Scenario: Caller opts in but policy fails

- **WHEN** the caller requests `goal_context` but any authorization, scope, review, evidence, freshness, or rollback gate fails
- **THEN** the section is omitted and the remaining context response is unchanged

#### Scenario: Caller opts in and policy passes

- **WHEN** the caller is authorized and all exact-scope visibility gates pass
- **THEN** the response may contain only the independently governed `goal_context` section without changing ordinary sections
