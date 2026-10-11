## Why

PC1 stabilizes the public Provider context JSON but does not distinguish permission to disclose memory content from permission to disclose a reference, or preserve source trust through governance. PC2 closes that boundary so a runtime can request a smaller authorized projection without turning its business role into a Stele permission or treating consolidated text as trusted instructions.

## What Changes

- Add bounded `projection_intent`, versioned `disclosure_profile`, and `response_mode` options to the closed Provider context input. Resolve effective disclosure from current exact-scope grants, the validated binding, server policy, and request constraints.
- Introduce two small server-owned profiles, `memory-reference-v1` and `memory-content-v1`, backed by administrator-managed PostgreSQL grant configuration. Recheck authorization on every request; a runtime cannot grant itself a profile.
- Define `reference_only` as a typed structured projection with no content, title, summary, lesson, arbitrary metadata, or free-text diagnostics. Preserve categorized sections and disclose only independently authorized identity, version, path, classification/trust, availability, and evidence references.
- Preserve PC1 categorized content in separately authorized `bounded_content` mode, with explicit source metadata and existing section gates. Reuse the existing assembler and selection authority.
- Persist typed, conservative source trust/classification snapshots through event ingestion, candidates, canonical versions, summary/insight derivation, chunks, and cached context projections. Missing metadata means `untrusted` and `restricted`; mixed derivation cannot increase trust or reduce classification.
- Separate server-attested data origin from instruction authority. Ordinary writes cannot self-assert attestation, and neither an attested source nor a governed lesson authorizes tools, writes, compiler submission, or task completion.
- Prevent restricted principals from bypassing disclosure through retrieve, sync, native reads, MCP, history, or write-result echoes. Routes without equivalent enforcement deny those principals before reading or serializing content; PC2 does not redesign each alternate transport.
- **BREAKING**: public exact-scope grants need explicit disclosure provisioning; context defaults to `reference_only`, and reference items have a distinct typed shape. Publish `context-disclosure-v1` capability negotiation, migration guidance, and explicit refusal behavior; do not silently fall back to content.
- Update affected OpenAPI, README, integration, governance/lifecycle, administration, self-hosting, and agent-skill documentation in the same implementation change. Add offline and fresh public PostgreSQL/pgvector evidence, including governance completion and restart.

## Non-goals

- No Danny code, SDK, UI, business role engine, tool permissions, write policy, prompt construction, or second memory store.
- No new ranking, reranker, embedding, canonicalization, or alternative retrieval mechanism.
- No PC3 budget-unit/selection-accounting redesign, PC4 context digest/continuation, PC5 reference-write API/governance receipt, or PC6 outcome/feedback projection.
- No arbitrary user-defined profiles, policy scripting language, trusted-instruction mode, automatic attestation, or bulk historical trust upgrade.
- No content-support rewrite of every native/MCP/sync endpoint. Unsupported disclosure enforcement is an explicit denial for constrained public principals, not an alternate disclosure path.

## Capabilities

### New Capabilities

- `context-disclosure-policy`: versioned grant-intersected disclosure, structured reference projection, safe content projection, and prevention of alternate-route disclosure.
- `memory-source-trust-lineage`: typed source metadata, conservative durable lineage propagation, append-only snapshots, and safe derived-artifact visibility.

### Modified Capabilities

- `agent-runtime-provider-adapter`: canonical disclosure fields, mode-specific typed context results, version discovery, and compatibility defaults.
- `scoped-principal-access`: administrator-managed exact-scope disclosure entitlements with immediate revocation and auditable provisioning.
- `runtime-memory-provider-contract`: projection, trust-lineage, alternate-route, migration, and fresh-restart contract evidence.

## Impact

- Service: `internal/provider`, `internal/auth`, focused app authorization/transport code, and existing retrieval eligibility/projection interfaces. Keep ordinary memory selection in `internal/retrieval`.
- Storage: PostgreSQL migration(s) for disclosure grant configuration and typed source metadata/version snapshots, plus affected repositories and migration compatibility checks. No in-process authority store.
- Governance: event ingestion, extraction, consolidation, mutation/intents, summary compaction, derived insights, chunks, and projection materialization must preserve or conservatively derive metadata.
- Public contract: Provider capabilities/context/events, affected native ingestion and administrative grant schemas, and bounded refusal outcomes on unsupported content routes.
- Verification: existing Provider assurance and public context verifier, with redacted evidence independent of retrieval ranking metrics.
- Maintenance docs: `docs/agent-runtime-memory-provider.md`, architecture/component/best-practice and administration/self-hosting guides, `docs/integrations/mcp-agent-skill.md`, `skills/stele-memory/SKILL.md`, README, and roadmap. Explain the new defaults, provisioning, rollback, source trust, and Danny/Stele ownership.
- No new dependency is assumed. Before adding core infrastructure, implementation must check mature Go libraries as required by `AGENTS.md`; this design reuses existing auth, PostgreSQL, strict JSON, and governance components.

## References

- [Roadmap PC2](../../../../docs/roadmaps/2026-05-28-stele-v1-roadmap.md#pc2-context-disclosure-and-source-trust)
- [Archived PC1](../069-provider-context-openapi-contract-alignment/proposal.md)
- [Provider specification](../../../specs/agent-runtime-provider-adapter/spec.md)
- [Principal specification](../../../specs/scoped-principal-access/spec.md)
- [Governance specification](../../../specs/memory-governance-pipeline/spec.md)
- [Canonical lifecycle](../../../specs/canonical-memory-lifecycle/spec.md)
- [Provider guide](../../../../docs/agent-runtime-memory-provider.md)
- [Proposal skill](../../../../.codex/skills/openspec-propose/SKILL.md) and [implementation skill](../../../../.codex/skills/openspec-apply-change/SKILL.md)
- Planning commands: `openspec status --change provider-context-disclosure-and-source-trust` and `openspec validate provider-context-disclosure-and-source-trust --strict`.
