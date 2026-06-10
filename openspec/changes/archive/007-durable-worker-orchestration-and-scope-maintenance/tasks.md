## 1. Raw Event Failure And Lease Contract

- [x] 1.1 Define the durable raw event execution fields and derived internal states for retryable failure, retry wait, processed, and exhausted outcomes
- [x] 1.2 Define the bounded retry policy inputs, including max attempts, retry backoff, and failure error summary handling
- [x] 1.3 Define the renewable lease contract, including ownership checks and renewal failure behavior

## 2. Repository And Governance Worker Changes

- [x] 2.1 Add repository contracts for recording claimed raw event failure, renewing a claim lease, and excluding exhausted items from future claims
- [x] 2.2 Update the governance worker orchestration path to persist retryable failure and exhausted terminal outcomes instead of relying only on lease expiry
- [x] 2.3 Add regression tests for retryable failure, exhausted raw events, and long-running lease renewal behavior

## 3. Scope-Aware Maintenance Dispatch

- [x] 3.1 Define repository or query contracts for discovering eligible maintenance scopes without relying only on a single configured default scope
- [x] 3.2 Introduce scheduler dispatch logic that runs scope-bound maintenance jobs per eligible scope and keeps runtime-global cleanup distinct
- [x] 3.3 Verify idempotent execution at the `job + scope + cadence window` level across repeated scheduler triggers and process restarts

## 4. Runtime Wiring And Configuration

- [x] 4.1 Extend runtime configuration with durable worker retry and lease renewal settings plus maintenance scope batch limits
- [x] 4.2 Wire the new worker and scheduler behavior through `internal/app` without changing the existing `api`, `worker`, and `scheduler` mode boundary
- [x] 4.3 Add targeted tests for runtime assembly defaults and config validation paths

## 5. Documentation And Verification

- [x] 5.1 Update operator-facing docs with retry budget, exhausted raw event semantics, and scope-aware maintenance behavior
- [x] 5.2 Document the separation between scope-bound maintenance jobs and runtime-global cleanup execution
- [x] 5.3 Run focused regression verification for `internal/jobs`, `internal/governance`, `internal/storage/postgres`, and `internal/app`
