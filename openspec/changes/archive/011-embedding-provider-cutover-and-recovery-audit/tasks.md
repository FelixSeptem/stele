## 1. Durable Cutover Model

- [x] 1.1 Add domain contracts for cutover plans, cutover items, plan actions, and embedding recovery history query inputs
- [x] 1.2 Add PostgreSQL schema and repository support for cutover plans, cutover items, and optional cutover attribution on embedding recovery ledger rows
- [x] 1.3 Add repository tests for plan creation, plan detail, plan action state transitions, and scoped recovery history reads

## 2. Admin Inspection And Control Surface

- [x] 2.1 Add service methods and admin HTTP or OpenAPI routes for creating, listing, reading, activating, pausing, and cancelling embedding cutover plans
- [x] 2.2 Add admin HTTP or OpenAPI routes for scope-level and memory-level embedding recovery history inspection with actor, action, and time filters
- [x] 2.3 Add handler and service tests covering scope isolation, invalid action transitions, runtime target validation, and cutover-linked recovery history responses

## 3. Scheduler Rollout Orchestration

- [x] 3.1 Add scheduler-owned cutover wave dispatch that advances active plans through the existing embedding rebuild eligibility path
- [x] 3.2 Enforce lease-safe pause and cancel behavior so already rebuilding work keeps worker ownership while unscheduled future waves stop advancing
- [x] 3.3 Add orchestration tests for bounded wave progression, failure hotspot visibility, and reverse-cutover rollback guidance assumptions

## 4. Docs And Verification

- [x] 4.1 Update self-hosting docs with provider cutover rollout, pause, cancel, and rollback workflows
- [x] 4.2 Add smoke-check guidance and examples for plan inspection plus embedding recovery history during rollout incidents
- [x] 4.3 Run targeted admin, repository, scheduler, config, and OpenAPI coverage; then update proposal-linked artifacts if route or rollout behavior changes
