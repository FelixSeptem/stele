## ADDED Requirements

### Requirement: Memory search supports exact and explicit prefix path filters
The public memory search contract SHALL accept optional normalized `path` and `path_prefix` selectors. `path` SHALL match exactly; `path_prefix` SHALL match descendants only at segment boundaries; and the service MUST reject conflicting or invalid selectors before retrieval.

#### Scenario: Exact path search excludes descendants
- **WHEN** a caller searches with `path=agents/research`
- **THEN** the response includes only exact-path hits and excludes `agents/research/preferences`

#### Scenario: Prefix search includes descendants
- **WHEN** a caller searches with `path_prefix=agents/research`
- **THEN** the response may include the root path and descendant paths inside the authorized scope, subject to existing ranking, temporal, lifecycle, and top-k rules

#### Scenario: Invalid path selector is bounded
- **WHEN** a caller supplies malformed path syntax or both incompatible selectors
- **THEN** the service returns a validation error without revealing records or changing ranking state
