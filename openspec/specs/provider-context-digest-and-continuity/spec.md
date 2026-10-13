# provider-context-digest-and-continuity Specification

## Purpose
Give external runtimes a versioned identity for the final authorized Provider context and durable, bounded fresh-read comparison without granting historical disclosure, synchronization or business authority.

## Requirements

### Requirement: Context digest identifies the final semantic projection

Every successful Provider context response SHALL contain closed `context_digest` with `contract_version=context-digest-v1`, `algorithm=sha-256`, `canonicalization_version=context-canonical-v1`, `selection_policy_version=context-selection-v1`, a 64-lowercase-hex `selection_policy_fingerprint` and 64-lowercase-hex `value`. Identity MUST cover binding-resolved tenant/project/namespace, effective response mode/profile/intent, effective four budget dimensions, relevant public disclosure/source/budget/packing/selection policy identities, final ordered category/item occurrences, provable selected memory version identities, paths/classes/availability/source metadata, exact disclosed content only in content mode, ordered authorized citations and safe selection metadata. Relevant active nonsecret policy/configuration identity MUST affect the policy fingerprint; hidden candidate data, secrets and privileged histories MUST NOT affect or appear in it. Digest MUST NOT be computed from a pre-admission candidate set or hidden content in reference mode.

#### Scenario: Reference and content projections are returned

- **WHEN** the same eligible memory is returned in reference-only and authorized bounded-content modes
- **THEN** both results contain supported digest metadata, mode differentiates identity, and only the content projection incorporates disclosed content bytes

#### Scenario: Selected source version or order changes

- **WHEN** the final selection changes a memory version, path, disclosed content, source metadata, authorized citation or item/category ordering
- **THEN** the versioned canonical identity changes and the computed digest differs, including when the same item identities remain selected

#### Scenario: Relevant policy changes without a schema change

- **WHEN** a relevant active nonsecret selection-policy configuration or revision changes while the OpenAPI schema and selected items remain unchanged
- **THEN** the selection-policy fingerprint and context digest change without exposing private configuration or evidence statistics

#### Scenario: Empty context is valid

- **WHEN** an authorized read returns empty required arrays
- **THEN** the result still contains a digest for its actual empty projection, effective allocation and safe selection metadata

### Requirement: Canonical identity is deterministic and independent of transport metadata

`context-canonical-v1` SHALL define fixed typed field ordering, category and citation ordering, string escaping, integer representation and absent/empty rules with published canonical-byte test vectors. SHA-256 MUST use the domain prefix `stele-context-digest-v1\n` followed by canonical UTF-8 JSON without whitespace or trailing newline. Validated string values MUST preserve case, whitespace and Unicode without normalization; invalid UTF-8 and unpaired JSON surrogate input MUST fail bounded validation. Duplicate item placements MUST remain distinct ordered occurrences. Equivalent inner/outer aggregate citations MUST be represented once canonically while retaining all transport byte costs. Identity MUST exclude request/operation/idempotency IDs, event sequence, agent/session/principal/binding/provider-instance attribution, timestamps/latency, requested-limit omission markers, budget usage/body bytes, receipt fields, continuity outcome and build/OpenAPI/sync digests. Same final semantic projection, scope and policy identity MUST produce the same digest across process restart and supported encoders.

#### Scenario: Operation metadata and receipt issuance change

- **WHEN** only request metadata, equivalent omitted/explicit effective limits or receipt token/expiry change for the same final semantic projection
- **THEN** the digest remains equal while actual response accounting still measures the emitted body

#### Scenario: Escaped or multibyte content is selected

- **WHEN** a fixture includes Chinese, emoji, quotes, controls, backslashes, HTML-sensitive characters or U+2028/U+2029
- **THEN** canonical bytes match the versioned test vector and consumer recomputation yields the supplied digest

#### Scenario: Same scope uses two runtime bindings

- **WHEN** two authorized bindings read identical final semantic projections in one exact scope
- **THEN** their digests can match but neither binding can reuse the other's continuity reference

### Requirement: Continuity input and output explicitly describe fresh reads

Provider context SHALL accept optional closed `input.continuity` as exactly `{mode:issue}` or `{mode:compare,reference:<token>}`. Tokens MUST match `^ctxc_[0-9a-f]{32}$`. Other mode/field combinations, nulls, duplicate keys, case aliases, wrong types and unknown fields MUST fail bounded 400 validation. Every success SHALL include closed `result.continuity` with `contract_version=context-continuity-v1`, `read_semantics` and `outcome`. Omitted requests MUST return `fresh/not_requested` without a token/expiry or receipt write; issue MUST return `fresh/issued`; compare MUST return `fresh_compare/unchanged` or `fresh_compare/changed`. Issue/compare MUST include the current reference and fixed UTC RFC3339-seconds `expires_at`. Compare MUST return the complete newly authorized/retrieved/admitted result and current digest, never a previous body, bodyless 304 or implicit snapshot replay. Neither reference nor digest SHALL grant permission to disclose a locally cached historical result.

#### Scenario: Runtime compares an unchanged projection after restart

- **WHEN** the same still-authorized durable binding submits a valid reference and identical normalized read after API/worker restart
- **THEN** the service reassembles current context, returns the full fresh result and reports unchanged only if its compatible digest equals the recorded digest

#### Scenario: Current eligible sources change

- **WHEN** a valid comparison finds a different final authorized projection under the same originating read
- **THEN** it returns changed with only current eligible evidence, current digest and current receipt, without historical content or a hidden-source reason list

#### Scenario: Caller sends a bare continuation or malformed token

- **WHEN** a caller submits a sync cursor, snapshot ID, caller digest, invalid reference shape or undocumented continuation field as context input
- **THEN** the service rejects the unsupported shape instead of interpreting it as a replay or authority credential

### Requirement: Continuity is bound to the originating read and full current authority

Receipt validation SHALL require the current authenticated principal, binding, exact tenant/project/namespace, agent, session and provider-instance identity. Current scope and disclosure authorization MUST precede receipt access. The stored request fingerprint MUST bind validated query, normalized selector/default root, full runtime identity, effective mode/profile/intent and four limits, and normalized option flags; operation metadata, continuity mode/token, requested-limit omission markers and active selection-policy revision MUST be excluded. An owned reference with a changed originating read MUST return bounded 409 `continuity_request_changed` before retrieval. Unknown, purged and foreign-identity references MUST share bounded 409 `continuity_reference_unavailable`, with no existence or ownership oracle. Retained owned expired references MUST return 409 `continuity_reference_expired`; unsupported stored digest/canonical contracts MUST return 409 `continuity_incompatible`. These conflicts MUST return no prior data. Current denial MUST retain 403 `disclosure_denied`, and authoritative dependency failures MUST retain bounded retryable 503 rather than successful empty/unchanged results.

#### Scenario: Revoked credential compares a formerly visible context

- **WHEN** current disclosure no longer permits the requested profile/mode and the caller presents its previously valid reference
- **THEN** the request receives disclosure denial before receipt lookup and exposes neither prior evidence nor reference state

#### Scenario: Another session presents a valid token

- **WHEN** a currently authorized runtime uses a token issued to another binding/session/scope/provider instance
- **THEN** it receives the same reference-unavailable outcome as a random unknown token with no foreign lookup result

#### Scenario: Effective budget or selector changes

- **WHEN** an owned compatible reference is compared with changed query, normalized selector, effective budget, profile/mode/intent or normalized option flags
- **THEN** it receives request-changed without retrieving a projection under the new read contract

#### Scenario: Equivalent allocation and metadata are reused

- **WHEN** the normalized read is unchanged despite different request IDs or explicit limits equivalent to omitted defaults
- **THEN** comparison is permitted and equality depends on the fresh final digest

#### Scenario: Reference expires or its record is purged

- **WHEN** a caller compares a retained expired own reference or a reference missing after bounded retention cleanup
- **THEN** it receives expired or unavailable respectively and must issue a new fresh reference, without a sync-replay or historical-body claim

### Requirement: Durable receipts are immutable metadata with bounded retention

Stele SHALL persist opt-in continuity receipts durably in PostgreSQL containing only opaque reference identity, full runtime attribution, request fingerprint, digest/canonical contract versions, digest value and issue/expiry timestamps. Query text, selected identities, content, citations and serialized context bodies MUST NOT be retained in receipts or raw logs. The reference MUST use at least 128 bits of cryptographic randomness and expiry MUST be capped by binding expiry. Equivalent binding/request/digest issuance MUST reuse a compatible unexpired receipt without extending its TTL; changed projection MUST create a new immutable receipt. Concurrent equivalent issuance MUST converge and all API replicas MUST enforce hard per-binding/global retained-row caps atomically. Default TTL MUST be 86400 seconds with allowed 60–604800; per-binding cap 128 with allowed 1–4096; global cap 100000 with allowed 1–1000000; cleanup batch 500 with allowed 1–5000. Per-binding cap MUST NOT exceed global cap and invalid configuration MUST fail startup. Caps include retained expired rows. Cleanup MUST delete only expired receipts in bounded batches; issuance MUST NOT evict live receipts. Required persistence/capacity failure MUST produce bounded retryable 503 with no successful partial receipt result.

#### Scenario: Equivalent concurrent requests issue references

- **WHEN** multiple API replicas issue the same supported projection for one binding and normalized read concurrently
- **THEN** they return one compatible live receipt identity without extending expiry or exceeding retained-row caps

#### Scenario: Capacity is exhausted

- **WHEN** bounded expired-row cleanup cannot free space under a hard cap for a new receipt
- **THEN** new issuance fails bounded 503 while existing compatible receipt reuse remains permitted and no live reference is evicted

#### Scenario: Failed serialization would leave unused state

- **WHEN** the full digest/continuity success body cannot serialize within the effective allocation
- **THEN** no new receipt is persisted and the request returns the existing bounded non-success budget outcome

#### Scenario: API is restarted after successful issuance

- **WHEN** PostgreSQL retains a compatible unexpired receipt and its binding remains authorized
- **THEN** another API process can compare the receipt without an in-process cache or historical context body

### Requirement: Discovery distinguishes context identity from replay and synchronization

Capabilities SHALL explicitly advertise supported context digest/continuity/canonicalization/selection versions, algorithm, reference size and configured TTL. The effective nonsecret policy fingerprint MUST be returned only with the authorized context result, not exposed as active scoped rollout identity through unscoped discovery. Capabilities MUST declare `fresh_compare` supported, `exact_snapshot_replay=false` and `context_watermark=false`. Context digest, provider OpenAPI schema digest and synchronization snapshot digest/cursor MUST remain separately named and noninterchangeable. Request IDs and idempotency keys MUST NOT imply context snapshot reuse. Consumers requiring PC4 MUST fail compatibility if the extensions are absent or unsupported, never rewrite requests, substitute another digest, or implement local selection/packing as a workaround. Any Danny child/business projection fingerprint MUST have a separate name and MUST NOT assert Stele-selection equivalence or business/tool/write authority.

#### Scenario: Consumer discovers an older Provider

- **WHEN** a consumer requiring PC4 cannot discover its supported digest and continuity contracts
- **THEN** it reports incompatibility without dropping continuity requirements or treating schema/sync metadata as context identity

#### Scenario: Consumer requests historical replay

- **WHEN** a runtime reads the capability document or attempts an undocumented snapshot/watermark context field
- **THEN** discovery states those features are unsupported and input validation rejects the unsupported field rather than promising replay
