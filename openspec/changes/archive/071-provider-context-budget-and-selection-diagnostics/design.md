## Context

See [proposal](proposal.md) for motivation and [PC2](../archive/070-provider-context-disclosure-and-source-trust/design.md) for disclosure policy. The current Provider DTO uses a positive item `budget`; the adapter caps it using `MaxContextBytes`. Internal `CharacterBudget` measures some chunk/projection UTF-8 text only. PC2 already marshals the final closed response and refuses bodies over `MaxContextBytes`; that route writes the marshaled bytes directly, without a terminal newline.

Provider imports retrieval, so a retrieval-owned admission path cannot import Provider serialization types. Existing assembly already owns section order, summary preference, deduplication, ranking and quotas. Its internal candidate/channel/latency planning budgets are separate from the public output allocation. PC2 disables unsupported public projection/insight/goal/diagnostic paths and establishes current-version, scope, lifecycle, source and classification checks.

## Goals / Non-Goals

**Goals:** enforce a measurable final allocation, separate units, preserve existing ordering/eligibility and public disclosure, give consumers a strict bounded accounting contract, and include operational documentation and live evidence in acceptance.

**Non-Goals:** change native/admin output defaults, create a tokenizer, introduce new ranking or memory persistence, open unsupported public sections, or implement PC4–PC6. Operation metadata and schema-document digests retain their existing meanings.

## Decisions

### 1. Preserve item allocation and add a closed limits object

Keep required `input.budget` as maximum item placements. Add optional `budget_limits` with optional integer fields `max_content_bytes`, `max_response_bytes`, and `max_citations`. Omission of the object or individual fields means the corresponding published server limit; an empty object is valid. Explicit nulls, duplicates, aliases, strings/fractions, negatives, unknown fields and trailing JSON are invalid. All request limits use portable integers through 2,147,483,647; larger numbers fail validation before arithmetic. `budget` and response bytes require at least one; content bytes and citations allow zero. Token fields, including `max_tokens` and `tokenizer`, are unknown and return the existing bounded 400 `invalid_request` outcome.

Effective limits are the minimum of caller and server limits independently. Normalize and clamp before candidate fanout or multiplication; retain existing bounded retrieval candidate caps. A zero-content budget permits whole eligible items whose content is actually empty; it cannot turn an authorized content request into reference mode. Zero citation budget permits no memory item because every public memory item requires its canonical citation.

| Unit | Request | Server capability/configuration | Default / hard bound |
| --- | --- | --- | --- |
| Item placements | `budget` | `max_context_items` / `STELE_PROVIDER_MAX_CONTEXT_ITEMS` | 100 / 1,000 |
| UTF-8 content bytes | `budget_limits.max_content_bytes` | `max_context_content_bytes` / `STELE_PROVIDER_MAX_CONTEXT_CONTENT_BYTES` | inherited response ceiling / 4 MiB |
| Complete JSON body bytes | `budget_limits.max_response_bytes` | existing `max_context_bytes` / `STELE_PROVIDER_MAX_CONTEXT_BYTES` | existing 1 MiB / 4 MiB |
| Unique citation identities | `budget_limits.max_citations` | existing `max_citations` / `STELE_PROVIDER_MAX_CITATIONS` | existing 100 / 1,000 |

New item configuration must be positive. Omitted content configuration inherits the response ceiling, including when operators override that ceiling; explicit content configuration permits zero and must not exceed the response ceiling. Invalid server settings prevent startup rather than being silently corrected. Discovery exposes resolved limits and `context-budget-v1`, accounting version `context-budget-v1`, fixed public packing-policy identity `context-packing-v1`, and `token_budget_supported=false`. This fixed identity describes admission semantics, not active experimental ranking plans or a projection digest.

Alternatives: replacing `budget` with a new object would force unnecessary request migration; treating bytes as tokens would make accounting false; adding model-specific tokenization would broaden PC3 and require a separate versioned contract.

### 2. Measure the final disclosed representation

Count item appearances across returned categories. Existing identity/lineage deduplication still runs; if a contract legitimately permits multiple appearances, each consumes an item and its displayed content bytes. Sum `len([]byte(content))` for every disclosed text-content field, without JSON escaping. Reference-only content usage is exactly zero. A citation is a unique canonical `(kind, id, version identity if present)` tuple across returned evidence; repeated appearances in item, result and outer citations consume body bytes each time but one citation identity. Required canonical citations travel with their items; an item is inadmissible if mandatory evidence cannot fit. Do not trim its text, strip its citation, or disclose a different version to fit.

Response bytes are the exact uncompressed UTF-8 JSON body sent by the context route, including operation metadata, all category arrays, paths, source metadata, repeated citation arrays and `result.budget_accounting`. HTTP headers, framing and compression are excluded. Preserve the direct-marshaled-byte write, without indentation or an appended newline. Escaped content, identifiers, paths and metadata are charged at their serialized size for this unit. Errors have separately bounded server bodies and do not promise to fit a caller's success-body limit.

`result.budget_accounting` is required and closed:

```text
contract_version: context-budget-v1
packing_policy_version: context-packing-v1
requested: { max_items: positive integer,
             max_content_bytes: integer or null when omitted,
             max_response_bytes: integer or null when omitted,
             max_citations: integer or null when omitted }
effective: { max_items, max_content_bytes, max_response_bytes, max_citations }
usage: { items, content_bytes, response_bytes, citations }
selection: { outcome: selected | empty | budget_exhausted,
             omission_reasons: ordered unique budget reason enums,
             unsupported_sections: ordered unique public capability enums }
```

Response-only nulls in `requested` distinguish omission from explicit zero; they do not relax request null rejection. Reason enum order is `item_limit`, `content_bytes`, `citations`, `response_bytes`. Unsupported-section order is `experience_insights`, `goal_context`, `diagnostics`, `feedback_diagnostics`; only explicitly requested, publicly unsupported sections appear. The caller has no public projection selector, so no projection-existence indicator is added. `selected` means at least one item returned, whether or not reasons are present. `empty` means no disclosable candidate remained in the bounded assembly stream. `budget_exhausted` means that stream had eligible candidates but no whole item fit. Reasons describe only evaluated disclosable candidates. There is no hidden, quality-rejected, grant-rejected or lifecycle-rejected count, identity, free text or raw plan/score.

### 3. Keep selection in retrieval and projection costing in Provider

Add an internal, optional final-projection admission interface defined in retrieval. Provider implements it using the existing closed disclosure DTO and a deterministic serializer/cost accumulator. The callback takes service-owned eligible selection state; it is not caller code, an external scoring function, or an alternate selection pipeline. Provider supplies current disclosure decision, operation metadata, effective limits and safe fixed diagnostics. Retrieval owns candidate eligibility, ordering, section priority/quotas, summary preference and deduplication; the admission component can accept or refuse a whole candidate but cannot change its score or fetch replacements.

Apply disclosure eligibility before an item enters the budget stream, including contributing-source visibility and current-version identity. Preserve relative order among admitted candidates and existing protected groups when they fit. Continue over the existing bounded candidate stream after an oversized item is omitted; do not fetch a wider path or additional scope to fill unused space. Public source/summary/relation categories share one allocation. Internal projection/chunk/optional-section code must share admission when applicable, but PC2 public gates remain closed. Native/admin calls without the final-projection interface retain their existing semantics; no Provider byte default leaks into them.

Use the same public DTO projection for costing and emission. This avoids undercounting source metadata or citations and avoids placing a second clipping/ranking pass in the HTTP handler. Alternatives considered: final suffix trimming would violate section/evidence policy; a second Provider packer would duplicate retrieval rules; exposing raw internal cost estimates would miss actual public-envelope overhead.

### 4. Reserve bounded diagnostics and finalize exact serialization

The accounting block is itself charged. During admission reserve a deterministic upper bound for accounting overhead: all four reason enums, all applicable fixed unsupported-section enums, the longest outcome, and maximum usage digit widths derived from resolved server bounds. Charge actual requested/effective/operation metadata bytes. The item-selection algorithm may conservatively leave this bounded reserve unused; it makes no maximal-packing guarantee. Publish this reservation behavior with the packing-policy version so callers do not mistake unused space for a selection bug.

After selection, populate the actual reason/outcome values and exact usage. Compute `response_bytes` starting at zero and remarshal until measured length equals the reported value. Because only a bounded nonnegative integer's digit width affects length, a bounded fixed-point loop converges; assert convergence and limits before writing. A conservative reserve must dominate the finalized block; disagreement or serializer error fails closed rather than deleting suffix items. Ensure count arithmetic is checked and bounded. Capture finalized bytes once and write those same bytes; no second encoder may change formatting or append whitespace.

Validate the mandatory empty envelope before retrieval. If it cannot fit the effective response ceiling, return 413 `context_limit_exceeded` with no result or partial stream. Final serialization or accounting disagreement also returns a bounded non-success with no projection. Malformed inputs use 400 `invalid_request`, current disclosure denial uses 403 `disclosure_denied`, and unavailable authoritative dependencies use a bounded retryable dependency failure (503); dependency failure is never an empty or budget-exhausted result.

### 5. Safe diagnostics and unchanged authority

Usage describes returned data. Budget reasons describe only already-authorized eligible candidates considered by the bounded assembly stream; unsupported-section labels describe fixed service capability rather than stored data. Changing grants or requested response mode resolves disclosure afresh before packing. Reference-only and content modes share item/citation/body constraints while measuring content differently. No high/restricted, hidden or foreign item may contribute a public identifier, omission reason or private counter.

The fixed packing identity and public limits are allowed public accounting metadata. Internal feedback, rollout, search, quality or query-plan identities remain privileged and absent. Do not reopen ordinary raw diagnostics through `include_diagnostics`; the new accounting block is mandatory independently of that option. Danny allocates memory space after reserving its own prompt components, consumes Stele selection, and retains responsibility for business/tool/write/compiler authority and treating source content as untrusted data.

### 6. Verification and maintained documentation

Conformance checks actual serialized HTTP responses against OpenAPI: missing accounting, impossible usage, overflow, wrong units, malformed response categories or unsupported versions fail the contract. Include multilingual/emoji/JSON escaping, large paths/provenance, repeated citations, zero limits, exact body boundaries and self-counting digit transitions. Compare ordinary eligible recall and ordering at ample limits against the PC2 baseline; tight allocations test deterministic protected-group priority without promising that unfit evidence can be returned.

Fresh owned PostgreSQL/pgvector evidence uses official bootstrap, disclosure grants, public governed ingestion and bounded completion observation, then tests both modes and lifecycle/scope safety before and after API/worker restart. Optional public sections are tested as unsupported with no leaked output; focused internal projection/chunk/insight tests are labeled separately from public live evidence. Save redacted service/image/OpenAPI/disclosure/budget-version provenance and bounded pass/fail metrics, never credentials, payloads, content, scope values or memory identities.

Every external semantic change requires matching OpenAPI, README, integration/self-host/configuration, architecture/component/best-practice, affected MCP, and agent-skill updates in this same implementation. Describe all units/defaults, envelope accounting/reserve, positive/zero validation, errors, section gates, budget negotiation, compatibility, consumer validation, and rollback. An API 2xx alone is not acceptance evidence.

## Risks / Trade-offs

- [Repeated serialization can cost CPU] → Bound server items/candidates/body size, reuse incremental content/citation costs, reserve diagnostics once, and measure worst-case assembly cost without adding a ranking latency budget to the public allocation.
- [Reserve leaves some bytes unused] → Document deterministic conservative admission; exact usage still reports the actual emitted body and no promise of maximal knapsack packing is made.
- [Strict consumers reject the accounting block] → Publish explicit capability discovery, updated closed schemas/examples and migration guidance; do not silently remove fields or retry without limits.
- [Eligibility changes between lookup and emission] → Preserve existing current-source disclosure checks, finalize through the same policy-bound cost projection, and fail closed on unprovable required metadata.
- [Byte limits do not equal prompt tokens] → Advertise token budgets as unsupported and make Danny's allocation/extra prompt space responsibility explicit.

## Migration Plan

1. Publish OpenAPI/configuration/defaults and conformance fixtures first. Implement the new limits/accounting and retrieval-owned admission together; advertise the extension only when all public enforcement exists.
2. Keep PC2 grants, source labels, reference defaults and required category/citation arrays. Deploy with independent item ceiling 100 and content ceiling inherited from the existing response ceiling; document formerly oversized item requests and strict response consumers as compatibility changes.
3. Run focused/full/race/vet/docs/OpenSpec gates and fresh initial/restart integration evidence. Update roadmap only after the appropriate verified stage.
4. Roll back by disabling Provider access for consumers requiring PC3 before deploying a binary that lacks the extension; retain PC2 enforcement and durable data. Consumers fail capability negotiation rather than stripping budgets, broadening disclosure, switching routes, or doing local packing. No database down-migration is required.
