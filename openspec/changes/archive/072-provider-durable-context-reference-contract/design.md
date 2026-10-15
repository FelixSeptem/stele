## Context

PC1–PC4 already provide the Provider context DTOs, disclosure controls, final
budget accounting and versioned context digest. Existing event ingestion,
governance, canonical memory, path and provenance services are durable, but the
public Provider boundary does not yet define how a caller proves that a task/run
or compilation reference completed governance and can be resolved deterministically
after restart. See `proposal.md` for the motivation and scope.

## Goals / Non-Goals

**Goals:**

- Add a versioned Provider reference capability without creating a second memory
  or business-state system.
- Reuse the existing event → candidate → canonical lifecycle and provenance;
  expose bounded status and exact-path resolution over those records.
- Make path preservation, UUID lineage, idempotent replay, lifecycle behavior and
  restart recovery observable and testable on fresh PostgreSQL.
- Keep reference reads subject to current scope, grant, disclosure and lifecycle
  authorization.

**Non-Goals:**

- Task state, schedule, lease, compilation commit, artifact publication and
  webhook delivery remain outside Stele's authority.
- Ranked retrieval, context packing, digest generation and autonomous insight
  loops are not redesigned.
- Opaque tokens do not provide snapshot replay or bypass lifecycle policy.

## Decisions

### 1. Add a capability-scoped reference protocol

Add a dedicated versioned capability with closed write, status and exact-read
DTOs. Capability discovery advertises supported event kinds, status categories,
read selectors and bounded field sizes. A strict consumer fails compatibility if
the capability is absent. This is preferable to overloading `/context` or
pretending that the base `provider-v1` schema implies reference semantics.

### 2. Reuse ordinary governance as the only write path

Reference writes enter the existing Provider event/intents admission and worker
pipeline. An allowlist maps event types to promotion semantics; archival events
remain archival. The status route reads durable raw-event, candidate, canonical
and provenance state and reports the first incomplete or failed stage using
bounded categories. This avoids a synchronous write path that could claim a
reference before asynchronous governance commits.

### 3. Separate deterministic reference resolution from ranking

Exact-path lookup uses normalized scope, path and a reference selector to resolve
the current eligible canonical version and its bounded lineage. It does not call
top-k ranking and does not interpret an empty ranked result as proof that a
reference never existed. Existing memory search remains available for discovery.

### 4. Make identity and idempotency transactional

The normalized write fingerprint includes authenticated principal/binding,
exact scope, supported event kind, normalized path, bounded value fingerprint,
attribution and idempotency key. Equivalent retries return the original event
identity; changed reuse returns a conflict. Event, intent and provenance IDs use
the repository's existing UUID-compatible types. Path and identity are carried
through candidate and canonical promotion rather than reconstructed at read time.

### 5. Reauthorize every status and read

Status and exact lookup first resolve exact scope and current grant, then apply
disclosure and lifecycle rules, then read the durable reference. Foreign and
hidden records return bounded outcomes with no existence leakage. Privileged
history can inspect lineage under its existing inspection contract, while
ordinary lookup cannot resurrect deleted or forgotten content.

### 6. Prove readiness with public fresh-stack fixtures

The live verifier will use the official bootstrap flow, a fresh PostgreSQL +
pgvector service, public Provider writes and bounded worker polling. It will
cover task/run and compilation references, identical/conflicting replay,
non-root exact paths, UUID lineage, API/worker restart and lifecycle removal.
Reports contain only bounded categories, counts, hashes and service/schema
provenance. Focused migration and repository tests remain separate from public
readiness evidence.

## Risks / Trade-offs

- **[Risk] Existing callers assume every accepted event is searchable.** →
  Document allowlisted event types and explicit governance statuses; strict
  consumers fail on incomplete contracts.
- **[Risk] Canonical promotion or deduplication drops path/lineage.** → Carry
  path and source identity through every persistence boundary and add regression
  tests against non-root paths and UUID columns.
- **[Risk] A reference reveals hidden or foreign state.** → Reauthorize every
  status/read and return bounded indistinguishable scope/lifecycle outcomes.
- **[Risk] Large reference values or lineage responses bypass budgets.** → Apply
  existing bounded event, disclosure and Provider response limits; return
  identifiers and bounded values only.
- **[Risk] Mixed-version deployment sees a new capability partially available.**
  → Use additive migrations, explicit capability negotiation, compatibility
  failure and rollback instructions that preserve existing PC1–PC4 behavior.

## Migration Plan

1. Add additive OpenAPI DTOs and capability discovery behind the new version;
   keep existing event/context routes unchanged for callers that do not opt in.
2. Add any reference indexes or status persistence using the repository's
   additive migration pattern; verify clean install and upgrade from the
   current migration.
3. Deploy API and worker with capability negotiation and bounded status/read
   paths. Mixed replicas must reject unsupported reference requests rather than
   silently downgrade.
4. Run focused repository/conformance checks, then the fresh public PostgreSQL
   fixture and API/worker restart verification before declaring readiness.
5. Roll back by disabling the capability for consumers; retain accepted events,
   provenance and existing memory lifecycle data. Do not delete durable history.
