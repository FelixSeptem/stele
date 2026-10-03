## MODIFIED Requirements

### Requirement: Intent envelope is explicitly typed and scoped

The service MUST accept only `remember`, `update`, `forget`, `contradiction`, and `feedback` intent types, and every intent MUST include an exact tenant, project, namespace, actor, reason, request identity, idempotency key, and bounded payload or evidence references. Intent admission MUST apply the governed-operation precedence contract so scope, lifecycle visibility, principal grant, and explicit intent-policy enablement are checked before idempotency lookup or queue handoff.

#### Scenario: Valid intent is accepted for durable processing

- **WHEN** a caller submits a bounded supported intent with a valid exact scope, required attribution, and enabled compatible policy
- **THEN** the service persists one pending intent record and returns its stable intent ID and status after the earlier precedence gates pass

#### Scenario: Foreign or malformed intent is rejected

- **WHEN** an intent has an invalid scope, unsupported type, missing actor/reason, oversized payload, or evidence outside the request scope
- **THEN** the service rejects it at the first applicable precedence gate without creating an idempotency record, candidate, queue item, or canonical-memory mutation

### Requirement: Idempotency replays the original result

The service MUST scope idempotency by tenant, project, namespace, and idempotency key, and MUST evaluate it only after scope, lifecycle, principal grant, and compatible intent-policy checks pass. An identical retry MUST return the original intent result, while a conflicting payload for the same key MUST fail closed with an idempotency conflict.

#### Scenario: Identical retry is replayed

- **WHEN** an authorized caller resubmits a byte-equivalent intent type, scope, attribution, payload, target identity, and policy version with the same idempotency key
- **THEN** the service returns the original intent ID and outcome without creating another intent or processing transition

#### Scenario: Conflicting retry is rejected

- **WHEN** a caller reuses an idempotency key with a different normalized request fingerprint after passing earlier gates
- **THEN** the service returns a conflict and preserves the first request and its history without exposing its payload
