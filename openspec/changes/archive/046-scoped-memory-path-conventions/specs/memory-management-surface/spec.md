## ADDED Requirements

### Requirement: Canonical memory resources expose path-aware filtering
The public canonical memory resource surface SHALL expose normalized memory path metadata and SHALL support exact `path` and explicit descendant `path_prefix` filters within the already resolved exact scope. Omitted paths SHALL preserve whole-scope behavior.

#### Scenario: Client browses one exact path
- **WHEN** a client lists canonical memory with `path=agents/research`
- **THEN** the service returns only lifecycle-visible resources at that exact path with stable pagination

#### Scenario: Client browses a path prefix
- **WHEN** a client lists canonical memory with `path_prefix=agents/research`
- **THEN** the service returns lifecycle-visible resources at the prefix and its segment descendants without including sibling prefixes

#### Scenario: Detail response reports path
- **WHEN** a client reads a visible canonical memory resource
- **THEN** its stable representation includes the normalized path or root-path value without exposing storage-specific columns
