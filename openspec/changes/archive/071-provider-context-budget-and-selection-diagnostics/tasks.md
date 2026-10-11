## 1. Public contract and configuration

Each checkbox is scoped to at most four hours and includes its own completion check. Implementation starts only after this draft is approved; live integration and final verification check the combined behavior.

- [x] 1.1 Publish OpenAPI request limits, required accounting, capability versions/ceilings and error schemas/examples; verify canonical examples and negative shapes through focused OpenAPI tests.
- [x] 1.2 Add failing serialized Provider request fixtures for omissions, explicit zero, nested unknown/duplicate/null/case fields, wrong types, portable integer overflow and unsupported tokens; verify each fixture exercises the actual public boundary.
- [x] 1.3 Implement strict public limit decoding and mapping while retaining positive item `budget` and existing disclosure/path defaults; verify all request-boundary fixtures pass without alias/retry rewrites.
- [x] 1.4 Add independent item/content configuration and resolved discovery limits with inherited content defaults, zero-content support and startup rejection of invalid bounds; verify config and capability tests cover every documented default and override.
- [x] 1.5 Normalize and independently clamp allocations before fanout arithmetic, removing the item/byte coupling without changing retrieval-result limits; verify large valid requests remain bounded and invalid arithmetic cannot overflow.

## 2. Final projection cost and accounting

- [x] 2.1 Add closed accounting DTOs with requested omission markers, effective limits, fixed contract/packing versions and deterministic enums; verify schema/serialization tests reject absent, malformed and privileged fields.
- [x] 2.2 Implement final disclosed content/item/citation costs using the same mode-specific DTO as output; verify multilingual/emoji, repeated placements, empty content, long paths/source metadata and canonical citation deduplication fixtures.
- [x] 2.3 Define the internal optional final-projection admission boundary without a retrieval-to-Provider import cycle or external callback/policy execution; verify package compilation and focused boundary tests preserve native/admin behavior.
- [x] 2.4 Implement deterministic bounded accounting reservation and exact self-counting final JSON serialization; verify digit-width transitions, escaping, repeated citations, no newline and equality with captured response-body bytes.
- [x] 2.5 Implement mandatory-envelope preflight and final limit defense with 413/no-partial output; verify minimum allocation, serializer/accounting disagreement and finalized-envelope error fixtures.

## 3. Existing assembly integration and safety

- [x] 3.1 Integrate whole-item admission after authoritative disclosure/source/lifecycle eligibility in the existing ordered selection stream; verify fit items keep baseline order and unsafe items cannot contribute costs/reasons or identifiers.
- [x] 3.2 Apply one allocation across profiles, session evidence, episodes, summaries and relations while retaining quotas, summary preference, deduplication, diversity and protected groups; verify deterministic ample/tight allocation regression fixtures.
- [x] 3.3 Preserve mandatory item citations and enforce unique citation limits while continuing only through the already bounded candidate stream; verify zero citations and oversized early candidates neither remove evidence nor trigger broader retrieval.
- [x] 3.4 Exercise projection/chunk evidence through the applicable internal admission contract while keeping PC2 public section gates closed; verify focused internal tests and public unsupported-section fixtures are separately identified and native defaults are unchanged.
- [x] 3.5 Produce safe selected/empty/budget-exhausted outcomes and ordered reasons/unsupported labels; verify fixtures with hidden, foreign, source-ineligible and high/restricted evidence expose no private counts, identities, text or ranking state.
- [x] 3.6 Recheck current grants and preserve validation/scope/disclosure/dependency outcomes; verify changed-grant and unavailable-authority fixtures return bounded denial or retryable 503 rather than empty/budget-exhausted success.

## 4. HTTP and consumer conformance

- [x] 4.1 Wire Provider/HTTP assembly to final admission and emit the exact finalized bytes; verify actual route bodies, both modes, default limits and client/server intersections match OpenAPI and usage.
- [x] 4.2 Extend offline consumer conformance for missing accounting, unsupported versions, incorrect counts, over-limit bodies and malformed categories; verify each becomes failed-contract rather than an empty pass and reports stay redacted.
- [x] 4.3 Add end-to-end response-boundary fixtures for multilingual/escaped content, large provenance, repeated citations and self-size boundaries; verify every successful response satisfies all four effective dimensions and failed responses emit no projection prefix.
- [x] 4.4 Run protected recall/order and lifecycle/scope regression coverage against the PC2 baseline with ample and constrained limits; verify public contract results remain separate from ranking metrics and optional raw diagnostics stay closed.

## 5. Fresh PostgreSQL and restart evidence

- [x] 5.1 Extend the owned public live fixture for budget discovery, official bootstrap/grants, labeled non-root ingestion and bounded governance completion; verify fixture provenance and raw-event/canonical completion without storing credentials or payloads.
- [x] 5.2 Run initial exact/prefix content/reference reads with zero/minimum/oversized allocations, safe omissions, unsupported sections and lifecycle/scope checks on fresh PostgreSQL/pgvector; verify actual bodies and save only bounded redacted usage/outcomes.
- [x] 5.3 Restart fixture API/worker and repeat eligible allocation, identity/evidence and safety checks; verify durable governance and restart results remain passing without claiming PC4 snapshots or PC5 reference APIs.
- [x] 5.4 Generate a linked initial/restart evidence report with pinned service/image/OpenAPI/disclosure/budget versions; verify stale/missing/skipped/degraded stages prevent readiness and focused internal section evidence is labeled separately.

## 6. Maintenance documentation and release checks

- [x] 6.1 Update README and Provider integration examples with discovery, explicit units, required accounting, zero/omitted behavior, exact body size, reservation, token refusal and strict consumer validation; verify examples match maintained OpenAPI and documentation checks pass.
- [x] 6.2 Update self-host/configuration/bootstrap and architecture/component/best-practice guidance for independent defaults, startup validation, deterministic admission, authority boundaries, errors, migration and rollback; verify every externally changed semantic has matching maintained guidance.
- [x] 6.3 Update affected MCP guidance and `skills/stele-memory/SKILL.md` for extension negotiation, unsupported public sections, source-data authority and no local ranking/packing or byte-to-token estimation; verify skill/examples consume the documented closed response.
- [x] 6.4 Update roadmap implementation status and write a requirement/task verification record linking commands, tests and fresh evidence; verify PC1/PC2 remain archived and PC4–PC6 remain pending.
- [x] 6.5 Run focused/full Go tests, relevant race coverage, `go vet ./...`, documentation checks, strict/all OpenSpec validation and whitespace checks; verify fresh output before claiming completion and record any genuine gate failures without substituting old evidence.
