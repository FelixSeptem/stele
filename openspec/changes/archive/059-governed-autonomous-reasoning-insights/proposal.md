## Why

Stele already has a provider boundary, governed derived insights, reserved-insight activation policy, and durable replay infrastructure, but it lacks one contract for turning optional model reasoning into evidence-backed, replayable candidates. A bounded offline/shadow path is needed now so reasoning-derived hypotheses and relationships can be measured and audited before any independently governed activation is allowed.

## What Changes

- Add a `governed-autonomous-reasoning-insights` capability for provider-backed candidate generation for reserved reasoning insight types such as `hypothesis`, `goal`, `contradiction`, and `causal_link`.
- Require every candidate to carry exact tenant/project/namespace scope, lifecycle eligibility, bounded evidence citations, source watermarks, provenance, provider/schema identity, uncertainty, and a deterministic replay identity.
- Define offline and shadow execution modes that are non-authoritative: they may report candidates and would-activate dispositions, but cannot mutate canonical memory, active insight state, default retrieval, or ordinary context assembly.
- Add fail-closed behavior for missing, stale, incompatible, redacted, foreign, hidden, or over-budget evidence and for provider output that requests direct activation or canonical mutation.
- Hand eligible candidates to the existing reserved-insight activation policy as an explicit admission step, preserving append-only insight versions, audit history, rollback, and idempotency.
- Extend replay reports and bounded observability with reasoning operation, mode, type, eligibility, fallback, freshness, and outcome categories without exposing prompts, chain-of-thought, raw provider payloads, identifiers, or scope values.

## Non-goals

- Do not automatically activate `hypothesis`, `goal`, `contradiction`, or `causal_link`; each remains disabled without an independently enabled, versioned, exact-scope policy.
- Do not rewrite, delete, or overwrite canonical memory, raw events, memory versions, vectors, source evidence, or prior derived-insight versions.
- Do not change default retrieval ranking or inject reasoning insights into ordinary context assembly; any future exposure requires an explicit authorized section and policy.
- Do not treat model output as fact, as the sole release gate, or as a substitute for deterministic evidence and governance checks.
- Do not add Redis, Kafka, a graph database, another system of record, a UI, SDK, MCP surface, or end-user product logic.
- Do not expose data outside exact scope, lifecycle visibility, redaction policy, execution budget, or provider contract limits.
- Do not couple reasoning-provider configuration to the embedding provider.

## Capabilities

### New Capabilities

- `governed-autonomous-reasoning-insights`: Generate, validate, replay, and hand off evidence-backed reasoning candidates under offline/shadow and governed activation constraints.

### Modified Capabilities

- `governed-experience-insights`: Extend the derived-insight substrate to preserve reasoning candidate provenance, evidence, uncertainty, and policy decisions while retaining reserved-type safeguards.
- `governed-reserved-insight-activation`: Admit reasoning-derived candidates only through explicit type-specific policies with deterministic shadow/replay, append-only lifecycle, and rollback.
- `derived-insight-replay`: Replay reasoning candidate envelopes and provider identities deterministically, reporting non-authoritative would-activate outcomes without mutating active state.
- `reasoning-provider-boundary`: Define bounded insight derivation requests/responses, evidence watermarks, redacted output handling, and fail-closed provider fallback.
- `context-assembly`: Keep reasoning-derived insights out of default retrieval/context and require explicit authorization, lifecycle, scope, citation, and budget checks for any future section.
- `service-observability`: Add low-cardinality telemetry and bounded diagnostics for reasoning derivation, replay, shadow, fallback, eligibility, and activation handoff.

## Impact

- Affected contracts: OpenAPI/admin replay and diagnostics surfaces, provider-neutral reasoning envelopes, derived-insight admission and replay reports, and optional context section authorization.
- Affected implementation areas: reasoning provider orchestration, derived insight persistence/governance, durable replay workers, context assembly guards, and observability instrumentation.
- Storage remains PostgreSQL-only; new records must use existing versioned, scoped, provenance, lifecycle, audit, and durable-work patterns.
- Verification must cover evidence binding, scope/lifecycle/redaction isolation, deterministic replay, stale dependency fallback, canonical-memory immutability, and policy rollback. The implementation should use the existing commands and skills: `openspec status`, `openspec validate --all --strict`, `openspec apply`, and the repository's Go test and PostgreSQL + pgvector verification paths.
