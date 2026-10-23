# Context disclosure and source trust

**Role:** security reference. Start at the
[documentation portal](README.md) for the architecture and integration map.

PC2 adds `context-disclosure-v1` and `source-metadata-v1` while retaining the
`provider-v1` envelope. It changes public access defaults. Consumers must discover
both extension versions and the served OpenAPI digest before using context.

## Provisioning

An active exact-scope grant needs an explicit disclosure configuration. Existing
grants have no configured profile and are denied; read/write access alone does not
grant disclosure. An administrator first creates the principal and scope grant,
then calls `PUT /v1/admin/principals/{principal_id}/grants/{grant_id}/disclosure`
with administrator authentication and exact tenant/project/namespace headers:

```json
{
  "disclosure": {
    "allowed_profiles": ["memory-reference-v1"],
    "default_profile": "memory-reference-v1",
    "source_attestation": false
  },
  "actor": "operator",
  "reason": "Provision reference access for the runtime"
}
```

The update and actor/reason audit are atomic. Grant reads expose the configured
`disclosure` object; an absent object means unconfigured access. Configuration
cannot be changed by the public principal. The official bootstrap script accepts
`-DisclosureProfile memory-reference-v1` (default) or `memory-content-v1`, and
always disables attestation initially. Never print the one-time credential.

| Profile | Permitted modes | Source classifications |
| --- | --- | --- |
| `memory-reference-v1` | `reference_only` | low, medium |
| `memory-content-v1` | `reference_only`, `bounded_content` | low, medium |

Every supported request reads current exact-scope authority. Downgrade and
revocation apply to already-issued bindings. Store failure returns a bounded 503;
unconfigured, revoked, or unavailable profiles return a generic 403. Capability
discovery lists supported registry entries, never individual entitlements.

## Endpoint enforcement matrix

| Surface | Public credential behavior | Protected lookup |
| --- | --- | --- |
| Health/version/OpenAPI/Provider capabilities | Fixed discovery metadata | No memory lookup |
| Provider runtime initialization/status | Current exact-scope grant required | Binding metadata only |
| `POST /v1/provider/context` | Authorized mode-specific projection | Eligible active low/medium sources only |
| `POST /v1/provider/events`, `POST /v1/events` | Current grant and write access; attestation separately authorized | Safe event ID/replay acknowledgment |
| `POST /v1/provider/intents` | Current grant and existing intent gates | Intent ID/status acknowledgment |
| Provider retrieve/sync/forget/turn/outcome | 403 | Denied before operation lookup |
| Native search/context/list/history/provenance/session/intent reads and lifecycle routes | 403 | Denied before operation lookup |
| MCP data tools, including reads, writes and lifecycle operations | Denied | Denied before source/binding lookup |
| Administrative native/MCP operations | Existing administrator and exact-scope checks | Operator privileges remain explicit |

Public runtimes use the Provider context contract. A denied native or MCP route
does not justify another credential, local ranking, a second memory store, or a
transport fallback. Future data routes remain closed until they implement the
same disclosure boundary. Fixed reasoning capability discovery remains usable.

## Source labels and lineage

Ordinary event inputs accept a closed optional `source_metadata` object:

```json
{"version":"source-metadata-v1","trust":"untrusted","classification":"low"}
```

Omission resolves to `untrusted/restricted`. Unknown fields, labels, versions,
duplicate keys, nulls and malformed values are invalid. Free-form application
metadata never grants trust. `attested_data` requires the authenticated principal's
independent exact-scope `source_attestation` entitlement; it verifies authorized
source attribution, never truth or instruction authority. Retry rechecks current
authority. Changing effective labels under the same idempotency key is a conflict.

The classification order is `low < medium < high < restricted`. Derived data uses
the greatest source classification and candidate sensitivity floor. All sources
must be attested with complete lineage to retain `attested_data`. Missing or
foreign sources contribute `untrusted/restricted`. Extractors, models and manual
updates cannot make historical labels more permissive.

Manual create and update operations introduce unattributed source text: their
new canonical snapshot resolves to `untrusted/restricted`, even when the prior
version was low or attested. The prior version retains its immutable labels.
Merge and reclassification preserve the restrictive join of contributing sources
and append a version; they cannot relabel old history.

Migration 0035 adds snapshots to raw events, candidates, canonical records,
versions, insights/evidence, chunks and projections. Canonical changes append a
version; versions and raw/candidate label snapshots remain immutable. Summary and
insight writes join durable exact-scope sources. Chunk/projection items resolve
source identity and version. Legacy snapshots remain untrusted/restricted.
Public context omits optional insight, goal, graph, chunk, cached projection and
calibration paths whose contributing evidence cannot be independently disclosed.
It does not report their identities, text or hidden counts.

A summary is eligible only when it has same-scope `create_summary_memory`
provenance, every contributing raw event is low or medium, and every raw event
has an active same-scope promoted canonical source with a stored version. Missing,
foreign, or hidden source edges cause the summary to be omitted before ordering
and `LIMIT`. Authorization dependency failures remain failures.

## Consuming context

Default mode is `reference_only`. Required categorized arrays remain present as
`[]` for genuine empty results. Items contain a closed `reference` and bounded
canonical citations. A reference carries kind, ID, version, normalized path,
memory class, availability and source metadata. It carries no content, title,
summary, lesson, arbitrary provenance metadata or diagnostic text.

Explicitly request `bounded_content` only with `memory-content-v1` authorization.
Content items contain `memory` with the same typed reference fields and content.
High/restricted, hidden, foreign or unverifiable sources disclose no identity or
path. Citations derive only from returned items; they do not authorize another
read. Raw-event/provenance IDs are omitted because they need independent evidence
authorization. Validate the mode-discriminated OpenAPI shape before use. Missing
fields or malformed results are contract failures, never empty successes.

PC3 `context-budget-v1` retains positive item-placement `budget` and adds optional
closed `budget_limits` for UTF-8 content bytes, complete JSON body bytes and
unique canonical citations. Omitted fields inherit independent server ceilings;
content/citation zero is valid without changing disclosure mode. Whole authorized
items retain their mandatory citations, and deterministic `context-packing-v1`
admission conservatively reserves accounting space. Required closed
`budget_accounting` reports requested omission nulls, effective limits, exact
usage and safe ordered reasons. Hidden/denied evidence contributes no counters
or existence hints; unsupported public sections stay closed.

Infeasible mandatory envelopes or finalized accounting/serialization refusal
return bounded 413 without partial projection. Current authority remains checked
before packing: malformed allocations use 400, disclosure denial 403, unavailable
authority retryable 503. Tokens/tokenizers remain unsupported. Discover budget
versions explicitly, update strict consumer schemas and validate actual body
usage. Disable dependent consumer access before rollback to an older binary;
retain PC2 enforcement/data. See [Provider allocation and migration](agent-runtime-memory-provider.md)
and [independent configuration](self-hosting.md#provider-context-allocation-configuration).
Digest/continuity remains PC4 and reference/outcome workflows PC5/PC6; archived
PC2 evidence does not establish PC3 readiness.

Danny owns business roles, tool/write permissions, compiler submission and task
completion. Stele owns scoped selection and durable memory evidence. All recalled
text is data for the consumer's policy; even attested data cannot authorize an
action or override instructions.

## Upgrade and rollback

Inventory active public grants and clients before rollout. Apply migration 0035,
provision explicit profiles, and deploy matching API/worker/scheduler binaries.
Keep public credentials disabled during mixed-version rollout: an old replica or
worker does not enforce this contract. Verify migration checksums, extension
versions, digest, governance completion and restart behavior before enabling
traffic. New grants and legacy data receive no implicit content or attestation.

Rollback first disables Provider and revokes/disables affected public credentials
across native and MCP surfaces. Then revert binaries while retaining additive
columns, grants, labels and append-only history. An older binary must never serve
those constrained credentials. Restore access only on a revision that enforces
the disclosure ceiling and passes its gates. Migration down is intentionally
unavailable because dropping labels would erase the security boundary.

See [Provider integration](agent-runtime-memory-provider.md),
[agent skill integration](integrations/mcp-agent-skill.md) and the
[PC2 implementation tasks](../openspec/changes/archive/070-provider-context-disclosure-and-source-trust/tasks.md).
