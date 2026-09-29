## 1. Domain contract

- [x] 1.1 Define the normalized root representation, segment grammar, length bounds, and shared validation errors.
- [x] 1.2 Implement exact and segment-boundary prefix selector parsing with mutual-exclusion validation.
- [x] 1.3 Add request fingerprint/cursor helpers that include normalized path and selector kind.

## 2. Persistence and migration

- [x] 2.1 Add path columns to event, governed-intent, canonical-version, and any required projection/resource tables.
- [x] 2.2 Write a restartable PostgreSQL migration that backfills existing rows to root and adds scope-leading exact/prefix indexes.
- [x] 2.3 Add migration verification for row counts, null handling, index readiness, and rollback read compatibility.

## 3. Ingestion and lifecycle

- [x] 3.1 Extend event and session/proof ingestion DTOs with validated path metadata.
- [x] 3.2 Persist path through event-to-candidate-to-active derivation without overwriting prior version paths.
- [x] 3.3 Ensure remember, update, forget, and delete selectors preserve existing intent, preview, audit, and lifecycle safeguards.

## 4. Retrieval and resources

- [x] 4.1 Add exact `path` and explicit `path_prefix` filters to search query planning after scope/lifecycle predicates.
- [x] 4.2 Add path metadata and filters to canonical memory list/detail resources with deterministic pagination.
- [x] 4.3 Verify temporal selection, ranking, hidden-memory exclusion, and sibling-prefix behavior.

## 5. Context assembly

- [x] 5.1 Propagate path selectors through context requests, projection eligibility, live retrieval, and evidence citations.
- [x] 5.2 Preserve section names, token/character budgets, summary preference, diagnostics redaction, and exact scope.
- [x] 5.3 Add path-aware omission and budget conformance cases.

## 6. Public adapters

- [x] 6.1 Update OpenAPI request/response schemas and validation documentation.
- [x] 6.2 Update MCP tool schemas and delegation mapping without adding MCP-specific semantics.
- [x] 6.3 Verify omitted fields preserve legacy whole-scope behavior and conflicting selectors fail closed.

## 7. Tests and evidence

- [x] 7.1 Add unit tests for normalization, bounds, root compatibility, exact matching, and segment-boundary prefixes.
- [x] 7.2 Add PostgreSQL integration tests for migration, indexes, idempotency fingerprints, pagination, lifecycle, and temporal behavior.
- [x] 7.3 Add API/MCP contract and cross-surface equivalence tests for exact scope isolation.
- [x] 7.4 Run the full Go, race, vet, OpenSpec strict, and real PostgreSQL/pgvector conformance suites.

## 8. Documentation and release

- [x] 8.1 Update OpenAPI and self-hosting docs with path grammar, root behavior, exact matching, and explicit prefix matching.
- [x] 8.2 Update the v1 roadmap immediate-next-step/status entry to reflect this proposal and preserve historical archive bookkeeping.
- [x] 8.3 Record migration, rollback, query-plan, and conformance evidence for release review.
