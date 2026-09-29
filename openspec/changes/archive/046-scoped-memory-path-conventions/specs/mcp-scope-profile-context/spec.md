## ADDED Requirements

### Requirement: MCP path arguments delegate to the shared path contract
MCP search, context, browse, remember, and forget tools SHALL expose only the bounded path fields supported by their delegated OpenAPI/service contract. `path` SHALL be exact, `path_prefix` SHALL be explicit for descendant matching, and all selectors SHALL be evaluated only inside the server-resolved exact grant.

#### Scenario: MCP exact path search
- **WHEN** an authorized MCP caller supplies `path=agents/research`
- **THEN** the adapter delegates an exact-path operation and returns no descendant records unless the delegated operation explicitly uses `path_prefix`

#### Scenario: MCP prefix search
- **WHEN** an authorized MCP caller supplies `path_prefix=agents/research`
- **THEN** the adapter delegates bounded descendant matching within the verified grant and preserves lifecycle, budget, and citation safety

#### Scenario: MCP path attempts to widen a grant
- **WHEN** a caller uses path syntax to reference a different tenant, project, or namespace
- **THEN** the adapter treats it as invalid or ordinary path data in the selected grant and never broadens scope

#### Scenario: MCP mutation retry includes path
- **WHEN** an equivalent remember or forget operation is retried with the same normalized path and idempotency metadata
- **THEN** the adapter returns the original delegated outcome without duplicate governed side effects
