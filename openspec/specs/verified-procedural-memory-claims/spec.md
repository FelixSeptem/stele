# verified-procedural-memory-claims Specification

## Purpose
Record attributable verification evidence for procedural-memory claims, trigger
bounded revalidation, and safely hand a refuted procedure to existing governed
memory correction without making verification a second authority.

## Requirements

### Requirement: Procedural claim verification is exact-scope and append-only
The service SHALL record a procedural-memory verification only for one active,
current procedural canonical-memory version in one exact tenant, project, and
namespace scope. Every verification record MUST retain a stable identity, target
memory/version, claim and applicability fingerprints, disposition, method,
actor, reason, policy version, source watermark, bounded evidence references,
and recorded time. A verification record MUST NOT overwrite an earlier record,
change the target memory, change lifecycle state, grant access, or constitute a
new memory class.

#### Scenario: Administrator records a supported procedure
- **WHEN** an authorized administrator records a bounded `supported`
  verification for an active current procedural-memory version with valid
  claim/applicability fingerprints and same-scope evidence
- **THEN** the service appends an immutable verification record and preserves
  the target memory/version and all earlier verification history

#### Scenario: Verification targets an ineligible memory
- **WHEN** a request names a foreign, hidden, forgotten, deleted, noncurrent,
  or nonprocedural memory/version
- **THEN** the service rejects the request before creating a verification,
  revalidation work item, correction intent, candidate, or canonical update
  without revealing the target condition

### Requirement: Verification distinguishes evidence attribution from corroboration
The service MUST store evidence method and source attribution separately from a
verification disposition. A transcript, model-generated assertion, repeated
self-citation, popularity signal, confidence score, or task verdict alone MUST
NOT be accepted as independent corroboration for a `supported` procedure.
Contrary evidence MUST be retainable for a `refuted` or `inconclusive`
verification without deleting supporting evidence or prior results.

#### Scenario: Model assertion lacks independent evidence
- **WHEN** an administrator submits a `supported` verification whose only
  evidence is transcript or model-generated assertion material
- **THEN** the service rejects the unsupported corroboration claim or records
  only an `inconclusive` result according to the declared policy, without
  promoting the procedure or changing canonical memory

#### Scenario: Contradictory evidence is recorded
- **WHEN** attributable same-scope evidence contradicts a previously supported
  procedural claim
- **THEN** the service appends a `refuted` or `inconclusive` verification with
  the contrary evidence and retains the prior supported record for authorized
  audit

### Requirement: Revalidation is bounded and source-version aware
The service SHALL create bounded exact-scope revalidation work only from an
explicit administrator request, an eligible correction signal, source-version
drift, or eligible stale/review feedback. The work item MUST identify the
target verification/memory version and source watermark, enforce policy and
attempt limits, use lease-safe retry, and append a terminal `supported`,
`refuted`, `inconclusive`, or `needs_revalidation` result. A changed, missing,
hidden, foreign, forgotten, or deleted source MUST produce a nonactive result
and MUST NOT broaden evidence selection.

#### Scenario: Source version changes after verification
- **WHEN** a source version cited by a current verification is superseded or no
  longer eligible
- **THEN** a later revalidation appends `needs_revalidation` with bounded
  source-change attribution and leaves the prior verification immutable

#### Scenario: Revalidation is retried after interruption
- **WHEN** a worker loses its lease or restarts while processing one eligible
  revalidation work item
- **THEN** the durable work can resume within its attempt limit without
  duplicate terminal verification records or a duplicate correction handoff

### Requirement: Refuted procedural claims use governed successor correction
A refuted verification MAY carry one bounded administrator-supplied corrected
procedural payload and reason. The service MUST hand it to ordinary governed
`update` intent processing with immutable verification attribution, expected
target version, scope, policy, and idempotency identity. Verification submission
and revalidation MUST NOT directly mutate canonical memory. The worker MUST
recheck current target/version, verification disposition, source eligibility,
scope, and policy before a candidate or successor is created.

#### Scenario: Refuted claim is corrected through governance
- **WHEN** a current refuted verification with an eligible corrected payload is
  accepted under an enabled policy
- **THEN** the service records an attributed governed update intent and later
  appends a successor canonical version only if ordinary governance accepts it

#### Scenario: Verification or target becomes stale before promotion
- **WHEN** the verification is superseded, a cited source becomes ineligible,
  the target version changes, or the policy is disabled before worker processing
- **THEN** the worker records a bounded nonactive intent/revalidation outcome
  and creates no candidate or canonical successor

### Requirement: Verification remains lifecycle-safe and inspectable only by administrators
The service SHALL provide exact-scope, administrator-only creation and
inspection of procedural claim verification and revalidation history. Responses
MUST be bounded and disclose only lifecycle-eligible evidence references and
authorized audit metadata. Ordinary retrieval, context assembly, public Provider
credentials, and MCP public-runtime paths MUST NOT gain a verification-history
or correction-discovery route.

#### Scenario: Administrator reads verification history
- **WHEN** an authorized administrator requests verification history for an
  in-scope procedural memory
- **THEN** the service returns stable append-only dispositions, source-watermark
  identities, bounded evidence/provenance references, and correction handoff
  status without foreign or hidden evidence content

#### Scenario: Public runtime requests verification data
- **WHEN** a public Provider credential or nonadministrator route requests
  procedural verification history or revalidation state
- **THEN** the service denies the request before record lookup and does not
  disclose verification, memory, or evidence existence
