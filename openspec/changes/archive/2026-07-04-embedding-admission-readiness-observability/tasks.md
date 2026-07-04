## 1. Admission And Readiness Core

- [x] 1.1 Add shared admission decision, diagnostic finding, blocker, warning, and readiness result types with unit tests for decision derivation.
- [x] 1.2 Add metrics-safe finding code and label normalization helpers with tests that reject high-cardinality labels.
- [x] 1.3 Add a mode-aware readiness evaluator abstraction that can compose service, PostgreSQL, and optional provider checks.

## 2. Embedding Cutover Admission

- [x] 2.1 Implement embedding cutover preflight input validation and report shaping for draft plans.
- [x] 2.2 Add repository reads for eligible memory totals, class breakdowns, missing or drifted semantic state, and same-scope active or paused plan conflicts.
- [x] 2.3 Implement the embedding cutover admission evaluator with blockers for unresolved target, unsupported class route, scoped plan conflict, and zero eligible memory.
- [x] 2.4 Update cutover activation to rerun admission and reject blocker reports before plan activation, membership registration, or wave scheduling.
- [x] 2.5 Add tests for allowed preflight, each blocker class, warning-only reports, draft coexistence, scoped active or paused conflict, and activation revalidation.

## 3. Admin API And OpenAPI

- [x] 3.1 Add `POST /v1/admin/embedding/cutovers/{cutover_plan_id}:preflight` with the same admin auth and scope boundary as other cutover routes.
- [x] 3.2 Return the structured admission report from both preflight and denied activation responses.
- [x] 3.3 Add `GET /livez`, `GET /readyz`, and `GET /metrics` handlers with mode-aware readiness wiring.
- [x] 3.4 Update OpenAPI contract and tests for preflight, health endpoints, metrics endpoint, and admission report schemas.

## 4. Metrics And Runtime Wiring

- [x] 4.1 Add concrete metrics recording for cutover preflight and activation admission decisions, blocker codes, and warning codes.
- [x] 4.2 Add concrete metrics recording for active or paused cutover plan counts, cutover item status counts, rebuild backlog counts, provider probe results, and scheduler wave dispatch.
- [x] 4.3 Wire runtime readiness so `api` checks service and PostgreSQL, while `worker` and `scheduler` include provider readiness only when embedding rebuild or cutover execution is enabled.
- [x] 4.4 Add tests for metrics output labels and readiness behavior across `api`, `worker`, and `scheduler` modes.

## 5. Documentation And Verification

- [x] 5.1 Update self-hosting documentation with the draft plan, preflight, activation, readiness, and metrics workflow.
- [x] 5.2 Add or update smoke-check examples for failed admission, warning-only admission, and provider readiness degradation.
- [x] 5.3 Run targeted package tests for admission/readiness, embedding cutover, admin HTTP, OpenAPI, jobs, and PostgreSQL repository behavior.
- [x] 5.4 Run the full Go test suite with `go test ./... -count=1`.
