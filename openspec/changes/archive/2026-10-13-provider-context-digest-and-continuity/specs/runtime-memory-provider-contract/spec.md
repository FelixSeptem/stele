## ADDED Requirements

### Requirement: Digest and continuity conformance verifies fresh public projections

Provider conformance SHALL validate actual serialized public context responses and discovery against authoritative OpenAPI and supported digest/continuity versions. It MUST independently verify canonical digest using authenticated binding scope and normalized original input, mandatory closed fields, reference/expiry formats, input/output union correspondence, matching citation summaries and exact whole-body budget accounting. Fixtures MUST cover both modes, empty/nonempty results, Unicode/escapes, duplicate placements, order/version/path/content/provenance/policy changes, equivalent effective limits and volatile metadata exclusion. Missing, malformed, forged or incompatible digest/continuity metadata MUST produce failed-contract rather than an empty/unchanged success. Receipt authority, changed-request, foreign/unknown/expired/retention-gap, unsupported-contract, dependency and capacity cases MUST be tested. Reports MUST remain bounded/redacted and distinct from ranking metrics; validation MUST NOT rerank or locally pack results.

#### Scenario: Service returns a plausible but incorrect digest

- **WHEN** a response has the documented shape but its canonical semantic projection does not hash to its supplied value
- **THEN** conformance records failed-contract regardless of its item count or HTTP status

#### Scenario: Compare response contradicts the request

- **WHEN** a compare request receives not-requested/issued metadata, malformed expiry, a missing reference or an unsupported digest version
- **THEN** the consumer rejects it rather than treating zero decoded context as unchanged success

#### Scenario: Receipt overhead is omitted from accounting

- **WHEN** a success body adds digest/continuity fields but its response usage excludes their serialized bytes
- **THEN** conformance records an exact-body accounting failure

### Requirement: Continuity live evidence proves durable restart and current authorization

PC4 integration evidence SHALL use a pinned service revision/image, fresh owned PostgreSQL/pgvector, official exact-scope bootstrap and disclosure grants, public labeled ingestion and database-proven governance completion. It MUST prove supported digest determinism, issue/compare in both modes, non-root exact/prefix and empty projections, retained metadata-only receipt durability and same-binding comparison after API/worker restart. Changed source versions/lifecycle, grant downgrade/revocation, expiry/unavailable references and bounded admission/error behavior MUST be verified through the public contract; SQL checks MUST supplement rather than replace public writes/reads. Focused transaction/concurrency/capacity/upgrade evidence MUST be labeled separately. Required missing/skipped/stale/degraded evidence MUST block PC4 readiness; historical PC1–PC3 artifacts MUST NOT substitute for a fresh PC4 run. Artifacts MUST contain only bounded outcome and service/image/OpenAPI/contract provenance, never credentials, raw content/payloads, scope/memory identities or raw references/digests.

#### Scenario: Fresh receipt survives API and worker restart

- **WHEN** public ingestion completes governance, a reference is issued and same-binding comparison passes after the owned stack restarts
- **THEN** evidence records durable fresh-compare behavior and compatible digest equality without claiming exact snapshot replay

#### Scenario: Source changes after the original projection

- **WHEN** public governed source changes complete and comparison runs with the original read/reference
- **THEN** evidence records the fresh changed result and verifies removed/ineligible evidence is absent

#### Scenario: Previous live evidence is offered as completion

- **WHEN** only PC3 or in-process fixtures exist without PC4 receipt/restart/current-grant evidence
- **THEN** readiness remains incomplete rather than inheriting an earlier context pass

### Requirement: Digest continuity documentation and migration are acceptance gates

PC4 acceptance MUST include matching OpenAPI, README, Provider integration, configuration/self-host/bootstrap, architecture/components/best-practice, affected MCP and agent-skill documentation in the same implementation. Documents MUST describe canonical identity and vectors, all discovery/input/output versions, fresh versus compare versus unsupported snapshot/watermark, reference-not-credential semantics, expiry/caps/cleanup/errors, byte-envelope overhead, strict-consumer migration, additive schema upgrade and safe rollback. Context/schema/sync digests MUST be distinguished; external tool/write/business policy and any child projection fingerprint MUST remain external and separately named. PC4 completion MUST NOT imply PC5 durable-reference or PC6 outcome workflow completion.

#### Scenario: Consumer follows maintained integration examples

- **WHEN** a runtime adopts the revised contract
- **THEN** examples require extension discovery, complete mode/accounting/digest/continuity validation and fresh current authorization without local selection or historical-body reuse

#### Scenario: Operator rolls back to a binary without PC4

- **WHEN** an operator follows maintained rollback instructions
- **THEN** PC4 consumers are disabled or receive compatibility failure, existing disclosure/budget protection and durable data are retained and unsupported continuity is not silently stripped
