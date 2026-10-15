## Why

PC1–PC3 now return a typed, authorized and budgeted Provider context, but consumers cannot identify the final selection with a Stele-issued digest or safely compare it after restart. PC4 closes that gap with deterministic projection identity and explicit fresh-read continuity, preserving current authorization and lifecycle checks.

## What Changes

- **BREAKING**: require versioned `result.context_digest` and `result.continuity` in both Provider context response modes, including genuine empty responses. Publish extension discovery and strict-consumer migration guidance without treating `provider-v1` alone as feature negotiation.
- Define `context-digest-v1`: SHA-256 over a versioned canonical representation of the final authorized ordered projection, scope, mode, effective budget, authorized provenance and relevant policy identity. Exclude transport and volatile operation metadata; distinguish context identity from OpenAPI and synchronization digests.
- Accept optional closed `input.continuity` requests to issue a reference or freshly compare a previous reference. Every comparison reauthorizes and assembles current context; expose `unchanged` or `changed` alongside the full fresh result.
- Persist bounded, expiring metadata-only continuity receipts in PostgreSQL, bound to the full runtime identity and normalized originating read. Define safe unavailable, expired, changed-request, incompatible and dependency outcomes with atomic capacity enforcement and cleanup.
- Include digest/continuity overhead in PC3 exact complete-body admission. Advertise historical snapshot replay and context watermark continuity as unsupported.
- Extend public consumer conformance, fresh owned PostgreSQL/pgvector restart verification, migration checks and maintenance documentation together.

## Capabilities

### New Capabilities

- `provider-context-digest-and-continuity`: deterministic final-projection identity, versioned fresh-compare semantics, authorization-safe durable receipts, expiry, retention and bounded persistence.

### Modified Capabilities

- `agent-runtime-provider-adapter`: explicit continuity input, mandatory typed digest/continuity result, capability negotiation and consumer migration.
- `provider-context-budget-accounting`: digest/continuity envelopes count toward exact final-body admission and accounting.
- `context-disclosure-policy`: continuity cannot restore historical disclosure or authorize cached content reuse.
- `runtime-memory-provider-contract`: digest/continuity fixtures, fresh durable restart evidence and same-change documentation gates.

## Impact

Changes touch `openapi/`, Provider DTOs/capabilities/admission, the context HTTP route, PostgreSQL repositories/migrations, bounded scheduler cleanup, configuration and assurance fixtures. Reuse existing context assembly, binding, source-version and disclosure services; standard-library JSON, cryptographic randomness and SHA-256 are sufficient. Review mature library and existing infrastructure choices before implementation introduces any dependency.

Strict consumers must discover the new extensions and accept the additional required fields. Operators gain explicit receipt TTL/capacity/cleanup settings and an additive migration. Danny stores Stele's digest and opaque reference; its tool/write/business authority and any separately named business projection fingerprint remain external.

## Non-goals

- Exact historical snapshot replay, persisted context bodies, sync cursor interchange or a context event watermark.
- A new retrieval, ranking, eligibility, embedding, canonicalization or local packing algorithm.
- PC5 durable task/run reference APIs, PC6 turn/outcome projection, SDK/UI or external runtime business logic.
- Treating a digest, continuity reference, request ID or idempotency key as permission, task completion evidence, or a commit receipt.

## References

- [Roadmap PC4](../../../docs/roadmaps/2026-05-28-stele-v1-roadmap.md#pc4-context-digest-and-continuity)
- [PC3 design](../archive/071-provider-context-budget-and-selection-diagnostics/design.md)
- [Provider adapter specification](../../specs/agent-runtime-provider-adapter/spec.md)
- [Budget specification](../../specs/provider-context-budget-accounting/spec.md)
- [Disclosure specification](../../specs/context-disclosure-policy/spec.md)
- [Proposal workflow skill](../../../.codex/skills/openspec-propose/SKILL.md)
- [Implementation workflow skill](../../../.codex/skills/openspec-apply-change/SKILL.md)
- Planning commands: `openspec status --change provider-context-digest-and-continuity`, `openspec validate provider-context-digest-and-continuity --strict`.
