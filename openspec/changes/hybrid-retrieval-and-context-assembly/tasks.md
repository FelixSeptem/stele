## 1. Retrieval Contract Foundation

- [x] 1.1 Add `internal/retrieval` package boundary plus service-facing search and context assembly contracts
- [x] 1.2 Define shared query, filter, citation, and score result models aligned with governed memory terminology
- [x] 1.3 Extend OpenAPI scaffolding for retrieval and context assembly request/response schemas

## 2. PostgreSQL Retrieval Read Models

- [x] 2.1 Extend the PostgreSQL schema for full-text indexed canonical content and pgvector-backed semantic retrieval fields
- [x] 2.2 Define repository interfaces for lexical search, semantic search, filtered canonical reads, and relation projection lookup
- [x] 2.3 Implement PostgreSQL repository tests for scope filtering, lifecycle-safe reads, and ranked candidate loading

## 3. Hybrid Search Pipeline

- [x] 3.1 Implement lexical retrieval over canonical memory and summary memory using PostgreSQL full-text search
- [x] 3.2 Implement semantic retrieval contracts and merge-ready candidate output for pgvector similarity search
- [x] 3.3 Implement merge, dedupe, and rerank logic across lexical and semantic candidates with score breakdowns

## 4. Scope And Policy Enforcement

- [x] 4.1 Enforce `tenant`, `project`, and `namespace` isolation plus optional lower-scope filters on every retrieval path
- [x] 4.2 Exclude suppressed, forgotten, expired, and deleted memory from default search and context assembly reads
- [x] 4.3 Verify policy-safe retrieval behavior with repository-backed and service-level tests

## 5. Public Search API

- [x] 5.1 Add `POST /v1/memories/search` request validation, auth wiring, and handler integration
- [x] 5.2 Return canonical hits with citations, memory metadata, and stable score fields
- [x] 5.3 Verify the search API rejects invalid scopes and does not leak hidden memory by default

## 6. Context Assembly

- [x] 6.1 Define structured context sections for `profile`, `recent_session`, `recent_episodes`, `relevant_summaries`, `related_entities`, and `citations`
- [x] 6.2 Implement token-budget-aware packing that prefers summary memory plus evidence over flat episodic dumps
- [x] 6.3 Verify context assembly returns stable sectioned output and respects scope and lifecycle filters

## 7. Relation-Enhanced Retrieval

- [x] 7.1 Define entity and relation projection lookup contracts as optional retrieval enrichments
- [x] 7.2 Implement bounded neighborhood expansion for entity-centric queries without bypassing scope or lifecycle filtering
- [x] 7.3 Verify relation expansion improves result enrichment without becoming a required dependency for baseline search
