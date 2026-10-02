## Why

Stele already has transport-specific intent calls and durable governance primitives, but there is no single contract that defines how a requested memory mutation becomes an auditable, scoped, idempotent work item. This gap makes retry behavior, approval boundaries, and lifecycle outcomes differ between `remember`, `update`, `forget`, `contradiction`, and `feedback`. A governed intent contract is the next bounded step because it can unify those requests without allowing a client or adapter to mutate canonical memory directly.

## What Changes

- Add a PostgreSQL-backed `memory intent` contract for `remember`, `update`, `forget`, `contradiction`, and `feedback`.
- Define an append-only intent lifecycle with explicit accepted, pending, rejected, suppressed, failed, and replayed outcomes.
- Enforce exact `tenant/project/namespace` scope, actor, reason, request identity, provenance, and bounded payload validation before persistence.
- Make `(scope, idempotency_key)` the retry boundary; identical retries replay the original result and conflicting payloads fail closed.
- Route accepted intents to existing candidate, governance, lifecycle, provenance, and durable work queue paths; intent submission itself never writes canonical memory inline.
- Define per-type validation and authorization gates, including target version checks for `update`/`forget` and evidence binding for `contradiction`/`feedback`.
- Add scoped inspection for intent status and audit history without exposing hidden content through ordinary retrieval or context.
- Add operator rollback/disablement semantics that stop new processing while retaining submitted intents, outcomes, provenance, and lifecycle history.
- Extend low-cardinality telemetry, OpenAPI schemas, and operator documentation for intent submission, replay, rejection, and processing outcomes.

## Non-goals

- Do not introduce a second system of record, queue, graph database, SDK, UI, or provider-specific intent implementation.
- Do not permit direct canonical-memory mutation from HTTP, MCP, or any other adapter.
- Do not change the synchronous event-ingest latency contract or default retrieval/context behavior.
- Do not infer intent payloads from unrestricted natural-language prompts in this change.
- Do not automatically activate contradiction insights or bypass the separately governed contradiction policy.
- Do not delete or overwrite prior intent records, outcomes, provenance, or canonical memory versions.

## Capabilities

### New Capabilities

- `governed-memory-intents`: Defines the scoped intent envelope, type-specific validation, append-only lifecycle, idempotency, durable handoff, status inspection, and rollback behavior.

### Modified Capabilities

- `memory-governance-pipeline`: Accepted intents enter the existing asynchronous candidate/governance pipeline with durable work and retry semantics.
- `canonical-memory-lifecycle`: Intent processing may create candidate or lifecycle transitions, but canonical versions remain append-only and are never mutated inline by submission.
- `memory-history-and-provenance`: Intent records and processing outcomes become inspectable provenance sources with actor, reason, request identity, and target-version lineage.
- `service-observability`: Add bounded intent lifecycle, replay, rejection, queue, and rollback categories without high-cardinality or sensitive labels.
- `runtime-api-contract-publication`: Publish OpenAPI-first intent submission and scoped status/history contracts; adapters remain clients of the public service boundary.

## Impact

- Affected areas include `internal/memory`, `internal/governance`, `internal/jobs`, `internal/workqueue`, `internal/storage/postgres`, `internal/app`, `internal/mcp`, `internal/telemetry`, OpenAPI schemas, and operator documentation.
- PostgreSQL remains the only system of record. The implementation will add versioned migrations, scoped indexes, append-only guards, and repository methods for intent records and outcomes.
- Existing MCP remember/forget behavior will map to the shared intent contract while preserving authorization and preview/apply boundaries.
- The change requires focused Go tests, migration tests, durable retry/idempotency tests, and an owned PostgreSQL integration run.

Related workflow references: `openspec-propose`, `openspec-apply-change`, `openspec-archive-change`, `verification-before-completion`, and `scripts/openspec-archive-seq.ps1`.
