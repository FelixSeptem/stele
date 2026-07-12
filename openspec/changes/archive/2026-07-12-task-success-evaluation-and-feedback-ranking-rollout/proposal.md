## Why

Stele now records scoped memory sessions, outcome verification, usefulness feedback, and approval-gated repair recommendations, but it still cannot connect memory quality to task-level success or safely roll those signals into default retrieval behavior. This change closes the next service-side product loop by making external task outcomes durable evidence and by adding governed, reversible feedback-aware ranking rollout.

## What Changes

- Add durable scoped task-success evaluation records that link external task objectives, success criteria, external verdicts, memory sessions, turns, outcome events, expected recall targets, usefulness feedback, and verification evidence.
- Add task-success reports that explain task outcome evidence, memory contribution signals, failure categories, and next actions without invoking models or exposing hidden or out-of-scope memory.
- Add admin inspection surfaces for task evaluations, task summaries, ranking rollout policies, dry-run reports, impact reports, and rollback history.
- Extend session reports and usefulness feedback summaries so task-success evaluations can reference session/turn/verification/feedback evidence and contribute bounded aggregate signals.
- Add feedback and task-success-aware ranking rollout governance for search and context assembly, including dry-run comparison, explicit rollout policy, impact reports, evidence thresholds, and rollback.
- Keep default ranking unchanged unless an authorized rollout policy is active for the resolved scope or an individual request explicitly opts into diagnostics/dry-run.
- Add quality bridge behavior so repeated task failures or degraded memory contribution can produce bounded quality findings and approval-gated repair recommendations.
- Add low-cardinality metrics, structured logs, OpenAPI contracts, and self-hosting docs for task evaluation and ranking rollout operations.

## Non-goals

- Do not add SDKs, UI, hosted-product behavior, or end-user application logic.
- Do not execute external agents, invoke models, build prompts, generate final answers, or perform LLM-as-judge scoring.
- Do not infer task success autonomously; Stele only validates, stores, links, and summarizes caller-provided verdicts and bounded evidence.
- Do not let task evaluations, feedback, or ranking policies rewrite canonical memory content, provenance, vector revisions, lifecycle state, or audit history in place.
- Do not enable global ranking changes without explicit scoped policy activation, dry-run evidence, and rollback history.
- Do not use superseded feedback, hidden memory content, forgotten memory, deleted memory, or out-of-scope evidence as active ranking input.
- Do not export tenant, project, namespace, memory id, session id, task id, actor, reason, or other high-cardinality identifiers as metric labels.

## Capabilities

### New Capabilities

- `task-success-evaluation`: Durable scoped task-level outcome evaluation records and reports for external agent integrations.
- `feedback-ranking-rollout-governance`: Scoped governance for applying usefulness, task-success, and verification signals to search/context ranking through dry-run, activation, impact reporting, and rollback.

### Modified Capabilities

- `scope-proof-and-session-loop`: Link task evaluations to memory sessions, turns, verification history, and session reports.
- `memory-usefulness-feedback`: Allow task evaluations to reference usefulness feedback summaries and contribute bounded task-success aggregate signals.
- `memory-search-contract`: Replace the prior rejection of scope-wide feedback-aware ranking with governed scoped rollout policy behavior.
- `context-assembly`: Replace the prior rejection of scope-wide feedback-aware context ranking with governed scoped rollout policy behavior.
- `memory-quality-admission-repair`: Map repeated task-level memory contribution failures into bounded quality findings and approval-gated repair recommendations.
- `admin-inspection-surface`: Expose task evaluation and ranking rollout inspection through existing admin-only boundaries.
- `service-observability`: Add low-cardinality metrics, structured logs, and diagnostics for task evaluations and ranking rollout policy lifecycle.
- `self-hosting-bootstrap`: Document the service-side task-success and ranking rollout loop for self-hosted operators.

## Impact

- Affected APIs: new public or service-side task evaluation endpoints; new admin ranking rollout endpoints; extended session report, search diagnostics, context diagnostics, quality diagnostics, and OpenAPI schemas.
- Affected storage: PostgreSQL migrations for task evaluation records, task evidence links, task summary materialization or rebuild helpers, ranking rollout policies, rollout impact reports, and rollback audit history.
- Affected services: memory session service, usefulness feedback service, retrieval service, context assembler, quality service, telemetry, HTTP handlers, and PostgreSQL repositories.
- Affected docs/tests: OpenAPI tests, repository tests, service tests, HTTP tests, retrieval/context ranking tests, isolation tests, observability tests, and self-hosting documentation.
- Artifact references: use `openspec validate task-success-evaluation-and-feedback-ranking-rollout --strict` before implementation and `openspec instructions apply --change task-success-evaluation-and-feedback-ranking-rollout --json` to work tasks.
