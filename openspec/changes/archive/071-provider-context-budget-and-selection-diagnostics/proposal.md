## Why

PC1 and PC2 establish a typed, authorized Provider context projection, but its item `budget` is still capped by a byte limit and callers cannot allocate explicit content, citation, or complete-response space. PC3 makes the final disclosed response obey a measurable caller allocation while keeping retrieval, ranking, lifecycle eligibility, and context packing in Stele.

## What Changes

- Retain required positive `input.budget` as the maximum number of returned item placements. Add optional closed `input.budget_limits` with `max_content_bytes`, `max_response_bytes`, and `max_citations`; publish independent server item/content/response/citation ceilings and their effective intersection.
- Account for disclosed UTF-8 content, unique structured citation identities, and the complete serialized JSON body, including envelope, paths, source metadata, repeated citations, and the accounting block itself. Reference-only output consumes item/citation/response space but zero content bytes.
- Integrate deterministic whole-item admission into existing context assembly after disclosure eligibility, ranking, summary preference, deduplication, and section policy. Preserve required citations and server response defenses; add no consumer-side selection or alternate ranking pipeline.
- Return versioned, bounded budget usage and safe selection reasons. Distinguish genuinely empty results, budget omissions, unsupported optional public sections, and infeasible response envelopes without exposing hidden-source counts or internal ranking details.
- Advertise `context-budget-v1` explicitly. Token budgets/tokenizers remain unsupported and unknown token fields fail validation; bytes never stand in for model tokens.
- **BREAKING:** independent item ceilings can reduce formerly oversized requests, required response accounting changes strict consumer schemas, and budget-aware omission can produce a valid bounded selection where PC2 returned an oversized-response failure. Update OpenAPI and maintenance/consumer documentation in the same implementation change.

## Capabilities

### New Capabilities

- `provider-context-budget-accounting`: explicit limits, final-response accounting, safe usage/selection metadata, and fail-closed budget errors.

### Modified Capabilities

- `agent-runtime-provider-adapter`: discoverable budget extension, strict public inputs, independent limits, and consumer compatibility.
- `context-assembly`: retrieval-owned admission using the final authorized projection cost while preserving existing selection and section gates.
- `context-disclosure-policy`: replace the PC3-pending item-budget wording with budgeted whole-item disclosure and retain final serialization refusal.
- `runtime-memory-provider-contract`: serialized budget conformance, fresh PostgreSQL/restart evidence, and maintenance documentation acceptance.

## Impact

Affected areas include `openapi/spec.go`, Provider context/capability/disclosure/operation code, retrieval assembly, HTTP output, configuration/bootstrap examples, contract fixtures, and documentation. No new persistence system, database migration, tokenizer, model dependency, or external SDK is planned. PostgreSQL remains authoritative; the accounting ledger is per-response data.

Implementation acceptance includes OpenAPI, README, `docs/agent-runtime-memory-provider.md`, architecture/component/best-practice and self-host/configuration guidance, affected MCP guidance, and `skills/stele-memory/SKILL.md`. Document defaults, units, error behavior, authorization, compatibility, and rollback together with the behavior change.

## Non-goals

- PC4 context digests, continuity, snapshots, or replay semantics; the budget contract identity is not a context digest.
- PC5 durable reference completion/status APIs or PC6 outcome/feedback projection workflows.
- Enabling PC2-closed public insights, goals, projections, calibration diagnostics, or unrestricted provenance.
- Token estimation, content truncation, broader searches to fill spare space, new ranking algorithms, local reference stores, or Danny business/tool authorization.

## References

- [Roadmap PC3](../../../docs/roadmaps/2026-05-28-stele-v1-roadmap.md#pc3-context-budget-and-selection-diagnostics)
- [Archived PC2 design](../archive/070-provider-context-disclosure-and-source-trust/design.md)
- [OpenSpec propose skill](../../../.codex/skills/openspec-propose/SKILL.md) and [apply skill](../../../.codex/skills/openspec-apply-change/SKILL.md)
- Review commands: `openspec status --change provider-context-budget-and-selection-diagnostics`, `openspec validate provider-context-budget-and-selection-diagnostics --strict`.
