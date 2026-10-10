## Context

See [proposal.md](proposal.md) for motivation and scope. The reviewed baseline is `a556ac0`.

- `internal/app/provider_http.go` decodes `input` as `retrieval.AssembleContextInput`. Its exported Go fields have no JSON tags, so `query` works through case folding while `path_prefix` and `include_relations` do not bind to the intended fields. Undocumented Go-name spellings can work instead.
- `internal/app/http.go` already has an explicit native `contextAssembleRequest`; Provider retrieve also uses a transport request mapper. These establish the pattern to follow.
- The Provider adapter derives scope and session from the authenticated runtime binding, applies governed-operation precedence, and calls the existing assembler. Internal scope/session options must not become public overrides.
- The assembler returns categorized hits, citations, optional insight/goal sections, and diagnostics. The Provider handler currently returns that internal value directly. Nested `SearchHit.score` is serialized despite the archived Provider contract excluding raw ranking scores.
- OpenAPI refers to the native context request schema and the generic `ProviderOperationResponse`. Neither supplies a complete Provider-specific wire contract. The external adapter described in the roadmap expects `references/content`, which is not the categorized response contract.
- The current adapter also compares an item budget with `MaxContextBytes`. Full-response byte accounting and budget-unit correction remain PC3 work; PC1 conformance must not imply those gaps are fixed.

## Goals / Non-Goals

**Goals:** make Provider context JSON independent of internal types; make documented options reach the existing service; preserve exact-scope/session precedence and lifecycle gates; serialize an explicit safe result; distinguish valid empty context from an incompatible result.

**Non-Goals:** see the proposal. In particular, the DTO mapper must not rank, select, repack, create new memory facts, synthesize a digest, or interpret a business role. Internal assemblers, native routes, and MCP keep their existing semantic contracts.

## Decisions

### 1. Use dedicated transport DTOs and explicit mapping

Define a Provider-specific request envelope/input in focused app transport code, and a Provider result projection in focused Provider/transport code. Keep `AssembleContextInput` and `AssembledContext` as internal service contracts. The route remains a small sequence: authenticated binding → strict envelope decode → supported-schema check → public input validation/mapping → existing Provider adapter → safe result shaping → typed envelope.

The public input is a deliberately closed set:

| Field | Validation and mapping |
| --- | --- |
| `query` | Required nonblank string; maps to existing query. |
| `budget` | Required positive integer; retains existing item-budget semantics and configured clamp behavior. |
| `path`, `path_prefix` | Optional normalized selectors; reuse existing mutually exclusive path validation and limits. |
| `include_relations` | Optional boolean, false by default; maps to existing relation option. |
| `include_experience_insights` | Optional boolean, false by default; uses existing governed insight eligibility. |
| `include_goal_context` | Optional boolean, false by default; opt-in still requires existing independent goal policy gates. |
| `include_diagnostics` | Optional boolean, false by default; grants no diagnostic privilege. |
| `include_feedback_diagnostics` | Optional boolean, false by default; preserves authorized diagnostic rules. |
| `feedback_aware_ranking` | Optional boolean, false by default; existing per-request ranking behavior. |
| `feedback_ranking_policy` | Optional string retained for the documented rejection case; every nonempty value is rejected. It cannot change scope policy. |

`scope`, `session_id`, `user_id`, `role`, `projection_intent`, `reference_only`, `character_budget`, `use_projections`, digest, and continuation fields are not part of this input. Runtime scope/session come from the binding. Existing internal projection defaults continue to apply. Later PC2–PC4 changes introduce their own explicit schema additions.

Use exact canonical JSON property names at the envelope/input boundary. `DisallowUnknownFields` alone is insufficient because `encoding/json` case-folds names; validate property names against the public DTO allowlist without relying on internal field names. Reject missing/null envelope, metadata, or input objects, malformed scalar types, unknown keys, and trailing JSON. Required fields cannot be made valid through case aliases or zero-value coercion. Keep validation messages bounded and avoid echoing submitted fields or content.

Alternative: add JSON tags directly to internal retrieval inputs. This would still expose service-owned attribution and future internal options at the public boundary. Alternative: reuse the native DTO wholesale. This reduces boilerplate but couples future native-only options and defaults to Provider runtime binding semantics. Reuse the path/validation helpers instead of the entire transport contract.

### 2. Preserve categorized output and apply an explicit safe serializer

Add `ProviderContextResponse`, `ProviderContextResult`, and Provider-specific hit/item schemas. The envelope contains `metadata`, `result`, and outer Provider `citations`. The result always contains `profile`, `recent_session`, `recent_episodes`, `relevant_summaries`, `related_entities`, and inner memory `citations`, with empty arrays represented as `[]`, never missing or `null`.

Memory items retain the existing `memory` plus memory `citations` organization, with the public canonical memory identity, exact authorized scope, path, class, lifecycle, content, timestamps, and existing temporal metadata explicitly described. They have no `score` field. Do not reuse `MemorySearchHit` in this response because it includes raw score breakdowns. Preserve item order and identity selected by the assembler; use allowlisted serialization rather than serializing an internal service result and deleting fields afterward.

The optional `known_failures`, `experience_lessons`, `goal_context`, and authorized `diagnostics` sections keep their existing semantics. Explicitly describe every public property used there and allowlist their nested output. Omit internal planner state, raw feedback history, query text, internal candidate pools, hidden identifiers, and unbounded provider payloads. Use the existing authorization and lifecycle gates; serialization is not a replacement for selection-time authorization. Do not claim that existing ordinary content is reference-only or trusted.

Inner memory citations describe selected evidence. Derive the aggregate inner list from returned memory items in categorized order, deduplicating citation tuples; the assembler's pre-budget candidate aggregate must not expose unselected identifiers. Use dedicated nested confidence, lesson, and citation DTOs so future internal fields cannot widen the public result. Outer Provider citations use the existing `source_kind/reference/version/watermark/availability` projection and configured citation limit. Normalize empty citation lists to arrays and document that the bounded outer summary can be shorter than inner evidence citations; an outer reference cannot authorize another source read. Preserve existing citation identities and do not invent version/watermark values unavailable from the assembler.

Alternative: add `references/content` aliases to satisfy one consumer. That would establish two competing result shapes and hide the upstream mismatch. Alternative: reuse internal `AssembledContext` as the response DTO. That would keep leaking future internal fields and ranking scores. A dedicated serializer preserves the categorized public contract and the existing redaction requirement.

### 3. Treat this as a documented provider-v1 contract repair

Keep the configured Provider schema version mechanism and the existing operation name. Continue to accept supported `provider-v1` requests using the documented canonical fields. Publish dedicated OpenAPI input/result schemas and examples, and let the existing OpenAPI digest identify the repaired document. Do not misuse the digest as a context selection digest or confuse the database migration version with the Provider schema.

Unsupported metadata schema versions fail with the existing bounded compatibility outcome before assembler dispatch. Capabilities/version/build metadata identify the running revision, but there is no new protocol, configuration variable, or duplicate legacy route. Deploy the repaired API replicas consistently; a mixed deployment can still intermittently reject corrected snake-case requests.

Document the intentional compatibility break for undocumented case aliases, caller-owned attribution fields, and accidentally serialized raw scores. A consumer depending on those must migrate. A consumer expecting `references/content` must validate the categorized shape and map its authorized items/citations, or explicitly report incompatibility. PC1 does not add references, a digest, or a consumer-side ranking fallback.

Alternative: advertise `provider-v2` for every operation. This would imply a wider version change without an implemented alternate contract. Alternative: keep accepting unsafe aliases/raw scores in a legacy mode. That conflicts with closed input and archived Provider redaction rules. The chosen repair retains documented semantics and explicitly reports its practical migration impact.

### 4. Test actual public JSON and consumer interpretation

Use the repository's existing OpenAPI loader/validator and HTTP test setup. Do not add a new web framework, client SDK, or schema engine. Tests must exercise the full authenticated Provider handler with official wire names, not marshal an internal Go input that reproduces the bug.

Use an assembler test double to assert every supported option and binding-derived attribution, while actual retrieval fixtures prove existing scope, path, and lifecycle behavior. Validate the handler's serialized JSON against the served authoritative OpenAPI schema, including nonempty/empty required arrays and optional-section eligibility. Seed internal score and diagnostic sentinel fields to prove they do not leak.

Extend the existing Provider contract family with response-shape validation cases: valid nonempty result, genuine empty arrays, missing result, missing required section, null/scalar section, wrong item/citation type, incompatible `references/content`, unsupported schema, invalid options, and foreign binding/session/path cases. Report these as contract outcomes, separate from ranking quality. This is Stele conformance documentation/fixture work, not production code in Danny.

A fresh owned PostgreSQL + pgvector public fixture uses the official principal/runtime bootstrap and normal governed ingestion, waits within a bounded verifier window for the existing governance completion evidence, and checks the documented context request and result before/after API/worker restart. An incomplete fixture remains incomplete. No fixed sleep, hidden alternate write path, or synthetic database canonical insert can substitute for public ingestion. This verifies reuse of the existing durable baseline, without expanding into PC5's reference contract.

### 5. Document the consumer mapping and residual gaps

Update the Provider guide with capability/binding setup, a canonical JSON request, exact output shape, validation errors, safe empty-result handling, and migration notes. The expected request example is:

```json
{
  "metadata": {
    "request_id": "context-request-1",
    "operation_id": "context-operation-1",
    "schema_version": "provider-v1"
  },
  "input": {
    "query": "task context",
    "budget": 8,
    "path_prefix": "runtimeprobe/tasks",
    "include_relations": false
  }
}
```

The surrounding HTTP headers carry the credential, exact scope, runtime binding, and runtime session; no credential is embedded in examples or evidence. This metadata shape matches the existing required request/operation/schema fields; read-only context does not introduce durable replay or require an idempotency key. Schema/example conformance tests will enforce this shape during implementation.

Document that Danny's business permissions and final prompt construction stay in Danny. Review MCP/skill/README guidance for consistency and update only the portions affected by the public Provider contract. PC2 disclosure/trust, PC3 complete response budgets, and PC4 digest/continuity remain explicitly pending.

## Risks / Trade-offs

- [Consumers use accidental fields or scores] → Provide a compatibility matrix and canonical examples; fail explicitly rather than silently accepting a policy field.
- [A typed schema hides a remaining budget gap] → Keep the current unit/clamp limitation visible and exclude PC3 readiness claims from PC1 evidence.
- [Internal DTO changes leak into public output] → Use allowlisted mapping and negative output-schema/redaction tests, including optional sections.
- [Conformance tests validate only mocks] → Require both captured service-input tests and fresh public PostgreSQL/pgvector evidence using official bootstrap/ingestion.
- [Optional diagnostics expose privileged internals] → Reuse current authorization gates and serialize only the bounded documented public diagnostic subset.
- [One revision works while replicas serve older contracts] → Roll out a consistent revision and compare capability/OpenAPI digests before declaring conformance.

## Migration Plan

1. Implement and validate explicit DTOs, schemas, and tests in an isolated development branch. No storage migration is needed.
2. Update examples/consumer mapping and run all relevant contract/doc checks. Record intentional alias/score compatibility changes and unresolved PC2–PC4 limitations.
3. Build a pinned service image and execute the fresh public PostgreSQL/pgvector fixture, including restart checks and redacted evidence.
4. Deploy one repaired revision consistently; use the existing capability/schema/build/OpenAPI metadata to identify it. Consumers migrate to canonical request fields and validate the typed categorized result before treating it as usable context.
5. If the repair fails conformance, keep readiness incomplete and disable Provider integration with the existing flag. Avoid restoring unsafe raw-score output as a compatibility workaround; no down-migration or canonical-memory deletion is required.

## References

- [Provider delta](specs/agent-runtime-provider-adapter/spec.md)
- [Conformance delta](specs/runtime-memory-provider-contract/spec.md)
- [Provider integration guide](../../../../docs/agent-runtime-memory-provider.md)
- [Roadmap PC1](../../../../docs/roadmaps/2026-05-28-stele-v1-roadmap.md#pc1-provider-context-openapi-contract-alignment)
