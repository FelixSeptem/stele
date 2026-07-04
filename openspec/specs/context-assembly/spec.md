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

