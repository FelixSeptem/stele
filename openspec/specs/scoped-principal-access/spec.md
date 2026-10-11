# scoped-principal-access Specification

## Purpose
Authenticate protected requests through durable principals and safely managed credentials.

## Requirements

### Requirement: Protected requests authenticate as durable principals
The service SHALL authenticate protected requests using a durable PostgreSQL
principal and active credential, and SHALL never persist or return a raw
credential after its one-time issuance response.

#### Scenario: Active credential authenticates a principal
- **WHEN** a caller presents an active, unexpired credential issued for a durable principal
- **THEN** the service resolves the principal identity and bounded role before route authorization

#### Scenario: Credential is disabled or expired
- **WHEN** a caller presents a disabled, revoked, unknown, or expired credential
- **THEN** the service rejects the request without exposing principal, credential, or grant existence

#### Scenario: Credential is inspected after issuance
- **WHEN** an administrator reads a principal or credential after creation or rotation
- **THEN** the response excludes raw credential material and one-way credential digests

### Requirement: Principal grants authorize exact request scope
The service SHALL require an active exact tenant, project, and namespace grant
for the authenticated principal before any protected handler receives scope
context.

#### Scenario: Granted public scope is used
- **WHEN** a public principal requests a scope for which it has an active exact grant
- **THEN** the service passes the normalized granted scope and principal context to the protected handler

#### Scenario: Caller changes to an ungranted scope header
- **WHEN** an authenticated principal supplies a valid-looking scope header outside its grants
- **THEN** the service rejects the request before reading or writing scoped resources

#### Scenario: Admin route requires admin role
- **WHEN** a public-role principal requests an admin route for an otherwise granted scope
- **THEN** the service rejects the request without executing the admin handler

### Requirement: Principal and credential lifecycle is bounded and auditable
The service SHALL allow administrators to create, rotate, disable, expire, and
inspect principals, credentials, and exact scope grants through an admin-only
surface, preserving bounded lifecycle audit history.

#### Scenario: Administrator creates scoped principal
- **WHEN** an authorized administrator creates a principal with a bounded role and one or more valid exact grants
- **THEN** the service persists the principal, grant history, and credential lifecycle record and returns the generated raw credential exactly once

#### Scenario: Administrator rotates credential
- **WHEN** an authorized administrator rotates a principal credential with actor and reason attribution
- **THEN** the service disables the prior credential, issues one new raw credential response, and preserves bounded rotation audit history

#### Scenario: Administrator revokes grant
- **WHEN** an authorized administrator revokes a principal's exact scope grant
- **THEN** subsequent requests for that scope are rejected immediately without mutating scoped memory records

### Requirement: Bootstrap operator is constrained to initial administration
The service SHALL support a configuration-supplied bootstrap operator only for
initial durable principal administration and SHALL restrict it to the configured
default scope.

#### Scenario: First deployment creates durable admin principal
- **WHEN** no durable admin principal exists and an operator presents the configured bootstrap credential for the configured default scope
- **THEN** the operator can create the first durable admin principal and exact grant

#### Scenario: Bootstrap credential is used outside default scope
- **WHEN** a caller presents the bootstrap credential with a scope other than the configured default scope
- **THEN** the service rejects the request before handler execution

#### Scenario: Durable admin already exists
- **WHEN** a durable admin principal exists and no explicit emergency override is enabled
- **THEN** the bootstrap credential is rejected and cannot bypass principal authorization

### Requirement: Access telemetry and audit are bounded
The service SHALL record authentication, authorization, credential lifecycle,
and grant lifecycle outcomes using bounded categories without raw credentials,
credential digests, principal labels, scope values, request payloads, or
free-form reasons in metrics or non-admin logs.

#### Scenario: Authorization is denied
- **WHEN** a credential is denied because it is invalid, expired, role-incompatible, or ungranted
- **THEN** telemetry records a bounded denial category without sensitive identifiers or secrets

### Requirement: Exact-scope disclosure and attestation entitlements are administrator owned

The service SHALL expose an admin-only, durable, auditable way to provision and update each public principal's exact-scope allowed disclosure profiles, default profile, and separate source-attestation entitlement. Profile lists MUST be bounded and drawn from the supported server registry; the default MUST belong to the allowed list. Missing disclosure configuration MUST deny public protected memory disclosure, and source attestation MUST default to false. Caller runtime initialization, request metadata, role hints, paths, or tokens MUST NOT modify these entitlements. Changes MUST retain actor/reason audit attribution without credentials, payloads, or unrestricted metadata in non-admin telemetry.

#### Scenario: Administrator provisions a reference-only runtime principal

- **WHEN** an authorized administrator creates or updates a public principal's exact-scope grant with only `memory-reference-v1`
- **THEN** the service persists the reference-only ceiling and configured default with an auditable grant change

#### Scenario: Default is not authorized or profile is unsupported

- **WHEN** an administrator submits an invalid/overlong profile list, unknown profile, or default outside the allowed list
- **THEN** the change is refused without creating a partially valid disclosure grant

#### Scenario: Runtime tries to provision itself

- **WHEN** a public runtime supplies disclosure or attestation entitlements through initialization or ordinary request metadata
- **THEN** the service rejects unsupported authority fields or treats application metadata only as data and does not modify its grant

#### Scenario: Legacy public grant lacks disclosure configuration

- **WHEN** a migrated public grant has no explicitly provisioned disclosure profile
- **THEN** protected memory disclosure is refused and no implicit content or attestation permission is assigned

### Requirement: Disclosure authority is rechecked across bindings and transports

The service MUST enforce current exact-scope disclosure entitlement on each protected public response, independently of runtime-binding creation and transport. A narrowed/revoked grant MUST take effect on subsequent requests using an existing binding or direct native/MCP credential. Read/write access mode, a public role, and an old successful operation MUST NOT override a narrower disclosure ceiling. Administrative inspection MUST remain authorized through its separate role and exact-scope rules.

#### Scenario: Content grant is downgraded while binding remains active

- **WHEN** an administrator changes a principal from content-capable to reference-only
- **THEN** its next bounded-content or alternate content read is refused and its authorized reference context remains available

#### Scenario: Same credential switches transport

- **WHEN** a constrained principal changes from Provider HTTP to native HTTP or MCP
- **THEN** its current disclosure ceiling continues to apply without dependence on Provider headers

#### Scenario: Grant authority is unavailable

- **WHEN** authoritative current entitlement lookup fails
- **THEN** protected disclosure fails closed with a bounded dependency outcome instead of using stale broad grants
