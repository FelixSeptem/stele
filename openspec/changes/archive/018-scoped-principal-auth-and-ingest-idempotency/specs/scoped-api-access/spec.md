## ADDED Requirements

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
