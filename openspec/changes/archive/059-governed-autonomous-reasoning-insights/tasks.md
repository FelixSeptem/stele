## 1. Contract And Schema

- [x] 1.1 Define provider-neutral reasoning request, candidate, evidence citation, scope proof, watermark, uncertainty, and disposition types; verify serialization round-trips preserve deterministic field ordering and reject missing required bounds.
- [x] 1.2 Add PostgreSQL migrations/models for candidate envelopes, evidence digests, replay identities, derivation provenance, and bounded dispositions using existing scoped/versioned/audit conventions; verify dirty-schema startup and migration tests pass.
- [x] 1.3 Define reserved reasoning type vocabulary and per-run limits without enabling any type by default; verify unsupported types are rejected or quarantined in unit tests.

## 2. Provider Derivation Pipeline

- [x] 2.1 Implement pre-invocation scope, lifecycle, redaction, watermark, budget, and provider-capability validation; verify foreign, hidden, stale, and over-budget evidence never reaches the provider.
- [x] 2.2 Normalize provider responses into candidates while removing raw prompts, chain-of-thought, and provider payloads; verify valid citations and uncertainty are retained and malformed or unsafe output is quarantined.
- [x] 2.3 Implement fail-closed timeout, schema mismatch, capability fallback, and unsafe-operation handling; verify no failure path creates canonical or active derived mutations.
- [x] 2.4 Compute stable request and candidate replay identities from normalized inputs, sorted evidence, scope proof, watermarks, provider/schema identity, and policy versions; verify identical fixtures produce identical identities.

## 3. Governed Admission Integration

- [x] 3.1 Add the explicit handoff from a validated reasoning candidate to the existing reserved-insight activation policy; verify admission requires exact scope, evidence digest, freshness, uncertainty, compatibility, and idempotency proof.
- [x] 3.2 Persist accepted candidates and activated insight versions with provider/schema, policy, provenance, evidence, and audit metadata without mutating canonical memory; verify append-only history and duplicate retry behavior.
- [x] 3.3 Implement policy disablement and rollback handling for reasoning-derived admissions; verify rollback stops new admissions while preserving prior versions and evidence history.

## 4. Offline And Shadow Replay

- [x] 4.1 Extend durable replay planning and reports for normalized reasoning envelopes, provider identity, watermarks, compatibility, and stable rejection/quarantine/skip categories; verify admin bounds are required before evidence scanning.
- [x] 4.2 Implement non-authoritative offline/shadow evaluation and would-activate reporting; verify replay never changes active insight state, canonical memory, default retrieval, or ordinary context.
- [x] 4.3 Add durable apply execution that reuses ordinary admission, idempotency, retry, and audit paths; verify worker restart/retry does not duplicate insight versions or lifecycle transitions.

## 5. Context And Observability

- [x] 5.1 Enforce exclusion of candidates and reserved insights from default retrieval/context and add explicit authorized reasoning-section checks for scope, lifecycle, freshness, citations, and budget; verify shadow and stale results are excluded.
- [x] 5.2 Add low-cardinality reasoning metrics and bounded lifecycle logs for derivation, replay, shadow, fallback, eligibility, handoff, and rollback; verify sensitive labels are rejected, redacted, or bucketed.
- [x] 5.3 Add authorized aggregate reasoning diagnostics with stable reason categories and no hidden/foreign identifiers or provider payloads; verify diagnostics are scope-bound and redacted.

## 6. Verification And Documentation

- [x] 6.1 Add focused Go tests for evidence binding, scope/lifecycle/redaction isolation, deterministic replay, stale dependency fallback, provider refusal, canonical immutability, activation idempotency, and rollback.
- [x] 6.2 Run owned PostgreSQL + pgvector integration/shadow tests covering migration, candidate persistence, replay reports, retrieval/context exclusion, and restart recovery; verify results are reproducible and redacted.
- [x] 6.3 Update OpenAPI/admin and operator documentation for bounded reasoning replay, diagnostics, policy handoff, and non-authoritative modes; verify docs consistency and generated contract checks pass.
- [x] 6.4 Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `openspec validate --all --strict`, and `git diff --check`; record any activation limitation as shadow-only evidence until the release gate is met.
