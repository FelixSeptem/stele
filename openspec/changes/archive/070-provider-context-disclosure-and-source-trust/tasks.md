## 1. Contract and boundary inventory

- [x] 1.1 Inventory protected content/identity outputs across Provider, native HTTP, MCP, history/provenance, sessions/intents, and write/replay responses; deliver an endpoint enforcement/denial matrix and verify every registered public content route is covered (at most 4 hours).
- [x] 1.2 Publish OpenAPI disclosure input enums/defaults, capability/source-metadata versions, mode-discriminated categorized results, and bounded error schemas; verify canonical reference/content examples and negative shapes with OpenAPI tests (at most 4 hours).
- [x] 1.3 Define the fixed profile registry, source-metadata value types, classification join, and attestation semantics; verify table tests for defaults, unknown labels, mixed sources, and floors (at most 4 hours).
- [x] 1.4 Review existing auth/storage/JSON/governance infrastructure and the `pkg.go.dev` library requirement before adding infrastructure; record reuse/dependency decisions and verify no parallel policy/fact store or speculative dependency is introduced (at most 2 hours).

## 2. Durable entitlements and administrative provisioning

- [x] 2.1 Add additive PostgreSQL grant/source-metadata schema migration(s) with conservative defaults and compatible upgrade behavior; verify migration manifest, fresh install, and PC1-schema upgrade tests without granting implicit content/attestation (at most 4 hours).
- [x] 2.2 Implement persisted exact-scope allowed/default profiles and independent source-attestation entitlement using existing principal repositories; verify bounded list validation, default membership, round trip, and scope isolation (at most 4 hours).
- [x] 2.3 Extend admin grant create/update/read surfaces with actor/reason audit and closed OpenAPI inputs; verify public self-provisioning denial, invalid profile refusal, and redacted admin/telemetry outputs (at most 4 hours).
- [x] 2.4 Resolve current disclosure authority per request from principal, scope, binding, and server policy; verify profile/mode intersection, missing-grant denial, repository failure, and grant downgrade/revocation using an old binding (at most 4 hours).
- [x] 2.5 Update official bootstrap and fixture provisioning to select explicit profiles with attestation disabled by default; verify public runtime initialization succeeds only with valid exact-scope provisioning and cannot override entitlements (at most 4 hours).

## 3. Ingestion and canonical source snapshots

- [x] 3.1 Add typed source metadata to native/Provider event inputs and shared ingest validation, applying conservative defaults and server floors; verify malformed/unknown metadata, reserved application-metadata spoofing, and unauthorized attestation are rejected or non-authoritative (at most 4 hours).
- [x] 3.2 Persist/restore raw-event metadata and authenticated attestation lineage, including label-aware durable idempotency fingerprints; verify equivalent replay, label-change conflicts, reauthorization, and restart repository round trips (at most 4 hours).
- [x] 3.3 Carry effective metadata through extraction and candidate persistence, including the existing candidate sensitivity floor; verify candidate source/path/sensitivity snapshots and source-event isolation (at most 4 hours).
- [x] 3.4 Preserve typed labels on ordinary and superseding canonical promotion plus every canonical version; verify append-only snapshots, non-root path retention, and provenance round trips for each promotion branch (at most 4 hours).
- [x] 3.5 Cover generated summary-canonical promotion and lifecycle/version transitions with the same restrictive join; verify summary creation and suppression/forgetting/deletion retain labels and old versions (at most 4 hours).
- [x] 3.6 Apply source-label rules to governed intents and manual create/update operations; verify existing source labels cannot be weakened by request metadata, target-version changes, or replay (at most 4 hours).
- [x] 3.7 Apply source-label rules to merge/reclassification and legacy repository reads; verify restrictive mixed-source joins and conservative legacy defaults without rewriting historical versions (at most 4 hours).

## 4. Derived artifacts and existing retrieval eligibility

- [x] 4.1 Propagate versioned source metadata through summary compaction and reflection; verify attested/untrusted and differently classified source combinations cannot upgrade trust or reduce classification (at most 4 hours).
- [x] 4.2 Propagate metadata through ordinary/experience insights and their evidence links; verify complete/incomplete lineage joins and inherited source classification (at most 4 hours).
- [x] 4.3 Cover reasoning/reserved/contradiction/goal insight derivation and optional visibility paths; verify unsupported or unverifiable text sections are omitted and model outputs cannot self-attest (at most 4 hours).
- [x] 4.4 Preserve metadata/source-version identity on chunks, materialized context projections, and rebuilds; verify stale/missing source snapshots and legacy caches cannot acquire permissive labels (at most 4 hours).
- [x] 4.5 Apply disclosure/source eligibility inside existing retrieval and section selection, with final independent evidence-reference checks; verify hidden/foreign/high/restricted sources cannot affect or appear in disclosed context, including cached summaries and citations (at most 4 hours).

## 5. Provider projection and alternate-route enforcement

- [x] 5.1 Extend PC1 closed context decoding with canonical projection/profile/mode fields and defaults; verify aliases, duplicate/null/type/trailing JSON, invalid enums, unsupported versions, and profile/mode refusal outcomes (at most 4 hours).
- [x] 5.2 Implement reference-only categorized DTO shaping and closed evidence allowlists; verify serialized content/title/summary/lesson/goal/metadata/diagnostic absence, authorized optional references, and typed empty arrays (at most 4 hours).
- [x] 5.3 Annotate separately authorized content-mode results with effective source metadata and preserve existing selection/order and independent insight/goal/diagnostic gates; verify explicit mode authorization and omission of unprovable sections (at most 4 hours).
- [x] 5.4 Advertise disclosure/source versions and supported modes/profiles without individual entitlements; verify capabilities/OpenAPI digest consistency and strict consumer validation of reference/content/malformed results (at most 4 hours).
- [x] 5.5 Enforce the existing server ceiling on fully serialized authorized context without partial output or local repacking; verify oversize refusal and distinguish the ceiling from pending PC3 budget units/accounting (at most 4 hours).
- [x] 5.6 Enforce the endpoint matrix at shared native/MCP/Provider authorization boundaries, denying unsupported content routes before lookup; verify direct native requests without bindings, retrieve/sync/history/session/intent paths, and alternate MCP tool access cannot bypass constrained grants (at most 4 hours).
- [x] 5.7 Shape safe successful/replayed event/intent/lifecycle acknowledgments and bounded errors for constrained principals; verify stored content, unrestricted metadata, hidden identities, and invalid request text never leak via write/replay/refusal outputs (at most 4 hours).

## 6. Offline and real-stack evidence

- [x] 6.1 Extend offline Provider conformance with mode-specific schema, text marker, citation authorization, grant manipulation, dependency failure, and alternate-route fixtures; verify context contract failures remain distinct from genuine empty results and ranking metrics (at most 4 hours).
- [x] 6.2 Extend the public context verifier with official provisioning and public low/medium labeled writes for reference/content/attestation/foreign principals; verify governance completion, exact/prefix reads, stored candidate/canonical/version labels, and replay/conflict results in fresh PostgreSQL/pgvector (at most 4 hours).
- [x] 6.3 Exercise grant downgrade/revocation on existing bindings, suppressed/forgotten/deleted sources, unsupported alternate routes, and marker-free references in the live fixture; verify bounded denials/omissions without raw content or credentials in evidence (at most 4 hours).
- [x] 6.4 Restart API/worker and repeat allowed/denied reads, current-grant checks, and idempotent replay; verify stored labels/history survive restart and publish bounded service/image/OpenAPI/disclosure/source-metadata provenance with missing/degraded dependencies remaining nonpassing (at most 4 hours).
- [x] 6.5 Run focused PostgreSQL integration cases for mixed-source summaries/insights/chunks/projections and schema upgrades where public live construction is unavailable; verify conservative joins/version history and label the coverage separately from public live evidence (at most 4 hours).

## 7. Maintenance documentation and completion gates

- [x] 7.1 Update Provider and principal/admin/bootstrap guides for profile provisioning, reference defaults, mode consumption, labels, and endpoint denials; verify all examples match OpenAPI and documented refusal/migration behavior (at most 4 hours).
- [x] 7.2 Update affected architecture/components/best-practice and governance/lifecycle/history documentation for durable lineage, trust versus instruction authority, and Danny ownership; verify no document claims attestation authorizes tools, writes, compiler submission, or business-state completion (at most 4 hours).
- [x] 7.3 Update README, MCP integration guide, and `skills/stele-memory/SKILL.md` for changed access/source semantics; verify skill references, supported tool behavior, conservative prompt treatment, and no adapter-side retrieval substitute or bypass advice (at most 4 hours).
- [x] 7.4 Document upgrade inventory, mixed-version readiness, credential-safe rollback, and the distinction from PC3–PC6; verify the documented rollback cannot restore broader access through an older binary (at most 4 hours).
- [x] 7.5 Run full Go tests, `go vet ./...`, strict OpenSpec validation, documentation checks, and `git diff --check`; reconcile roadmap with actual evidence and record remaining limitations in a verification artifact, declaring PC2 complete only when all required public/durable gates pass (at most 4 hours).
