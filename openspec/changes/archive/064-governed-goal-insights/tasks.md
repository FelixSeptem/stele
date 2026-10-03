## 1. Goal Contract And Normalization

- [x] 1.1 Add the normalized goal metadata envelope with bounded title, summary, state, validity interval, review state, uncertainty, and policy fields; verify valid and malformed goal fixtures are accepted or rejected deterministically.
- [x] 1.2 Extend reasoning candidate normalization and replay identity inputs for `goal`; verify equivalent normalized requests produce identical identities and changed state, evidence, or policy inputs produce different identities.
- [x] 1.3 Add unit coverage for allowed goal states, interval ordering, size limits, required scope proof, compatibility metadata, and provider attempts to request direct activation or execution; verify unsafe candidates are quarantined.

## 2. Evidence And Derivation

- [x] 2.1 Integrate `goal` into the provider-neutral offline/shadow derivation path without adding a provider-specific API; verify the path remains bounded by evidence, time, output, and uncertainty limits.
- [x] 2.2 Validate goal evidence against exact scope, lifecycle visibility, source watermark, provenance, redaction policy, and optional validity interval; verify hidden, foreign, stale, redacted, and invalid evidence fail closed before handoff.
- [x] 2.3 Preserve goal state, validity, evidence digest, source watermark, provider/schema identity, and derivation mode in the retained candidate; verify reviewable candidates contain enough metadata for replay and audit.
- [x] 2.4 Add reasoning tests for offline, shadow, stale dependency, missing evidence, budget exhaustion, and deterministic replay outcomes; verify no canonical or active insight mutation occurs.

## 3. Policy And Review Handoff

- [x] 3.1 Extend reserved insight policy validation with a goal-specific policy that defaults to disabled, requires exact scope and compatible versions, and declares evidence, freshness, allowed state, review, and rollback rules; verify incomplete policies are incompatible.
- [x] 3.2 Route goal admission through the governed-operation precedence evaluator and explicit review handoff; verify scope, lifecycle, grant, policy, replay, handoff, and mutation stages stop in order and provider output cannot set active state.
- [x] 3.3 Persist review-required, would-activate, rejected, quarantined, and policy-disabled goal dispositions through the existing append-only derived insight history; verify retries are idempotent and prior evidence remains inspectable.
- [x] 3.4 Add rollback and disablement behavior for goal policy versions; verify new admissions stop for the affected exact scope while prior goal history and audit attribution remain unchanged.

## 4. Visibility And Observability

- [x] 4.1 Add explicit default filters so goal candidates and derived goal records remain absent from ordinary retrieval and context assembly; verify hidden goal existence is not disclosed through counts, ranking, or errors.
- [x] 4.2 Extend bounded reasoning telemetry and metrics with goal operation, state, review, freshness, replay, policy, and disposition categories; verify labels exclude goal text, scope values, identifiers, prompts, payloads, and raw errors.
- [x] 4.3 Add authorized, redacted goal diagnostics with aggregate state, review, policy, replay, freshness, and rollback categories; verify foreign scope and hidden evidence produce only bounded non-disclosing results.
- [x] 4.4 Add unit and integration coverage for visibility filters, telemetry redaction, diagnostics authorization, and lifecycle transitions; verify the existing contradiction and hypothesis paths remain unchanged.

## 5. PostgreSQL And Real-Stack Conformance

- [x] 5.1 Confirm the existing derived insight schema can preserve bounded goal metadata; if a durable index or column is required, add a reversible versioned migration and verify migration apply, rollback, and manifest tests.
- [x] 5.2 Extend PostgreSQL repository tests for goal candidate persistence, append-only review/lifecycle history, exact-scope reads, idempotent replay, policy rollback, and redacted inspection; verify no canonical memory row is mutated.
- [x] 5.3 Add a bounded product-verification/conformance path using an explicitly supplied PostgreSQL + pgvector DSN; verify offline/shadow, review-required, disabled-policy, replay, rollback, restart, and default-visibility cases and report prerequisite skips without readiness claims.
- [x] 5.4 Document the operator run, evidence categories, cleanup, and failure interpretation; verify the documentation contains no credentials, DSNs, raw goal content, or hidden identifiers.

## 6. Contract, Roadmap, And Release Verification

- [x] 6.1 Update OpenAPI or authorized diagnostic contract artifacts only where the goal review and bounded diagnostics are externally exposed; verify generated or embedded contract validation passes.
- [x] 6.2 Reconcile the authoritative roadmap so archived changes 060–063 are no longer listed as pending and the next candidate references reflect the current `governed-goal-insights` proposal; verify archive index and roadmap references agree.
- [x] 6.3 Run focused tests, full `go test ./... -count=1`, `go vet ./...`, `git diff --check`, and `openspec validate --all --strict`; verify all checks pass and no default retrieval or context behavior changes.
