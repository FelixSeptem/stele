# governed-operation-policy-precedence Specification

## Purpose
Define one observable, versioned precedence contract for every governed operation so exact scope isolation, lifecycle safety, authorization, policy approval, replay handling, governance handoff, and mutation boundaries behave consistently across adapters.

## Requirements

### Requirement: Governed operations evaluate gates in a fixed order

Every governed operation MUST evaluate the following gates in order: exact tenant, project, and namespace scope; lifecycle visibility of referenced records; principal grant and role; explicit approval or policy enablement and version compatibility; idempotency and replay identity; governance handoff; and finally canonical or derived mutation. A failed earlier gate MUST stop evaluation of all later gates.

#### Scenario: Foreign scope is rejected before state lookup

- **WHEN** a request names a tenant, project, or namespace outside the authenticated principal's exact grant
- **THEN** the service returns a bounded scope denial without checking lifecycle state, policy state, idempotency records, or target existence

#### Scenario: Hidden target is rejected before handoff

- **WHEN** a request is in scope but its target is suppressed, forgotten, deleted, or otherwise invisible to the operation
- **THEN** the service returns a bounded lifecycle denial and does not invoke a provider, enqueue governance work, or mutate records

### Requirement: Decisions carry bounded compatibility and attribution metadata

Each governed decision MUST carry a bounded decision version, operation kind, exact-scope proof, principal and grant reference, policy and approval version when applicable, request and operation identity, idempotency identity, and audit attribution. Public responses and non-admin telemetry MUST expose only stable categories, versions, and redacted references.

#### Scenario: Compatible decision is handed off

- **WHEN** all gates pass for a supported operation
- **THEN** the service emits one decision with the normalized metadata and passes that decision to the existing governance or mutation boundary

#### Scenario: Incompatible precedence version fails closed

- **WHEN** a caller or stored work item presents an unsupported decision or precedence version
- **THEN** the service returns a bounded compatibility denial and performs no handoff or mutation

### Requirement: Replay and conflict behavior is deterministic

Idempotency MUST be evaluated only after scope, lifecycle, grant, and policy gates pass. An identical normalized request MUST return the original bounded decision and outcome without a second transition; conflicting reuse MUST fail closed without revealing the original payload or target existence.

#### Scenario: Identical replay returns the first outcome

- **WHEN** an authorized caller retries an operation with equivalent normalized scope, policy, operation, payload fingerprint, and idempotency key
- **THEN** the service returns the original operation identity and bounded outcome without enqueuing or mutating again

#### Scenario: Conflicting replay is denied

- **WHEN** an authorized caller reuses an idempotency key with a different operation kind, scope proof, policy version, or payload fingerprint
- **THEN** the service returns a bounded idempotency conflict and preserves the first decision and its history

### Requirement: Governance handoff and mutation remain separate

Passing precedence gates MUST authorize only the existing governance handoff appropriate to the operation. Canonical memory and derived insight mutation MUST remain append-only, provenance-linked, and subject to their existing admission or lifecycle controls; provider or model output MUST NOT be treated as an admission decision.

#### Scenario: Accepted intent is handed to the durable path

- **WHEN** an intent passes all precedence gates
- **THEN** the service records the decision and hands off to the existing durable asynchronous candidate, governance, or lifecycle path without claiming activation inline

#### Scenario: Provider attempts a bypass

- **WHEN** provider output requests direct canonical mutation, lifecycle activation, or policy grant
- **THEN** the service rejects the handoff with a bounded governance denial and leaves canonical and derived records unchanged

### Requirement: Rollback and re-enable preserve exact-scope history

Disabling or rolling back a policy MUST stop new acceptance at the approval or policy gate while retaining submitted work, decisions, audit, provenance, and lifecycle history. Re-enabling a compatible policy MUST resume only eligible pending work through the complete precedence sequence and exact original scope.

#### Scenario: Rollback blocks new work and preserves history

- **WHEN** an operator rolls back a policy for one exact scope
- **THEN** new operations under that policy are rejected or held according to the policy, prior records remain inspectable, and other scopes and policy versions continue independently

#### Scenario: Compatible re-enable resumes eligible work

- **WHEN** an operator explicitly enables a compatible policy version for the same exact scope
- **THEN** only eligible pending work resumes after fresh scope, lifecycle, grant, approval, and idempotency checks

### Requirement: Conformance proves precedence without activating data

The service MUST provide bounded conformance fixtures for each representative governed adapter. Fixtures MUST verify gate ordering, exact isolation, lifecycle filtering, grant and policy denial, replay/conflict behavior, handoff safety, rollback/re-enable, append-only history, and redaction without executing an external agent or changing default retrieval/context behavior.

#### Scenario: Conformance records a stage-specific failure

- **WHEN** a fixture intentionally violates one precedence gate
- **THEN** the run records only the fixed stage and outcome category, returns no hidden or foreign data, and leaves canonical records unchanged

#### Scenario: Conformance passes all gates

- **WHEN** all fixtures complete against one exact scope with compatible dependencies
- **THEN** the service records a passing bounded run with contract versions, counters, and cited in-scope evidence without claiming broader readiness
