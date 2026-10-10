# Agent runtime memory provider

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

## Operation metadata and citations

Provider operations use bounded `request_id`, `operation_id`,
`idempotency_key`, `event_seq`, and `schema_version` metadata. Equivalent
durable retries return the original result. Reusing a key for a different
normalized payload returns `conflict`; duplicate or stale sequence values never
replay side effects. Ingest, intents, retrieval, context, and lifecycle requests
delegate to the existing governed service contracts.

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

`POST /v1/provider/context` uses the dedicated `ProviderContextRequest` and
`ProviderContextResponse` schemas in the served `/openapi.yaml`. Send the
credential in `X-API-Key`, the server-issued `X-Stele-Runtime-Binding` and
`X-Stele-Runtime-Session`, and the exact `X-Stele-Tenant`, `X-Stele-Project`,
and `X-Stele-Namespace` headers. Scope and session are binding-derived;
they cannot be overridden in `input`.

Minimal request (ProviderContextRequest):

```json
{
  "metadata": {
    "request_id": "context-request-1",
    "operation_id": "context-operation-1",
    "schema_version": "provider-v1"
  },
  "input": {"query": "current task", "budget": 4}
}
```

Full request (ProviderContextRequest):

```json
{
  "metadata": {
    "request_id": "context-request-2",
    "operation_id": "context-operation-2",
    "schema_version": "provider-v1"
  },
  "input": {
    "query": "current task",
    "budget": 8,
    "path_prefix": "tasks/demo",
    "include_relations": true,
    "include_experience_insights": true,
    "include_goal_context": true,
    "include_diagnostics": true,
    "include_feedback_diagnostics": true,
    "feedback_aware_ranking": true,
    "feedback_ranking_policy": ""
  }
}
```

`query` must be nonblank and `budget` a positive integer. Boolean options
default to false. Use either exact `path` or explicit `path_prefix`; shared
path validation normalizes leading/trailing slashes and rejects conflicting
selectors. A nonempty `feedback_ranking_policy` is always rejected. Optional
insights, goals, and diagnostics retain their existing authorization, lifecycle,
and governed policy gates; setting a flag does not grant access.

The result is categorized context. Ordinary section items contain `memory`
and `citations`, preserving selected order, identity, path, class, active state,
content, timestamps, and temporal metadata. They omit ranking scores.
`known_failures`, `experience_lessons`, `goal_context`, and `diagnostics` are
optional and may be omitted when empty. Insights expose only documented
identity, summary, confidence, lesson, and timestamp fields; diagnostics expose
section/status/reason and aggregate counts. Raw feedback, insight payloads,
derivation metadata, planner state, candidate pools, and query text are absent.

A genuine empty response (ProviderContextResponse):

```json
{
  "metadata": {
    "request_id": "context-request-1",
    "operation_id": "context-operation-1",
    "schema_version": "provider-v1"
  },
  "result": {
    "profile": [],
    "recent_session": [],
    "recent_episodes": [],
    "relevant_summaries": [],
    "related_entities": [],
    "citations": []
  },
  "citations": []
}
```

Validate the HTTP response against the dedicated response schema before using
it. Missing `result` or required sections, null/scalar sections, invalid item
or citation types, and a substituted `references/content` shape are
`failed-contract`, never an empty success. Preserve inner memory citations
(`memory_id/raw_event_id/operation`) as selected evidence. The aggregate
`result.citations` is deduplicated from the returned memory items in categorized
order; it excludes citations belonging only to candidates omitted by the item
budget or section selection. Outer Provider
citations (`source_kind/reference/availability` plus optional version/watermark)
are a configured bounded summary and may be shorter. A reference grants no
permission for another read; missing version/watermark values are not invented.

### Compatibility and maintenance notes

This is a documented `provider-v1` repair, with no database migration or alternate
legacy route. It deliberately rejects accidental aliases and removes accidental
raw ranking output. Deploy a consistent repaired revision across API replicas.
Capabilities now publish `schema_digest=sha256:<hex>` calculated from the same
served OpenAPI bytes; that digest identifies the API document, not selected
context or the PostgreSQL migration version.

| Consumer behavior | Repaired behavior | Migration |
| --- | --- | --- |
| Documented snake_case options | Accepted and explicitly mapped | Use the request examples above. |
| Go-name or case aliases, duplicate keys, unknown fields, null/missing required objects | HTTP 400 validation | Use exact canonical property names and typed values. |
| Caller scope/session/user/role/projection overrides | HTTP 400 validation | Retain binding-owned attribution; business permissions stay in the runtime. |
| Nonempty `feedback_ranking_policy`, blank query, nonpositive budget, conflicting paths | HTTP 400 validation | Correct the request; do not retry a policy override. |
| Unsupported schema version | HTTP 400 compatibility before assembly | Use an advertised version. |
| Foreign binding/session/scope | Authentication or scope denial before assembly | Reinitialize the permitted binding; never widen scope. |
| Reading ordinary `score` or arbitrary insight/diagnostic fields | Fields omitted | Consume documented content/citations and allowlisted optional fields. |
| Using aggregate citations to discover unselected candidates | Only returned items contribute aggregate evidence | Read candidate-independent evidence from the selected items; citations do not extend the item budget. |
| Expecting `references/content` or treating malformed results as empty | Incompatible contract | Validate categorized results and map authorized items without reranking. |

PC2 disclosure profiles, reference-only responses, and source-trust metadata
remain pending. PC3 remains pending: `budget` counts items and retains the
existing clamp against `MaxContextBytes`; this does not enforce the byte size
of the complete serialized response. PC4 Stele-issued context digest and
continuity remain pending. This repair does not establish PC5 reference or PC6
turn/outcome contracts. The runtime owns business permissions and final prompt
construction; Stele owns existing retrieval and selection rules.

### Public live verification

Build a repaired image, then run the owned fresh PostgreSQL/pgvector verifier:

```powershell
docker build --build-arg STELE_GOPROXY=https://goproxy.cn,direct -t stele-pc1-verify:local .
pwsh -NoProfile -File scripts/stele-provider-context-verify.ps1 -ReportPath .tmp/provider-context-evidence.json
```

The script uses official bootstrap, normal `conversation.message` ingestion,
bounded public governance-completion checks, exact and prefix paths, a genuine
empty read, scope denial, lifecycle exclusion, event replay, and API/worker
restart with the original binding. Its retained report contains only bounded
check names, completion counts, and build/schema/scope/identity/provenance
hashes. The source revision plus source digest records the verifier's host
worktree, including uncommitted source changes; the image digest pins the actual
tested image. Build the image from that worktree before running the verifier.
The report checks the served OpenAPI against the worktree but does not independently
prove source-to-image linkage through embedded build metadata. It removes its
owned stack, volume, and ephemeral credentials.
Missing dependencies or incomplete verification produce nonpassing evidence.
Unit tests skip this fixture when no live environment is configured; that skip
is not readiness evidence. The verification reader's size bound is a fixture
safety limit, not PC3 runtime response-budget enforcement.

The product verifier can run this same context gate on its owned stack by
setting `STELE_PRODUCT_VERIFY_PROVIDER_CONTEXT=1`; it writes a separate
`<ReportPath>.context.json`. An adapter-only conformance result proves no public
HTTP shape. Context conformance requires HTTP and response-schema evidence plus
the OpenAPI digest; contract outcomes remain separate from retrieval quality.

## Snapshot and reconnect synchronization

After the runtime binding handshake, request an initial bounded snapshot through
the provider boundary:

```bash
curl -sS "$STELE_URL/v1/provider/sync" \
  -H "X-API-Key: $STELE_RUNTIME_API_KEY" \
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

To roll back, set `STELE_PROVIDER_ENABLED=false` and restart API replicas. The
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
