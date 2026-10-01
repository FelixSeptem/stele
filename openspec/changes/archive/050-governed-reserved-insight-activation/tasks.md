## 1. Policy and domain contracts

- [x] 1.1 Define the versioned, exact-scope reserved-insight activation policy and bounded policy validation; verify disabled-by-default, scope mismatch, expiry, provider/schema compatibility, and rollback-state unit tests pass.
- [x] 1.2 Define candidate admission/disposition envelopes with policy version, source watermark, provenance, confidence/uncertainty, candidate fingerprint, and idempotency metadata; verify malformed, over-budget, foreign-scope, hidden-evidence, and duplicate inputs fail closed.
- [x] 1.3 Add per-type policy capability with `hypothesis` as the only initially activatable type and `goal`, `contradiction`, and `causal_link` disabled; verify type-disabled dispositions are deterministic and explicit.

## 2. Governed admission and lifecycle

- [x] 2.1 Implement the candidate-to-admission evaluator that reuses existing scope, lifecycle, evidence, provenance, and insight-governance checks; verify an eligible hypothesis creates a derived version without mutating canonical memory.
- [x] 2.2 Persist append-only policy decisions, activation audit records, evidence links, and idempotent candidate fingerprints in PostgreSQL-backed governance state; verify retries do not duplicate insight versions or lifecycle transitions.
- [x] 2.3 Implement suppression, expiry, stop, and rollback behavior for an activation policy; verify rollback prevents new admissions while preserving prior evidence and audit history.
- [x] 2.4 Keep provider output unable to activate or mutate directly; verify direct activation, canonical mutation, evidence deletion, and lifecycle-bypass responses are rejected with no side effects.

## 3. Replay, shadow, and diagnostics

- [x] 3.1 Extend derived-insight replay to evaluate activation policy/provider/source-watermark compatibility and emit reject, quarantine, would-activate, stale, and incomplete categories; verify repeated normalized replay is deterministic and non-mutating.
- [x] 3.2 Add shadow evaluation for live candidates that records would-disposition only; verify shadow output never changes active insight state, default retrieval, or context assembly.
- [x] 3.3 Expose bounded authorized inspection for policy versions, type counts, dispositions, freshness, and redacted evidence references; verify prompts, chain-of-thought, credentials, raw provider payloads, hidden IDs, and foreign scopes are absent.

## 4. Public contracts and rollout documentation

- [x] 4.1 Publish any required OpenAPI/admin diagnostic schema changes for activation policy and disposition reports; verify route/schema/auth coverage and exact-scope denial tests pass.
- [x] 4.2 Update self-hosting documentation with disabled, offline, shadow, reviewed hypothesis activation, stop, rollback, and reserved-type sequencing; verify documentation consistency and smoke checks pass.
- [x] 4.3 Update the v1 roadmap from the archived P8.4 adapter to P8.5 governed reserved-insight activation; verify roadmap status tests identify this change as the active next proposal.
- [x] 4.4 Run focused reasoning/insight/replay tests, full Go tests, race/vet checks, strict OpenSpec validation, documentation checks, and diff checks; verify no provider endpoint or credential is required for release evidence.
