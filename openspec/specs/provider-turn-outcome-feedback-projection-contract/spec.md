# provider-turn-outcome-feedback-projection-contract Specification

## Purpose

Define a versioned, exact-scope Provider contract for recording runtime-reported
turn, outcome and feedback evidence without transferring runtime business-state
authority into Stele.

## Requirements

### Requirement: Projection capability and closed public contract are negotiated

The Provider SHALL advertise `turn-outcome-feedback-projection-v1` separately
from the base schema and SHALL expose dedicated, versioned, closed JSON
projection operations. A strict consumer MUST fail closed when that capability
or required version is absent and MUST NOT reinterpret generic turn, outcome,
or feedback input objects as the projection contract.

#### Scenario: Compatible runtime discovers projection support
- **WHEN** an authorized runtime reads Provider capabilities from a compatible service
- **THEN** the response identifies supported projection operations, contract and
  schema versions, accepted reference-attribution states, bounded limits and
  machine-readable outcome categories without disclosing grants or evidence

#### Scenario: Older service lacks projection support
- **WHEN** a strict runtime requires the projection capability but it is absent
- **THEN** it records a bounded compatibility outcome and does not fall back to
  a generic endpoint, ranked retrieval, or a local business-state assertion

### Requirement: Projection attribution is exact-scope and runtime-binding owned

Every projection SHALL derive tenant, project, namespace, agent, session and
provider-instance attribution from the authenticated runtime binding. It SHALL
accept only bounded operation/effect identity, source-outcome revision,
occurrence time, provenance references, context digest and documented durable
reference selectors; caller-supplied scope, principal, grant, task-state or
authorization overrides MUST be rejected before durable admission.

#### Scenario: Bound runtime records evidence
- **WHEN** an authorized bound runtime submits a syntactically valid projection
- **THEN** the service records the resolved attribution and bounded reported
  evidence without treating any caller field as a wider scope or authority

#### Scenario: Foreign runtime attribution is supplied
- **WHEN** a projection contains a foreign scope, session or binding override
- **THEN** the service returns a bounded scope or validation outcome before
  operation dispatch and reveals no foreign-record existence

#### Scenario: Reference selector is foreign or absent
- **WHEN** a correctly bound projection supplies a foreign or nonexistent reference selector
- **THEN** both selectors receive the same bounded unavailable attribution with
  no event identity, lineage or content and cannot become eligible feedback

### Requirement: Logical projection delivery is idempotent and corrections preserve history

The service SHALL identify a logical projection by exact scope, runtime binding,
projection kind and bounded effect identity. Equivalent delivery with the same
idempotency key and normalized payload MUST return the original projection
identity and disposition; conflicting reuse MUST fail closed. A corrected source
outcome revision SHALL append a new projection or explicit supersession linked
to prior evidence and MUST NOT overwrite or erase earlier evidence.

#### Scenario: Equivalent projection is replayed after restart
- **WHEN** a runtime redelivers an equivalent projection before or after API/worker restart
- **THEN** the service returns the original logical projection identity and no
  duplicate feedback, session outcome, insight input or durable evidence is created

#### Scenario: Reported outcome is corrected
- **WHEN** a runtime submits a later valid source-outcome revision for the same logical effect
- **THEN** the service retains the earlier evidence and records the correction or
  supersession lineage with bounded history rather than mutating it in place

### Requirement: Durable-reference attribution represents governance state without widening access

Projection evidence SHALL allow documented PC5 durable references only under the
same exact scope and SHALL persist their bounded attribution state as accepted,
pending, completed, failed or unavailable. Pending, failed or unavailable
references MUST NOT assert canonical completion, current visibility, authorized
content, task completion, feedback eligibility, or ranking eligibility; later
reads and derivations MUST recheck grants and lifecycle.

#### Scenario: Pending reference is attributed
- **WHEN** a projection includes a same-scope accepted reference whose governance is pending
- **THEN** the service records a bounded pending attribution without returning
  content or treating the reference as completed feedback evidence

#### Scenario: Referenced memory becomes hidden
- **WHEN** a referenced memory is later suppressed, forgotten, deleted or denied
- **THEN** ordinary projection reads and derivations return or retain only the
  bounded unavailable attribution and do not resurrect content or older evidence

### Requirement: Runtime outcomes remain evidence and not business-state receipts

The service SHALL label every accepted projection as runtime-reported evidence.
Neither acceptance, governance, feedback summarization nor derived insight
status SHALL mark a task complete, commit a generation, release an execution
lease, publish an artifact, or confirm webhook delivery.

#### Scenario: Runtime reports a successful execution
- **WHEN** a runtime projects a reported successful outcome with task-completion context
- **THEN** Stele stores only the bounded evidence and does not expose it as a
  task, execution, compilation, publication or delivery state transition

### Requirement: Projection feedback feeds governed paths asynchronously and remains separately activated

The service SHALL map admitted usefulness and user-feedback evidence to existing
scoped feedback and insight inputs only through asynchronous, evidence-backed,
versioned and lifecycle-filtered paths. Recording a projection MUST NOT directly
change retrieval ranking, memory lifecycle, insight visibility or activation;
those effects require the existing summary, policy and activation gates.

#### Scenario: Feedback is recorded but no ranking activation is authorized
- **WHEN** a projection contains valid feedback while no compatible activation policy is enabled
- **THEN** the feedback remains durable scoped evidence and retrieval behavior is unchanged

#### Scenario: Governed derivation consumes eligible feedback
- **WHEN** an asynchronous policy later consumes active, in-scope, eligible projection feedback
- **THEN** it records its policy and evidence attribution and preserves the
  projection and feedback history without direct runtime-controlled activation

### Requirement: Projection readiness requires public durable evidence

Projection readiness SHALL require a fresh owned PostgreSQL + pgvector public
fixture, official bootstrap, serialized OpenAPI conformance, exact-scope
authorization, duplicate/conflict/correction coverage, pending/completed/hidden
reference attribution, and API/worker restart recovery. Reports MUST pin bounded
service/OpenAPI/contract/migration provenance and omit credentials, scope values,
raw payloads, reference values and unbounded feedback text.

#### Scenario: Incomplete evidence is observed
- **WHEN** verification has only HTTP success, an in-process result, skipped
  dependencies or no restart/provenance proof
- **THEN** readiness is nonpassing and does not claim durable projection support

