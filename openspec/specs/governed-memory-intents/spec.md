# governed-memory-intents Specification

## Purpose
Provide one exact-scope, append-only, idempotent contract for requested memory operations so adapters can submit governed work without mutating canonical memory directly.

## Requirements

### Requirement: Intent envelope is explicitly typed and scoped

The service MUST accept only `remember`, `update`, `forget`, `contradiction`, and `feedback` intent types, and every intent MUST include an exact tenant, project, namespace, actor, reason, request identity, idempotency key, and bounded payload or evidence references. Intent admission MUST apply the governed-operation precedence contract so scope, lifecycle visibility, principal grant, and explicit intent-policy enablement are checked before idempotency lookup or queue handoff.

#### Scenario: Valid intent is accepted for durable processing

- **WHEN** a caller submits a bounded supported intent with a valid exact scope, required attribution, and enabled compatible policy
- **THEN** the service persists one pending intent record and returns its stable intent ID and status after the earlier precedence gates pass

#### Scenario: Foreign or malformed intent is rejected

- **WHEN** an intent has an invalid scope, unsupported type, missing actor/reason, oversized payload, or evidence outside the request scope
- **THEN** the service rejects it at the first applicable precedence gate without creating an idempotency record, candidate, queue item, or canonical-memory mutation

### Requirement: Intent lifecycle is append-only and explicit
The service MUST represent processing outcomes with explicit `pending`, `accepted`, `rejected`, `suppressed`, `failed`, and `replayed` states, preserving every state transition with actor, reason, timestamp, and bounded diagnostic category.

#### Scenario: Governance accepts an intent
- **WHEN** the asynchronous processor completes all scope, lifecycle, provenance, and policy checks
- **THEN** it records an accepted outcome and the resulting candidate or lifecycle work reference without deleting the original intent

#### Scenario: Governance suppresses or rejects an intent
- **WHEN** policy, evidence, lifecycle, or authorization checks fail
- **THEN** it records a suppressed or rejected outcome with a stable bounded reason and keeps the original request inspectable

### Requirement: Idempotency replays the original result

The service MUST scope idempotency by tenant, project, namespace, and idempotency key, and MUST evaluate it only after scope, lifecycle, principal grant, and compatible intent-policy checks pass. An identical retry MUST return the original intent result, while a conflicting payload for the same key MUST fail closed with an idempotency conflict.

#### Scenario: Identical retry is replayed

- **WHEN** an authorized caller resubmits a byte-equivalent intent type, scope, attribution, payload, target identity, and policy version with the same idempotency key
- **THEN** the service returns the original intent ID and outcome without creating another intent or processing transition

#### Scenario: Conflicting retry is rejected

- **WHEN** a caller reuses an idempotency key with a different normalized request fingerprint after passing earlier gates
- **THEN** the service returns a conflict and preserves the first request and its history without exposing its payload

### Requirement: Submission never mutates canonical memory inline
Intent submission MUST hand off to the existing asynchronous candidate, governance, lifecycle, provenance, and durable work queue paths; the submission response MUST NOT claim canonical activation before those paths complete.

#### Scenario: Submit returns before governance processing
- **WHEN** a valid intent is submitted through an API or adapter
- **THEN** the response reports pending or accepted-for-processing and canonical memory remains unchanged in the submission transaction

#### Scenario: Worker resumes an intent after restart
- **WHEN** a worker restarts after claiming an intent or loses its lease
- **THEN** the durable queue can reclaim and retry the intent using the same intent identity without duplicate canonical transitions

### Requirement: Intent status and history are exact-scope inspection surfaces
The service MUST expose scoped status and append-only history for an intent, including lifecycle outcomes, target references, provenance references, and bounded diagnostics, while ordinary retrieval and context MUST exclude pending, rejected, suppressed, failed, and replay metadata.

#### Scenario: Authorized caller reads intent history
- **WHEN** an authorized caller requests an intent within the exact owning scope
- **THEN** the service returns stable ordered transitions and provenance references without leaking foreign-scope or hidden content

#### Scenario: Foreign caller reads an intent
- **WHEN** a caller requests an intent using a different tenant, project, or namespace
- **THEN** the service returns not found or an equivalent non-disclosing response

### Requirement: Rollback disables new processing without erasing history
The service MUST support scoped intent processing disablement or rollback that prevents new acceptance and handoff while retaining submitted intents, outcomes, provenance, and audit history.

#### Scenario: Operator disables intent processing
- **WHEN** an operator activates a versioned rollback or disablement policy for a scope
- **THEN** new intents are rejected or held pending according to policy, and prior intent records remain inspectable

#### Scenario: Operator re-enables a compatible policy
- **WHEN** a later policy version is explicitly enabled for the same exact scope
- **THEN** only eligible pending intents may resume through normal idempotent governance checks

### Requirement: Correction-attributed updates verify source lineage

An opt-in correction-attributed `update` intent SHALL carry only a
server-verified correction association from the Provider correction-update
operation. Before idempotency lookup and handoff, the service MUST verify the
correction source's current exact binding/scope, current revision, eligible
reference lineage, active target lifecycle and expected target version. The
intent fingerprint and durable history MUST preserve the verified association;
generic caller-provided evidence references MUST NOT be interpreted as an
equivalent correction association.

#### Scenario: Verified correction update is admitted
- **WHEN** a Provider correction-update request supplies a current eligible
  correction source and matching active target
- **THEN** the service records one append-only update intent with immutable
  correction attribution and queues the existing governed worker path

#### Scenario: Unverified correction evidence is supplied
- **WHEN** a caller attempts to submit a correction-attributed update without
  a server-verified Provider correction source or with foreign, stale, hidden
  or mismatched lineage
- **THEN** the service rejects the request before idempotency persistence,
  queue handoff, candidate creation or canonical mutation without disclosing
  the source condition

### Requirement: Correction attribution is rechecked by asynchronous processing

The intent worker MUST recheck a correction-attributed update's current source
and target eligibility before routing it. A stale, superseded, revoked, hidden
or target-version-conflicted association MUST produce an append-only bounded
nonactive transition and MUST NOT create a candidate or canonical version.

#### Scenario: Valid association survives to worker processing
- **WHEN** source, target, policy and scope remain eligible at processing time
- **THEN** the worker routes the update through normal governance and preserves
  correction attribution in successor provenance and intent history

#### Scenario: Association changes after admission
- **WHEN** the correction source is superseded or the linked target becomes
  unavailable after intent admission but before worker execution
- **THEN** the worker records a bounded nonactive transition and retains the
  admitted request for authorized audit

### Requirement: Memory-intent queue handoff and completion are transactional

Governed memory-intent admission SHALL persist the authoritative intent,
immutable correction attribution when present, initial transition and River
job in one PostgreSQL transaction. River job arguments MUST contain only the
intent identifier. At execution, governance side effects, intent transition
and River job completion MUST commit in one transaction. Retryable failures
MUST roll back those effects and be returned to River for retry; the accepted
intent remains durable. Stale or denied work MUST commit a bounded terminal
transition and job completion without creating a candidate or successor.
Other derived-work kinds may continue using the existing derived queue.

#### Scenario: Admission transaction fails before commit
- **WHEN** intent admission fails after constructing its River job but before
  the PostgreSQL transaction commits
- **THEN** neither the intent nor an executable River job is visible

#### Scenario: Worker transaction rolls back and River retries
- **WHEN** a retryable error occurs after a candidate, promotion effect or
  transition has been attempted but before `JobCompleteTx` commits
- **THEN** all changes in that execution transaction are rolled back, the
  accepted intent remains, and River schedules the retry

#### Scenario: Worker commits governance and job completion together
- **WHEN** a valid intent finishes processing
- **THEN** the governance result, append-only intent transition and River job
  completion become durable together, with no second legacy worker execution

#### Scenario: Legacy memory-intent work is handed off
- **WHEN** deployment migrates recoverable memory-intent work from the legacy
  derived queue to River
- **THEN** each eligible intent has exactly one executable River job, terminal
  historical work is not replayed, and other derived-work kinds remain
  available to their existing consumers
