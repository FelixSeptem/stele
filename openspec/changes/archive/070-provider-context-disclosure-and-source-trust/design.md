## Context

See [proposal.md](proposal.md) for motivation. The reviewed baseline is PC1 commit `6bebe3e`.

- `internal/provider/context.go` has closed input DTOs and explicit categorized response types. It intentionally rejects disclosure fields and emits memory content and optional insight/goal text without a source-trust contract.
- `internal/provider/runtime.go` validates principal, exact scope, binding expiry/revocation, and session on each operation. `auth.ScopeGrant` persists read/write access but no disclosure entitlements.
- Raw events have unrestricted application metadata. Candidates have `Sensitivity` (`low`, `medium`, `high`, `restricted`), but canonical/version types lack a complete typed source metadata snapshot. Candidate sensitivity is not an end-to-end disclosure policy.
- Extraction, canonical promotion, summary compaction, chunks, insights, and materialized projections are existing derivation paths. PC2 must connect them to one conservative lineage rule, not build a new memory pipeline.
- Native reads, Provider retrieve/sync, history, MCP, and some write responses expose content independently of the context response shaper. A context-only serializer cannot establish a principal's disclosure ceiling.

## Goals / Non-Goals

**Goals:** explicit data-disclosure authority; content-free references; durable source labels; current authorization at every disclosure boundary; clear migration and executable public evidence.

**Non-Goals:** proposal exclusions apply. PC2 cannot give Danny business execution authority, make text into instructions, define a new ranking strategy, guarantee the caller's full model context window, or claim digest/continuity/durable-reference completion.

## Decisions

### 1. Use a small versioned profile registry and durable exact-scope entitlements

Prefer fixed server-owned profile definitions with administrator-managed grants over a generic role/policy engine. Request tags are bounded projection hints, never permission claims. The effective envelope is the intersection of current principal grant, validated binding scope, server policy, requested profile/mode, path selector, lifecycle, and existing optional-section gates.

Initial profiles under `context-disclosure-v1`:

| Profile | Permitted modes | Disclosable classification | Meaning |
| --- | --- | --- | --- |
| `memory-reference-v1` | `reference_only` | `low`, `medium` | Structured authorized references only. |
| `memory-content-v1` | `reference_only`, `bounded_content` | `low`, `medium` | Separately authorized memory data, with conservative source labels. |

`high`/`restricted` evidence is omitted from both ordinary profiles, including its identity/path. Privileged administration retains its independent audit paths. No profile bypasses suppression, forgetting, deletion, source visibility, or a denied optional section. Trust is disclosure metadata, not a filter that makes untrusted data unusable by definition.

Persist the allowed profile IDs and one default profile on each exact public scope grant. Require a nonempty bounded allowlist (at most these two initial IDs), with a default belonging to it. Update through the existing admin principal/grant surface with actor, reason, and bounded audit records. Grant profile edits/revocation take effect on the next request, including requests using old runtime bindings; bindings cannot cache a broader authorization ceiling. Missing configuration denies public memory disclosure. Bootstrap/admin authority cannot be inferred from a caller-supplied profile or projection intent.

For reference-only scope grants, the credential's content ceiling is reference-only even if a route omits a profile. A request on a content-capable credential can independently choose reference-only for that operation; that does not revoke its other authorized capabilities. Documentation must distinguish a narrow request from a narrow credential.

Alternatives considered:

1. Let consumers filter full responses. This discloses content before the policy decision and creates another context mechanism outside Stele.
2. Use caller `role` or arbitrary metadata as authority. This lets callers claim privileged roles and couples Stele to Danny's business engine.
3. Selected: explicit profile entitlements with a small versioned registry, reusing current principal and scope checks. It adds provisioning/migration work but gives an enforceable data boundary.

### 2. Extend the PC1 closed request without ambiguous aliases

Keep `metadata.schema_version=provider-v1`; advertise the separately versioned `context-disclosure-v1` extension and authoritative OpenAPI digest. Consumers must check the extension, not infer support from the base schema string. Unknown Provider schema remains a compatibility error before dispatch.

| Input | Contract |
| --- | --- |
| `projection_intent` | Optional enum `conversation`, `task_context`, `compilation`, `handoff`; default `conversation`. A routing/attribution tag with identical authorization requirements. |
| `disclosure_profile` | Optional exact registry ID, at most 64 ASCII token bytes. Omitted means the current grant's configured default; unknown or unauthorized IDs yield the same generic 403 refusal. |
| `response_mode` | Optional enum `reference_only`, `bounded_content`; default `reference_only`. An unsupported value is 400; a known mode denied by the profile is 403. |

Retain PC1 query/item-budget/path and optional selector semantics, strict canonical spelling, duplicate/null/type/trailing-JSON rejection, and binding-owned scope/session. `role`, bare `reference_only`, scope overrides, and caller policy versions remain unsupported. A string default is not authorization. Metadata path and other attribution must continue to obey existing normalization rules and must not smuggle unbounded content into a content-free projection.

Failures use bounded current error categories: 400 `validation/invalid_request`; 403 `scope/disclosure_denied`; unsupported base version uses `compatibility`; unavailable authoritative grant/metadata dependencies use 503 `dependency` rather than legacy content fallback. The response must not echo profile inputs, queries, hidden counts, denied identifiers, or submitted content.

### 3. Preserve categorized sections with mode-specific item DTOs

Keep PC1 required category arrays and both citation arrays, including `[]` for a genuine empty projection. Add required `result.response_mode` and `result.disclosure` containing the effective `profile`, `policy_version=context-disclosure-v1`, `projection_intent`, and `source_metadata_version=source-metadata-v1`. These identify disclosure decisions; they are not PC4 projection digests.

Use a discriminated OpenAPI result union. Each categorized item is either the existing content item, now with typed source metadata, or a closed reference item. A reference item contains `reference` and `citations`; it cannot contain `memory`, `insight`, or other text-bearing fields.

Reference fields are limited to:

| Field | Disclosure rule |
| --- | --- |
| `kind` | Fixed supported entity kind (`memory` or `derived_insight`). |
| `id` | Authorized stable object identifier, not an arbitrary external URI. |
| `version_id`, `version` | Included only when the selected authorized version is known; never synthesize current-version identity from an unrelated history head. |
| `memory_path` | Normalized memory path for a memory reference. Treated as protected metadata and subject to the same classification/identity eligibility. |
| `availability` | Fixed `available` for currently eligible returned evidence. No tombstone or hidden-reference existence oracle. |
| `source_metadata` | Closed effective trust/classification/version object. |
| `citations` | Bounded authorized source/evidence identifiers with fixed kind/relation/operation tags; no source context, payload, actor strings, arbitrary URLs, or unrestricted metadata. |

In reference mode omit content, titles, summaries, lessons/guidance/avoid/prefer, goal text, temporal free text, and diagnostics. Insight references still require opt-in and all existing insight gates. Omit goal context until it has an authorized stable reference contract; a goal title is not a substitute identifier. Do not echo input query or intent payload. Use only bounded operation metadata IDs/fixed precedence tags in the response envelope.

In content mode retain PC1 categorized text, bounded governed insights/goals, and independently authorized diagnostics; annotate every disclosed text-bearing object with conservative metadata. Where optional goal/insight lineage cannot be established, omit the section rather than guess trust or classification. Outer/inner citations must derive from the final returned authorized items, never pre-budget or rejected candidates. Every source identifier needs its own exact-scope and visibility check. Unavailable/foreign sources are omitted; if safe eligibility of a derived item's contributing sources cannot be established, omit the derived item itself.

Serialize the completed response into a bounded buffer and enforce the existing configured Provider response ceiling. If it cannot fit, return a bounded non-success response with no partial content; do not locally rank/repack to make it fit. Keep caller `budget` as PC1 item count. Explicit caller byte/token allocation, selection accounting, and packing redesign remain PC3 work.

### 4. Define typed source metadata with conservative defaults

Use `source-metadata-v1`:

```json
{
  "version": "source-metadata-v1",
  "trust": "untrusted",
  "classification": "low"
}
```

Trust values are `untrusted` and `attested_data`. `attested_data` means the service verified an explicitly authorized source attribution, not factual correctness, instruction authority, or permission to act. It is not inferred from an admin-looking actor string, event type, memory class, confidence, success status, path, canonical promotion, or LLM output.

Expose a closed optional `source_metadata` object on ordinary event ingestion and Provider events, and map equivalent write paths to the same internal domain contract. Omitted metadata resolves to `untrusted/restricted`; malformed or unknown labels fail validation. An ordinary writer can declare untrusted source data and classification for new content, subject to the server's scope/source classification floor. It cannot self-attest or lower the classification of existing inputs by replay, update, merge, or arbitrary metadata. Grant `source_attestation` is a separate explicit administrator-managed boolean, false by default, checked against the authenticated principal and exact scope for any `attested_data` write. Attestation permission conveys no data-disclosure privilege.

Legacy free-form metadata keys that resemble trust/classification remain application data and confer no authority. Include the normalized typed metadata in the durable idempotency fingerprint: label changes under the same key are conflicts; equivalent replay retains the original event and metadata. Retry must still pass current authorization.

Reuse the existing four sensitivity labels as the classification order:

`low < medium < high < restricted`.

Derivation is conservative and deterministic: classification is the maximum of all contributing source labels, existing candidate sensitivity, and server floors; trust is `attested_data` only when every contributing source is independently attested and lineage is complete, otherwise `untrusted`. Missing/unknown source metadata contributes `untrusted/restricted`. Never let an extractor, summarizer, reranker, or selected mode upgrade labels. PC2 introduces no retroactive attestation or classification-lowering workflow.

### 5. Preserve append-only lineage across every existing derivation

Add typed metadata snapshots to raw events, candidates, canonical records and each memory version, plus versioned derivation records for summaries, insights, chunks, and context projections. Use existing PostgreSQL migrations/repositories and provenance, not a parallel fact store. Preserve source-version identity so a current canonical label cannot be retroactively applied to an older version.

Ensure extraction, ordinary/superseding promotion, generated summary promotion, governed intents/manual mutation, merge/reclassification, compaction/reflection, and reserved/experimental insight paths apply the same derivation function. Canonical label changes are material version changes with audit/provenance; never overwrite old version snapshots. Derived artifact rebuild writes a new projection/artifact version without rewriting canonical history.

At retrieval, use the source labels and current eligibility of the selected version. Materialized summaries/chunks/projections cannot sanitize hidden, stale, restricted, or unknown lineage. Filter disallowed evidence inside existing retrieval eligibility before ranking/section packing where it could influence the ordinary projection, and recheck all references at disclosure. If stored metadata is absent or a cache/source version no longer matches, fall back to an authorized canonical path only when labels/lineage can be established; otherwise omit. An unavailable authorization/metadata repository fails the operation, rather than masquerading as an empty result.

### 6. Enforce the ceiling at alternate disclosure boundaries

Centralize a bounded disclosure decision at public principal/exact-scope authorization. Inventory all text/identifier-producing surfaces: Provider retrieve/status/sync, native search/context/raw event/history/provenance/session/turn/intent reads, MCP, and successful or replayed write/lifecycle responses.

PC2 fully supports profiled context and safe event/intent write acknowledgments. A constrained principal can receive its own stable event/operation ID and fixed acceptance/replay state; it cannot receive stored event content or unrestricted metadata through an acknowledgment or idempotent replay. Existing write access and lifecycle permission gates continue to apply.

For an alternative route, either prove equivalent mode/classification/source checks before disclosure, or deny constrained public principals with a generic bounded 403 before scoped lookup. Default to denial for legacy content routes and sync payloads; do not rewrite all transports in PC2. Denial must cover native/MCP access without runtime headers, so a caller cannot escape the Provider binding gate. Administrative audit uses separate durable admin principals and existing authorization, not the runtime's `projection_intent`.

Keep fixed control-plane capability and initialization/status metadata usable when it contains no protected object or source content. No global/public capability response lists an individual principal's entitlements or denied evidence. Existing broad roles/access modes cannot override an explicitly narrower disclosure ceiling.

### 7. Verify the real boundary and maintain consumer guidance

Offline conformance must parse serialized public HTTP/MCP output rather than inspect only an internal struct. Reference-mode fixtures deliberately place markers in content, titles, summaries, lessons, goals, provenance metadata, IDs of foreign/hidden records, and diagnostics. Assert a closed response schema and field/value absence; substring checks alone are insufficient.

Fresh PostgreSQL/pgvector verification uses official bootstrap/admin provisioning and public writes with explicit low/medium metadata. Create reference-only, content-capable, attestation-enabled, unauthorized, and foreign-scope principals. Exercise exact/prefix context, mixed-source derivation, idempotent label conflicts/replay, lifecycle transitions, grant downgrade/revocation with an old binding, and alternate-route refusals. Establish raw-event governance completion and stored candidate/canonical/version metadata; repeat supported reads and denied reads after API/worker restart. Use bounded operator verification for database evidence, not SQL to create the fixture.

Public write paths may not naturally produce every experimental artifact. Cover those derivation branches with focused repository/domain integration tests; clearly identify their evidence separately from the live public test. Do not claim a summary/insight branch passed live if its required public fixture was unavailable.

Update maintenance documentation with implementation in the same change: OpenAPI; Provider/default/migration examples; principal administration and bootstrap; governance/version/lineage behavior; self-host rollout/rollback; architecture/best practices; README; MCP and agent skill. The skill must treat recalled text as data, explain route denial for constrained credentials, and never treat attestation as permission to execute. Danny implementation remains external.

## Risks / Trade-offs

- [Default and provisioning changes break existing public clients] → Explicit capability/OpenAPI discovery, admin provisioning, consumer migration examples, and no unsafe silent compatibility mode.
- [Cross-route denial reduces existing functionality for constrained principals] → Publish the supported endpoint matrix. Prefer a small complete security boundary over claiming unsupported redacted native/sync behavior.
- [Legacy records become undisclosable] → Conservative defaults, bounded inventory tooling/docs, and explicit operator review. Do not bulk-label legacy data trusted or low-classification.
- [Derived lineage is incomplete or stale] → Version snapshots, restrictive join, invalidation/revalidation, branch-specific tests, and omission when source eligibility cannot be proven.
- [Paths or evidence identifiers reveal sensitive associations] → Treat structured metadata as disclosure, authorize each edge, omit hidden/foreign identities and counts, and test all citation collections.
- [Attestation is mistaken for instructions] → Distinct `attested_data` name, independent issuance permission, no instruction-trust enum, and clear consumer ownership docs.
- [PC2 expands into a ranking/budget platform] → Reuse current selection, require only a configured serialized-response ceiling, and reserve budget units/diagnostics/digests for PC3/PC4.

## Migration Plan

1. Apply additive PostgreSQL migrations with conservative source metadata defaults and no implicit public disclosure/attestation entitlements. Verify upgrade from the PC1 schema, append-only history, and fresh installation.
2. Inventory affected public scope grants and consumers before enabling the new runtime. Provision explicit default/allowed profiles through admin APIs. Existing bindings must be rechecked against current entitlements; bindings need no authority snapshot.
3. Deploy consistent API/worker/scheduler versions. Keep profiled disclosure unavailable while old workers or incompatible schema/source metadata dependencies can produce unlabeled artifacts; do not claim successful readiness during partial rollout.
4. Publish updated OpenAPI and `context-disclosure-v1` capabilities. Consumers explicitly request content only when authorized, handle typed reference items, validate labels, and fail on missing/incompatible shapes. Update official bootstrap/verifier examples in the same release.
5. Run fresh public governance/restart verification and conformance before declaring PC2 complete. Keep PC3–PC6 pending.
6. For rollback, disable Provider and deny affected constrained public credentials across native/MCP surfaces before reverting binaries; revoke/disable credentials if an older binary cannot enforce the ceiling. Preserve new columns and append-only history. Do not erase labels/grants or restore broad access as an availability fallback. Re-enable only after an enforcing revision and its gates pass.
