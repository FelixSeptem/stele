# self-hosted-product-delivery-verification Specification

## Purpose
Verify the supported self-hosted product stack against real PostgreSQL and pgvector.
## Requirements
### Requirement: Product verification uses real PostgreSQL and pgvector
The repository SHALL provide a bounded automated product-verification suite
that exercises the supported self-hosted stack against real PostgreSQL with
pgvector rather than only mocks or in-memory substitutions.

#### Scenario: Verification environment is created
- **WHEN** the documented product-verification command runs in an environment
  with its container prerequisite available
- **THEN** it starts isolated labelled PostgreSQL/pgvector and Stele runtime
  resources with generated test secrets and bounded timeouts

#### Scenario: Container prerequisite is unavailable locally
- **WHEN** an operator runs the product-verification command without its
  documented container prerequisite
- **THEN** it returns a clear prerequisite or explicit local-skip result without
  claiming that product verification passed

### Requirement: Product verification proves the protected memory lifecycle
The product-verification suite SHALL prove the documented bootstrap-admin-first
flow, durable principal creation, exact scope grant enforcement, idempotent
event ingestion, asynchronous governance, retrieval, and context assembly.

#### Scenario: Fresh stack completes golden memory flow
- **WHEN** the suite starts a fresh supported stack
- **THEN** it bootstraps the first durable administrator, creates a scoped
  runtime principal, performs an idempotent event retry, observes background
  processing, and verifies the resulting data through authorized retrieval and
  context APIs

#### Scenario: Caller crosses scope boundary
- **WHEN** the suite uses a credential against a valid-looking ungranted
  tenant, project, or namespace
- **THEN** the request is denied without revealing the target scope or allowing
  reads or writes

### Requirement: Product verification proves restart and drain safety
The product-verification suite SHALL prove bounded shutdown and restart behavior
for API, worker, and scheduler modes without duplicating idempotent writes or
leaking unavailable work into ready status.

#### Scenario: API receives termination during normal operation
- **WHEN** the suite sends the documented termination signal to API mode
- **THEN** readiness becomes non-ready before drain completes, in-flight work is
  bounded by the configured shutdown timeout, and the process exits cleanly

#### Scenario: Runtime restarts after durable work
- **WHEN** the suite restarts an affected runtime after an accepted idempotent
  event or pending background work
- **THEN** the system resumes through durable state, does not duplicate the raw
  event, and eventually produces lifecycle-safe retrieval/context results

### Requirement: Product verification proves migration and recovery paths
The product-verification suite SHALL test forward upgrade from a prior populated
schema fixture and a backup/restore verification target before the release gate
can pass.

#### Scenario: Prior schema fixture upgrades
- **WHEN** the suite applies the current service migration path to its prior
  populated schema fixture
- **THEN** it proves migration status is clean and existing authorized scoped
  data remains usable

#### Scenario: Backup is restored into verification target
- **WHEN** the suite creates a bounded test backup and restores it into its own
  distinct disposable target
- **THEN** restore verification proves schema currency and scoped read behavior
  without mutating the original source target

### Requirement: Verification cleanup is ownership-safe
The product-verification suite SHALL clean up only resources it owns and SHALL
preserve diagnostics on failure.

#### Scenario: Verification completes or fails
- **WHEN** the suite exits successfully or unsuccessfully
- **THEN** it removes only uniquely labelled containers, networks, volumes, and
  databases it created, retains or reports bounded diagnostic artifacts on
  failure, and never targets an operator's unlabelled PostgreSQL data

### Requirement: Product verification covers the governed memory intent lifecycle
The real-stack product-verification suite MUST exercise governed memory intent submission and processing through the supported API or enabled MCP adapter, PostgreSQL persistence, the durable memory-intent queue, and the worker-owned governance path using an explicitly owned exact scope.

#### Scenario: Intent lifecycle completes on a disposable stack
- **WHEN** product verification submits a bounded `remember` or `update` intent and waits for worker processing
- **THEN** it can observe the same intent through scoped status/history, verify the durable queue handoff and governance outcome, and confirm no duplicate canonical version is created by an identical retry

#### Scenario: Reserved intent remains review-only
- **WHEN** product verification submits `contradiction` or `feedback` evidence
- **THEN** the run records the bounded review or candidate outcome and proves that the request does not directly activate an insight or alter default retrieval/context behavior

### Requirement: Product verification covers intent isolation and idempotency
The product-verification suite MUST verify exact tenant, project, and namespace isolation and MUST distinguish identical replay from conflicting reuse of an intent idempotency key.

#### Scenario: Foreign scope is denied
- **WHEN** a valid-looking intent status, history, or submission request uses a different tenant, project, or namespace
- **THEN** the request is denied or returns an equivalent non-disclosing result and no foreign intent, queue item, or canonical mutation is observable

#### Scenario: Conflicting retry fails closed
- **WHEN** the same scoped idempotency key is reused with a materially different normalized payload
- **THEN** the run records a bounded conflict and verifies that the original intent and its transition history remain unchanged

### Requirement: Product verification covers intent restart, retry, and rollback recovery
The real-stack suite MUST cover API drain/restart, worker restart, durable lease recovery, retry exhaustion, policy disablement, and compatible re-enable for intent work without losing inspectability or claiming activation prematurely.

#### Scenario: Worker restarts after an accepted intent
- **WHEN** a worker is stopped or loses a lease after an accepted intent is queued
- **THEN** a restarted worker reclaims the same durable work identity, records at most one effective governance outcome, and preserves the append-only transition history

#### Scenario: Disabled policy retains pending work
- **WHEN** the operator disables intent processing before a queued intent is completed
- **THEN** new processing is rejected or held according to policy, the pending intent remains inspectable, and only a compatible exact-scope re-enable permits eligible work to resume

### Requirement: Product verification produces owned redacted evidence
The product-verification command MUST run only against disposable resources it owns, use bounded timeouts and deterministic fixture cleanup, and retain a report that identifies prerequisite, lifecycle, isolation, replay, recovery, rollback, and redaction results without sensitive payloads.

#### Scenario: Verification passes with owned resources
- **WHEN** all hard gates pass against the supplied PostgreSQL + pgvector environment
- **THEN** the report contains a stable run identity, bounded phase results, compatible schema/provider evidence, and cleanup confirmation without raw DSNs, credentials, scopes, IDs, or source content

#### Scenario: Prerequisite or hard gate is missing
- **WHEN** Docker, PostgreSQL/pgvector, schema compatibility, exact scope ownership, or a hard safety gate is unavailable
- **THEN** the command reports an explicit skip or failure and does not claim product readiness or enablement
