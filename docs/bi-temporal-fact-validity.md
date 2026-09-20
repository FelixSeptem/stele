# Bi-temporal fact validity

Stele keeps two independent clocks for factual canonical memory versions:

- **Recorded time** is when Stele ingested or wrote the version. Existing
  `time_from` and `time_to` search fields keep this recorded/update-time
  meaning.
- **Valid time** is the interval in which the fact was true. Factual
  `profile`, `episodic`, and `procedural` versions use the half-open interval
  `[valid_from, valid_to)`; a null `valid_to` is open-ended.

This distinction is append-only. A correction appends a successor version and
an auditable correction-ledger record; it never overwrites a previous canonical
version, raw event, provenance record, chunk, embedding, or projection.

## Search and lifecycle semantics

Ordinary search is a current-fact read. Before lexical, semantic, relation, or
chunk candidates are fused, Stele requires exact `tenant`, `project`, and
`namespace` scope, ordinary lifecycle visibility, and validity at one captured
request evaluation instant. An expired, conflicting, suppressed, forgotten, or
deleted version is not eligible merely because it has a stronger similarity
score.

Historical selection is explicit:

- `as_of` selects a version valid at one RFC 3339 instant.
- `valid_from` and `valid_to` together select versions overlapping that
  half-open interval.

`as_of` is mutually exclusive with the interval pair, and a partial or invalid
interval is rejected. These valid-time selectors do not reinterpret
`time_from` or `time_to`. Historical retrieval stays exact-scope, lifecycle
safe, bounded by the normal retrieval envelope, and requires the authorized
temporal plan/rollout path; it never silently broadens an ordinary request.

## History, provenance, and derived evidence

Privileged history and provenance views identify the selected version's
temporal fact identity, recorded time, valid bounds, validity source, and
append-only correction lineage. Ordinary results expose only the stable public
response shape and bounded temporal selection summary; they do not echo a
caller-supplied instant, hidden version identity, raw interval detail, or
internal diagnostic.

Relations, chunks, embeddings, citations, and context projections retain the
immutable source-version and validity snapshot used to derive them. A
successor creates new derived lineage; historical materialization that is
missing yields a bounded channel omission or baseline fallback, never a
substitution from another version or scope.

## Legacy compatibility and rollout

Migration `0014_bi_temporal_fact_validity` treats legacy rows as
`legacy_current_compatible`: their recorded creation time becomes `ingested_at`
and `valid_from`, and open-ended `valid_to` preserves current retrieval.
Stele does not invent historical truth before that compatibility interpretation.

Temporal behavior begins in diagnostics/shadow. Promotion to
`active_for_scope` requires compatible, explicitly owned PostgreSQL + pgvector
evidence proving no stale-fact win, validity ambiguity, provenance mismatch,
hidden-version leak, scope leak, or resource overflow. Missing owned-DSN
evidence is a stable non-pass. Disabling the temporal policy returns the
approved current baseline without deleting history; the operational procedure
is documented in [Bi-temporal migration, recovery, and rollback](bi-temporal-migration-and-rollback.md).
