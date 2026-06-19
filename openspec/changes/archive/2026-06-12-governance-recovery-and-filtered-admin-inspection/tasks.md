## 1. Governance Raw Event Inspection Contract

- [x] 1.1 Define the derived admin-visible raw event states and filter vocabulary for governance inspection
- [x] 1.2 Add repository query contracts for listing raw events, reading one raw event detail, and reading recovery history
- [x] 1.3 Add regression tests for state derivation, scope filtering, and cursor-safe listing behavior

## 2. Recovery Action And Ledger Persistence

- [x] 2.1 Define recovery action contracts for `retry`, `reschedule`, and `requeue`, including actor and reason requirements
- [x] 2.2 Add a dedicated governance recovery ledger schema and transactional persistence behavior
- [x] 2.3 Add regression tests for action state guards, transaction safety, and leased or processed conflict behavior

## 3. Admin HTTP Surface

- [x] 3.1 Introduce `/v1/admin/governance/raw-events/...` routes for filtered inspection, detail read, recovery history, and recovery actions
- [x] 3.2 Enforce admin auth, scoped headers, actor attribution, and action validation without weakening existing public API boundaries
- [x] 3.3 Add handler and app assembly tests for response shapes, validation errors, and recovery conflict mapping

## 4. Worker Compatibility And Recovery Re-entry

- [x] 4.1 Ensure recovered raw events re-enter the existing worker poll path without introducing a side execution channel
- [x] 4.2 Verify `retry`, `reschedule`, and `requeue` preserve durable worker ownership and exhausted-state semantics
- [x] 4.3 Add regression coverage showing recovery actions do not execute leased items or bypass worker orchestration

## 5. Documentation And Verification

- [x] 5.1 Update operator-facing docs with governance recovery routes, action semantics, and audit expectations
- [x] 5.2 Document the first-phase boundaries: single-item recovery only, no bulk remediation, no leased takeover, no ignore or drop
- [x] 5.3 Run focused regression verification for `internal/app`, `internal/storage/postgres`, `internal/jobs`, and `internal/governance`
