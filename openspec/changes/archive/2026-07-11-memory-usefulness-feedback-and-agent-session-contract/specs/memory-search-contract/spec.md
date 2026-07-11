## ADDED Requirements

### Requirement: Search results can include usefulness diagnostics
The service SHALL allow retrieval responses or diagnostics to include bounded usefulness feedback signals for returned memory hits.

#### Scenario: Search hit has usefulness summary
- **WHEN** a search result includes a memory with feedback history in the requested scope
- **THEN** the response or diagnostics can include aggregate usefulness categories such as useful, noisy, stale, irrelevant, missing expected, or needs review

#### Scenario: Search diagnostics include unsafe feedback
- **WHEN** feedback indicates a retrieval safety issue such as hidden memory being observed
- **THEN** retrieval diagnostics expose a stable safety code without exposing hidden memory content through public search results

### Requirement: Search ranking remains stable unless feedback-aware ranking is explicit
The service MUST keep default search ranking behavior stable unless feedback-aware ranking is explicitly requested for that search request.

#### Scenario: Default search runs
- **WHEN** a caller searches memory without feedback-aware ranking enabled
- **THEN** usefulness feedback does not silently alter ranking or filtering

#### Scenario: Feedback-aware search runs
- **WHEN** feedback-aware ranking is explicitly enabled for one search request
- **THEN** retrieval may use usefulness summaries as bounded ranking hints while preserving lifecycle and scope isolation guarantees

#### Scenario: Scope-wide ranking policy is requested
- **WHEN** an operator attempts to enable feedback-aware search ranking as a default scope-wide policy in this change
- **THEN** the service rejects or ignores that setting because this proposal only supports explicit per-request ranking hints
