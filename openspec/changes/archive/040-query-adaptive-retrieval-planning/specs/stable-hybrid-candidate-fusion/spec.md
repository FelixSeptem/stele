## ADDED Requirements

### Requirement: Approved plans can parameterize bounded fusion
The service SHALL allow a compatible approved retrieval plan to select a subset of existing fusion channels, per-channel candidate limits, an aggregate limit, and an explicit supported fusion strategy with query-family parameters. Planned parameters MUST remain complete, versioned, deterministic, and within fusion and request hard limits.

#### Scenario: Query family selects a channel subset
- **WHEN** an active compatible plan enables lexical and semantic channels but omits optional relation and chunk channels
- **THEN** fusion uses only declared validated candidate lists and does not query or score an omitted channel
