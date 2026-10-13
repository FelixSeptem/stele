## 1. Contract and regression baseline

Each checkbox is scoped to at most four hours and includes its verification. Complete contract/failing-fixture tasks before implementing the corresponding behavior; do not mark acceptance from historical PC1–PC3 evidence.

- [x] 1.1 Review existing binding, disclosure, admission, migration and scheduler libraries plus mature alternatives on pkg.go.dev before infrastructure work; record reuse/no-new-dependency decisions and a requirement-to-code/test matrix in `verification.md` and verify every PC4 requirement has an owner.
- [x] 1.2 Publish closed continuity input/result unions, mandatory digest schema and discovery extensions in OpenAPI; verify spec generation and examples cover omitted/issue/compare, both modes, errors and unsupported snapshot/watermark.
- [x] 1.3 Add failing public-handler request fixtures for valid unions, aliases/duplicates/nulls/unknown fields, malformed tokens and invalid Unicode; verify failures exercise documented JSON rather than internal DTO construction.
- [x] 1.4 Add failing actual-body fixtures for required digest/continuity fields, canonical byte vectors and exact envelope overhead; verify baseline fails for the intended missing-contract behavior and existing disclosure/budget cases remain intact.

## 2. Canonical projection identity

- [x] 2.1 Implement strict continuity DTO decoding and canonical valid-Unicode checks without weakening existing request gates; verify request fixtures and invalid-input cases pass with no assembly on rejection.
- [x] 2.2 Implement the versioned typed canonical document and domain-separated SHA-256 over final admitted output; verify golden bytes for empty/nonempty both modes, Unicode/escaping, absent/empty rules, citation equivalence and repeated ordered placements.
- [x] 2.3 Define the reviewed nonsecret active selection-policy manifest and its fingerprint, reusing current pipeline policy identity; verify applicable config/policy changes alter identity while secrets/private candidate data never enter canonical bytes or discovery.
- [x] 2.4 Add digest invariance/change regressions for metadata, equivalent effective limits, bindings in one scope, source versions/order/path/content/trust/citations, scope/mode/intent and policy; verify independent consumer vectors agree without retrieval/ranking/packing code.

## 3. Bounded durable receipt storage

- [x] 3.1 Add the next additive PostgreSQL migration, metadata-only receipt schema and lookup/expiry/dedup indexes using correct existing text identifier types; verify clean migration and upgrade from `0035` preserve grants/source labels/history and accept provider-created principal IDs.
- [x] 3.2 Implement domain-separated normalized originating-read fingerprints and full-identity receipt lookup; verify query/selector/options/effective-budget changes differ, operation metadata/equivalent defaults match, and foreign binding/scope/session lookups disclose nothing.
- [x] 3.3 Implement random reference generation, PostgreSQL-authoritative issue/expiry time and immutable compatible-receipt reuse; verify TTL/binding-expiry capping, exact token/expiry formatting, no TTL renewal and absence of query/content/evidence payload storage.
- [x] 3.4 Implement transactional cross-process deduplication and global/per-binding retained-row capacity enforcement; verify concurrent equivalent issuance converges and distinct issuance cannot exceed caps, including retained expired rows and rollback/interruption cases.
- [x] 3.5 Add validated TTL/capacity/cleanup configuration with documented defaults/hard bounds; verify startup rejects out-of-range values and per-binding cap above global cap while defaults resolve correctly.
- [x] 3.6 Wire bounded expired-only cleanup through existing scheduler and issuance paths; verify batch bounds, live-reference preservation, expired-to-unavailable transition, reuse at capacity and bounded new-issue dependency failure.

## 4. Fresh-read route and complete-body accounting

- [x] 4.1 Reserve digest/continuity fixed envelope costs in PC3 admission before selection; verify minimum/decimal-boundary/multibyte/repeated-citation bodies in both modes and no receipt creation when mandatory output cannot fit.
- [x] 4.2 Implement compare validation after current authority and preflight, preserving scope/schema precedence; verify revoked/downgraded grants precede receipt lookup, foreign/unknown/expired/request-changed/incompatible cases return the exact bounded non-success codes and do not retrieve prior bodies.
- [x] 4.3 Integrate fresh assembly, final canonical digest and changed/unchanged outcomes for omitted/issue/compare; verify all successes retain categorized current evidence and no bodyless response, historical resurrection, second retrieval pass or mode widening occurs.
- [x] 4.4 Finalize actual digest/continuity body before durable receipt persistence and re-finalize fixed-width dedup substitutions before HTTP success; verify serialization/persistence failures emit no partial result, no false success and exact byte usage without hash self-reference.
- [x] 4.5 Add discovery versions/algorithm/reference size/TTL and explicit unsupported replay/watermark flags, keeping the effective policy fingerprint in authorized results; verify no scoped rollout/entitlement/occupancy leakage, schema/sync digests remain distinct and extensions are not advertised without compatible wiring.

## 5. Consumer, lifecycle and concurrency conformance

- [x] 5.1 Extend strict assurance validation for request/result union correspondence, supported versions, expiry/token formats, citation agreement, canonical digest recomputation and full-body accounting; verify malformed/forged metadata fails contract without treating missing output as empty/unchanged.
- [x] 5.2 Add public-route current-source tests for version replacement, suppression/forgetting/deletion, restrictive classification/trust, foreign sources and fresh reference-mode text absence; verify comparison contains only current authorized evidence and no hidden-source explanation/count.
- [x] 5.3 Add authority/source/storage outage, transaction race, TTL boundary, capacity and incompatible stored-contract cases; verify bounded non-success and run race checks on the changed concurrent paths plus PostgreSQL-backed transaction tests.
- [x] 5.4 Extend offline Provider conformance fixtures and bounded report categories for digest/continuity safety independently of ranking metrics; verify invalid hashes, input union conflicts and overhead accounting each produce failed-contract, and logs/artifacts contain no raw references/digests/payloads.

## 6. Fresh owned PostgreSQL and restart evidence

- [x] 6.1 Extend the public live verifier with PC4-versioned report identity, receipt issue/compare and both-mode budget/digest checks; verify the script rejects missing/bootstrap/governance/restart dependencies and does not accept earlier PC3 reports as PC4 readiness.
- [x] 6.2 Run a pinned repaired image against fresh owned PostgreSQL/pgvector with official bootstrap and public labeled ingestion; verify governance completion in PostgreSQL, non-root exact/prefix and empty context, metadata-only receipt storage, both modes and deterministic ample-budget digest results before restart.
- [x] 6.3 Restart the owned API/worker and repeat same-binding comparison; verify durable compatible receipt lookup and stable projection digest with a complete fresh response, labeling focused SQL/upgrade tests separately from public live coverage.
- [x] 6.4 Exercise public source/lifecycle changes, current grant downgrade/revocation, expired/unavailable references and infeasible allocations in the owned fixture; verify changed/refusal outcomes, no historical disclosure and bounded redacted evidence with service/image/schema/contract provenance.

## 7. Maintenance documentation and release gates

- [x] 7.1 Update README and Provider integration with complete versioned discovery/issue/compare examples, canonical vectors, strict-consumer migration, digest distinctions, errors and fresh-read/no-replay authority boundaries; verify examples match OpenAPI and closed DTO behavior.
- [x] 7.2 Update architecture/components, configuration/self-hosting/bootstrap and best practices for metadata-only persistence, TTL/caps/cleanup, additive migration, mixed-replica upgrade, overhead and safe rollback/logging; verify operator instructions match defaults and dependency outcomes.
- [x] 7.3 Update affected MCP documentation and `skills/stele-memory/SKILL.md` for available Provider semantics and unsupported alternatives while retaining external business/tool/write policy; verify no unsupported MCP tool or PC5/PC6 completion is claimed.
- [x] 7.4 Run appropriate full/focused Go tests, changed-path race checks, vet, documentation gates, strict change/all OpenSpec validation and diff checks; record fresh command outcomes and all required PC4 evidence in `verification.md`, verify all implementation checkboxes have independent evidence and update roadmap status only to the state actually achieved.
