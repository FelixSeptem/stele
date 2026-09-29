# scoped-memory-path-conventions Specification

## Purpose
Define a bounded, portable path convention for organizing and selecting memory inside an already authorized tenant, project, and namespace scope.

## Requirements

### Requirement: Memory paths are normalized and bounded
The service SHALL accept an optional memory path on governed memory inputs and SHALL expose it as a normalized slash-separated value on path-aware memory representations. A normalized path MUST use non-empty segments, MUST reject `.` and `..`, wildcards, encoded separators, and control characters, and MUST obey documented total-length and per-segment bounds. The root path MUST have one stable representation.

#### Scenario: Valid path is normalized
- **WHEN** a caller supplies a path with permitted segments and separator formatting
- **THEN** the service stores and returns one canonical representation without changing its authorized tenant, project, or namespace

#### Scenario: Malformed path is rejected
- **WHEN** a caller supplies an empty segment, traversal segment, wildcard, encoded separator, control character, or overlong path
- **THEN** the service rejects the request before persistence or retrieval and returns a bounded validation category

#### Scenario: Path is omitted
- **WHEN** a caller omits the optional path
- **THEN** the service treats the memory as belonging to the canonical root path for compatibility

### Requirement: Path selectors have explicit exact and prefix semantics
The service SHALL interpret a supplied `path` selector as an exact normalized path match. Recursive descendant matching SHALL require an explicit `path_prefix` selector, and a request MUST NOT use both selectors with conflicting values.

#### Scenario: Exact path search is requested
- **WHEN** a caller supplies `path=agents/research`
- **THEN** only records whose normalized path is exactly `agents/research` are eligible

#### Scenario: Descendant path search is requested
- **WHEN** a caller supplies `path_prefix=agents/research`
- **THEN** records at `agents/research` and below it at segment boundaries are eligible, while sibling paths such as `agents/researcher` are excluded

#### Scenario: Recursive matching is not implicit
- **WHEN** a caller supplies `path=agents/research` without `path_prefix`
- **THEN** descendant records are not returned solely because they share a textual prefix

### Requirement: Path filtering cannot widen authorization
Path matching SHALL execute only after the request has resolved an authorized exact tenant, project, and namespace scope. A path or path prefix MUST NOT select records outside that scope, and omitted scope MUST NOT be inferred from a path.

#### Scenario: Authorized path filter is applied
- **WHEN** a caller requests a valid path within an authorized exact scope
- **THEN** filtering is applied only to records in that scope

#### Scenario: Path attempts to cross scope
- **WHEN** a caller uses a path value that resembles another tenant, project, or namespace
- **THEN** the service treats it only as path data within the resolved scope and never returns records from another scope

### Requirement: Path behavior is consistent across public surfaces
Ingestion, search, canonical memory reads, context assembly, and MCP operations SHALL accept, validate, propagate, and report the same normalized path and exact-versus-prefix semantics. Lifecycle visibility, temporal selectors, budgets, citations, and idempotency rules SHALL remain in force.

#### Scenario: Same selector is used across surfaces
- **WHEN** a caller uses an equivalent path selector for search, browse, context, and MCP
- **THEN** each surface returns the same path eligibility set subject to that surface's existing ranking, budget, and lifecycle rules

#### Scenario: Hidden record matches a path
- **WHEN** a suppressed, forgotten, expired, or deleted record matches an otherwise valid path selector
- **THEN** default public surfaces continue to exclude it

### Requirement: Existing records and pagination remain compatible
Existing records without path metadata SHALL be represented as root-path records. Path-aware list and search operations SHALL retain stable bounded pagination and deterministic ordering, and equivalent path selectors SHALL participate in request normalization used by idempotency and cache keys.

#### Scenario: Legacy record is queried at root
- **WHEN** a caller queries the root path in a scope containing a pre-migration record
- **THEN** the record is eligible under the same lifecycle and authorization rules as a newly written root-path record

#### Scenario: Path selector changes a retry fingerprint
- **WHEN** an otherwise identical governed write is retried with a different normalized path
- **THEN** the service treats the normalized payloads as different for idempotency and does not return the prior path's outcome

#### Scenario: Paginated path query is repeated
- **WHEN** a caller repeats a bounded path-filtered list or search request with the same cursor and normalized selectors
- **THEN** the service preserves deterministic page boundaries and does not broaden the path match
