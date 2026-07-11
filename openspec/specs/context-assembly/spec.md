# context-assembly Specification

## Purpose
Define the structured context assembly surface that turns governed retrieval results into agent-ready sections with citations and budget-aware packing.
## Requirements
### Requirement: Agent-ready context assembly endpoint
The service SHALL expose a context assembly capability that returns structured sections rather than a flat result list.

#### Scenario: Context is assembled for an agent request
- **WHEN** a client requests assembled context for a scoped query or interaction
- **THEN** the service returns structured sections including `profile`, `recent_session`, `recent_episodes`, `relevant_summaries`, `related_entities`, and `citations`

### Requirement: Summary-preferred context packing
Context assembly MUST prefer summary memory when it can represent a relevant episodic cluster without losing required evidence traceability.

#### Scenario: Summary can replace dense episodic detail
- **WHEN** a relevant summary memory exists for a dense or stale episodic cluster
- **THEN** the service includes the summary before expanding the full underlying episodic set and preserves evidence citations

### Requirement: Budget-aware context shaping
The service MUST support bounded context packing so the assembled response stays within a caller-provided or service-default budget.

#### Scenario: Context budget is constrained
- **WHEN** a client requests context assembly with a limited budget
- **THEN** the service trims and prioritizes sections according to retrieval ranking and summary preference instead of returning unbounded memory

### Requirement: Context assembly can include governed experience insights
The service SHALL support optional context assembly sections for active, evidence-backed derived insights.

#### Scenario: Caller requests known failure context
- **WHEN** a scoped context assembly request asks to include experience insights
- **THEN** the response can include `known_failures` entries derived from active `failure_pattern` insights that match the request scope and budget

#### Scenario: Caller requests experience lessons
- **WHEN** a scoped context assembly request asks to include experience lessons
- **THEN** the response can include `experience_lessons` entries that are backed by active failure patterns and evidence citations

#### Scenario: Insight section is not requested
- **WHEN** a context assembly request does not request experience insight sections
- **THEN** the service preserves the existing context assembly shape and does not include derived insights by default

#### Scenario: Hidden insight exists
- **WHEN** a matching derived insight is suppressed, forgotten, deleted, or out of scope
- **THEN** context assembly excludes that insight from `known_failures` and `experience_lessons`

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

### Requirement: Context assembly can verify replayed insight visibility
The service SHALL allow the operator smoke loop and authorized diagnostics to verify whether replayed active insights participate in optional context assembly sections according to scope, lifecycle, quality, and budget rules.

#### Scenario: Replayed insight is active and requested
- **WHEN** replay produces or updates an active insight that matches a scoped context assembly request with insight sections enabled
- **THEN** context assembly can include that insight in `known_failures` or `experience_lessons` with citations when budget and quality policy allow it

#### Scenario: Replayed insight is hidden or out of scope
- **WHEN** replay preserves, suppresses, skips, or creates an insight outside the request scope or visible lifecycle states
- **THEN** ordinary context assembly excludes that insight even if the replay report references it

#### Scenario: Operator requests context diagnostics
- **WHEN** an authorized admin or debug path requests diagnostics for replayed insight context
- **THEN** the response can identify whether replay output was included, omitted by budget, omitted by quality policy, or hidden by lifecycle and scope rules

