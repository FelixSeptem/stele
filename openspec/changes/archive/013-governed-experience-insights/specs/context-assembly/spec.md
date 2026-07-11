## ADDED Requirements

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
