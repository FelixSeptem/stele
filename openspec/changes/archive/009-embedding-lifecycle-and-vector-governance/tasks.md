## 1. Vector Lifecycle Schema And Repository Contracts

- [x] 1.1 Add PostgreSQL schema for vector revision audit records, active revision linkage, and embedding rebuild eligibility state for current canonical projections
- [x] 1.2 Add repository contracts for recording rebuild eligibility, appending vector revisions, promoting active revisions, and listing eligible reindex work under scope-safe constraints
- [x] 1.3 Add regression tests for append-only revision persistence, compare-and-promote guards, supersession lineage, and missing or stale eligibility state

## 2. Provider Abstraction And Routing Policy

- [x] 2.1 Introduce internal embedding provider interfaces that return provider identity, model identity, dimensions, and vector payload without changing public retrieval APIs
- [x] 2.2 Add deterministic model-routing policy inputs and persistence rules so canonical projections can be evaluated for provider or model target drift
- [x] 2.3 Add tests for provider abstraction, routing determinism, and drift detection across memory classes or configured defaults

## 3. Mutation-Driven Reindex Eligibility

- [x] 3.1 Update manual create, update, merge, reclassify, and relevant lifecycle mutation paths to mark semantic rebuild eligibility while continuing to invalidate stale active embeddings immediately
- [x] 3.2 Preserve vector revision audit continuity when mutation advances the canonical projection, including source version and content-hash linkage
- [x] 3.3 Add regression tests showing material mutation clears stale semantic participation, preserves lexical consistency, and queues durable rebuild eligibility instead of regenerating inline

## 4. Background Reindex And Rotation Execution

- [x] 4.1 Add durable worker or scheduler execution paths that claim eligible embedding backfill, rebuild, and provider-drift work without blocking write paths
- [x] 4.2 Implement failure handling and retry-safe compare-and-promote activation so stale or superseded rebuilds are recorded but never activated
- [x] 4.3 Add regression coverage for backfill, rebuild after mutation, provider rotation drift processing, and failure-retry semantics

## 5. Retrieval Activation And Fallback Semantics

- [x] 5.1 Update semantic retrieval reads to use only the active vector revision for the current canonical projection while preserving existing lifecycle and scope filters
- [x] 5.2 Preserve hybrid retrieval fallback so lexical and relation recall continue functioning when semantic projection is missing, rebuilding, failed, or superseded
- [x] 5.3 Add retrieval and repository tests for active-vector selection, no-active-vector fallback, and stale-revision exclusion from default semantic recall

## 6. Observability And Documentation

- [x] 6.1 Extend telemetry hooks and backlog diagnostics to cover embedding rebuild eligibility, execution outcomes, provider or model drift, and retry behavior
- [x] 6.2 Update self-hosting and governance documentation with internal embedding lifecycle semantics, background rebuild expectations, and the explicit non-goal that admin control surfaces remain deferred
- [x] 6.3 Run focused verification for `internal/storage/postgres`, `internal/retrieval`, `internal/memory`, `internal/app`, `internal/jobs`, and any new embedding lifecycle package coverage
