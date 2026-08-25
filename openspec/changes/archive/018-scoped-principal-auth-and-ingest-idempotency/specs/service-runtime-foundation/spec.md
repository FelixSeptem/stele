## ADDED Requirements

### Requirement: Principal bootstrap configuration is fail-safe
The service MUST validate bootstrap-operator and deprecated legacy key settings
before serving protected traffic or starting background execution.

#### Scenario: Bootstrap configuration is incomplete
- **WHEN** a bootstrap credential is configured without a valid default tenant, project, and namespace
- **THEN** startup fails with an actionable configuration error

#### Scenario: Deprecated unrestricted key configuration is present
- **WHEN** legacy public or admin key allow-list settings are configured without the explicit constrained bootstrap migration setting
- **THEN** startup fails before the service accepts protected requests

#### Scenario: Principal-backed runtime starts
- **WHEN** PostgreSQL principal storage and valid principal-access configuration are available
- **THEN** API mode starts protected routes and worker or scheduler modes use the same principal-access configuration without exposing credentials
