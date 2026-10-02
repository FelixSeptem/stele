## 1. Contract And Domain Semantics

- [x] 1.1 Define contradiction keys, mutually-exclusive policy classes, pair/group candidates, review states, and temporal dispositions; verify serialization round-trips preserve deterministic ordering and reject unbounded or incomplete inputs.
- [x] 1.2 Add contradiction candidate provenance fields for both source versions, validity intervals, evidence digests, watermarks, uncertainty, policy, and replay identity; verify append-only identity and redaction tests.
- [x] 1.3 Implement exact-scope and lifecycle eligibility checks for both evidence sides; verify foreign, hidden, suppressed, forgotten, deleted, stale, and redacted sources never become eligible.
- [x] 1.4 Implement half-open interval comparison for overlap, temporal coexistence, and unresolved temporal state; verify disjoint historical facts are not classified as contradictions.

## 2. Bounded Detection And Provider Boundary

- [x] 2.1 Build deterministic fact grouping and bounded pair/group selection from normalized contradiction keys; verify pair, evidence, output, and execution budgets fail closed.
- [x] 2.2 Add optional provider classification for an already-selected pair without allowing provider output to set scope, evidence, temporal state, activation, or canonical mutation; verify refusal and malformed output quarantine.
- [x] 2.3 Normalize contradiction results into the existing reasoning candidate envelope with sorted citations, overlap interval, uncertainty, and stable fingerprints; verify identical fixtures produce identical identities.
- [x] 2.4 Add stale watermark, incompatible schema, timeout, budget, and incomplete-provenance dispositions; verify no failure path creates an active insight.

## 3. Governed Admission And Review

- [x] 3.1 Extend reserved-insight policy validation with contradiction-specific evidence, temporal overlap, uncertainty, review, freshness, and rollback gates; verify disabled and review-required policies fail closed.
- [x] 3.2 Route eligible contradiction candidates through ordinary reserved activation, idempotency, append-only versioning, and audit persistence; verify duplicate apply does not create duplicate insight versions.
- [x] 3.3 Add contradiction review and feedback transitions for confirmed, coexists, incorrect, and stale outcomes; verify each transition preserves prior evidence and attribution.
- [x] 3.4 Implement source-correction handling that marks affected candidates stale or schedules bounded rebuild work without mutating canonical versions; verify stale history remains inspectable.

## 4. Replay, Context, And Observability

- [x] 4.1 Extend offline/shadow replay planning and reports with contradiction, temporal_coexistence, unresolved_temporal, stale_evidence, review_required, and would_activate categories; verify replay never activates state.
- [x] 4.2 Add durable apply execution for reviewed contradiction candidates using existing worker retry, lease, idempotency, and audit paths; verify restart and retry recovery do not duplicate transitions.
- [x] 4.3 Keep candidates, shadow results, unresolved conflicts, and temporal-coexistence records out of default retrieval/context; verify an authorized fresh reviewed section includes only cited active insights within budget.
- [x] 4.4 Add low-cardinality contradiction metrics, lifecycle logs, and authorized aggregate diagnostics; verify source content, claims, scopes, identifiers, prompts, payloads, and raw errors are excluded.

## 5. Persistence And Integration Verification

- [x] 5.1 Add or extend PostgreSQL migrations and repositories for contradiction candidate/version/review metadata using existing scoped and append-only conventions; verify migration manifest and dirty-schema checks.
- [x] 5.2 Add focused Go tests for key normalization, evidence binding, temporal coexistence, provider refusal, deterministic replay, policy disablement, review transitions, context exclusion, and rollback.
- [x] 5.3 Add owned PostgreSQL + pgvector integration tests for candidate persistence, exact-scope isolation, stale source handling, durable apply recovery, and redacted diagnostics; verify reproducible reports.

## 6. Documentation And Release Gates

- [x] 6.1 Update OpenAPI/admin and operator documentation for contradiction replay, review, temporal dispositions, policy handoff, and non-authoritative modes; verify generated contract checks.
- [x] 6.2 Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `openspec validate --all --strict`, and `git diff --check`; verify contradiction activation remains disabled unless an explicit policy is present.
- [x] 6.3 Record owned PostgreSQL + pgvector shadow evidence, rollback evidence, and bounded diagnostic output; verify no activation rollout is claimed without a fresh compatible exact-scope release gate.
