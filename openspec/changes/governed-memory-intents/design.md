## Context

The proposal builds on the existing `MemoryIntentInput`/`MemoryIntentRecord`
types used by the MCP adapter, PostgreSQL-backed canonical lifecycle and
provenance repositories, and the durable derived-work queue. The new contract
must preserve exact `tenant/project/namespace` isolation, append-only history,
and the rule that hot request paths do not perform consolidation inline. See
`proposal.md` and the delta specs for the required external behavior.

## Goals / Non-Goals

**Goals:**

- Give all supported memory operations one normalized fingerprint and state
  machine.
- Persist intent request, outcome, and transition history transactionally with
  scoped uniqueness and append-only protections.
- Reuse existing worker leases, retries, candidate admission, lifecycle
  transitions, evidence references, and audit records.
- Make status/history inspection safe, bounded, and usable by OpenAPI clients
  and adapters.
- Provide deterministic rollback and replay behavior that never duplicates a
  canonical transition.

**Non-Goals:**

- No new queue technology or direct provider integration.
- No synchronous consolidation or automatic contradiction activation.
- No generalized natural-language command parser.

## Decisions

### 1. Use one normalized intent envelope with type-specific payloads

Keep a common envelope containing scope, intent type, actor, reason, request
ID, idempotency key, normalized fingerprint, target identity, source evidence,
and bounded metadata. Store type-specific fields in validated JSONB rather than
creating five unrelated tables. This keeps the API and worker contract stable
while allowing each type to enforce its own target/evidence rules.

Alternative: separate tables per operation. Rejected because it duplicates
idempotency, audit, lifecycle, and retention logic and makes cross-type status
inspection inconsistent.

### 2. Make idempotency a database-enforced scope boundary

Derive a canonical request fingerprint after normalization and enforce a unique
index on `(tenant, project, namespace, idempotency_key)`. An identical request
returns the existing record as `replayed`; a different fingerprint returns a
bounded conflict without changing the first record. Application checks remain
for useful error messages, but PostgreSQL is authoritative under concurrent
submissions.

Alternative: process-local idempotency cache. Rejected because it fails across
restarts and workers and would not protect the system of record.

### 3. Separate submission status from processing transitions

The intent row records immutable request identity and current status. Every
status change appends an intent transition row with actor, reason, bounded
category, queue/work reference, and timestamp. A durable work item references
the intent ID and carries the same scope and idempotency identity. Workers claim
and complete work through existing lease/retry contracts; canonical writes and
intent completion occur in one database transaction where possible.

Alternative: update one status row without a ledger. Rejected because restart,
retry, rollback, and operator review would lose the audit trail.

### 4. Keep target and evidence authorization before handoff

`update` and `forget` require an exact target memory/version identity and
revalidate lifecycle visibility at processing time. `contradiction` requires
two in-scope evidence references and remains subject to the reserved
contradiction activation policy. `feedback` must reference an existing in-scope
insight and never directly changes its lifecycle. Stale targets become bounded
rejected or suppressed outcomes rather than implicit rebases.

Alternative: accept a target ID and resolve the latest version automatically.
Rejected because retries could apply to a different canonical version than the
caller reviewed.

### 5. Reuse the public API boundary and keep MCP thin

Add OpenAPI request/response schemas and handlers backed by a service interface.
The MCP adapter maps its existing remember/forget calls to this interface and
does not gain direct repository access. Read APIs expose only exact-scope
intent state/history; ordinary retrieval and context remain unchanged.

Alternative: add MCP-only intent endpoints. Rejected because it would create a
second authorization and contract surface.

### 6. Use bounded categories for diagnostics

Persist and emit categories such as `accepted`, `pending`, `rejected`,
`suppressed`, `failed`, `replayed`, `scope_denied`, `target_stale`,
`evidence_incomplete`, `policy_disabled`, `retry_exhausted`, and `rolled_back`.
Never put payload, claim text, scope values, IDs, prompts, credentials, or raw
provider/database errors in telemetry.

## Risks / Trade-offs

- **Intent table grows faster than canonical memory** -> use bounded JSONB,
  scoped indexes, append-only retention jobs, and paginated history; retain
  audit records according to existing governance retention policy.
- **Concurrent duplicate submissions race** -> rely on the scoped unique index
  and transaction retry handling; never use a process-local lock as authority.
- **Worker crash after canonical write but before completion** -> use the
  existing work idempotency key and transactional outcome record so recovery
  returns a replayed result instead of applying a second transition.
- **Adapters depend on old response shapes** -> preserve existing MCP response
  fields and add intent identity/status fields compatibly; publish OpenAPI
  contract tests before changing handlers.
- **Rollback leaves pending work** -> mark the policy disabled, stop new claims,
  and leave pending intents inspectable for explicit later resumption.

## Migration Plan

1. Add the intent and transition tables, scoped unique/index constraints, and
   append-only triggers in a new PostgreSQL migration.
2. Introduce the normalized domain envelope and repository/service contract with
   no change to existing adapter behavior.
3. Add durable worker handoff and idempotent outcome transitions, then map MCP
   remember/forget calls and expose OpenAPI status/history routes.
4. Run focused unit/repository tests and an owned PostgreSQL integration suite
   covering concurrent retries, exact-scope isolation, restart recovery, stale
   targets, rollback, and redacted diagnostics.
5. Enable processing only through an explicitly versioned scope policy. Rollback
   disables new intent acceptance/claims and preserves all records; the down
   migration is reserved for disposable development databases.
