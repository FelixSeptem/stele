## Why

Stele now has the core memory service pieces for ingestion, governance, retrieval, context assembly, derived insights, quality evaluation, repair, and self-hosted smoke checks, but operators still lack one durable service-side loop that proves a new scope is usable and then proves an agent-style memory session can use that scope end to end. This change connects scope onboarding proof with a bounded session memory loop so a deployment can answer: "Can this scope accept memory, assemble useful context for an agent turn, write back outcomes, and verify the result?"

## What Changes

- Add scoped proof runs that exercise a tenant/project/namespace from baseline readiness through ingest, worker processing, retrieval, context assembly, optional derived insight replay, quality evaluation, and repair recommendation.
- Add service-side memory session runs that model an external agent interaction without implementing the agent: session start, context assembly, turn outcome ingestion, governed processing, post-turn recall verification, and session report.
- Add durable proof/session records, step records, verdicts, bounded fixture metadata, evidence links, and failure reason taxonomy in PostgreSQL.
- Add admin APIs for creating, listing, inspecting, reporting, and rerunning scope proof runs.
- Add public or scoped service APIs for creating memory sessions and recording memory-relevant turn outcomes while preserving the existing event ingestion contract.
- Execute proof/session verification work through the existing worker or scheduler model with leases, retries, idempotency, and scoped evidence.
- Bridge proof and session failures into existing quality evaluation and repair planning without auto-approving repair actions.
- Add low-cardinality metrics and operator diagnostics for proof/session status, step failures, verdicts, and remaining product-loop gaps.
- Update OpenAPI and self-hosting docs so the manual smoke run becomes an auditable proof workflow and the service exposes a concrete memory-session integration contract.

## Non-goals

- Do not add SDKs, UI, chat interfaces, model invocation, prompt orchestration, or end-user agent product logic.
- Do not make Stele responsible for generating agent responses; external agents remain responsible for model calls and final answers.
- Do not replace `POST /v1/events`, retrieval, context assembly, quality evaluation, repair plans, replay, or embedding controls.
- Do not auto-approve repair plans or mutate canonical memory outside existing governed lifecycle, provenance, version, and audit rules.
- Do not introduce cross-scope proof/session behavior; every API, worker claim, query, and report remains tenant/project/namespace scoped.
- Do not turn smoke fixture data into hidden global state; fixture records must be explicit, scoped, auditable, and safely distinguishable from business memory.

## Capabilities

### New Capabilities

- `scope-proof-and-session-loop`: Covers durable scope proof runs and service-side memory session runs, including step execution, verdicts, evidence links, reruns, and reports.

### Modified Capabilities

- `self-hosting-bootstrap`: Replace manual-only smoke verification with a durable proof run workflow and troubleshooting report.
- `event-ingestion`: Allow proof/session-generated memory-relevant turn outcome events to carry explicit proof/session attribution while preserving the stable event write contract.
- `context-assembly`: Allow session start and proof verification to request context with diagnostic attribution and expected recall checks.
- `memory-quality-admission-repair`: Allow proof/session failures to create or reference quality evaluations and repair recommendations without automatic repair approval.
- `worker-orchestration-and-maintenance-jobs`: Add durable proof/session step execution through existing lease, retry, idempotency, and failure-state semantics.
- `service-observability`: Add metrics and diagnostics for proof/session runs, step outcomes, verdicts, and failure categories.
- `admin-inspection-surface`: Add scoped admin inspection of proof/session reports and linked evidence without exposing out-of-scope or hidden memory content.

## Impact

- API: Adds admin scope proof endpoints and service-side memory session endpoints; extends event and context metadata with optional proof/session attribution.
- Storage: Adds durable tables for proof runs, session runs, step records, reports, fixture metadata, and evidence links.
- Workers/scheduler: Adds bounded proof/session execution jobs that reuse existing durable orchestration.
- Memory lifecycle: Uses existing event ingestion, governance, retrieval, context, quality, replay, and repair contracts rather than direct canonical rewrites.
- Observability: Adds low-cardinality metrics and structured logs for proof/session status, step result, verdict, and failure category.
- Docs: Updates self-hosting runbooks to show proof creation, session loop execution, failure diagnosis, repair recommendation, rerun, and remaining gaps.

## Remaining Gap After This Change

After implementation, Stele should have a complete service-side product loop for proving a scope and exercising an agent-style memory session:

`scope proof -> session context -> external agent turn -> turn outcome ingestion -> governance -> retrieval/context verification -> quality/repair recommendation -> rerun proof/session`

The following product gaps will remain outside this proposal:

- SDK/UI onboarding remains out of repo scope.
- External agent runtime integration remains caller-owned; Stele will expose memory contracts but will not call models or manage prompts.
- Production alert routing to Slack, PagerDuty, email, or incident systems remains out of scope.
- Load testing, capacity planning, backup/restore drills, and disaster recovery proof remain future operational proposals.
- Long-term memory usefulness scoring across real user tasks remains future work beyond scoped proof fixtures and session verification.

## Artifact References

- Proposal/apply workflow: `.codex/skills/openspec-propose/SKILL.md`, `.codex/skills/openspec-apply-change/SKILL.md`
- Archive workflow after implementation: `.codex/skills/openspec-archive-change/SKILL.md`, `scripts/openspec-archive-seq.ps1`
- Expected verification: `openspec validate scope-proof-and-agent-session-memory-loop --strict`, `go test ./... -count=1`
