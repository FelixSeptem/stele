## ADDED Requirements

### Requirement: Administrators manage scoped principals through an admin boundary
The service SHALL expose admin-only routes to create, read, list, rotate,
disable, and expire principals and credentials, and to create, list, and revoke
exact scope grants.

#### Scenario: Administrator inspects principal access
- **WHEN** an authorized administrator reads a principal within an authorized scope
- **THEN** the response includes bounded role, status, timestamps, and exact grants without raw credentials, digests, or unrelated scope data

#### Scenario: Administrator requests ungranted principal record
- **WHEN** an administrator requests a principal, credential, grant, or audit record outside the administrator's authorized scope
- **THEN** the service rejects the request without exposing record existence

#### Scenario: Credential issuance response is not replayable through inspection
- **WHEN** an administrator creates or rotates a principal credential and later lists or reads principal administration records
- **THEN** the raw credential appears only in the original issuance response and never in inspection responses
