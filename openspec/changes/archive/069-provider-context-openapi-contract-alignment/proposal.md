## Why

The PC1 roadmap review found that `/v1/provider/context` strict-decodes an internal Go service input while OpenAPI documents snake-case options, and declares its response as a generic object. A consumer can consequently receive a validation failure for a documented option or mistake an incompatible response for an empty successful projection. Stabilizing this boundary is the prerequisite for the disclosure, budget, and continuity proposals PC2–PC4.

## What Changes

- Define explicit public Provider context request and response DTOs, independent of internal retrieval field names, and map requests to the existing assembler.
- Publish a dedicated request schema with supported snake-case fields, validation, default behavior, exact-path/prefix rules, and server-owned scope/session attribution.
- Publish a typed response envelope with categorized context items, citations, optional governed sections, and bounded authorized diagnostics; required empty sections are arrays.
- Enforce the existing Provider redaction contract when shaping context items: exclude raw ranking scores, plans, queries, hidden evidence, and internal service fields.
- Specify `provider-v1` repair compatibility through the existing supported-schema configuration, capability document, build identity, and OpenAPI digest. Unsupported schema versions and unsupported policy fields fail explicitly.
- **BREAKING**: reject undocumented Go-name/case aliases and caller-owned scope/session overrides; remove accidentally serialized raw ranking scores from Provider context results. Document migration to canonical snake-case fields and the typed result rather than introducing an unsafe legacy mode.
- Add public-route, schema, consumer-shape, and fresh PostgreSQL/pgvector conformance cases. Missing or malformed expected result sections cannot count as a successful empty result.
- Update Provider documentation and integration guidance, and reconcile roadmap PC1 status against verified implementation evidence.

## Non-goals

- No Danny implementation, SDK, UI, agent execution, business permission engine, or second memory store.
- No new `role`, disclosure profile, reference-only mode, source-trust semantics, context digest, continuation token, or exact replay protocol; those belong to PC2 and PC4.
- No budget-unit redesign, complete serialized-response budget guarantee, new selection diagnostics, or ranking/packing changes; those belong to PC3.
- No durable reference write/read redesign, new governance status API, outcome projection, database migration, or provenance repair; those belong to PC5–PC6 or their existing baselines.
- No broad rewrite of native, retrieve, intent, turn, MCP, or sync contracts.

## Capabilities

### New Capabilities

None. This change tightens existing Provider and conformance capabilities.

### Modified Capabilities

- `agent-runtime-provider-adapter`: explicit Provider context JSON, server-owned attribution, typed/redacted result, compatibility and validation outcomes.
- `runtime-memory-provider-contract`: context request/result conformance, including malformed response versus genuine empty context and public-boundary evidence.

## Impact

- Service: `internal/app/provider_http.go`, focused transport DTO/mapping code, `internal/provider`, and route/conformance tests. Reuse `internal/retrieval` as the selection and assembly authority.
- Public contract: `openapi/spec.go`, `/v1/provider/context`, and the existing capability OpenAPI digest. Native context and other Provider routes keep their existing behavior.
- Verification: `openapi`, `internal/assurance`, existing Provider contract fixtures, and the self-hosted public product verifier where its context case applies.
- Documentation: `docs/agent-runtime-memory-provider.md`, relevant integration/best-practice guidance, README links if needed, and the maintained roadmap. MCP tool signatures and the repository skill's MCP workflow need a consistency review, not a new Provider transport workflow.
- Consumer boundary: document how an external adapter validates and reads categorized items and citations. Danny changes remain in its own repository; no source dependency or adapter workaround is added here.
- No new Go dependency or storage schema is expected.

## References

- [Roadmap PC1](../../../../docs/roadmaps/2026-05-28-stele-v1-roadmap.md#pc1-provider-context-openapi-contract-alignment)
- [Provider baseline](../../../specs/agent-runtime-provider-adapter/spec.md)
- [Context assembly baseline](../../../specs/context-assembly/spec.md)
- [Provider conformance baseline](../../../specs/runtime-memory-provider-contract/spec.md)
- [Provider integration guide](../../../../docs/agent-runtime-memory-provider.md)
- [Proposal skill](../../../../.codex/skills/openspec-propose/SKILL.md) and [implementation skill](../../../../.codex/skills/openspec-apply-change/SKILL.md)
- Before archive, workflow commands were `openspec status --change provider-context-openapi-contract-alignment` and `openspec validate provider-context-openapi-contract-alignment --strict`. After archive, validate synchronized main specifications with `openspec validate --all --strict`.
