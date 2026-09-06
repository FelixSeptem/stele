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
is active, complete, same-scope, and lifecycle-safe. Rebuilds create derived
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
