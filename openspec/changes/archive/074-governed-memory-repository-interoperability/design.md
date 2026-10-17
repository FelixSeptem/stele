## Context

See [proposal.md](proposal.md) for motivation and reference sources. Existing governed intents already provide scoped authorization, idempotency, asynchronous processing, and append-only outcomes. Existing trust lineage and context projections already govern what can be disclosed. The missing component is an adapter from an explicitly supplied external snapshot to those contracts.

## Goals / Non-Goals

**Goals:** deterministic preview, explicit import selection, recoverable dispatch, inspectable provenance, and bounded navigation using existing eligibility rules.

**Non-Goals:** Git transport, generic Markdown rendering, automatic update/forget decisions, new retrieval ranking, automatic context injection, and a new scheduler. This version exposes HTTP resources, not new MCP tools.

## Decisions

### 1. Supplied snapshots, with registered source identity

Register a repository under one exact `(project, tenant, namespace)` tuple and obtain a server-generated `repository_id` and generation `0`. The display name and optional source URL are untrusted labels, not identity or access grants. Registration requires the existing scoped write entitlement; preview/submission also require governed-intent submission authorization. Inspection requires the existing scoped read entitlement and current disclosure policy. Administrative historical disclosure remains on existing explicit admin/debug paths.

Clients submit a JSON snapshot containing `format_version: memory-repo-v1`, `repository_id`, `source_revision`, and a list of `{path, content}` UTF-8 Markdown files. A revision is a bounded, opaque claimed identifier; a Git SHA is neither proof of authenticity nor a sortable version. Generation is Stele's concurrency token. The same revision cannot identify different snapshot digests within one repository; reject that ambiguity.

No server-side file paths, remotes, credentials, archives, or symlinks are accepted. Files are strings in the request. This is simpler to self-host than a Git checkout service and eliminates a second authority over canonical state.

Initial contract limits are 128 Markdown files, 64 KiB UTF-8 per file, 1 MiB total decoded content, 2 MiB request body, 1,000 parsed entries, 8 KiB per entry, 256 bytes per path, 256 bytes per revision, 16 metadata pairs per entry, and 2,048 bytes per metadata value. Validate both raw request size and decoded content. Limits are enforced before expensive parsing; over-limit input is rejected, never truncated into an importable preview. Runtime configuration may lower limits; publish effective limits and parser version in previews and capability documentation.

### 2. Explicit, conservative Markdown subset

`MEMORY.md` at the root is required. Paths use case-sensitive, NFC-normalized, slash-separated relative names ending in `.md`; reject duplicate normalized paths, absolute paths, drive prefixes, backslashes, empty/dot/dot-dot segments, controls, percent escapes, and paths longer than the limit. Reject non-NFC spelling instead of silently aliasing it. Repository paths remain provenance, never implicitly become Stele `memory_path` values.

Recognize top-level, single-line `- ` bullets outside code fences. A bullet containing only a `[[target]]` is navigation. A fact bullet can have a final `[key: value; key: value]` metadata suffix. Recognize `source` and `added`; preserve other keys as opaque bounded metadata without mapping them to scope, class, trust, lifecycle, or policy. Duplicate keys, malformed suffixes, and invalid RFC 3339/date-only `added` values are blocking diagnostics. Without a suffix the whole bullet is fact text. Headings, prose, nested/multiline lists, code blocks, and unsupported link syntax produce explicit non-imported diagnostics; submission must acknowledge the preview's warning digest. This is a documented subset of the reference format, not a claim of complete Markdown compatibility.

Root-relative wiki links omit `.md` for Markdown targets. Links with another extension are unresolved artifact references and never cause a file read or execution. Schemes, fragments, aliases, traversal, and links outside the supplied set cannot resolve. Preserve bounded unresolved diagnostics. Resolve each edge once without recursive expansion, so cycles do not expand the payload. `source` URLs are opaque text, never fetched. All entry text remains data even when it says to run a command or change an instruction.

Before implementation, evaluate mature Markdown parsers on `pkg.go.dev` (including Goldmark) for bounded AST traversal and fence/list detection. Record the choice with fixtures; do not build a general Markdown parser or add a renderer. Library evaluation is an implementation task, not completed research in this draft.

### 3. Preview is the complete reviewable import plan

Draft additive HTTP resources, following existing request scope and error-envelope conventions:

| Method and path | Purpose / successful response |
| --- | --- |
| `POST /v1/memory-repositories` | Register source; `201` with scoped ID and generation |
| `POST /v1/memory-repositories/{id}/previews` | Parse snapshot and explicit selections; `201` with preview ID, digest, generation, expiry, diagnostics, plan, and limits |
| `POST /v1/memory-repositories/{id}/imports` | Submit immutable preview; `202` with import ID, generation, and status URL; equivalent replay returns the same import |
| `GET /v1/memory-repositories/{id}/imports/{import_id}` | Read bounded progress and currently disclosable entry outcomes |
| `GET /v1/memory-repositories/{id}/manifest` | Read bounded current navigation with import/generation watermark |

A preview request includes actor/reason, expected generation, and explicit per-path selections of existing memory class and destination `memory_path`. No class is inferred from filenames or prose; every selected fact requires a class and a valid destination. Use the existing five classes and scoped path validator. Unselected paths remain navigation/evidence only. Preview contains server-generated stable entry keys, proposed remember payloads, duplicates/skips, and diagnostics. Preview never enqueues intents or changes canonical state, though its immutable record is persisted for review and submission.

Persist previews for a 24-hour submission window. Submission carries `preview_id`, `plan_digest`, acknowledged warning digest, `expected_generation`, and an idempotency key. Digest binds canonicalized snapshot, revision, selection/mapping, effective trust metadata, parser version, and all semantic plan fields. File ordering is canonicalized by path; content bytes, including line endings, remain exact source evidence. Policy/authorization changes are rechecked and never silently alter the approved plan. A changed plan requires another preview. Expiry blocks a new import; an already accepted equivalent replay remains inspectable subject to current authorization.

Use the existing structured error envelope: invalid JSON `400`, unauthenticated `401`, scope/grant/disclosure denial `403`, absent in authorized scope `404`, fingerprint/revision/generation/policy mismatch `409`, size limit `413`, format/selection errors `422`, rate limit `429`. An expired unsubmitted preview yields `409 preview_expired`. Inspect scope before resource existence or idempotency; denial reveals no foreign IDs, counts, content, or generation. Disabled functionality uses the existing feature-disabled contract, documented in OpenAPI before handlers are added.

### 4. Stable identities and optimistic submission

An entry fingerprint hashes versioned canonical fact text plus sorted metadata. Canonicalization trims bullet delimiters and surrounding whitespace, but does not rewrite prose. Entry identity includes repository ID, normalized file path, entry fingerprint, class, destination path, and effective trust metadata. Line numbers are locators only. Identical bullets in one file collapse to one proposed submission with all occurrence locators. Reordering or changing `source_revision` does not create another submission for an unchanged entry identity. A move to another file or changed text/metadata/mapping is new evidence, shown explicitly in preview.

Within one transaction, reauthorize, check idempotency, compare expected generation, persist the import and per-entry dispatch records, and advance generation once. Concurrent distinct submissions from the same generation yield one winner and one `409 stale_generation`. Equivalent replay checks the stored fingerprint before generation, since a successful first submission has already advanced it. A reused key with different semantic input returns conflict. Accepted identities remain deduplicated even when their original memory is later suppressed, forgotten, or deleted; a new import cannot resurrect them implicitly. Durable tombstones retain non-content deduplication keys under existing retention policy.

An accepted plan can finish with rejected or failed entries; generation tracks acceptance, not canonical success. Missing files and entries have no deletion semantics. Automatic matching of changed prose to old canonical IDs was rejected because it could overwrite unrelated or consolidated memories. Explicit update/forget remains available through existing governed APIs.

### 5. Durable asynchronous dispatch and provenance

Reuse existing workers and governed `remember` submission. A committed plan is the durable dispatch source; deterministic child idempotency keys derive from import ID and entry key. A worker crash after child submission but before local acknowledgement replays the same child, rather than creating another candidate. Scope, current authorization, and enabled policy are checked again before dispatch; revocation terminates undispatched items with bounded outcomes. Already accepted children remain governed by their own lifecycle and audit contracts.

Import processing states are `queued`, `running`, and `completed`, with per-entry `submitted`, `duplicate`, `rejected`, or `failed` dispatch outcomes and append-only attempts. Transient failures use existing bounded retry/backoff; exhausted failures remain recorded. `completed` means dispatch is settled, not that every memory is active. Inspection separately reports existing child-intent/candidate outcomes when disclosable. No new client retry endpoint is introduced; a terminal failure is not implicitly retried by resubmitting the same snapshot.

Carry repository ID, claimed revision, snapshot digest, source path, occurrence locators, fingerprint, preview/import IDs, parser version, actor/reason, and server observation time through intent/event/candidate/canonical provenance. Claimed `added` is separate from server timestamps. Effective source metadata follows the existing trust contract: absent means `untrusted/restricted`; an explicit attestation requires its existing entitlement and remains part of the plan fingerprint. A Markdown key, URL, or commit hash never grants trust. Existing most-restrictive derivation rules apply during consolidation.

### 6. A manifest is navigation, with current disclosure checks

Manifest reads use the latest accepted import generation and include its import ID plus processing state. While processing is incomplete, it can contain fewer eligible targets; it never claims that all submitted entries are active. Use `MEMORY.md` links and fact occurrences to order navigation, with a hard 100-item maximum per response and no automatic traversal. Return only canonical IDs/versions and source paths whose backing memory currently passes authorization, lifecycle, and disclosure checks. Labels come from eligible canonical content or eligible source paths; raw entry-point prose is not injected or exposed as a trusted summary. Pending, failed, suppressed, forgotten, deleted, or unauthorized targets cannot produce visible nodes, labels, or hidden-target counts.

Import status and retained preview evidence are not an alternate retrieval channel: content-bearing diagnostics and outcomes must obey current disclosure rules. Safe processing status can remain visible to the authorized submitter; historical payloads require the existing explicit administrative evidence path. Source occurrences must follow canonical eligibility after derivation; incomplete lineage fails closed. Avoid shared response caching in v1 to prevent stale grants/lifecycle from leaking content.

Manifest limits and watermark are separate from existing L0/L1/L2 projections. No `always_visible` flag, score boost, or prompt instruction is derived from being in `MEMORY.md`. Clients wanting context continue through existing governed retrieval/projection APIs.

## Risks / Trade-offs

- Narrow syntax can skip useful prose → preview lists every unsupported construct, requires warning acknowledgement, and documents exact fixtures.
- Changed entries can coexist with earlier facts → preview calls them new evidence; existing consolidation handles contradictions and explicit lifecycle operations handle removal.
- External provenance may be false → label revision/source/date as claims; trust needs existing independent authorization.
- Saved source payloads could bypass forgetting → gate all content-bearing reads by current lineage and existing evidence retention/disclosure policy; never put payloads in logs or metric labels.
- Batch failure is not atomic canonical publication → document per-entry outcomes and distinguish dispatch completion from activation.
- Retained snapshots add storage cost → enforce admission limits and existing retention/purge policy, retaining only permitted audit/tombstone data after payload expiry.

## Migration Plan

1. Publish OpenAPI shapes, error codes, defaults, scope/grant mappings, and fixtures before implementing handlers.
2. Add scoped PostgreSQL tables and unique constraints for repositories, previews, import plans, dispatch attempts, and deduplication/provenance mappings. No existing memory backfill or rewritten canonical rows.
3. Deploy behind `memory_repository_import_enabled=false`; enable only after parser, database, worker, disclosure, and public contract checks pass. Use existing `api` and `worker` modes; existing cleanup scheduling handles preview retention.
4. Update README, the new operations guide, integration guide, and agent skill in the same implementation change. Include preview/submit/inspect examples, warning acknowledgement, limits, compatibility subset, one-way semantics, and no implicit forgetting.
5. Roll back admission by disabling the flag. Already committed imports continue governed draining/reconciliation; workers must retain schema compatibility until drained. Rollback does not delete imported canonical memory: use existing explicit lifecycle operations if needed. Preserve audit records and avoid destructive down-migrations while referenced.

## References

- [Behavior requirements](specs/memory-repository-interoperability/spec.md)
- [Implementation tasks](tasks.md)
- [Proposal and pinned upstream sources](proposal.md#references)
