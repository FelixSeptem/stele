# Stele v1 Roadmap

## Purpose

This roadmap decomposes the approved v1 plan into execution phases and concrete tasks. It is intended to guide implementation order, checkpoint reviews, and milestone acceptance before broader feature expansion.

Scope remains unchanged:

- service only
- Go implementation
- PostgreSQL as the only system of record
- built-in API key plus tenant isolation
- governance-first memory service

## Milestone View

### M0: Project Foundation

Goal: make the repository runnable as a service skeleton with clear runtime modes and operational boundaries.

Exit signal:

- service starts in `api`, `worker`, and `scheduler` modes
- configuration loads cleanly
- PostgreSQL connectivity and migration bootstrap are wired

### M1: Event And Canonical Memory Base

Goal: ingest raw events into durable storage and establish the canonical memory schema and versioning model.

Exit signal:

- `POST /v1/events` persists raw event records
- memory domain types and schemas exist
- provenance and version history are modeled

### M2: Governance Pipeline

Goal: turn raw events into governed memory through extraction, scoring, consolidation, suppression, and summarization.

Exit signal:

- async worker can promote candidates into active memory
- profile updates and episodic contradictions follow explicit rules
- retention and forgetting primitives are available

### M3: Retrieval And Context Assembly

Goal: provide hybrid, scoped retrieval and an agent-ready context assembly surface.

Exit signal:

- memory search works across semantic plus lexical signals
- context assembly returns structured output
- suppressed or forgotten memory is excluded by default

### M4: Operations And Self-Hosting

Goal: make the service operable as a self-hosted product with deployment assets and inspection tooling.

Exit signal:

- worker and scheduler operations are observable
- admin inspection surfaces exist
- deploy assets and bootstrap docs are usable

### M5: Retrieval Quality And Memory Representation

Goal: improve retrieval quality without weakening the existing governance, provenance,
scope-isolation, or PostgreSQL system-of-record boundaries.

Exit signal:

- memory representation preserves raw-event provenance while exposing bounded,
  independently retrievable facts
- lexical, semantic, and relation candidates are fused with calibrated and
  observable ranking behavior
- duplicate and low-diversity context packing is measured and controlled
- ranking changes are evaluated offline and can be rolled out or rolled back by scope

## Phase Breakdown

## Phase 1: Foundation

### Task 1.1: Repository bootstrap

- Initialize `go.mod`.
- Create top-level directories: `cmd/stele`, `internal/app`, `internal/storage/postgres`, `openapi`, `deploy`, `docs`.
- Define package boundaries for `auth`, `memory`, `governance`, `retrieval`, `jobs`, and `policy`.

Outputs:

- bootable Go module
- stable directory layout

Done when:

- `go test ./...` runs against an empty but valid module layout

### Task 1.2: Runtime mode entrypoint

- Add a single binary entrypoint with mode selection for `api`, `worker`, `scheduler`.
- Define startup lifecycle hooks: config load, logger init, database init, mode dispatch.
- Keep mode wiring minimal and testable.

Outputs:

- CLI/runtime contract for service startup

Done when:

- each mode can start with a no-op runner path

### Task 1.3: Configuration system

- Define config schema for service mode, HTTP listen address, PostgreSQL DSN, auth defaults, and job settings.
- Support environment-based loading.
- Add validation for required config.

Outputs:

- typed config package
- startup validation rules

Done when:

- invalid config fails fast with actionable errors

### Task 1.4: PostgreSQL bootstrap and migrations

- Select migration mechanism and folder layout.
- Add initial migration runner integration.
- Wire health checks for DB readiness.

Outputs:

- migration entrypoint
- DB bootstrap package

Done when:

- service can apply migrations on a clean database

### Task 1.5: Service baseline endpoints

- Add `health` and `ready` endpoints.
- Add request middleware scaffolding: request ID, logging, panic recovery, auth hook point.
- Publish initial OpenAPI skeleton for system endpoints and upcoming memory APIs.

Outputs:

- minimal HTTP server
- baseline middleware stack

Done when:

- API mode exposes health endpoints and OpenAPI stub

### Task 1.6: Auth and isolation primitives

- Define `project`, `tenant`, and `namespace` request context model.
- Implement API key parsing and request scoping middleware.
- Add rejection paths for invalid or missing scopes.

Outputs:

- auth middleware primitives
- scoped request context

Done when:

- protected routes can reject unauthenticated or unscoped requests

Dependencies:

- Tasks 1.2, 1.3, 1.5

## Phase 2: Memory Model And Event Ingestion

### Task 2.1: Domain model for memory

- Define enums and types for memory classes, memory states, scope hierarchy, identifiers, and timestamps.
- Define canonical memory aggregate and raw event aggregate.
- Define version and provenance records.

Outputs:

- stable domain types for memory

Done when:

- downstream packages can depend on domain types without guessing field semantics

### Task 2.2: Schema design and migrations

- Add relational schemas for raw events, canonical memories, memory versions, provenance links, policies, and deletion markers.
- Add indexes for scope, timestamps, and lifecycle state.
- Reserve extension hooks for `pgvector` and full-text search fields.

Outputs:

- database schema for the first usable vertical slice

Done when:

- migrations create all base memory tables successfully

Dependencies:

- Task 1.4

### Task 2.3: Repository interfaces and Postgres implementations

- Define repository interfaces for event ingest, memory lookup, version history, and provenance persistence.
- Implement PostgreSQL repositories for raw event creation and base memory reads.
- Keep repository contracts aligned with scope isolation rules.

Outputs:

- storage abstractions
- first Postgres-backed repositories

Done when:

- tests can persist and load raw events through repository interfaces

### Task 2.4: `POST /v1/events` API

- Define request and response shapes for event ingestion.
- Validate scope, event type, content, timestamps, and metadata.
- Persist raw events and return stable identifiers.

Outputs:

- first product API endpoint

Done when:

- API can ingest an event and write it to PostgreSQL

Dependencies:

- Tasks 1.5, 1.6, 2.1, 2.3

### Task 2.5: Audit and provenance baseline

- Record creation source, actor, timestamps, and request lineage for ingested events.
- Make provenance retrievable internally even before external history APIs are built.

Outputs:

- consistent provenance capture at ingest time

Done when:

- every ingested event has auditable origin metadata

## Phase 3: Governance Pipeline

### Task 3.1: Candidate extraction pipeline contract

- Define the worker input/output contract from raw event to candidate memory.
- Create extraction interfaces so summarization or LLM-backed extraction can be plugged in later.
- Keep v1 extraction path simple and deterministic where possible.

Outputs:

- extraction pipeline boundary

Done when:

- worker can load raw events and emit candidate memory records

### Task 3.2: Candidate persistence and scoring

- Add candidate memory storage.
- Attach governance fields: confidence, importance, freshness, sensitivity, mutability, retention class.
- Define initial scoring and normalization rules.

Outputs:

- governed candidate layer

Done when:

- candidates persist with required governance metadata

### Task 3.3: Consolidation rules

- Implement dedupe and merge rules for mutable profile memory.
- Implement temporal coexistence rules for episodic contradictions.
- Promote accepted candidates into active memory versions.
- Suppress losing or stale candidates without destroying audit history.

Outputs:

- first governed consolidation path

Done when:

- worker can transform candidates into active or suppressed records

Dependencies:

- Tasks 2.1, 2.2, 3.2

### Task 3.4: Summary generation and compaction

- Define summary records for sessions or topic clusters.
- Add compaction rules for stale episodic material.
- Preserve evidence links from summaries back to underlying events or memories.

Outputs:

- summary memory path

Done when:

- worker can produce summary memories with provenance links

### Task 3.5: Forgetting and retention actions

- Add suppression, expiry, and delete action models.
- Implement retention job contracts and mutation audit logging.
- Ensure default reads exclude forgotten and suppressed content.

Outputs:

- lifecycle control primitives

Done when:

- forgetting actions change retrieval visibility according to policy

## Phase 4: Retrieval And Context Assembly

### Task 4.1: Search query model

- Define search request model: query text, scope filters, class filters, time window, top-k, summary inclusion, relation inclusion.
- Define retrieval result model with scores and citations.

Outputs:

- stable search contract

Done when:

- query orchestration can run without ad hoc request shaping

### Task 4.2: Lexical and semantic retrieval base

- Add full-text indexing strategy for canonical content.
- Add vector storage and retrieval path for semantic similarity.
- Define merge and rerank logic across lexical and semantic results.

Outputs:

- hybrid retrieval core

Done when:

- search can combine both recall paths into a single ranked output

Dependencies:

- Tasks 2.2, 3.3

### Task 4.3: Scope-aware filtering and policy enforcement

- Enforce `tenant`, `project`, `namespace`, and optional lower-scope filters in retrieval.
- Exclude suppressed, forgotten, and expired memories by default.
- Respect class and time filters.

Outputs:

- safe default retrieval behavior

Done when:

- retrieval does not leak hidden or out-of-scope memory

### Task 4.4: `POST /v1/memories/search`

- Expose the first public retrieval API.
- Return canonical memory hits with citations and metadata.
- Add request validation and auth integration.

Outputs:

- first retrieval endpoint

Done when:

- clients can retrieve governed memory through HTTP

### Task 4.5: `POST /v1/context/assemble`

- Build structured context sections: `profile`, `recent_session`, `recent_episodes`, `relevant_summaries`, `related_entities`, `citations`.
- Add token-budget-aware packing strategy.
- Prefer summary plus evidence over flat chunk dumps.

Outputs:

- agent-ready context assembly endpoint

Done when:

- service can return structured context instead of raw hit lists

### Task 4.6: Relation-enhanced retrieval

- Add entity and relation projection reads.
- Expand local neighborhoods for entity-centric queries.
- Keep graph enhancement optional and bounded.

Outputs:

- graph-assisted retrieval layer

Done when:

- relation-aware search improves recall without becoming the primary dependency

## Phase 5: Operations And Self-Hosting

### Task 5.1: Worker orchestration

- Finalize worker loop, job reservation, retry policy, and failure handling.
- Add idempotency protections for repeated jobs.

Outputs:

- stable background execution path

Done when:

- governance jobs can run reliably under restart and retry scenarios

### Task 5.2: Scheduler and maintenance jobs

- Add periodic triggers for retention, expiry, compaction, and cleanup.
- Separate maintenance cadence from request path behavior.

Outputs:

- scheduled maintenance subsystem

Done when:

- lifecycle maintenance can run unattended

### Task 5.3: Observability

- Add structured logs, metrics, and tracing hooks for API, worker, and scheduler modes.
- Expose operational signals for ingest latency, consolidation lag, retrieval latency, and forgetting actions.

Outputs:

- baseline observability

Done when:

- operators can inspect service health and backlog status

## Phase 6: Retrieval Quality And Memory Representation

This phase is an evolvable quality track layered on top of the existing memory,
governance, and retrieval contracts. It must not replace PostgreSQL as the system of
record, bypass lifecycle visibility rules, or make raw events mutable. Each step is
independently deployable and has a default-off or shadow mode until its acceptance
criteria are met.

### Task 6.1: Retrieval evaluation baseline

Purpose: establish a repeatable measurement loop before changing ranking behavior.

- Define an internal evaluation fixture with single-fact, multi-hop, temporal,
  procedural, profile, contradiction, noisy-neighbor, and hidden-memory cases.
- Store expected evidence IDs and acceptable evidence groups without storing model
  answers as memory records.
- Measure `Recall@1/5/10`, `MRR`, `nDCG@k`, multi-hop evidence coverage, duplicate
  rate, cross-scope leakage rate, p50/p95 latency, and candidate-pool size.
- Add a deterministic replay command that runs the same fixture against a selected
  ranking version.

Outputs:

- versioned retrieval evaluation fixture
- baseline report for the current ranking implementation
- redacted ranking diagnostics suitable for local and CI runs

Done when:

- every ranking change can be compared with a prior baseline
- cross-scope leakage is a hard zero-tolerance assertion
- latency and quality regressions have explicit thresholds

Dependencies:

- Tasks 3.5, 4.2, 4.3, and 4.4

Rollback:

- no production behavior changes; this task is measurement-only

### Task 6.2: Hierarchical memory representation and bounded chunking

Purpose: improve retrieval granularity while preserving complete source evidence.

- Keep raw events immutable and introduce derived source-chunk metadata: parent
  event/memory ID, ordinal, source message range, session ID, extraction version,
  character/token bounds, and timestamps.
- Prefer message, sentence, paragraph, list, and code boundaries before applying a
  bounded maximum size.
- Generate chunks as derived candidates; never treat a chunk as a replacement for
  the canonical memory or its provenance.
- Support parent-child lookup so a hit can receive a small, bounded amount of adjacent
  or parent context.
- Apply memory-class-specific granularity: atomic facts for profile, event units for
  episodic, rule/step groups for procedural, larger coverage units for summaries,
  and relation facts for relation memory.

Outputs:

- chunk metadata schema and repository contract
- deterministic chunker with configurable size bounds
- parent-child retrieval and provenance tests

Done when:

- every derived chunk resolves to an authorized source event and canonical memory
- chunks do not cross tenant, project, namespace, session, or user boundaries
- chunking improves evidence coverage without increasing duplicate rate beyond the
  evaluation threshold

Rollback:

- retain the previous canonical-memory retrieval path and disable chunk candidates
  through a versioned feature flag

### Task 6.3: Stable hybrid candidate fusion

Purpose: remove score-scale coupling between lexical, semantic, and relation paths.

- Retrieve bounded candidate pools from lexical, semantic, and optional relation
  searchers before final truncation.
- Add Reciprocal Rank Fusion as the default merge strategy so each channel contributes
  by rank rather than incomparable raw score ranges.
- Keep normalized weighted fusion as an experimental strategy for offline comparison;
  do not make weights implicit in code.
- Preserve per-channel ranks and bounded score diagnostics for explainability without
  exposing internal data through public responses.
- Use deterministic tie-breaking by score, memory class policy, source timestamp, and
  stable memory ID.

Outputs:

- versioned fusion strategy interface
- RRF default implementation and normalized-fusion experiment
- ranking diagnostics and regression tests

Done when:

- changing an embedding model or lexical score distribution does not silently change
  channel dominance
- hybrid ranking improves or maintains Recall@k and MRR against the baseline
- default ranking remains safe when one optional channel is unavailable

Rollback:

- scoped ranking rollout returns to the previous fusion strategy without schema
  rollback or data rewrite

### Task 6.4: Evidence deduplication and diversity-aware packing

Purpose: maximize useful evidence per context token and prevent one fact cluster from
occupying the entire result set.

- Deduplicate by canonical memory ID, source event, parent memory, and configurable
  semantic-similarity clusters.
- Add bounded diversity selection across memory class, source session, entity, and
  time slice.
- Use maximal marginal relevance or an equivalent deterministic selection policy after
  candidate fusion and before final context packing.
- Preserve citations for all selected evidence and record omitted-by-duplicate or
  omitted-by-diversity diagnostics.
- Keep profile, episodic, procedural, summary, and relation packing policies explicit
  in context assembly.

Outputs:

- deduplication and diversity policy interface
- context-packing implementation with bounded token accounting
- duplicate-rate and evidence-coverage tests

Done when:

- repeated versions or near-identical memories do not crowd out independent evidence
- selected context remains within its declared budget
- hidden, suppressed, forgotten, and out-of-scope records remain excluded before and
  after diversity selection

Rollback:

- disable diversity selection while retaining identity deduplication and lifecycle
  filtering

### Task 6.5: Query understanding and multi-signal retrieval

Purpose: improve recall for implicit, temporal, entity-centric, and multi-hop queries
without replacing the caller's original query.

- Preserve the original query as an immutable retrieval signal.
- Add bounded normalization for whitespace, aliases, terms, and mixed-language input.
- Extract optional entities, time windows, memory-class hints, and query intent as
  additional signals.
- Support bounded subquery decomposition for multi-hop queries; cap the number of
  subqueries and merge their evidence through the same fusion and isolation path.
- Treat query rewrites as additional candidates, never as a replacement for the
  original query.
- Return only memory evidence; intermediate query plans and reasoning remain internal
  diagnostics.

Outputs:

- query-analysis interface with provider-independent contracts
- bounded multi-signal retrieval orchestration
- temporal, multi-hop, and ambiguous-query evaluation fixtures

Done when:

- query understanding improves multi-hop and temporal evidence coverage without
  reducing simple factual recall
- malformed or adversarial query analysis fails closed to the original-query path
- query analysis does not expand scope or expose hidden retrieval metadata

Rollback:

- disable query analysis and continue with the original query through the stable
  fusion path

### Task 6.6: Quality-aware signals and controlled reranking

Purpose: use feedback, task outcomes, and optional rerankers only after evidence and
evaluation thresholds are established.

- Convert usefulness and task-evaluation signals into confidence-weighted features that
  account for evidence count, freshness, source reliability, and conflict state.
- Keep insufficient evidence, diagnostics-only, dry-run, scoped rollout, and default
  states explicit.
- Add optional deterministic feature reranking before considering an online model or
  cross-encoder reranker.
- If a model reranker is introduced later, enforce provider, model, timeout, cost,
  privacy, and fallback contracts and keep it disabled by default until shadow results
  meet the evaluation gate.
- Mine hard negatives from false positives and duplicate clusters without copying
  evaluation data into training or product analytics stores.

Outputs:

- confidence-aware quality feature model
- shadow and scoped rollout reports
- explicit fallback and timeout behavior for optional rerankers

Done when:

- feedback cannot dominate ranking with a single low-confidence signal
- quality gains persist across memory classes and query categories
- every rollout has a reversible version, owner, evidence minimum, and stop condition

Rollback:

- revert to the last approved fusion strategy and disable all quality adjustments

### Task 6.7: Retrieval release gate and long-term maintenance

Purpose: make retrieval quality a maintained product contract rather than a one-time
tuning exercise.

- Run the evaluation fixture in CI for ranking, chunking, isolation, and latency
  regressions within bounded test budgets.
- Publish a redacted retrieval-quality report with every ranking or representation
  version.
- Add migration and re-indexing runbooks for chunk metadata, embedding revisions, and
  duplicate clusters; never require destructive down migrations.
- Define retention and deletion behavior for derived chunks, query diagnostics, and
  evaluation fixtures.
- Schedule periodic review of thresholds, stale memories, embedding drift, and query
  category coverage.

Outputs:

- retrieval release checklist
- re-index/rebuild and rollback runbooks
- versioned quality report and maintenance ownership

Done when:

- no retrieval change can ship without quality, isolation, latency, and rollback
  evidence
- derived representations can be rebuilt from durable source records
- cleanup removes evaluation and diagnostic artifacts according to documented policy

Dependencies:

- Tasks 6.1 through 6.6

### Task 5.4: Admin and inspection endpoints

- Add admin surfaces for job status, memory history inspection, and operational diagnostics.
- Keep admin paths clearly separated from public APIs.

Outputs:

- operational inspection surface

Done when:

- operators can inspect internal state without direct database access

### Task 5.5: Deployment and bootstrap docs

- Add deployment assets for local and self-hosted environments.
- Document required PostgreSQL extensions, startup modes, config variables, and bootstrapping steps.

Outputs:

- self-host onboarding assets

Done when:

- a new operator can bring up the service from documentation alone

## Near-Term Product Hardening

The core memory, governance, retrieval, assurance, and integration-evidence
surfaces are now present. The next work should improve the product properties
that make those surfaces safe and dependable for multiple external integrations,
rather than add another memory type or an end-user application feature.

### Stage 1: Scoped Principal Authorization And Idempotent Ingestion

Goal: bind every protected request to a durable principal and explicit allowed
scope, then make public raw-event retries safe.

Scope:

- Replace process-wide allow-list-only API key validation with durable scoped
  principals, hashed keys, roles, expiry/disable state, and explicit scope
  grants.
- Resolve scope from the authenticated principal and reject requested headers
  that exceed its tenant/project/namespace grants.
- Preserve a bootstrap operator path for first deployment without storing raw
  secrets in PostgreSQL or logs.
- Add scoped idempotency to `POST /v1/events`, returning the original durable
  event and admission result for an exact retry while rejecting payload reuse
  with incompatible content.
- Record bounded authentication, authorization, and idempotency audit history.

Exit signal:

- A valid key cannot read or write an ungranted scope merely by changing scope
  headers.
- Disabled, expired, or rotated keys are rejected without leaking grants or key
  material.
- Retried event writes produce one raw event, one provenance chain, and a
  stable response.

### Stage 2: Versioned Migrations And Runtime Hardening

Goal: make upgrades and public runtime exposure safe for a long-lived
self-hosted deployment.

Scope:

- Introduce ordered, checksummed PostgreSQL migrations with applied-version
  history and expand/migrate/contract rollout guidance.
- Add real PostgreSQL plus pgvector integration coverage for bootstrap,
  upgrades, foreign-key behavior, leases, retention, and scope isolation.
- Add HTTP body limits, server read/write/idle timeouts, connection-pool
  configuration, and bounded request concurrency or rate limits.
- Publish backup/restore and upgrade runbooks with readiness checks.

Exit signal:

- An operator can upgrade an existing database deterministically and verify
  the applied schema version before traffic is accepted.
- Untrusted clients cannot consume unbounded request bodies or connections.

### Stage 3: Durable Multi-Scope Maintenance

Goal: ensure asynchronous maintenance reaches every active product surface and
recovers safely across replicas or process restarts.

Scope:

- Discover maintenance scopes from all durable scoped surfaces, not only
  promoted candidates and active canonical memory.
- Model workflow diagnostic and cleanup work with per-run durable claims,
  leases, retry budgets, and bounded failure summaries.
- Make worker identity instance-specific and make execution leases configurable
  by job class.
- Expose worker and scheduler metrics from every runtime mode or through a
  shared telemetry path, then define operational SLOs for ingest lag,
  governance backlog, workflow completion, and cleanup.

Exit signal:

- A scope with only workflow, session, feedback, or assurance data still
  receives scheduled maintenance.
- Restarted or horizontally scaled workers do not strand or duplicate eligible
  workflow work.

### Stage Sequencing

1. `scoped-principal-auth-and-ingest-idempotency`
2. `versioned-migrations-and-runtime-hardening`
3. `durable-multi-scope-maintenance`

Reasoning:

- Explicit identity and idempotent writes are the authorization boundary for
  every existing and future public API.
- Safe schema evolution and runtime limits must precede broad external
  exposure.
- Durable maintenance then makes the already implemented workflow and
  assurance surfaces reliable across scopes and replicas.

## Reference-Informed Expansion Backlog

This section captures external reference findings that should inform post-v1 planning without changing the current v1 execution order. The immediate reference is `alash3al/stash`, a Go, PostgreSQL, pgvector, and MCP-oriented agent memory project that emphasizes continuous agent experience memory. Treat this section as a research backlog, not as an approved implementation plan.

### Stash Reference Summary

Useful patterns to study:

- MCP-first agent ergonomics: Stash exposes memory actions as direct agent tools such as remember, recall, consolidate, fact, relationship, goal, failure, and hypothesis operations.
- Experience-to-insight pipeline: Stash frames memory as episodes, facts, relationships, causal links, patterns, contradictions, goals, failures, and hypotheses.
- Derived cognitive records: Stash explicitly models goals, failures, hypotheses, contradictions, causal links, and patterns as first-class agent memory concepts.
- Hierarchical namespaces: Stash uses path-like namespaces to organize memory and support broader recall scopes.
- Self-model conventions: Stash encourages agent-specific spaces for capabilities, limits, preferences, and operational lessons.
- Fast self-hosting path: Stash documents a short Docker Compose and MCP smoke path that gets an operator from startup to first recall quickly.

Risks and assumptions to validate before implementation:

- Product claims are not Stele requirements. Stash's cognitive stages are useful vocabulary, but each stage needs a Stele-specific data model, provenance rule, lifecycle state, and operator contract before adoption.
- MCP ergonomics should not drive storage or lifecycle design. Any MCP work must remain an adapter over OpenAPI-backed service behavior.
- Namespace subtree recall may be useful, but it can also weaken Stele's explicit `tenant/project/namespace` mental model if introduced too early.
- Autonomous hypothesis or causal inference likely requires a reasoning provider boundary that Stele has not yet defined. Do not tie that work to embedding provider configuration.
- Failure-pattern extraction is more concrete than hypothesis inference because Stele already has raw events, procedural memory, job execution records, recovery history, and embedding failure state to derive from.

Stele constraints that remain non-negotiable:

- Keep Stele OpenAPI-first. MCP can be added later as an adapter over existing service contracts, not as the primary system boundary.
- Keep PostgreSQL as the only system of record and preserve append-only versions, provenance, lifecycle state, and audit history.
- Enforce `tenant`, `project`, and `namespace` boundaries before any namespace-tree or agent-self convention is considered.
- Do not let LLM-style consolidation overwrite canonical memory in place. New insight records must remain derived, evidence-backed, versioned, and lifecycle-governed.
- Do not add SDK, UI, hosted-product, or end-user product logic to this repository.

### Candidate Expansion Track: Governed Experience Insights

Goal:

- Add a governed derived-insight layer that helps agents avoid repeated mistakes and surface experience-based context without weakening canonical memory semantics.

Possible insight vocabulary:

- `failure_pattern`: repeated user, agent, runtime, or workflow failures with evidence and remediation notes.
- `lesson`: a procedural or operational takeaway distilled from repeated evidence.
- `hypothesis`: an open assumption that can later be supported, contradicted, closed, or made stale by new evidence.
- `goal`: an inferred or explicit objective that should shape future context assembly.
- `contradiction`: evidence that two active or historical claims conflict and need scoped interpretation.
- `causal_link`: a bounded relationship between an action, condition, and observed outcome.

Recommended next proposal boundary:

- Build the derived insight substrate first: insight identity, scope, type, lifecycle, confidence, evidence citations, derivation metadata, provenance, and admin inspection.
- Implement only `failure_pattern` as an active derived insight in the first slice because it has concrete existing evidence sources.
- Allow `lesson` as a derived output when it is evidence-backed by a `failure_pattern`, but do not introduce free-form wisdom generation.
- Keep `hypothesis`, `goal`, `contradiction`, and `causal_link` as reserved vocabulary or design extension points until Stele has a reasoning-provider boundary and clearer evaluation tests.
- Derive insights asynchronously from existing raw events, canonical memory, procedural memory, summaries, relations, recovery history, job execution records, and embedding failure records.
- Expose `failure_pattern` and `lesson` through optional context assembly sections such as `known_failures` and `experience_lessons`.
- Add admin inspection for insight provenance and lifecycle decisions before adding any MCP adapter or agent-facing convenience tool.

Non-goals for the first slice:

- No direct MCP server implementation.
- No graph database or non-PostgreSQL store.
- No automatic rewriting of canonical memory based on inferred insight.
- No autonomous hypothesis, causal, contradiction, or goal inference.
- No global agent-self namespace that bypasses existing scope isolation.
- No provider-specific reasoning dependency tied to embedding provider configuration.

### Candidate Follow-up Tracks

1. OpenAPI-backed MCP adapter:
   Add an optional MCP surface that maps to existing Stele APIs and preserves API key, scope, lifecycle, and audit behavior.

2. Namespace path conventions inside existing scopes:
   Add an optional `memory_path` or `topic_path` convention inside one `tenant/project/namespace`, with bounded subtree retrieval and no cross-scope expansion.

3. Agent self-model conventions:
   Standardize scoped memory conventions for capabilities, limits, preferences, and lessons learned while keeping them ordinary governed memories.

4. Self-hosting first-ten-minutes smoke path:
   Improve operator onboarding with a short path covering startup, ingest, worker processing, retrieval, context assembly, readiness, and metrics.

5. Reasoning-provider boundary:
   Define a provider-independent interface for optional LLM-assisted derivation before implementing autonomous hypothesis, causal, contradiction, or goal inference.

## Execution Order

Recommended build order:

1. Phase 1
2. Phase 2
3. Phase 3 Tasks 3.1 to 3.3
4. Phase 4 Tasks 4.1 to 4.4
5. Phase 3 Tasks 3.4 to 3.5
6. Phase 4 Tasks 4.5 to 4.6
7. Phase 5
8. Phase 6 Task 6.1
9. Phase 6 Tasks 6.2 to 6.4
10. Phase 6 Task 6.5
11. Phase 6 Tasks 6.6 to 6.7

Reasoning:

- event ingestion and canonical memory must exist before governance can do real work.
- governance must exist before retrieval can be trustworthy.
- context assembly and graph enhancement should sit after the base retrieval path is stable.
- operations hardening should happen after the core runtime surfaces exist.
- retrieval quality work should begin with measurement, then representation, fusion,
  diversity, query understanding, and only then feedback or model-based reranking.
- every representation or ranking change must remain reversible and must not bypass
  lifecycle or scope enforcement.

## Review Gates

Before moving between phases, verify:

- Phase 1 to 2: service boot, config, DB bootstrap, and auth scaffolding are stable.
- Phase 2 to 3: event ingest path persists raw events with scope and provenance intact.
- Phase 3 to 4: active memory promotion and suppression behavior are test-covered.
- Phase 4 to 5: retrieval and context assembly exclude hidden memory and respect scope boundaries.
- Phase 5 to 6.1: runtime and storage behavior are stable enough to produce a repeatable
  retrieval baseline.
- Phase 6.1 to 6.2: the evaluation fixture and hard isolation gates are green before
  changing memory representation.
- Phase 6.2 to 6.3: chunk provenance, rebuildability, and parent-child scope tests are
  green before changing ranking.
- Phase 6.3 to 6.4: hybrid fusion is stable before diversity or context packing changes.
- Phase 6.4 to 6.5: duplicate-rate and budget gates are green before query decomposition.
- Phase 6.5 to 6.6: query analysis has a fail-closed fallback before quality signals or
  optional rerankers are enabled.
- Phase 6.6 to 6.7: every rollout has a measured gain, no isolation regression, and a
  tested rollback path before becoming a release requirement.

## Immediate Next Step

If implementation begins next, start with Phase 6 Task 6.1 on the current product
baseline. Do not change chunking or ranking until the baseline fixture, metrics, and
zero-leakage assertions are reproducible. The original foundation sequence remains the
required path for a fresh implementation; Phase 6 is the next quality track for the
current product.
