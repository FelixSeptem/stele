## ADDED Requirements

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
