## Context

See [proposal.md](proposal.md) for motivation. PC3 finalizes the actual disclosed HTTP body through `ContextAdmission`; PC2 restricts that body to current authorized source versions and mode-specific items. Public context has five categorized arrays, item citations and matching inner/outer aggregate citations. Optional insights, goals and privileged diagnostics currently remain unsupported on this route.

The existing PostgreSQL runtime binding supplies exact scope, principal, agent, session and provider-instance identity. Provider synchronization has separate snapshot/cursor semantics; it cannot establish the identity or historical replay of a context selection. PostgreSQL is the only durable authority. The current highest migration is `0035_context_disclosure_source_metadata`.

## Goals / Non-Goals

**Goals:** reproducible identity of the final disclosed projection, safe fresh comparison after API/worker restart, explicit bounded receipt persistence, and independently verifiable consumer/operational evidence.

**Non-goals:** materializing historical context bodies, claiming a serializable global memory snapshot, adding a synchronization watermark or another selection algorithm. No receipt authorizes reuse of a locally cached body, even when comparison returns `unchanged`.

## Decisions

### 1. Digest always, durable continuity only when requested

Three approaches were considered: digest-only reads have minimal persistence but cannot validate an expiring prior reference after restart; metadata-only receipts support that validation without retaining historical text; exact replay snapshots require a broader retention and disclosure design. Choose metadata-only receipts with fresh comparison. Omitted continuity performs no receipt write and always returns a digest.

The optional closed input union is:

```json
{"continuity":{"mode":"issue"}}
```

```json
{"continuity":{"mode":"compare","reference":"ctxc_0123456789abcdef0123456789abcdef"}}
```

These examples show the continuity fragment inside the existing `input`, not complete requests. `issue` allows only `mode`; `compare` requires exactly `mode` and `reference`. Nulls, duplicates, aliases, unknown fields, wrong types and invalid combinations fail 400. Reference format is exactly `^ctxc_[0-9a-f]{32}$` (37 ASCII bytes), using 128 bits of cryptographic randomness. It contains no identity or digest and is not a bearer credential.

Successful results always contain closed `context_digest`:

```json
{
  "contract_version":"context-digest-v1",
  "algorithm":"sha-256",
  "canonicalization_version":"context-canonical-v1",
  "selection_policy_version":"context-selection-v1",
  "selection_policy_fingerprint":"<64 lowercase hex>",
  "value":"<64 lowercase hex>"
}
```

They also contain a closed `continuity` union. All variants require `contract_version:"context-continuity-v1"`, `read_semantics` and `outcome`:

| Request | Read semantics | Outcome | Additional required fields |
| --- | --- | --- | --- |
| Omitted | `fresh` | `not_requested` | None; reference/expiry absent |
| Issue | `fresh` | `issued` | `reference`, `expires_at` |
| Compare | `fresh_compare` | `unchanged` or `changed` | Current `reference`, `expires_at` |

Expiry uses UTC RFC3339 seconds, fixed `YYYY-MM-DDTHH:mm:ssZ`. Compare returns the full fresh current result and its current digest/reference, never a 304, a historical body, or an echoed previous digest. `unchanged` means equality of the supported semantic projection identity; it is not a snapshot or business-state assertion.

Capability discovery gains closed `context_digest` and `context_continuity` blocks. Advertise the contract/canonicalization/algorithm/selection versions, `fresh_compare` support, reference byte limit and configured TTL, plus `exact_snapshot_replay:false` and `context_watermark:false`. The effective selection-policy fingerprint belongs to the authorized context result; unscoped discovery does not expose active scoped rollout identities. Keep OpenAPI schema digest and synchronization discovery independently named. No principal-specific grant, scope or receipt occupancy is discoverable.

### 2. Canonical identity follows finally admitted evidence

Define a typed canonical JSON document, encoded as UTF-8 with no whitespace or trailing newline. The fixed top-level key order is `contract_version`, `canonicalization_version`, `scope`, `response_mode`, `projection_intent`, `disclosure`, `budget`, `selection_policy`, `selection`, `categories`, `citations`. SHA-256 input is ASCII `stele-context-digest-v1\n` followed by these canonical bytes; `value` is lowercase hex.

Use explicit ordered DTOs and golden canonical-byte fixtures, not maps, arbitrary serialized HTTP JSON or floating-point rank scores. Strings preserve their exact validated Unicode scalar sequences, case and whitespace; no Unicode normalization or lowercasing. Reject invalid UTF-8 and unpaired JSON surrogate escapes rather than silently replacing them before hashing. JSON uses Go `encoding/json` default string escaping: quotes/backslashes/control characters escaped, `<`, `>` and `&` escaped as lowercase Unicode escapes, U+2028/U+2029 escaped; other valid non-ASCII text remains UTF-8. Decimal integers have no leading zeros. Missing optional canonical string values become `""`; empty collections are `[]`, never null. Contract fixtures document all nested field order and these omission rules.

Canonical contents:

- `scope`: ordered `tenant`, `project`, `namespace` from the binding. Agent/session/principal/binding/provider-instance IDs do not enter semantic identity; identical final projections in the same exact scope can share a digest. Receipts bind all these identities separately.
- Effective response mode, default-resolved projection intent, and `disclosure` ordered as `policy_version`, `profile`, `source_metadata_version`, using the existing effective disclosure decision. No principal grant record or private entitlement history.
- `budget`: effective `max_items`, `max_content_bytes`, `max_response_bytes`, `max_citations`, in that order. Requested-versus-omitted markers do not affect identity when effective limits match.
- `selection_policy`: ordered disclosure/source/budget/packing/selection version identities and selection-policy fingerprint. A fixed algorithm version alone is insufficient when active policy/configuration changes selection behavior.
- `selection`: safe outcome, ordered omission reasons and ordered unsupported section labels from finalized PC3 accounting. These identify observable semantic selection decisions without hidden-source statistics.
- `categories`: a fixed ordered array of `profile`, `recent_session`, `recent_episodes`, `relevant_summaries`, `related_entities`; each entry has `name`, then ordered `items`. Every occurrence counts, including duplicate placements. Each canonical item has ordered `kind`, `id`, `version`, `memory_path`, `class`, `availability`, `source_metadata`, then `content` only in content mode, then `citations`. Source metadata uses `version`, `trust`, `classification`. Never hash hidden content in reference mode. Canonical items flatten the transport reference/memory wrapper; mode disambiguates their schema.
- Aggregate authorized `citations`, preserving deterministic first-occurrence order and existing citation tuple identity. Each canonical citation has `source_kind`, `reference`, `version`, `watermark`, `availability` in that order, with absent version/watermark represented as empty strings. Canonical representation includes the aggregate once; transport inner/outer arrays must agree, and each serialized appearance still costs response bytes. Selection uses ordered `outcome`, `omission_reasons`, `unsupported_sections`; policy uses `disclosure_version`, `source_metadata_version`, `budget_version`, `packing_version`, `selection_policy_version`, `selection_policy_fingerprint` in that order.

The selection-policy fingerprint is SHA-256 of a versioned ordered manifest of relevant active **nonsecret** policy/configuration identities used by this context pipeline: selection/eligibility/quality/fusion policy versions, active controlled-ranking rollout revision when applicable, and public source/disclosure/budget/packing versions. Policy implementations must declare their stable nonsecret identity, including effective behavior-affecting configuration; use precise string/integer configuration encodings, not volatile metrics or raw floating-point ranking results. Secrets, DSNs, provider URLs, private candidate state and diagnostic histories are excluded. Freeze the reviewed manifest and golden fixtures during implementation; changed applicable policy identity changes the fingerprint. It is an opaque policy identity, never an attestation or privilege grant.

Request/query/path affect selection; they are not additional digest inputs when the final semantic projection and policy identity are identical. They are bound by the receipt request fingerprint below. Exclude request/operation/idempotency IDs, event sequence, timestamps, latency, build/schema digests, receipt/expiry/continuity outcome, budget usage and serialized body bytes. Thus receipt issuance or accounting self-size cannot cause a hash cycle. Source version/order/path/content/citation/classification/trust, scope, mode, effective allocation or relevant policy changes alter identity. Source changes outside the final authorized projection do not.

### 3. Authorization precedes receipt access; comparison is a fresh read

Request fingerprint hashes a separately domain-separated typed document containing validated query bytes, normalized selector kind/value (omission resolves to root prefix), full runtime identity, effective mode/profile/intent, effective four limits and all normalized option booleans. Exclude continuity mode/reference, operation metadata, requested-limit omission markers and the active selection-policy fingerprint. The latter must permit policy changes to be detected through a fresh `changed` response. A changed effective budget or default profile instead changes the originating read contract and requires a new issue request.

Sequence: existing authentication/scope/schema/strict-input gates → current disclosure authorization → effective limits and mandatory-envelope preflight → scoped receipt lookup and request/contract validation for compare → existing bounded assembly/admission → digest and fresh equality comparison → finalized actual body → receipt persistence → HTTP success. No scoped lookup occurs before current authority is established. Current source/lifecycle/version safety remains in the existing assembly boundary; do not broaden it or add a second retrieval pass. Dependency failures retain non-success outcomes.

| Condition | Bounded outcome |
| --- | --- |
| Invalid continuity/input | 400 `invalid_request` |
| Current disclosure grant denied/downgraded for requested mode | 403 `disclosure_denied`, before receipt lookup |
| Mandatory success envelope cannot fit | 413 `context_limit_exceeded`, no receipt creation |
| Unknown, purged or foreign-identity reference | 409 `continuity_reference_unavailable` |
| Visible owned reference at/after expiry | 409 `continuity_reference_expired` |
| Originating normalized read changed | 409 `continuity_request_changed` |
| Stored digest/canonical contract unsupported | 409 `continuity_incompatible` |
| Same compatible read; current projection differs | 200 `changed`, current authorized result only |
| Same compatible read; digest equal | 200 `unchanged`, current authorized result only |
| Authority/source/receipt persistence unavailable or receipt capacity exhausted | Bounded retryable 503 dependency outcome |

Use existing Provider error shape/categories (`conflict`, `compatibility`, `dependency` as appropriate), no new free-text reason payloads. Foreign reference lookup uses all exact identity predicates and is indistinguishable from random unknown input. Owned request/contract conflicts are checked after scoped existence and expiry, before retrieval. No reference/digest/payload is echoed in errors or logged. A retained expired row yields expired; cleanup turns it into unavailable, explicitly representing a retention gap requiring a new fresh issue. Lifecycle changes yield generic changed with freshly eligible items, not a hidden-ID list or source-deletion explanation. Expired/revoked bindings retain their existing authentication/scope behavior ahead of these context-specific rules.

### 4. PostgreSQL stores immutable receipts with hard resource bounds

Add the next sequential migration (currently `0036`) for a metadata-only receipt table. Persist opaque random receipt identity, full binding/principal/scope/agent/session/provider-instance attribution, request fingerprint, supported digest/canonical contract versions, digest value and issue/expiry times. Use existing text types for `id_...` principal/binding identifiers; never cast them to PostgreSQL UUID. No query, selected IDs, content, citations, serialized body or raw payload is stored. Index exact-identity lookup, active deduplication key and expiry cleanup.

Issuance and comparison reuse an existing unexpired compatible receipt with identical binding/request fingerprint/digest without extending its expiry. Otherwise create an immutable new receipt; do not overwrite old records. Concurrent identical issuance must converge on one live receipt using a PostgreSQL transaction and durable locking/uniqueness. Expired rows can be deleted in bounded cleanup, not updated into a new receipt. Use PostgreSQL-authoritative time rounded to UTC seconds; expiry is the earlier of configured TTL and binding expiry. A reference with no remaining valid second cannot be issued. Bindings that remain valid across API restart can compare; a new binding/session must issue its own receipt.

| Configuration | Default | Valid range |
| --- | --- | --- |
| `STELE_PROVIDER_CONTEXT_CONTINUITY_TTL_SECONDS` | 86400 | 60–604800 |
| `STELE_PROVIDER_CONTEXT_CONTINUITY_MAX_RECEIPTS_PER_BINDING` | 128 | 1–4096 |
| `STELE_PROVIDER_CONTEXT_CONTINUITY_MAX_RECEIPTS` | 100000 | 1–1000000 |
| `STELE_PROVIDER_CONTEXT_CONTINUITY_CLEANUP_BATCH` | 500 | 1–5000 |

Per-binding cap cannot exceed global cap; invalid configuration fails startup. Caps count all retained rows, including expired rows. Existing bounded scheduler work and bounded issuance cleanup delete only expired receipts. Global capacity check/dedup/insert/cleanup must be transactional across API processes, using a durable shared capacity lock rather than per-process counters. Reuse may succeed at capacity; new issuance fails bounded 503 without evicting live references or silently omitting a requested receipt. Concurrent PostgreSQL tests must prove these invariants. A failed HTTP delivery can leave a committed receipt, but retry deduplicates; a failed serialization must not create one. Ordinary digest-only reads do not depend on this write path.

### 5. New envelopes participate in existing whole-item admission

Preflight reserves the full fixed digest shape and the largest applicable continuity variant, including a 37-byte reference and 20-byte expiry, before candidate selection. Digest value, fingerprint and expiry placeholders have fixed widths. Compute the digest only after authorized whole-item admission and safe selection metadata finalize. Finalize the real body with digest/continuity fields and PC3 accounting self-size, excluding these transport fields from canonical identity.

Persist/deduplicate a receipt only after a body can serialize within all effective limits. If deduplication selects an existing reference/expiry, substitute those fixed-width values and re-finalize byte accounting with the unchanged selected projection. Do not rerank/requery or trim the response. Verify the transaction returned matching request/digest identity before emission. Required persistence must commit before a successful body is written. Serializing the new mandatory empty envelope may produce 413 for formerly feasible tiny allocations; document this migration effect.

### 6. Contract validation and fresh evidence are separate gates

Extend actual-handler/OpenAPI fixtures and `internal/assurance` strict consumer validation to require both new fields and validate versions, formats, union/request correspondence, canonical digest recomputation using the authenticated binding scope and original normalized request, citation equivalence and complete-body accounting. Canonical byte fixtures must cover empty/nonempty both modes, Unicode/escapes, duplicate placements, source-version/order/path/provenance/policy changes, effective-limit equivalence and volatile metadata exclusion. Consumer validation verifies the supplied digest, never selects/ranks/packs context or treats the hash as provenance authentication.

Use the existing `scripts/stele-provider-context-verify.ps1` path with a new PC4 evidence identity. Fresh owned PostgreSQL/pgvector, official bootstrap, public labeled ingestion and database-proven governance completion precede issue/compare tests. Prove digest stability and receipt survival after API/worker restart; then public governed source version/lifecycle changes, grant changes, expiry and unavailable references must produce documented safe outcomes. SQL checks inspect completion and receipt metadata/bounds, never replace public ingestion or retrieval. Test invalid input and both modes, exact/prefix selection, empty results, budget overhead, dependency/capacity failures and current-grant precedence. Focused repository/concurrency/upgrade tests supplement live evidence and must be labeled honestly. Missing/skipped/stale/degraded required evidence blocks acceptance. Persist only bounded redacted outcomes/version provenance, no scope, memory IDs, raw references/digests, credentials or content.

## Risks / Trade-offs

- [Mandatory fields break closed-schema consumers and increase minimum body size] → explicit extension discovery, strict migration examples and envelope-boundary regression tests.
- [Canonicalization drift or policy changes silently preserve identity] → frozen typed field order, canonical-byte vectors, full policy manifest review and independent consumer recomputation fixtures.
- [Receipt metadata can correlate requests] → no query/body retention, full-identity lookup, no raw token/digest logging, bounded TTL and purge.
- [Global capacity locking adds contention] → opt-in writes, deduplicate identical reads, indexed bounded cleanup and transaction/concurrency evidence; no distributed cache authority.
- [Fresh context can change during multi-query assembly] → preserve existing source/version/current-authority gates, fail closed when unprovable and make no exact-snapshot claim. This proposal does not add global serialization of memory reads.
- [Opaque hashes can be misread as permission or business receipts] → document data-only identity, reference-not-credential semantics and Danny's retained policy/state ownership.

## Migration Plan

1. Apply the additive receipt migration with existing upgrade/validate policy; test upgrade from `0035` with existing grants/labels/history intact. Review existing packages and mature library options before adding infrastructure.
2. Deploy the new contract, configuration validation and cleanup wiring; advertise extensions only when compatible repositories and digest/admission code are configured. Mixed old/new API replicas must not share an advertised endpoint until every serving replica supports PC4; retained receipts survive restart and compatible upgrades.
3. Update OpenAPI and README, Provider integration, architecture/components, configuration/self-hosting/bootstrap, best practices, affected MCP documentation and `skills/stele-memory/SKILL.md` in the same implementation. Include fresh versus compare versus unsupported replay, all errors/defaults/caps, digest distinctions, consumer migration and safe logging.
4. Require focused/full Go tests, race checks for changed concurrent paths, vet, docs gates, strict/all OpenSpec validation and new fresh public live evidence before marking tasks complete.
5. Rollback disables consumers requiring PC4 or reports capability incompatibility before routing to old binaries; retain PC2 authorization and PC3 budgets, and preserve receipt data until retention expires. Never strip continuity/digest requirements or treat schema/sync digests as substitutes. A destructive down migration requires an explicit maintenance decision and must not imply references remain usable afterward.
