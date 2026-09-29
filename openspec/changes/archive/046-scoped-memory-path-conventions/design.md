## Context

Stele already resolves an exact `tenant`/`project`/`namespace` scope before governed ingestion, retrieval, context assembly, and MCP delegation. The current contracts do not provide a portable organization key for related memories, so callers either filter client-side or overload content. This design adds one path dimension without creating a second authorization or persistence model. The behavioral contract is defined in the proposal and delta specs in this change.

## Goals / Non-Goals

**Goals:**

- Use one normalized path value across raw events, governed intents, canonical versions, retrieval hits, context evidence, public resources, and MCP arguments.
- Make exact matching the safe default and make descendant matching an explicit, bounded choice.
- Preserve exact scope authorization, lifecycle visibility, temporal selectors, budgets, citations, deterministic pagination, and replay/idempotency behavior.
- Migrate existing rows to a root-path representation without making old clients provide a path.

**Non-Goals:**

- Path values are not grants, tenants, projects, namespaces, filesystem locations, graph edges, or a query language.
- This change does not add agent self-model, autonomous inference, new memory classes, or a second store.

## Decisions

### Canonical representation

Represent a non-root path as slash-separated UTF-8 segments after trimming only the documented boundary formatting and normalizing separators. Reject empty segments, `.`/`..`, wildcards, encoded slash or backslash sequences, control characters, and values beyond fixed total and segment limits. Use a single explicit root marker in storage and responses (the implementation should choose the repository's established empty/root convention and document it in the API schema). Do not silently lowercase or Unicode-normalize segment content unless the existing API already guarantees that behavior; changing case would make paths surprising and could merge distinct user data.

Validation belongs in the shared domain boundary so API, worker, scheduler, and MCP callers cannot diverge. Prefix matching compares normalized segments, not raw strings, so `agents/research` does not match `agents/researcher`.

### Selector precedence

Expose `path` and `path_prefix` as mutually exclusive selectors. A missing selector means the whole resolved exact scope, preserving current behavior. `path` compiles to equality; `path_prefix` compiles to equality-or-descendant matching. Reject both fields together unless they are exactly equivalent and the public contract explicitly permits that equivalence; the simpler and safer contract is to reject the combination.

Apply scope predicates first, then lifecycle and temporal visibility, then path predicates, then ranking/budget/pagination. This ordering makes it impossible for a path prefix to act as an authorization shortcut and keeps hidden records out of diagnostics and counts.

### Data model and migration

Add a nullable-at-write, non-null-after-migration normalized path column to the canonical memory/version representation and the event or governed-intent records that must carry provenance. Backfill all existing rows to the root marker in one versioned PostgreSQL migration. Add a B-tree index keyed by exact scope columns followed by `memory_path`; add a second index or operator strategy for prefix scans that remains bounded by the exact scope predicate. Keep the path column out of authorization tables.

The migration must be restartable and observable. This release uses a migration-first rollout: apply the migration and verify backfill/index readiness before deploying code whose queries reference the new columns. Existing clients can continue omitting path fields and retain whole-scope behavior. Rollback means disabling path-aware request fields and retaining the populated root values and columns; no data rewrite is required.

### Propagation and idempotency

The normalized path is part of the event payload, governed intent, canonical version, resource/search hit metadata, context evidence, and MCP delegation DTOs where those surfaces already expose memory identity. Include the normalized path and selector kind in request fingerprints and cache keys. This prevents a retry or cached page for one path from being returned for another. Existing idempotency records remain valid for payloads whose omitted path normalizes to root.

### Pagination and bounds

Path-aware list and search requests reuse existing top-k, page-size, cursor, and deterministic tie-breaker limits. Prefix requests must not introduce an unbounded recursive traversal or a separate expansion endpoint. Cursor serialization includes the normalized selector and resolved exact scope, so a cursor cannot be replayed against a different path or grant. Counts and diagnostics are computed after lifecycle and scope filtering and remain bounded/redacted under existing contracts.

### API and MCP compatibility

OpenAPI schemas should add optional path fields at the same request/response levels as existing memory filters, with shared descriptions and validation limits. MCP schemas should mirror only those fields and continue to delegate to the OpenAPI/service layer. No MCP-specific path parsing, storage, or authorization is allowed. Older clients that omit fields receive unchanged whole-scope behavior; newer clients can opt into exact or prefix filtering.

### Temporal and lifecycle interaction

Path is an additional eligibility predicate, not a replacement for recorded-time, valid-time, lifecycle, suppression, forgetting, expiry, deletion, projection freshness, quality, or budget rules. A historical version retains the path associated with that version; an update that changes path creates a new governed version rather than mutating history. Forget and delete operations use the path only as a bounded selection filter and retain their existing preview, audit, and exact-ID safeguards.

### Verification strategy

Use unit tests for normalization and segment-boundary matching, migration tests for root backfill and indexes, contract tests for OpenAPI/MCP schemas, and PostgreSQL conformance tests covering exact scope plus sibling and descendant paths. Add replay tests proving path participates in idempotency fingerprints, and isolation tests proving a broad prefix cannot cross tenant/project/namespace boundaries. Run existing lifecycle, temporal, pagination, context-budget, and MCP suites unchanged to detect regressions.

## Risks / Trade-offs

- [Risk] Prefix scans can become expensive in large scopes. -> Mitigation: fixed path bounds, scope-leading indexes, bounded page/top-k limits, and query-plan/conformance checks.
- [Risk] A chosen root marker may collide with user input. -> Mitigation: reserve and validate the marker centrally; expose it through the schema rather than accepting ambiguous spellings.
- [Risk] Different Unicode spellings may look identical to users. -> Mitigation: avoid undocumented Unicode folding now, document byte/character limits, and leave stronger normalization for a separately governed change.
- [Risk] Propagation omissions could make API, worker, or MCP behavior disagree. -> Mitigation: use shared DTO validation and cross-surface equivalence fixtures in conformance tests.
- [Risk] Backfill briefly increases migration load. -> Mitigation: batch the update, monitor lock duration, build indexes with the repository's online strategy, and keep root fallback readable during rollout.

## Migration Plan

1. Apply the PostgreSQL migration, backfill legacy rows to root, create scope-leading indexes, and verify row counts/index health.
2. Deploy path-aware API, worker, and scheduler binaries; path request fields remain optional for old clients.
3. Publish OpenAPI/MCP schema updates and conformance evidence.
4. If rollback is required, deploy the prior application version without reverting the migration; retain path columns, values, and indexes, and continue serving legacy whole-scope requests.

## Open Questions

None that change the contract or implementation approach. The exact root marker and numeric limits should follow the repository's established schema/configuration conventions during implementation and be recorded in the API documentation and migration tests.
