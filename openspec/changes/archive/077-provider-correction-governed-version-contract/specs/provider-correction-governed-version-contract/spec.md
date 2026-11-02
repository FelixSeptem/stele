## Purpose

Provide a closed, exact-scope and binding-owned Provider operation that turns a
verified external correction revision into an asynchronously governed canonical
successor without granting the correction direct mutation authority.

## ADDED Requirements

### Requirement: Correction update contract is separately negotiated and closed

The Provider SHALL advertise `provider-correction-governed-version-v1`
separately from base intent and projection support. A compatible runtime SHALL
use the closed correction-update operation with canonical metadata, one
nonzero projection UUID, one exact target memory UUID/version, bounded update
content and reason. A strict runtime MUST fail closed when the capability or
operation is absent and MUST NOT substitute an unchecked generic evidence
reference or local correction ledger.

#### Scenario: Compatible runtime discovers correction update support
- **WHEN** an authorized runtime reads Provider capabilities from a compatible
  service
- **THEN** the response identifies the correction contract version, operation,
  bounded limits and machine-readable result categories without exposing
  projection, memory, scope, grant or binding identities

#### Scenario: Older service lacks correction update support
- **WHEN** a strict runtime requires the correction-update capability but the
  service does not advertise it
- **THEN** the runtime records a bounded compatibility outcome and does not use
  generic Provider intents or direct canonical mutation as a fallback

### Requirement: Correction attribution is server-resolved before governed handoff

The service MUST resolve a correction update only from a persisted Provider
projection revision that is a later revision of one logical effect, belongs to
the current exact runtime binding and principal, and is the current head of its
logical projection history. The service MUST recheck that the correction has a
currently eligible same-scope durable-reference lineage matching the supplied
target memory and expected target version. Caller-supplied scope, binding,
principal, grant, projection revision, lineage, role, lifecycle or disclosure
overrides MUST NOT be trusted.

#### Scenario: Current correction references the update target
- **WHEN** the current bound runtime submits a correction update naming its
  latest corrected projection revision and a matching eligible target memory
  version
- **THEN** the service persists a verified immutable correction-to-intent
  association and submits one normal governed `update` intent for asynchronous
  processing

#### Scenario: Foreign, missing, stale or mismatched correction is supplied
- **WHEN** a correction update names a foreign binding, missing projection,
  first revision, superseded revision, unavailable reference, hidden source or
  target lineage/version that does not match the correction
- **THEN** the service returns one bounded unavailable or stale outcome before
  creating an idempotency record, intent, work item, candidate or canonical
  mutation and does not reveal which condition occurred

### Requirement: Correction-linked updates remain governed and append-only

Correction update admission MUST only create a durable governed intent; it MUST
NOT mutate canonical memory, lifecycle, ranking, insight state, task state,
lease, commit, publication or delivery state inline. The worker MUST recheck
the correction attribution, exact scope, active target lifecycle, expected
target version and applicable policy before handoff. An accepted update SHALL
create a candidate and append a successor canonical version with immutable
correction, intent, candidate and provenance linkage; prior memory versions and
correction revisions remain readable only through authorized history.

#### Scenario: Worker promotes a verified correction update
- **WHEN** a correction-linked update remains eligible through worker
  processing
- **THEN** the worker records its transition and appends a successor canonical
  version without overwriting the targeted version or claiming a business-state
  receipt

#### Scenario: Source becomes stale before worker processing
- **WHEN** a newer correction revision arrives or the linked reference/target
  becomes unavailable before the worker processes the accepted intent
- **THEN** the worker records a bounded suppressed or rejected transition and
  creates no candidate or canonical successor

### Requirement: Replay, restart and active-signal behavior are deterministic

Correction update idempotency SHALL include the verified correction identity,
current source revision, target identity/version, content, reason, exact scope
and policy identity. Equivalent delivery SHALL return the original intent
result across API or worker restart; conflicting reuse SHALL fail closed.
Superseded correction-derived feedback and summaries MUST be excluded from
active aggregation under current lifecycle/grant checks, while retained
projection, feedback, intent, canonical and audit history remains immutable.

#### Scenario: Equivalent correction update is replayed after restart
- **WHEN** a runtime redelivers the same correction update with the same
  idempotency identity after API or worker restart
- **THEN** the service returns the original intent outcome and does not append a
  second candidate, version, feedback transition or correction association

#### Scenario: Corrected signal is superseded
- **WHEN** a later projection revision supersedes the correction source
- **THEN** active feedback and summary consumers exclude the earlier signal and
  retain bounded supersession history without automatically rewriting an
  already-promoted canonical successor

### Requirement: Correction readiness has owned public durable evidence

Readiness for the correction-update capability SHALL require serialized
OpenAPI conformance, official runtime bootstrap, fresh owned PostgreSQL plus
pgvector evidence for successful and denied attribution, replay/conflict,
append-only successor, active-summary exclusion, and API/worker restart.
Reports MUST retain only bounded categories, counters, contract/migration
provenance and hashes; they MUST omit credentials, scope values, content,
projection IDs, memory IDs, paths, raw payloads and ranking details.

#### Scenario: Evidence is incomplete
- **WHEN** verification has only an HTTP acceptance, in-process result, skipped
  dependency or no restart/provenance evidence
- **THEN** the capability is nonpassing and does not claim correction-update
  readiness
