# Context Projections

Context projections are derived, append-only PostgreSQL read models. They are
scoped by exact `tenant`, `project`, and `namespace` values and reference
canonical memory versions or raw-event evidence; they never replace canonical
memory or delete provenance.

Projection reads are opt-in and disabled by default. Set
`STELE_CONTEXT_PROJECTION_CONSUMPTION_ENABLED=true` only after operators inspect
the projection kind,
version, status, source watermark, policy version, renderer version, and
bounded item counts before enabling consumption. A rebuild reads the authorized
source snapshot and creates a new projection version, retaining prior versions
for audit and rollback.

Ordinary context excludes suppressed, forgotten, expired, and deleted sources.
Profile items may enter `always_visible` only after confidence and size gates;
summaries may enter `session`; episodic, procedural, and relation memories stay
on-demand; raw events remain archival evidence only.

If a rollout is stale or unsafe, disable projection consumption and continue
with live retrieval. Do not widen scope, increase the request budget, mutate
canonical memory, or delete projection history as a rollback operation.
