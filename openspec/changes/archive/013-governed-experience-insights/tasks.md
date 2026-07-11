## 1. Domain And Storage Foundation

- [x] 1.1 Add derived insight domain types for insight id, type, lifecycle state, confidence, derivation metadata, evidence references, lesson payload, and validation rules.
- [x] 1.2 Add PostgreSQL schema and migrations for derived insights, insight evidence links, lifecycle/audit transitions, and derivation fingerprints.
- [x] 1.3 Implement repository create, update, list, read, evidence, and lifecycle transition methods with scope isolation tests.
- [x] 1.4 Add unit tests that reject unsupported autonomous insight types, missing evidence, invalid scope, and ungrounded lessons.

## 2. Failure Pattern Derivation

- [x] 2.1 Define deterministic failure evidence inputs from raw event failures, job execution failures, recovery history, embedding rebuild failures, and procedural/canonical memory evidence.
- [x] 2.2 Implement stable failure pattern fingerprinting from scope, normalized failure key, evidence kind, and bounded evidence window.
- [x] 2.3 Implement a derivation evaluator that activates `failure_pattern` only when minimum repeated evidence thresholds are met.
- [x] 2.4 Implement evidence-backed `lesson` projection tied to a source failure pattern without free-form unsupported lesson creation.
- [x] 2.5 Add tests for repeated evidence activation, isolated evidence rejection, idempotent re-derivation, evidence updates, and confidence changes.

## 3. Worker And Scheduler Integration

- [x] 3.1 Add a scope-aware derived insight derivation job that runs through the existing scheduler or worker maintenance model.
- [x] 3.2 Ensure derivation jobs use durable job execution records and fail safely without partially activating unsupported insights.
- [x] 3.3 Wire configuration defaults for derivation cadence, batch size, and minimum evidence threshold.
- [x] 3.4 Add tests for duplicate scheduler windows, restart-safe idempotency, and foreground ingest/mutation paths not running insight derivation.

## 4. Admin Inspection Surface

- [x] 4.1 Add admin list endpoint for derived insights with scope, type, lifecycle, confidence, and evidence-count filters.
- [x] 4.2 Add admin detail endpoint that returns insight payload, evidence references, lifecycle history, derivation metadata, and lesson output when present.
- [x] 4.3 Add admin lifecycle action for suppressing noisy derived insights with actor and reason attribution.
- [x] 4.4 Update OpenAPI contract and tests for derived insight inspection, hidden insight inspection, and lifecycle action responses.

## 5. Context Assembly Integration

- [x] 5.1 Extend context assembly request options to include governed experience insights explicitly.
- [x] 5.2 Add `known_failures` and `experience_lessons` response sections with citations and budget-aware trimming.
- [x] 5.3 Enforce lifecycle visibility and scope isolation so suppressed, forgotten, deleted, and out-of-scope insights are excluded from context assembly.
- [x] 5.4 Add tests for default exclusion, requested inclusion, budget pressure, citations, and hidden insight filtering.

## 6. Documentation And Verification

- [x] 6.1 Update self-hosting and operator documentation with derived insight derivation, inspection, suppression, and context assembly examples.
- [x] 6.2 Document non-goals for MCP, autonomous hypothesis/causal/goal inference, and reasoning-provider work.
- [x] 6.3 Run targeted tests for domain validation, repository behavior, derivation job, admin HTTP, context assembly, OpenAPI, and scheduler integration.
- [x] 6.4 Run the full Go test suite with `go test ./... -count=1`.
- [x] 6.5 Validate the OpenSpec change with `openspec validate governed-experience-insights --strict`.
