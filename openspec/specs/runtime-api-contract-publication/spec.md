# runtime-api-contract-publication Specification

## Purpose
TBD - created by archiving change product-ready-self-hosting-foundation. Update Purpose after archive.
## Requirements
### Requirement: API mode publishes the authoritative OpenAPI document
API mode SHALL expose the authoritative OpenAPI document used by repository
contract validation through a stable unauthenticated runtime endpoint. The
served document MUST describe the public routes implemented by the running
image and MUST use an appropriate OpenAPI content type and cache validator.

#### Scenario: Integration discovers the API contract
- **WHEN** an unauthenticated client requests the documented OpenAPI endpoint
  from a healthy API mode
- **THEN** the service returns the complete authoritative OpenAPI document with
  a stable digest or ETag and without requiring repository source access

#### Scenario: Contract has not changed
- **WHEN** a client conditionally requests the OpenAPI endpoint using the
  current cache validator from the same image
- **THEN** the service returns a valid not-modified response or the same
  document digest without rebuilding a different contract

#### Scenario: Protected routes are described
- **WHEN** an integration reads the published OpenAPI document
- **THEN** it can determine the documented authentication, exact scope, request,
  response, idempotency, and error requirements for protected public routes

### Requirement: API mode publishes bounded service compatibility metadata
API mode SHALL expose a stable unauthenticated version and compatibility
endpoint containing only bounded service metadata required by consumers to
identify the running contract and database compatibility.

#### Scenario: Agent Runtime checks provider compatibility
- **WHEN** an external integration requests the documented version endpoint
- **THEN** the response includes service version, build identifier when supplied,
  OpenAPI version or digest, current schema version, and supported migration
  compatibility information

#### Scenario: Build metadata is not supplied
- **WHEN** a locally built image has no version, commit, or build timestamp
  injected
- **THEN** the version endpoint returns documented bounded `unknown` or default
  values while retaining a valid OpenAPI digest and schema version

#### Scenario: Client probes public metadata
- **WHEN** an unauthenticated caller reads the version endpoint
- **THEN** the response excludes database DSNs, secrets, credentials, scope
  values, principal data, migration SQL, operational backlog, and internal error
  details

