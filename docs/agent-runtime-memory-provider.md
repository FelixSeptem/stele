# Agent runtime memory provider

**Role:** reference for public agent runtime integration. Start at the
[documentation portal](README.md) for the integration overview. A Provider
denial does not authorize MCP, native-read, alternate-credential, local-memory,
independent-ranking, or context-reconstruction fallback.

Stele exposes an opt-in, OpenAPI-backed memory-provider contract for external
agent runtimes. The provider is a service adapter: it does not execute agents,
build prompts, call models, or maintain a second memory store.

## Enablement and discovery

Set `STELE_PROVIDER_ENABLED=true` in all API replicas after migrations have
reached the schema version reported by `GET /version`. The default is `false`.
Configure supported contract versions with
`STELE_PROVIDER_SCHEMA_VERSIONS=provider-v1`; request, result, citation, and
metadata limits use the `STELE_PROVIDER_MAX_*` variables shown in
`.env.local.example`.

Clients read the bounded capability document before initializing a runtime.
It publishes supported operation names, exact scope dimensions, versions,
limits, and an OpenAPI digest. It never publishes credentials, DSNs, principal
records, scope values, backlog data, provider payloads, or internal errors.

The capability document also advertises the transport-neutral synchronization
contract. The first enabled transport is `openapi_pull`; WebSocket and SSE are
reserved for future adapters and are not enabled by this release. The sync
contract publishes event kinds, maximum batch and snapshot sizes, cursor size,
and the replay retention window.

## Runtime scope handshake

Runtime initialization requires an active durable principal and one exact
`tenant/project/namespace` grant. The request separates `agent_id`,
`session_id`, and optional `conversation_id`. Stele returns an opaque runtime
binding plus a server-generated provider-instance identity. Every later
operation carries that binding and session identity; the service revalidates
the binding, expiry, principal, and exact grant before repository access.
Changing scope headers or an identity returns a generic denial and never
discloses whether foreign records exist.

## Durable reference contract (PC5)

When a runtime needs a durable task, run, handoff, or compilation reference,
discover the `durable_references` capability and require
`contract_version=durable-reference-v1` before sending a request. The contract
is additive to `provider-v1`; a strict client fails closed when the capability
or version is absent and never substitutes ranked search. The supported event
types are `provider.reference.task`, `provider.reference.run`,
`provider.reference.handoff`, and `provider.reference.compilation`.

Use `POST /v1/provider/references` with the standard metadata envelope and
`reference_id` (UTF-8, at most 128 bytes), normalized `memory_path` (at most
512 bytes), one supported `event_type`, and a value at most 4096 UTF-8 bytes.
Normalized operation metadata must encode to at most 4096 JSON bytes. The
value must also fit the complete 16384-byte completed response, including JSON
escaping and a reserve for the largest legal scope, metadata, and lineage.
Escaping-heavy values can therefore return HTTP 400 before admission even
below the UTF-8 ceiling; plain ASCII values may use the full 4096-byte limit.
The response is HTTP 202 and reports `accepted` or `pending`; an event ID and
`replayed` flag identify idempotent admission. Acceptance is not governance
completion. Poll `POST /v1/provider/references/status` with the event ID, or
resolve an exact `reference_id` and path with
`POST /v1/provider/references/lookup`. A `completed` response contains the
event, candidate, memory, version, and provenance UUIDs. `retry_after_ms` is
bounded to 30000; retryable failures use only `retryable` or `dependency`
categories.

`reference_only` omits the value. `bounded_content` requires the current
content disclosure grant. Omitted source metadata defaults to
`untrusted/restricted`; an
`attested_data` source is accepted only with a current exact-scope attestation
grant. Status completion requires raw-event processing, candidate promotion,
an active current canonical version, and promotion provenance. Missing,
foreign, hidden, suppressed, forgotten, or deleted references return
`unavailable` without identity or value leakage. Exact lookup chooses the most
recent source for the requested ID and path, then checks its current visibility
and lifecycle; it never resurrects an older version when that source is hidden.

The capability advertises `max_response_bytes=16384` and `poll_after_ms=1000`.
Reference routes require all bound scope dimensions to use ASCII letters,
digits, `_`, `.`, `:`, or `-` and at most 128 bytes each; conversation may be
empty. An incompatible binding receives HTTP 400 `validation` with code
`invalid_reference_scope` before write/read dispatch. Existing non-reference
runtime identity limits are unchanged.
Reference writes do not own task state, scheduling, compilation commits,
artifact publication, or webhook delivery. PC5 reuses existing scope/path
indexes and needs no migration or new dependency. Mixed-version rollout keeps PC1–PC4 routes
unchanged; disable the capability to roll back while retaining accepted events,
provenance, and lifecycle history.

The [reference maintenance guide](provider-durable-references.md) records the
reuse baseline, requirement-to-test matrix, rollout and evidence requirements.

## Evidence projection contract (PC6)

Negotiate `projections` at `turn-outcome-feedback-projection-v1` independently
from `provider-v1`, context and durable references. Dedicated
`POST /v1/provider/projections` (202) and
`POST /v1/provider/projections/history` (200) use closed metadata/input envelopes,
the full current runtime binding and exact-scope disclosure. Admission requires
read-write authority; history allows read-only reference disclosure. Requests
are bounded to 32768 JSON bytes, responses to 65536 bytes, references/provenance
to eight each and history to eight records.

One logical effect has append-only source revisions and stable idempotent
delivery. Greater revisions correct prior evidence; conflicting key reuse and
stale revisions fail closed. Records label `runtime_reported_evidence` and
separate admission reference snapshots from current reauthorized attribution.
Only a completed eligible PC5 reference can feed the asynchronous versioned
feedback mapper; writes return `none|pending`, never ranking/insight activation.
PC4 receipt verification requires the same binding and matching digest.

The [projection maintenance guide](provider-evidence-projections.md) supplies
the full request/response shapes, error categories, correction/history example,
grant/lifecycle behavior, scheduler enablement, migration 0037, rollback and
fresh-stack evidence requirements. Generic turns/outcomes remain supported;
strict PC6 consumers never use them as a fallback. Danny task, lease, commit,
publication and delivery state stay with the runtime.

## Operation metadata and citations

Provider operations use bounded `request_id`, `operation_id`,
`idempotency_key`, `event_seq`, and `schema_version` metadata. Equivalent
durable retries return the original result. Reusing a key for a different
normalized payload returns `conflict`; duplicate or stale sequence values never
replay side effects. Ingest, intents, retrieval, context, and lifecycle requests
delegate to the existing governed service contracts.

For durable references, `event_seq` is echoed compatibility attribution rather
than an ordering or replay gate. Durable idempotency keys preserve equivalent
retry identities after later writes and restart; distinct keys may share a
sequence value. Reference writes do not advance the ordinary ingestion sequence
tracker. Exact lookup chooses the latest durable source by descending
`created_at`, then `id`.

Visible results may include bounded citations with source kind, stable
reference, version or projection watermark, and availability. Ordinary
provider responses omit raw queries, hidden candidates, scores, ranking plans,
provider payloads, credentials, and foreign identifiers. Suppressed, forgotten,
deleted, stale, or out-of-scope evidence is excluded by default.

Stable provider error categories are `authentication`, `scope`,
`compatibility`, `validation`, `conflict`, `lifecycle`, `stale`, `dependency`,
`retryable`, and `resync_required`. Unsupported schema versions fail before
operation dispatch. A `resync_required` result is fail-closed: the runtime must
start a fresh snapshot instead of assuming that skipped events were applied.

## Provider context JSON contract

`POST /v1/provider/context` uses the served `ProviderContextRequest` and
`ProviderContextResponse` OpenAPI schemas. Discover `context-disclosure-v1` and
`source-metadata-v1`, `context_budget_version=context-budget-v1`,
`context_packing_policy_version=context-packing-v1` and
`token_budget_supported=false`; compare `schema_digest` with `/openapi.yaml`
before use. The base `provider-v1` alone does not establish budget compatibility.
The capability document also advertises `context_digest`
(`context-digest-v1`, SHA-256, `context-canonical-v1`, and
`context-selection-v1`) and `context_continuity` (`context-continuity-v1`). A
client that relies on either extension must require those exact versions rather
than treating a missing capability as compatible older behavior.
Send `X-API-Key`, server-issued `X-Stele-Runtime-Binding` and
`X-Stele-Runtime-Session`, and exact tenant/project/namespace headers. Scope and
session are binding-owned. The current exact-scope grant must be explicitly
provisioned by an administrator; existing grants are unconfigured and denied.

Minimal request (ProviderContextRequest):

```json
{
  "metadata": {"request_id":"context-request-1","operation_id":"context-operation-1","schema_version":"provider-v1"},
  "input": {"query":"current task","budget":4}
}
```

The defaults are `projection_intent=conversation`, the grant's configured
`disclosure_profile`, and `response_mode=reference_only`. The reference profile
permits only references; the content profile permits references or explicitly
requested bounded content. Both disclose only active low/medium sources.

### Projection digest and fresh continuity

Every successful context response has a closed `result.context_digest` and
`result.continuity` object, including an empty projection. The digest is a
domain-separated SHA-256 over the canonical authorized projection: exact scope,
mode, projection intent, disclosure policy, effective budget, active
selection-policy fingerprint, section order, returned source versions/path/
metadata, and citations. It is not a credential, cache validator, snapshot,
watermark, business-state receipt, or permission to dereference a memory outside
the normal Provider contract.

Omit `continuity` for an ordinary fresh read. That returns
`{"read_semantics":"fresh","outcome":"not_requested"}` and performs no
receipt I/O. To retain a short-lived metadata-only comparison point, request an
issue:

```json
{
  "metadata": {"request_id":"context-request-issue","operation_id":"context-operation-issue","schema_version":"provider-v1"},
  "input": {"query":"current task","budget":4,"continuity":{"mode":"issue"}}
}
```

The response is still a complete fresh context body and has
`{"read_semantics":"fresh","outcome":"issued","reference":"ctxc_<32 lowercase hex>","expires_at":"<UTC RFC3339 seconds>"}`.
Keep the opaque reference out of logs, prompts, diagnostics and source control;
it is not a credential, but it is correlation-sensitive runtime metadata.

To compare a later equivalent read, use the same runtime binding, session,
scope, query, selector, response mode, profile, intent, effective allocation
and relevant context options:

```json
{
  "metadata": {"request_id":"context-request-compare","operation_id":"context-operation-compare","schema_version":"provider-v1"},
  "input": {
    "query":"current task", "budget":4,
    "continuity":{"mode":"compare","reference":"ctxc_0123456789abcdef0123456789abcdef"}
  }
}
```

Comparison returns a complete current body with `read_semantics=fresh_compare`
and `outcome=unchanged` or `changed`; it never returns 304, a historical body,
an exact snapshot replay, a watermark, or an explanation/count for changed
hidden evidence. A foreign, unknown or expired reference returns a bounded 409
continuity refusal; a changed originating request returns 409
`continuity_request_changed`; an incompatible stored contract returns 409
`continuity_incompatible`. Current grant and disclosure checks happen before
receipt lookup. Storage/capacity dependency failures are bounded retryable 503
`provider_unavailable`. A client must consume neither partial output nor a
locally reconstructed result on any of these failures.

Receipts contain only binding identity, request fingerprint, digest identity,
issue/expiry timestamps and opaque reference. PostgreSQL supplies the time and
enforces TTL/capacity; query text, context content, selected IDs, citations and
serialized response bodies are never persisted in the receipt. The definitive
canonical-byte fixtures (empty/nonempty reference and bounded-content cases,
including Unicode escaping) live in
[`internal/provider/testdata/context-digest-v1`](../internal/provider/testdata/context-digest-v1/).
Independent consumers must recompute and verify them; they must not calculate a
second retrieval ranking, eligibility, lifecycle or packing policy.

Full request (ProviderContextRequest):

```json
{
  "metadata": {"request_id":"context-request-2","operation_id":"context-operation-2","schema_version":"provider-v1"},
  "input": {
    "query":"current task","budget":8,"path_prefix":"tasks/demo",
    "budget_limits":{"max_content_bytes":4096,"max_response_bytes":16384,"max_citations":8},
    "projection_intent":"task_context","disclosure_profile":"memory-content-v1",
    "response_mode":"bounded_content","include_relations":true
  }
}
```

`query` must be nonblank and `budget` a positive item-placement count through
2,147,483,647. Optional closed `budget_limits` contains only
`max_content_bytes`, `max_response_bytes`, and `max_citations`, each an integer
through 2,147,483,647. Response bytes require at least one; content bytes and
citations permit zero. Omitted objects/fields and an empty object use published
server ceilings; explicit zero survives independent caller/server clamping.
Zero content does not switch content mode to references: only whole items with
actually empty content can fit. Zero citations returns no memory item because
every public item requires a canonical citation. Token fields (`max_tokens`,
`tokenizer`) are unknown and fail validation; bytes are not token estimates.
Use exact `path` or
explicit `path_prefix`; conflicting selectors are invalid. Canonical spellings,
closed objects and typed values are required: aliases, duplicate keys, nulls,
unknown fields and trailing JSON fail with bounded 400. Unknown or unauthorized
profiles and known but unauthorized modes return generic 403. Missing authority
dependencies return 503. A business role or projection intent cannot grant access.

Reference items contain `reference` and `citations`; content items contain
`memory` and `citations`. Both use kind, ID, version, path, class, availability and
effective source metadata. Only content mode includes content. High/restricted,
hidden, foreign and unverifiable sources disclose no identity or path. Optional
insight, goal, diagnostic and cached derivation sections remain omitted because
their evidence is not independently authorized by this public contract.

A genuine empty response (ProviderContextResponse):

```json
{
  "metadata": {"request_id":"context-request-1","operation_id":"context-operation-1","schema_version":"provider-v1"},
  "result": {
    "context_digest": {
      "contract_version":"context-digest-v1","algorithm":"sha-256",
      "canonicalization_version":"context-canonical-v1","selection_policy_version":"context-selection-v1",
      "selection_policy_fingerprint":"55dcb1869c32b2b972fa0f628fa5a948457403cf9da803267a4297574c4dfac2",
      "value":"e150aeaab65c4c28303cd9b5f215770e3a374f8127fb1f11b5613aafe5e36a78"
    },
    "continuity": {"contract_version":"context-continuity-v1","read_semantics":"fresh","outcome":"not_requested"},
    "response_mode":"reference_only",
    "disclosure": {
      "profile":"memory-reference-v1","policy_version":"context-disclosure-v1",
      "projection_intent":"conversation","source_metadata_version":"source-metadata-v1"
    },
    "budget_accounting": {
      "contract_version":"context-budget-v1","packing_policy_version":"context-packing-v1",
      "requested":{"max_items":4,"max_content_bytes":null,"max_response_bytes":null,"max_citations":null},
      "effective":{"max_items":4,"max_content_bytes":1048576,"max_response_bytes":1048576,"max_citations":100},
      "usage":{"items":0,"content_bytes":0,"response_bytes":1387,"citations":0},
      "selection":{"outcome":"empty","omission_reasons":[],"unsupported_sections":[]}
    },
    "profile":[],"recent_session":[],"recent_episodes":[],
    "relevant_summaries":[],"related_entities":[],"citations":[]
  },
  "citations":[]
}
```

The example is pretty-printed for reading; its usage describes its compact JSON
form, without a terminal newline. Actual usage always describes the exact body
sent by the route, rather than a reformatted copy.

Validate the mode-discriminated response before use. Missing `result`, missing or
null required arrays, mixed item modes, unknown fields or substituted shapes are
`failed-contract`, never empty success. Item and aggregate citations can name only
returned canonical IDs and versions. Raw-event/provenance evidence is omitted.
References confer no permission for a subsequent read. No query, stored application
metadata, hidden counts, ranking scores or arbitrary policy attribution is echoed.

`budget_accounting` is required and closed in both modes, independently of
`include_diagnostics`. Its requested object repeats `budget` as `max_items`;
response-only nulls identify omitted optional limits, while explicit zeros remain
zeros. Effective values are independent minima of caller and server ceilings.
Usage counts returned item appearances across categories, unescaped UTF-8 bytes
of every content string, unique canonical `(kind,id,version)` citation identities,
and the exact uncompressed serialized JSON body. Paths, source metadata, JSON
escaping, metadata, all arrays, repeated inner/outer citations and accounting
itself consume response bytes; headers, compression and framing do not. Reference
mode consumes zero content bytes. Required citations stay with every whole item.

Selection preserves eligibility, current sources/versions, ranking, section
priority, summary preference, quotas and deduplication. An oversized item is
omitted whole; Stele continues through the existing bounded candidate stream,
without truncation, citation stripping, version replacement, wider lookup or a
second consumer packing/ranking pass. Deterministic admission reserves bounded
accounting space conservatively under `context-packing-v1`; unused space is
possible and maximal packing is not promised. Final `response_bytes` includes
its own digit width and equals the length of the exact emitted compact body.

`selection.outcome` is `selected` when any item is returned (even with omissions),
`empty` only when no disclosable candidate remained, or `budget_exhausted` when
eligible candidates existed but no whole item fit. Ordered unique
`omission_reasons` are `item_limit`, `content_bytes`, `citations`, `response_bytes`.
Only evaluated authorized candidates contribute; hidden, foreign, grant-,
quality- or lifecycle-rejected evidence contributes no reason, private count,
identity or text. Ordered unique `unsupported_sections` are
`experience_insights`, `goal_context`, `diagnostics`, `feedback_diagnostics`, only
when explicitly requested. Those fixed capability labels do not reopen public
insight/goal/projection/feedback/raw-diagnostic gates or reveal stored evidence.

Before retrieval, the mandatory empty envelope must fit the effective response
ceiling. Infeasible allocation returns 413 `context_limit_exceeded` with no result
or partial stream. Final serializer/accounting/ceiling disagreements fail closed.
Malformed limits use 400 `invalid_request`; current disclosure denial uses 403
`disclosure_denied`; unavailable authoritative dependencies use retryable 503,
never empty or budget-exhausted success. Error bodies have separate server bounds
and need not fit caller success allocations. Reducing only the item budget cannot
repair an envelope that itself exceeds the body allocation.

Consumers reserve their other prompt components before allocating memory space.
Require fixed budget/packing versions, correct mode, all categories, all closed
accounting fields, ordered enums, actual item/content/citation counts and exact
captured response-byte length; ensure every usage dimension fits its effective
limit and effective values match discovered ceilings and the request. Compare
reported requested limits with the originating request, preserving omitted/null
versus explicit zero, and require unsupported-section labels to match its section
flags. A self-consistent response for a different allocation is a contract failure. Missing
accounting, overflow, incompatible versions, impossible usage or over-limit bodies
are `failed-contract`, never empty results. Source data still grants no business,
tool, write or compiler authority. Tokenizers/token budgets remain unsupported.
Context digest and continuity are available under the versioned PC4 contract
above. PC5 uses the separate durable reference routes described earlier; PC6
outcome/feedback evidence uses the separately negotiated projection contract.

### Compatibility and maintenance notes

PC3 requires updating strict consumer schemas for mandatory accounting and
explicit budget/packing discovery. The independent server item ceiling (default
100) can reduce formerly oversized item requests; budget omissions may now yield
a bounded selection where PC2 refused a large body. There is no PC3 database
migration or new persistence store. PC1/PC2 archived evidence does not prove PC3
readiness; verify fresh initial and API/worker restart allocation/isolation/
lifecycle evidence before rollout. Before rollback to a service without PC3,
disable access for consumers requiring the extension while retaining PC2 grants,
source enforcement and durable data. Consumers must fail compatibility rather
than stripping limits, broadening disclosure, switching routes or packing locally.

This changes public defaults and requires migrations 0035 and 0036 plus explicit grant
provisioning. Old bindings are checked against current grants on every supported
request. Provider retrieve/sync and native/MCP data routes deny public credentials
before lookup. Administrative operations retain existing exact-scope checks.

Source metadata on event ingestion defaults to `untrusted/restricted`.
`attested_data` requires a separate explicitly provisioned entitlement and never
authorizes instructions, tools, writes or task completion. Label changes under the
same idempotency key conflict. All stored versions preserve conservative lineage.
See [disclosure/source guide](context-disclosure-and-source-trust.md) for the
endpoint matrix, provisioning, upgrade inventory and credential-safe rollback.

## Snapshot and reconnect synchronization

The archived synchronization contract below is available only to an explicitly
authorized administrator runtime. PC2 public credentials receive 403 before
lookup, including content-profile credentials. A public context denial does not
authorize switching to this route. After an administrator binding handshake,
request an initial bounded snapshot through the provider boundary:

```bash
curl -sS "$STELE_URL/v1/provider/sync" \
  -H "X-API-Key: $STELE_ADMIN_API_KEY" \
  -H "X-Stele-Runtime-Binding: $STELE_RUNTIME_BINDING" \
  -H "X-Stele-Runtime-Session: $STELE_RUNTIME_SESSION" \
  -H "Content-Type: application/json" \
  -d '{"schema_version":"sync-v1","max_events":100}'
```

The response contains a snapshot watermark, a bounded `next_cursor`, and a
`sync_complete` flag. Persist that cursor only after the runtime has accepted
the complete response. To reconnect, send the acknowledged cursor in the next
request:

```json
{
  "schema_version": "sync-v1",
  "cursor": "<last-acknowledged-cursor>",
  "max_events": 100
}
```

Events are ordered within the exact runtime scope and carry a sequence, replay
identity, source watermark, schema version, and bounded redacted payload or
reference. Repeating a cursor is deterministic and does not replay a write.
When the cursor is expired, malformed, incompatible, or outside the retained
window, the service returns HTTP 409 with `category=resync_required`. Discard
the cursor and repeat the initial snapshot flow. Synchronization is read/replay
only; event ingest, governed intents, and admin lifecycle actions remain the
only write paths.

## Conformance and rollback

Provider conformance uses deterministic fixtures in one explicitly owned exact
scope. It checks capability compatibility, scope denial, idempotent replay,
lifecycle filtering, citation completeness, dependency freshness, and
restart/fallback behavior. A skipped, incomplete, stale, or degraded run is not
provider-readiness evidence. Conformance is diagnostic and does not invoke an
external agent or mutate canonical memory except through ordinary governed
fixture ingestion.

The self-hosted product verifier extends this evidence to governed memory
intents. It submits through the public OpenAPI boundary, confirms the durable
PostgreSQL `memory_intent` work reference, replays and conflicts an idempotency
key, checks exact-scope inspection and foreign-scope denial, and verifies worker
and scheduler restart recovery. It also stops the worker, verifies that
`STELE_MEMORY_INTENT_POLICY_ENABLED=false` rejects new persistence while the
queued record remains inspectable, then re-enables the same
`STELE_MEMORY_INTENT_POLICY_VERSION` and confirms only the exact-scope pending
work resumes. The resulting report is redacted and uses only bounded phase,
recovery, rollback, cleanup, freshness, and consumability categories. A
missing rollback-policy fixture or other hard gate is recorded as degraded
evidence and never enables a provider or changes default retrieval.

To roll back PC2, first revoke or disable constrained public credentials across
native/MCP surfaces, set `STELE_PROVIDER_ENABLED=false`, and restart API replicas.
An older binary must never serve those credentials. The
existing public APIs, PostgreSQL canonical records, append-only history, and
provider conformance evidence remain intact. Do not down-migrate or delete
canonical memory as part of provider rollback.

Synchronization can also be disabled independently while provider operations
remain enabled: advertise `synchronization.enabled=false` (with no enabled
transport) and do not register the synchronization source. Synchronization
requests then fail closed with a bounded `provider_unavailable` response;
existing ingest, intent, retrieval, context, and lifecycle routes continue to
use their normal governed contracts. Re-enable synchronization only after the
advertised contract, retention window, and source projection are available.

Related workflow commands are `openspec validate --all`,
`go test ./internal/provider ./internal/app ./internal/assurance ./openapi`, and
the repository quality gate.

## Optional turn-bound context reads (STL-1)

Strict consumers may require the `context-turn-binding-v1` capability from
`GET /v1/provider/capabilities`, create a Provider turn, and send its optional
`turn_id` in the following context request:

```json
{"metadata":{"request_id":"ctx-1","operation_id":"ctx-1","schema_version":"provider-v1"},"input":{"query":"current task","budget":4,"turn_id":"turn_abc"}}
```

The ID is a nonempty ASCII token matching `^[A-Za-z0-9_.:-]+$` and at most 128
bytes. Stele verifies the immutable turn association against the complete
server-issued runtime binding, session, and exact tenant/project/namespace
scope before disclosure, continuity, or retrieval. Missing, foreign,
unassociated, wrong-session, and wrong-binding turns share one non-disclosing
denial. The field is attribution only: it cannot widen retrieval, select a
memory path, override disclosure, represent a business role, estimate tokens,
or replay a historical snapshot.

Turn-bound responses use `context-turn-digest-v1` and
`context-turn-canonical-v1`; the verified ID is included in their digest and
request fingerprint and echoed in `result.turn_id`. Turnless requests retain
the v1 digest bytes and receipts. Comparing a receipt across turns, or between
turn-bound and turnless requests, returns `continuity_request_changed`. PC6
receipt attribution is verified only when its `turn_id` matches. Migration
0040 adds immutable ownership and bounded attribution metadata; legacy turns
are never silently claimed, and audit records contain no query, content,
selected IDs, citations, hidden candidates, or ranking data.
