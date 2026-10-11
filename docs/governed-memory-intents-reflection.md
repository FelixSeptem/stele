# Governed Memory Intents, Reflection, and Compaction Evidence

Stele treats external memory requests and model-derived memory as governed,
auditable records. A request is first written to the scoped `memory_intents`
ledger. It carries an actor, reason, request/operation identifiers, an
idempotency key, and provenance. The ledger is append-only; it is not a direct
write path to canonical memory.

Intent targets are checked against the requested tenant/project/namespace,
active lifecycle state, and expected memory version before downstream work is
accepted. Retries using the same scoped idempotency key return the original
durable result. A different payload for an existing key is rejected.

Reflection is asynchronous. Each `reflection_runs` record has a trigger,
transcript schema version, input watermark, processed offset, lease, attempt
budget, bounded failure category, and replay identity. Workers checkpoint
before completion and may resume after lease expiry. Reflection output remains
derived/candidate data until the configured governance and review path accepts
it.

Administrative review decisions are append-only. Supported decisions are
`accept`, `suppress`, `reject`, and `request_more_evidence`. Suppressed,
rejected, forgotten, deleted, stale, or foreign-scope records are excluded
from default retrieval and context projection.

Compaction records retain source watermark and source references, derivation and
summary versions, token estimates, evidence coverage, and bounded recent-tail
references. A summary is eligible for default projection only when its evidence
is active, complete, same-scope, and lifecycle-safe. For public disclosure each
raw source must also have low/medium labels and an independently active same-scope
promoted canonical version; missing or hidden source edges omit the summary
before ranking and limit. Rebuilds create derived
projection records and never mutate canonical memory versions.

## Operational checks

- Inspect intent and reflection status only through scoped admin endpoints.
- Treat lease expiry as recoverable and retry only within the configured
  attempt budget.
- Replay with the original input watermark and transcript schema version when
  investigating deterministic drift.
- Mark compaction evidence stale when a source version or lifecycle state is no
  longer eligible; rebuild the derived artifact instead of editing it in place.
- Keep diagnostics bounded and redacted; do not expose raw cross-tenant
  evidence in logs or admin responses.

## Current disclosure and source-lineage boundary

Public credentials use profiled Provider context and safe Provider event/intent
acknowledgments. Native history/provenance, lifecycle and MCP data tools require
administrator access. Accepted writes do not prove canonical promotion or business
completion. Canonical versions retain immutable typed source snapshots. Manual
create/update introduces unattributed `untrusted/restricted` text while preserving
prior version labels; merge and reclassification append restrictive joined
versions. Existing suppression, forgetting and deletion do not erase historical
labels. Summaries,
reflection and insights use restrictive joins across exact-scope evidence, with
missing lineage resolving to untrusted/restricted. Attested source attribution
never authorizes tools, writes or instructions. See the
[disclosure/source guide](context-disclosure-and-source-trust.md)
for grant provisioning, migration and rollback constraints.
