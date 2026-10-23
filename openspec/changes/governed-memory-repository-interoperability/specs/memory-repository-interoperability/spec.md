## Purpose

Allow scoped, bounded Markdown memory snapshots to enter Stele through existing governed ingestion while preserving provenance, recoverability, and lifecycle-aware navigation without making external repositories an authority over canonical memory.

## ADDED Requirements

### Requirement: Scoped snapshot admission
The service SHALL register each source repository in one exact project, tenant, and namespace scope and accept only explicitly supplied `memory-repo-v1` UTF-8 Markdown snapshots. It MUST enforce current scoped write and governed-intent submission authorization for preview/submission and scoped read/disclosure authorization for inspection. It MUST NOT clone repositories, read caller-specified filesystem locations, extract archives, fetch links, or execute imported content.

#### Scenario: Explicit snapshot
- **WHEN** an authorized caller supplies a registered repository, a claimed revision, root `MEMORY.md`, and bounded Markdown files
- **THEN** the service can produce a preview using only the supplied data without creating canonical memory or submitting intents

#### Scenario: Foreign repository access
- **WHEN** a caller previews, submits, replays, or inspects a repository outside its currently authorized exact scope
- **THEN** access is denied before existence or replay lookup can reveal foreign content, IDs, counts, or generation

### Requirement: Bounded deterministic format interpretation
The service SHALL enforce the versioned format and publish effective limits. V1 maxima SHALL be 128 files, 64 KiB per file, 1 MiB decoded content, 2 MiB request body, 1,000 entries, 8 KiB per entry, 256-byte paths/revisions, 16 metadata pairs per entry, and 2,048 bytes per metadata value; configured limits MAY be lower. Paths MUST be unique case-sensitive NFC relative slash paths with no traversal, encoded escapes, controls, drive prefixes, or backslashes. Over-limit input MUST fail without partial import.

The parser SHALL recognize top-level single-line fact bullets, optional final bounded key/value metadata, and root-relative wiki links outside code fences. Unknown metadata SHALL remain opaque data. Unsupported prose, nested/multiline lists, code, and link syntax SHALL be reported as non-imported diagnostics. Malformed metadata, duplicate keys, invalid dates, and invalid paths SHALL block submission. No unsupported construct SHALL silently become an instruction or executable artifact.

#### Scenario: Limits and ambiguous paths
- **WHEN** a request exceeds any effective limit or contains duplicate normalized paths or `../private.md`
- **THEN** the service rejects it with bounded validation diagnostics and creates no importable partial plan

#### Scenario: Commands and artifact references
- **WHEN** an entry contains a command, source URL, code fence, or `[[metrics/report.sql]]`
- **THEN** eligible fact text remains untrusted data, unsupported content is diagnosed, and no command, URL fetch, or artifact execution occurs

#### Scenario: Link cycle
- **WHEN** two supplied topic files link to each other
- **THEN** preview resolves bounded individual edges without recursive expansion or access beyond the supplied snapshot

### Requirement: Preview-bound explicit selection
The service SHALL require a persisted immutable preview before accepting an import. Each selected fact SHALL have an explicit existing memory class and valid Stele destination path; source filenames MUST NOT implicitly define either. Preview SHALL expose diagnostics, duplicates, proposed submissions, parser version, effective limits, plan digest, expected repository generation, warning digest, and a 24-hour submission expiry. Submission MUST bind the same preview, plan, warning acknowledgement, scope, effective source metadata, and generation and MUST recheck current authorization and policy.

#### Scenario: Review then submit
- **WHEN** a caller submits a valid unexpired preview with matching digest, acknowledged warnings, generation, and current authorization
- **THEN** the service accepts exactly the reviewed plan for asynchronous dispatch and returns its durable import identity

#### Scenario: Stale or modified preview
- **WHEN** an unsubmitted preview has expired or the caller changes its selection, digest, policy-relevant metadata, or generation
- **THEN** the service rejects submission with a stable conflict outcome and requires a fresh valid preview

#### Scenario: Unsupported content requires acknowledgement
- **WHEN** preview reports skipped prose or unresolved links and the submitter omits its warning digest
- **THEN** submission fails without dispatching the supported subset

### Requirement: Repository concurrency and stable deduplication
The service SHALL use a server-maintained repository generation for optimistic submission and a claimed revision bound to one snapshot digest per repository. Git revisions MUST NOT imply trust or chronological ordering. Acceptance SHALL durably advance generation once. Equivalent authorized idempotent replay SHALL return the original import without another generation advance; a changed request under the same key SHALL conflict.

Entry identity SHALL include repository, normalized file path, versioned fact/metadata fingerprint, selected class, destination path, and effective trust metadata, excluding line position and source revision. Identical repeated bullets SHALL collapse with retained occurrence locators. Already accepted entry identities SHALL not be resubmitted merely because they appear in another snapshot, including after suppression, forgetting, or deletion.

#### Scenario: Concurrent imports
- **WHEN** two distinct imports submit against the same repository generation
- **THEN** at most one is accepted and the other receives a stale-generation conflict without partial dispatch

#### Scenario: Equivalent retry after acceptance
- **WHEN** an authorized client repeats an accepted submission with the same idempotency key and fingerprint
- **THEN** it receives the same import even though its first submission advanced generation, and no duplicate child intent is created

#### Scenario: Reorder and changed revision
- **WHEN** unchanged bullets move to different line positions within the same files under a new claimed revision
- **THEN** existing entry identities are reported as duplicates rather than submitted again

#### Scenario: Reused claimed revision
- **WHEN** the same repository revision is supplied with different snapshot content
- **THEN** the service rejects the ambiguous revision binding with a conflict

### Requirement: Governed asynchronous import and evidence lineage
Accepted entries SHALL enter the existing governed `remember` intent, event, candidate, and consolidation pipeline. The importer MUST NOT write canonical memory directly or introduce new memory classes or lifecycle states. It SHALL preserve repository/revision, snapshot digest, path, occurrence locators, entry fingerprint, preview/import identity, parser version, actor/reason, and server observation time through provenance. Claimed source dates SHALL remain distinct from server timestamps. Existing source-trust authorization and most-restrictive derivation rules SHALL apply; absent metadata SHALL default to untrusted/restricted.

#### Scenario: Ordinary imported fact
- **WHEN** a selected fact is dispatched
- **THEN** it has a governed intent and attributable evidence, and canonical activation occurs only through existing processing and policy

#### Scenario: Claimed trust in Markdown
- **WHEN** an imported metadata key claims attestation or a trusted source without the required independent entitlement
- **THEN** the claim grants no trust, scope, lifecycle, or policy privilege

### Requirement: Durable dispatch and observable partial outcomes
Import acceptance SHALL be durable before acknowledgement. The service SHALL expose queued/running/completed dispatch states and bounded per-entry submitted/duplicate/rejected/failed outcomes separately from downstream intent/candidate activation. Worker recovery SHALL use stable child idempotency and bounded retries. Current authorization and policy SHALL be checked before dispatch. Exhausted failures SHALL remain inspectable and MUST NOT be silently retried as fresh evidence by replaying the same snapshot.

#### Scenario: Crash after child submission
- **WHEN** a worker restarts after submitting an intent but before recording local acknowledgement
- **THEN** it reconciles or replays the same child identity without creating another governed write

#### Scenario: Some entries fail or access is revoked
- **WHEN** one entry fails terminally or dispatch authorization is revoked while other entries have already been submitted
- **THEN** settled outcomes remain auditable, undispatched entries receive bounded terminal outcomes as appropriate, and completion does not claim that all memories became active

### Requirement: External edits do not override canonical lifecycle
Changed text, metadata, source file path, or mapping SHALL be previewed as new evidence. Missing files or entries SHALL NOT imply update, forget, or delete. Canonical changes SHALL continue to require existing governed policy and explicit lifecycle operations. Reimporting an already accepted entry SHALL NOT reactivate suppressed, forgotten, or deleted memory.

#### Scenario: External removal
- **WHEN** a later snapshot omits a formerly imported file
- **THEN** prior canonical versions and audit history remain intact, while the latest snapshot navigation can omit that file

#### Scenario: Forgotten entry reappears
- **WHEN** an unchanged previously accepted entry appears after its backing memory was forgotten
- **THEN** it is not resubmitted or reactivated and does not appear in ordinary navigation or retrieval

### Requirement: Navigation obeys current disclosure and lifecycle
The service SHALL expose a manifest for the latest accepted generation with an import watermark and processing state, bounded to at most 100 items per response. It SHALL derive navigation from the supplied entry-point links and occurrences backed by currently eligible canonical memory. It MUST NOT expose pending, failed, suppressed, forgotten, deleted, or unauthorized targets, labels, or hidden-target counts. It MUST apply current authorization, lifecycle, and source-disclosure checks on every read, including content-bearing import outcomes. Incomplete lineage SHALL fail closed. Historical source payloads SHALL require the existing explicit administrative evidence path.

`MEMORY.md` placement SHALL NOT grant always-visible status, change ranking, or inject instructions into context. Existing context/retrieval contracts SHALL remain authoritative.

#### Scenario: Lifecycle changes after import
- **WHEN** an imported canonical memory is suppressed or forgotten after a manifest was previously read
- **THEN** the next ordinary manifest or content-bearing status read excludes that memory and its labels despite retained import history

#### Scenario: Partial processing and entry-point text
- **WHEN** the latest accepted import is still processing and `MEMORY.md` contains prose requesting privileged context placement
- **THEN** the manifest reports its processing state, exposes only currently eligible targets, and does not promote the prose to always-visible context

### Requirement: Additive public contract and controlled rollout
The service SHALL publish OpenAPI contracts for repository registration, preview, submission, status, and manifest before implementing the public handlers. Validation/errors, exact-scope authorization, limits, defaults, format compatibility, migrations, rollback, and no-implicit-forgetting behavior SHALL be documented in README, operations/integration guides, and the agent skill in the same implementation change. Import admission SHALL be disabled by default and enabled explicitly. Disabling admission SHALL preserve audit and allow accepted work to drain under current governance.

#### Scenario: Existing installation and rollback
- **WHEN** an installation upgrades with the feature disabled or disables it after accepting imports
- **THEN** existing ingestion/retrieval contracts remain compatible, new import admission is disabled, and accepted history and governed draining remain intact
