## 1. Policy and persistence foundations

- [x] 1.1 Add additive PostgreSQL schema and migration support for versioned goal visibility policies, append-only visibility decisions, review attribution, and rollback transitions; verify migration apply and rollback checks pass.
- [x] 1.2 Extend exact-scope policy models and precedence evaluation with principal grant, scope proof, review, freshness, evidence, replay, expiry, and rollback gates; verify unit tests cover every fail-closed disposition.
- [x] 1.3 Add deterministic visibility replay identity and idempotent decision persistence using existing replay primitives; verify identical inputs return the same bounded decision without duplicate inclusion.

## 2. Goal review and admin contract

- [x] 2.1 Implement the authorized exact-scope goal review service and redacted projection; verify authorized requests return bounded review data while invalid scope or grant requests are indistinguishable from no-record responses.
- [x] 2.2 Add admin inspection handlers and OpenAPI contract for review, policy, freshness, evidence, inclusion, disablement, and rollback categories; verify contract and isolation tests reject raw goal content, prompts, provider payloads, and foreign identifiers.
- [x] 2.3 Record append-only review and visibility lifecycle transitions with actor or policy attribution; verify history remains inspectable after disablement and rollback.

## 3. Experimental context projection

- [x] 3.1 Add the optional `goal_context` request selector and response section to the context contract while preserving backward-compatible ordinary responses; verify ordinary clients and snapshots are unchanged when the selector is absent.
- [x] 3.2 Implement isolated `goal_context` assembly behind successful visibility-policy evaluation; verify the section is omitted on any failed gate and does not change ordinary candidates, ordering, budgets, counters, or fallback behavior.
- [x] 3.3 Enforce provider and replay boundaries so neither can directly activate or expose a goal; verify unauthorized direct activation is rejected or quarantined with a bounded precedence result.

## 4. Observability and verification

- [x] 4.1 Add low-cardinality metrics, bounded logs, and authorized diagnostics for review, visibility evaluation, inclusion, omission, freshness, fallback, disablement, and rollback; verify sensitive fields are rejected, redacted, or bucketed.
- [x] 4.2 Add ingestion, review, retrieval, context, isolation, replay, rollback, and redaction tests for profile and goal lifecycle interactions; verify suppressed, forgotten, deleted, stale, and foreign evidence never enters `goal_context`.
- [x] 4.3 Add product-verification evidence and documentation for default-disabled rollout, exact-scope enablement, ordinary-context compatibility, and rollback; verify the repository quality gates and OpenSpec validation pass.

## 5. Release readiness

- [x] 5.1 Run `go test ./... -count=1`, `go vet ./...`, `git diff --check`, and the relevant product-verification scripts; verify all commands complete successfully.
- [x] 5.2 Run `openspec validate governed-goal-review-and-experimental-visibility --type change` and `openspec validate --all --strict`; verify the new change and repository specs pass without warnings.
