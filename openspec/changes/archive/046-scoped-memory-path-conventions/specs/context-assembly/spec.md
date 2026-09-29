## ADDED Requirements

### Requirement: Context assembly honors path selectors within its existing envelope
Context assembly SHALL accept optional normalized `path` and explicit `path_prefix` selectors and SHALL restrict eligible evidence to those selectors after exact scope, lifecycle, temporal, projection, quality, and authorization checks. Path filtering MUST NOT increase the caller budget or alter section names and citation rules.

#### Scenario: Context is restricted to an exact path
- **WHEN** a caller assembles context with `path=agents/research`
- **THEN** only eligible evidence at that exact path contributes to existing sections

#### Scenario: Context uses an explicit prefix
- **WHEN** a caller assembles context with `path_prefix=agents/research`
- **THEN** eligible evidence at the prefix and segment descendants may contribute subject to existing ranking and budget rules

#### Scenario: Prefix cannot escape scope
- **WHEN** a path prefix resembles a broader organizational namespace
- **THEN** context assembly still evaluates only records in the resolved exact tenant, project, and namespace scope

#### Scenario: Path-filtered evidence exceeds budget
- **WHEN** matching evidence cannot fit the existing context budget
- **THEN** the assembler applies its existing omission and diagnostic behavior without fetching broader paths or increasing the budget
