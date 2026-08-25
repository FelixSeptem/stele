# scoped-api-access Specification

## Purpose
Define the baseline authentication and scope isolation contract for protected memory APIs, including API key validation and normalized request scope resolution.
## Requirements
### Requirement: API key protected memory APIs
The service SHALL require API key authentication for protected memory API routes.

#### Scenario: Missing API key on protected route
- **WHEN** a client calls a protected memory route without an API key
- **THEN** the service rejects the request as unauthorized

#### Scenario: Invalid API key on protected route
- **WHEN** a client calls a protected memory route with an invalid API key
- **THEN** the service rejects the request as unauthorized

### Requirement: Request scope resolution
The service SHALL resolve request scope for protected memory routes using `project`, `tenant`, and `namespace` values.

#### Scenario: Valid scoped request
- **WHEN** a client submits a protected request with a valid API key and valid scope values
- **THEN** the service resolves a scope context that is available to downstream handlers

#### Scenario: Missing required scope
- **WHEN** a protected request omits a required scope value
- **THEN** the service rejects the request before executing the handler

### Requirement: Scope-safe handler execution
Protected handlers MUST consume resolved scope context rather than reading unvalidated scope fields directly from the request payload.

#### Scenario: Handler receives validated scope context
- **WHEN** a protected handler executes after authentication and scope resolution
- **THEN** the handler can access the normalized scope context from request context

### Requirement: Authentication and scope resolution are principal-bound
The service SHALL resolve API credentials to durable principals and SHALL
authorize the requested tenant, project, and namespace against the principal's
active exact grants before placing scope context on a protected request.

#### Scenario: Protected handler receives authorized principal and scope
- **WHEN** a request has an active credential, compatible route role, and exact active scope grant
- **THEN** the handler receives normalized principal and scope contexts rather than trusting raw request headers

#### Scenario: Authorization fails before scoped lookup
- **WHEN** credential authentication, route role validation, or exact scope grant validation fails
- **THEN** the service rejects the request before invoking the scoped handler or repository query

#### Scenario: Authorization error does not reveal tenancy
- **WHEN** a caller attempts to access an ungranted tenant, project, or namespace
- **THEN** the response does not reveal whether that scope or any resource within it exists

