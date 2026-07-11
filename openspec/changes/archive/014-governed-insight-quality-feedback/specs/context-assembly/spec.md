## ADDED Requirements

### Requirement: Insight context assembly respects quality feedback
The service SHALL account for effective quality feedback when assembling optional derived insight context sections.

#### Scenario: Useful insight competes under budget
- **WHEN** multiple active derived insights match an optional insight section and the context budget is constrained
- **THEN** context assembly can prioritize insights with active `useful` feedback when evidence relevance is otherwise comparable

#### Scenario: Noisy insight competes under budget
- **WHEN** a matching insight has active `noisy`, `incorrect`, `stale`, or `needs_review` feedback
- **THEN** context assembly deprioritizes or omits that insight according to policy and budget without exposing raw admin feedback by default

#### Scenario: Feedback summary is requested for diagnostics
- **WHEN** an authorized admin or debug assembly path requests quality diagnostics for insight sections
- **THEN** the response can include summarized quality state without exposing feedback history through ordinary public context assembly

### Requirement: Feedback does not bypass lifecycle visibility rules
The service MUST keep lifecycle state and scope isolation as the primary visibility controls for insight context sections.

#### Scenario: Useful feedback exists on hidden insight
- **WHEN** an insight is suppressed, forgotten, deleted, or out of scope but has active `useful` feedback
- **THEN** context assembly excludes that insight from ordinary `known_failures` and `experience_lessons` sections

#### Scenario: Negative feedback exists on active insight
- **WHEN** an active insight has negative feedback but has not been suppressed by governed policy
- **THEN** context assembly treats the feedback as a ranking or omission signal rather than rewriting the insight lifecycle inline
