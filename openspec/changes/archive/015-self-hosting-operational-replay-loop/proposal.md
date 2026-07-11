## Why

Stele has accumulated the core pieces for governed memory, retrieval, embedding operations, derived insights, feedback, and admin inspection, but operators still lack one repeatable path that proves a self-hosted deployment works end to end. Derived insight replay is also needed as a bounded operational recovery and verification tool so insight quality can be tested or backfilled without rewriting canonical memory.

## What Changes

- Add a first-ten-minutes operator smoke loop that validates startup, readiness, event ingestion, worker processing, scheduler maintenance, retrieval, context assembly, admin inspection, and observability in one documented flow.
- Add admin-only derived insight replay planning for scoped, time-windowed dry runs that report which insights would be created, updated, suppressed, skipped, or left unchanged.
- Add bounded derived insight replay apply/backfill execution through the durable background job model with idempotency, audit attribution, and replay reports.
- Extend admin inspection so operators can inspect replay plans, executions, reports, failures, and linked insight or feedback effects without direct PostgreSQL access.
- Extend context assembly and observability requirements so the smoke loop can prove active memories and replayed insights are actually retrievable, explainable, and measurable.
- Update self-hosting documentation and OpenAPI contracts for the new smoke and replay workflows.

## Capabilities

### New Capabilities

- `derived-insight-replay`: Admin-only dry-run and bounded apply/backfill for derived insight derivation over scoped historical windows.

### Modified Capabilities

- `self-hosting-bootstrap`: Add a complete first-ten-minutes smoke path that validates the full ingest-to-context and insight replay loop.
- `worker-orchestration-and-maintenance-jobs`: Route replay apply/backfill through durable, idempotent background execution instead of foreground admin requests.
- `admin-inspection-surface`: Add admin inspection and control contracts for replay planning, execution status, reports, and failures.
- `context-assembly`: Ensure replayed active insights can participate in optional context sections and diagnostics during the smoke loop.
- `service-observability`: Add low-cardinality metrics and diagnostics for smoke checks and derived insight replay execution.
- `governed-experience-insights`: Define how replay evaluates derived insights without mutating canonical memory, bypassing lifecycle governance, or discarding evidence history.

## Non-goals

- No SDK, UI, hosted onboarding product, or MCP adapter.
- No new storage engine; PostgreSQL remains the only system of record.
- No canonical memory history rewrite, vector history mutation, or in-place provenance replacement.
- No new memory classes or autonomous reasoning-provider inference.
- No general-purpose data repair framework beyond derived insight replay/backfill.
- No public replay API; replay remains admin-only and scope-bound.

## Impact

- Affected API/OpenAPI areas: admin replay planning and execution routes, admin replay status/report reads, readiness or smoke diagnostics where needed.
- Affected runtime areas: scheduler and worker job registration, idempotency keys, replay execution reports, insight derivation services, and audit/provenance recording.
- Affected retrieval areas: optional context assembly of replayed active insights and diagnostics that prove replay output is visible only when lifecycle and scope rules allow it.
- Affected observability areas: metrics for smoke loop outcomes, replay dry-run/apply results, replay failures, skipped records, and feedback-influenced decisions using low-cardinality labels.
- Affected docs: `docs/self-hosting.md`, any operator runbook or smoke fixture documentation, and artifact references for `openspec validate --strict`, `go test ./...`, and `pwsh -File scripts/openspec-archive-seq.ps1 -ChangeName "<change-name>"`.
