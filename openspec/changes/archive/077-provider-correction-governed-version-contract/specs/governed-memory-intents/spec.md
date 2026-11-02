## ADDED Requirements

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
