# Provider evidence projections maintenance

PC6 adds the independently negotiated `turn-outcome-feedback-projection-v1`
capability. PostgreSQL remains the only system of record. These operations
record runtime-reported evidence; Stele does not complete Danny tasks, release
leases, commit generations, publish artifacts, or acknowledge delivery.

## Admission and inspection

Require `projections.enabled=true`, the exact contract version and supported
operations from `GET /v1/provider/capabilities`. The base `provider-v1`, PC4
context receipts and PC5 durable references do not imply PC6 availability.
Strict consumers fail closed without this capability and validate the served
OpenAPI; generic turn/outcome routes are retained for compatibility and are
never a substitute for projection admission.

Both operations require an API key, exact tenant/project/namespace headers,
`X-Stele-Runtime-Binding` and `X-Stele-Runtime-Session`. Attribution derives from
the current full binding, including principal, agent, conversation and Provider
instance. Use the same ASCII scope limits as PC5 (128 bytes per dimension).
Admission requires a current read-write grant whose default disclosure profile
permits reference-only responses; both `memory-reference-v1` and
`memory-content-v1` support this mode. History permits a read-only grant with
the same disclosure constraint. Missing/foreign bindings and
scope changes fail before repository dispatch.

`POST /v1/provider/projections` returns HTTP 202. Its closed JSON envelope is:

```json
{
  "metadata": {
    "request_id": "report-1",
    "operation_id": "report-1",
    "schema_version": "provider-v1",
    "idempotency_key": "effect-42-revision-1"
  },
  "input": {
    "contract_version": "turn-outcome-feedback-projection-v1",
    "kind": "outcome",
    "effect_id": "effect-42",
    "source_revision": 1,
    "occurred_at": "2026-10-16T00:00:00Z",
    "reported_outcome": "succeeded"
  }
}
```

Kinds are `turn`, `outcome` and `feedback`; reported outcomes are `succeeded`,
`failed`, `cancelled` and `unknown`. The request may include existing
`session_run_id` and `turn_id`, `context_digest` and `context_reference`,
`references` containing PC5 `event_id` selectors, `provenance` tokens,
`feedback`. Read the authoritative OpenAPI for the closed enums. A supplied
turn ID requires its session run; they must match the bound session and an
existing exact-scope turn. Reported outcomes provide bounded completion
context, never business-state authority. Arbitrary task/lease/commit/
publication/delivery fields, caller scope and grant overrides are rejected.

Requests allow 32768 encoded JSON bytes, canonical metadata 4096 bytes,
responses 65536 bytes, eight unique references and eight unique provenance
tokens. Effect/provenance tokens use ASCII letters, digits, `_`, `.`, `:` and
`-`, with at most 128 bytes. Source revisions are integers 1–2147483647;
occurrence times are UTC RFC3339 seconds. Feedback accepts `useful`,
`irrelevant` or `needs_review` with a required bounded `reason_code` (64
bytes), and requires at least one reference. Empty, null, duplicate, unknown,
aliased or malformed fields fail closed; nothing is silently truncated.
Omit unused optional strings instead of supplying an empty string. Metadata
must already be canonical: surrounding token whitespace and paths requiring
normalization are rejected by PC6. Legacy Provider metadata behavior is retained.
The zero timestamp `0001-01-01T00:00:00Z` is rejected. Encoded metadata and
Unicode path byte limits are enforced by the service and strict consumer in
addition to the OpenAPI schema's field constraints.

The response contains echoed metadata, contract version, exact scope and a
`record`: logical/projection UUIDs, source revision, optional `supersedes_id`,
`accepted|corrected` disposition, `evidence_kind=runtime_reported_evidence`,
reference snapshots, current reference states, context-digest attribution and
`feedback_state=none|pending`. No raw outcome payload, memory content, feedback
text or business-state receipt is returned.

## Replay, corrections and current visibility

One logical effect is owned by full binding, kind and effect ID. Reusing an
idempotency key with equivalent normalized input returns the original
projection identity and disposition, even after a later correction or restart.
Transport metadata is not semantic input; reference/provenance ordering is
normalized. Different input under that key conflicts. A greater source revision
appends a correction linked to the previous head; equal revisions conflict and
delayed lower revisions are stale. Replaying an older accepted delivery never
overwrites the current head or creates duplicate feedback.

`POST /v1/provider/projections/history` accepts the same metadata envelope plus
`input.contract_version`, `kind`, `effect_id` and required `limit` (1–8). It
returns newest-first `records` and `has_more`. This is a bounded inspection
window: `has_more=true` does not claim the whole history is present. There is
no content-bearing historical snapshot or broader administrative bypass in
these Provider operations.

`references` records admission attribution; `current_references` rechecks PC5
under current authorization and lifecycle. Accepted/pending/failed records
cannot claim feedback eligibility. Completed records carry canonical UUID
lineage and `eligible=true`. Missing, foreign, hidden, suppressed, forgotten
and deleted sources share `unavailable`, with no event ID or lineage. Reads
also redact formerly visible admission identity when its current reference
is unavailable. A bare context digest is `reported`; only an unexpired PC4
receipt belonging to the full binding with a matching digest is
`verified_receipt`. Neither status confers task completion or content access.
When that receipt is turn-bound (`result.turn_id` was present on the Provider
context read), the projection must supply exactly the same `turn_id`; an
omitted or different value is unavailable rather than a verified receipt.
This check adds correlation only. It does not grant a role, widen retrieval,
or expose a context snapshot, hidden candidate, or ranking detail.

## Asynchronous feedback policy

Enable `STELE_PROVIDER_ENABLED=true` in the scheduler as well as API replicas
to run the bounded exact-scope `provider_projection_feedback` job. It scans at
most 100 logical effects per scope per dispatch and records policy
`projection-feedback-v1`. A projection write leaves feedback `pending`; this
field describes admission, not a promise of processing or ranking activation.
All referenced evidence must currently be completed and eligible before the
job maps the newest revision into the existing usefulness-feedback log with
immutable projection/source/version links. Pending, failed and unavailable
evidence is retained without active feedback influence.

Scope discovery reads the projection ledger, including scopes with only pending
projections and no canonical memory. It prioritizes unchecked scopes, then the
oldest last-service checkpoint, while retaining the configured scope batch limit
and the 100-effect per-scope limit. Persisted checkpoints preserve rotation across
repository restarts; an unchecked backlog in one scope does not monopolize batches.

Active feedback consumers recheck current binding/grant, source metadata,
canonical lifecycle and exact lineage through `eligible_usefulness_feedback`.
Corrections and revocations exclude old evidence before the next job scan.
The job adds ordinary feedback supersession audit records; restoring visibility
does not reactivate a superseded revision. A later valid source revision can
provide new evidence. Existing summary, ranking-policy and insight-activation
gates remain required; admission and mapping do not activate them.

## Errors and operation

| HTTP | Category / code | Client action |
| --- | --- | --- |
| 400 | `validation` / `invalid_request`, `invalid_scope` or `invalid_projection_scope` | Correct the closed input or incompatible binding |
| 403 | `authentication` / `unauthorized`; `scope` / `scope_denied`, `runtime_binding_denied` or `disclosure_denied` | Resolve current authority; do not broaden scope |
| 409 | `compatibility` / `unsupported_projection_contract` or `unsupported_schema` | Negotiate a supported version; no generic fallback |
| 409 | `conflict` / `projection_conflict` | Preserve the original key; submit a genuine correction with a greater revision and new key |
| 409 | `stale` / `projection_stale` | Inspect the current history; do not overwrite newer evidence |
| 410 | `lifecycle` / `projection_unavailable` | Treat missing/foreign/expired evidence identically |
| 413 | `validation` / `projection_limit_exceeded` | Reduce the request before admission |
| 503 | `dependency` / `projection_dependency_unavailable` | Retry equivalent delivery with the same key |

Both PC6 routes return the JSON `ProviderProjectionErrorResponse` envelope for principal,
scope and runtime-binding denials as well as operation errors. Disclosure or
authorization dependencies may also return `disclosure_unavailable` or
`authorization_unavailable` with category `dependency` and HTTP 503. Responses
never include underlying storage errors or distinguish missing from foreign
bindings. Existing routes retain their established authentication envelopes.
The envelope and nested error object are closed. All four error fields
(`category`, `code`, `message`, `retryable`) are required; messages are nonempty
and limited to 256 UTF-8 bytes, and the encoded envelope is limited to 4096
bytes. Strict consumers validate the status/category/code combination and
accept `retryable=true` only for dependency errors.

Migration 0037 is additive: immutable projection identities/revisions and
feedback source links, mutable scheduling checkpoints, scope constraints and
the current-eligibility view. Apply official migrations and verify the checksum
ledger before enabling compatible API/scheduler replicas. No dependency, SDK,
UI, queue or second business-state database is added. Rollback disables PC6
routes/capability and consumers while retaining projection, feedback and audit
history; the down migration deliberately refuses destructive removal.

Fresh-stack readiness requires the official Provider verifier on an owned
PostgreSQL + pgvector stack, serialized public requests against the served
OpenAPI, exact-scope denial, replay/conflict/correction, PC4/PC5 attribution,
lifecycle exclusion and API/worker restart. Reports contain bounded counts,
categories, booleans and provenance hashes, never credentials, DSNs, scope
values, reference values or raw bodies. Skipped tests and HTTP acceptance alone
do not establish PC6 readiness.

Run the official verifier against an image built from the source being checked:

```powershell
pwsh -NoProfile -File scripts/stele-provider-context-verify.ps1 `
  -Image stele-pc6-verify:local `
  -ProjectionReportPath .tmp/pc6/projection.json
```

The default owned mode creates a fresh stack and official credentials, stops
worker and scheduler to observe public pending attribution, restarts the API,
resumes ordinary consumers, and repeats the settled checks after API/worker
restart. It removes only its owned stack, volume and temporary credentials.
The PC6 report and its `.pending.json`, `.initial.json` and `.restart.json`
snapshots are separate from PC4/PC5 reports. A failed or incomplete phase writes
a nonpassing report; another contract's passing report cannot establish PC6
readiness. `-UseExistingStack` requires explicit owned-stack credentials and an
observation DSN and does not establish fresh-stack provenance by itself.

## Correction-to-version handoff

The separately negotiated `corrections` capability is available only when the
service has both a correction resolver and an enabled memory-intent policy.
The capability document advertises the `corrections` operation token only
when both are ready; its nested `operation` field names the exact route. A
strict runtime validates the whole document and requires the enabled contract,
operation token and version before calling
`provider-correction-governed-version-v1` at
`POST /v1/provider/corrections`; it does not reuse generic intent evidence as
proof of correction ownership. The closed request names a corrected
`projection_id`, target memory UUID and `expected_version`, plus bounded content
and reason. Exact binding, projection-head ownership, current PC5 reference
eligibility and target lineage are resolved server-side before intent
idempotency or queue admission.

The public acknowledgement exposes only the governed intent UUID, status and a
fixed verified-attribution state. It never returns source payload, event IDs,
candidate IDs, ranking details or target lineage. Missing, foreign, first,
hidden and target-mismatched sources collapse to HTTP 410
`lifecycle/correction_unavailable`; stale source/target is HTTP 409
`stale/correction_stale`; key collision is HTTP 409
`conflict/correction_conflict`. Authorization or storage dependencies return
retryable HTTP 503. Compatible clients replay an equivalent request with the
same key and do not switch to generic intents, direct SQL, alternate scope, or
local mutation on denial.

Accepted intents remain asynchronous. The worker revalidates source and target,
then uses the existing candidate/promotion path to append a successor version
and immutable provenance. It does not overwrite the previous version or
complete the external task. Riverqueue v0.44.1 is used only for memory-intent
execution: job arguments carry the intent UUID, admission inserts the River job
in the intent transaction, and governance writes, intent transition, and
`JobCompleteTx` commit in one transaction. A failed attempt rolls back all
three and remains retryable. Other derived-work kinds continue using the
existing queue. The dedicated `stele_river` schema is migrated with the normal
PostgreSQL migration policy; validate/off startup requires the schema to be at
the pinned River migration version. Legacy recoverable memory-intent queue rows
are transactionally handed off once, while terminal historical rows are not
replayed.

Migration `0042` stores immutable correction attribution and `0043` records the
River handoff. Rollback disables the capability and worker route, but retains
both migration history and accepted data; do not drop the River schema or
requeue completed history as an operational rollback shortcut. Full readiness
still requires owned fresh-stack evidence for correction promotion,
supersession, restart, retry and handoff; successful admission alone is not a
release gate.

Run `pwsh -File scripts/stele-provider-correction-verify.ps1` to execute the
owned PostgreSQL 18 + pgvector correction, River retry/handoff, projection
supersession and API/worker restart fixtures. The script starts and removes its
own temporary container and writes only bounded suite status and image digest
to its report; it does not retain DSNs or request data.
