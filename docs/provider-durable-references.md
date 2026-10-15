# Durable Provider references maintenance

The `durable-reference-v1` capability adds governed task, run, handoff, and
compilation evidence to Stele's existing event-to-canonical pipeline. Stele
does not become the task scheduler, business-state owner, compilation authority,
artifact publisher, or webhook sender. PC5 does not imply PC6 support.

## Reuse and dependency baseline

The implementation reuses the current Provider binding, operation metadata,
event idempotency, ingestion, candidate governance, canonical versions,
provenance, disclosure grants, and lifecycle services. PostgreSQL remains the
only system of record. Exact reference lookup is a repository operation and
never falls back to ranked retrieval. The library baseline is the existing
`pgx/v5`, `google/uuid`, and `kin-openapi`; no new dependency, queue, SDK, UI, or
business-state store is required. Mature library review should use their
maintained package documentation: [pgx](https://pkg.go.dev/github.com/jackc/pgx/v5),
[UUID](https://pkg.go.dev/github.com/google/uuid), and
[OpenAPI](https://pkg.go.dev/github.com/getkin/kin-openapi/openapi3).

| Requirement | Implementation boundary | Verification |
| --- | --- | --- |
| Explicit capability/version | Provider capability discovery and closed OpenAPI DTOs | Strict consumer rejects absent capability, wrong version, aliases, unknown fields, nulls |
| Governed write and replay | Public reference write delegates to ordinary ingestion | Serialized HTTP fixtures; identical replay keeps identity and changed reuse conflicts |
| Completion and lineage | Durable raw event, candidate, current canonical version and promotion provenance | Repository tests plus fresh public verifier's bounded PostgreSQL joins |
| Exact ID/path lookup | Scoped reference repository, separate from ranking | Exact non-root path, descendant miss, selected version and UUID assertions |
| Current authorization | Existing exact-scope grant and disclosure policy | Foreign scope, grant changes, reference-only omission and bounded-content checks |
| Lifecycle exclusion | Current canonical lifecycle and latest reference source | Suppression, forgetting, deletion and unavailable reads without older resurrection |
| Restart durability | Same accepted event and canonical lineage after API/worker restart | Initial/restart PC5 identity hashes tied to image, source and OpenAPI digests |

## Public maintenance contract

Every request uses the standard `metadata` plus `input` envelope, the current
runtime binding/session headers, and exact tenant/project/namespace headers.
Before any reference write or read, all bound scope dimensions (tenant, project,
namespace, agent, session, optional conversation, and provider instance) must
contain only ASCII letters, digits, `_`, `.`, `:`, or `-`, with at most 128 bytes
per dimension. Required dimensions must be nonempty. A binding outside these
reference-specific limits returns HTTP 400 with category `validation`, code
`invalid_reference_scope`, and `retryable=false`, before ingestion or lookup.
The wider limits of existing non-reference runtime operations remain unchanged;
use a binding with compatible identities when enabling this capability.
Use canonical snake_case only. The write input carries `contract_version`,
`reference_id`, `memory_path`, `event_type`, `value`, and optional
`source_metadata`. Supported event types are `provider.reference.task`,
`provider.reference.run`, `provider.reference.handoff`, and
`provider.reference.compilation`. Values use UTF-8 byte limits (4096), IDs use
128 bytes, paths use 512 bytes, and responses use 16384 bytes. Empty, null,
unknown, malformed, unsupported, or oversized values fail validation rather
than being rewritten or silently truncated.

Normalized operation metadata must encode to at most 4096 JSON bytes. A value
must also fit the complete encoded completed response within 16384 bytes after
reserving space for the largest legal scope, metadata, and lineage. JSON
escaping counts toward that bound: escaping-heavy values can fail with HTTP
400 before admission even when their UTF-8 size is below 4096 bytes. Plain
ASCII values may use all 4096 bytes. This admission check preserves the ability
to return accepted values after governance and restart within the response
allocation.

Writes return HTTP 202, a UUID event ID, and a replay indication. Poll status
with that event ID or look up an exact ID/path through the two dedicated POST
read routes. `accepted` means durable admission; `pending` means governance is
incomplete; `failed` means the pipeline failed; `completed` requires committed
promotion lineage; `unavailable` is indistinguishable for missing, foreign, or
currently hidden evidence. Completion includes event/candidate/memory/version/
provenance UUIDs and the canonical version number. `retry_after_ms` is bounded
to 30000 and discovery recommends 1000 ms. Error categories remain bounded
`retryable` or `dependency`; HTTP validation, scope, conflict and authorization
errors follow the Provider error contract without internal SQL or raw errors.

Equivalent writes using the same scoped idempotency key return the original
event identity, including after later writes and restart; changed semantic input
or attribution conflicts. `metadata.event_seq` is validated and echoed as
compatibility attribution. It does not order reference writes or reject an
otherwise valid durable replay, and separate idempotency keys can share a
sequence value. Latest-source lookup orders durable records by `created_at`
and then `id`, both descending. Reference writes do not reserve sequence values
in the ordinary Provider event-ingestion tracker.

Every read reauthorizes the current binding and grant. The default mode is
`reference_only`, which omits `value`; `bounded_content` requires current content
disclosure authority. Omitted reference write labels default to
`untrusted/restricted`; use explicit low/medium labels only when appropriate.
`attested_data` requires current source-attestation authority, including during
replay. Labels do not grant instruction or business authority. Hidden or removed
latest sources cannot resurrect an older record under the same selector.

## Upgrade, compatibility, and rollback

PC5 reuses existing scope/path indexes and needs no migration. Verify clean
installation and upgrade from the prior schema, then deploy compatible API and
worker versions before enabling strict consumers. Keep PC1–PC4 consumers unchanged;
the base `provider-v1` does not establish PC5 support. A missing capability or
unsupported contract version is an explicit compatibility failure. Mixed
replicas must reject unsupported requests instead of downgrading into ordinary
events or ranked search.

Rollback disables PC5 for consumers and removes capability availability with
the older service version. Retain accepted events, versions, source metadata,
and provenance; do not erase durable history or reverse lifecycle operations.
Reference and content disclosure still depend on current grants after rollback.

## Public release evidence

Run `scripts/stele-provider-context-verify.ps1` against an owned, fresh
PostgreSQL/pgvector stack. It uses official bootstrap credentials, checks the
served OpenAPI against the source schema and capability digest, writes all four
supported event kinds through public HTTP, polls governance, checks exact lookup
and direct durable lineage, then restarts API/worker and repeats before lifecycle
removal. `-ReferenceReportPath` stores a separate bounded PC5 report; stage
copies retain `.initial.json` and `.restart.json`. The restart report must verify
unchanged lineage, suppressed/forgotten/deleted unavailable outcomes, and fresh
service provenance. It also writes a new independently governed source for the
same reference ID/path, checks exact lookup selects it, hides that latest source
without falling back to the older source, and confirms the earlier canonical
version and provenance remain available to privileged history inspection.
The lifecycle fixture uses the existing administrator actions `:suppress`,
`:expire` (which records `forgotten`), and `:delete`, with the required audit
actor and reason, and verifies the resulting states directly in PostgreSQL.
Each PC5 report pins the Provider schema and fixture versions, migration version
and manifest digest, and verifies every applied migration checksum against the
source SQL assets before claiming readiness.
Reports contain only categories, counts, booleans and hashes;
never credentials, DSNs, authorization headers, scope values, reference values,
or raw error bodies.

Missing Docker, bootstrap, PostgreSQL, pgvector, or restart evidence remains a
nonpassing verifier result. Focused unit tests and HTTP acceptance alone cannot
establish fresh-stack readiness. Preserve that distinction in roadmap and release
decisions until the live evidence passes.
