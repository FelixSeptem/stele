## ADDED Requirements

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
