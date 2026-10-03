# Governed operation policy precedence

Stele evaluates governed operations through one fixed order. The evaluator is
metadata-only and does not replace the policy stores, PostgreSQL repositories,
durable work queue, or canonical and derived mutation services.

## Evaluation order

1. **Scope** — the request must carry the exact tenant, project, and namespace
   granted to the principal. A foreign scope stops evaluation before target,
   lifecycle, policy, or idempotency state is read.
2. **Lifecycle** — referenced records must be visible for the operation. A
   suppressed, forgotten, deleted, or otherwise unavailable target produces a
   bounded lifecycle denial.
3. **Grant** — the principal and its exact grant must authorize the operation.
4. **Approval** — an enabled, fresh, compatible policy version must explicitly
   allow the operation when the owning capability requires approval.
5. **Replay** — an identical normalized request reuses the original result;
   conflicting reuse of an idempotency key fails closed.
6. **Handoff** — the operation is passed to the existing asynchronous
   governance, lifecycle, or admission boundary.
7. **Mutation** — only the existing append-only canonical or derived mutation
   path may change durable state.

The current contract identifier is
`governed-operation-precedence-v1`. It is carried with durable intent records
and worker processing so a restart cannot silently apply a different order.

## Bounded outcomes

The public and non-admin diagnostic vocabulary is limited to the fixed stage and
outcome categories exposed by `internal/memory/operation_precedence.go`.
Responses and telemetry omit scope values, payloads, target identifiers,
credentials, provider responses, and raw errors. Authorized inspection can
include bounded counts, contract versions, and redacted references.

## Rollback and re-enable

Disabling or rolling back a policy stops new acceptance at the approval gate and
retains submitted records, provenance, decisions, and lifecycle history.
Re-enabling a compatible policy for the same exact scope resumes only eligible
pending work after the complete precedence sequence runs again.

## Rollout and rollback procedure

Enable the contract in this order: intent submission/worker processing, insight
admission, provider/MCP operations, then privileged lifecycle and manual
mutation handlers. Run the exact-scope conformance report after each boundary
is enabled and retain the report with its contract version.

If a boundary must be rolled back, disable enforcement at its approval or
handoff boundary. Leave submitted records, decision metadata, queue references,
provenance, and audit history intact, and route new work through the owning
capability's existing disabled or held state. Re-enable only after the same
scope has a compatible policy version and the conformance report is complete.

## Verification

Focused checks:

```text
go test ./internal/memory ./internal/provider ./internal/insights ./internal/jobs ./internal/telemetry
go test ./internal/storage/postgres
```

Repository contract checks:

```text
openspec validate --all --strict
git diff --check
```

The product conformance fixtures must run against one exact scope and remain
diagnostic. They must not execute an external agent, activate data during
shadow/replay, or change default retrieval and context behavior.
