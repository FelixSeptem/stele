## Context

See proposal.md for motivation. Reuse `TaskSeriesManifest`, checksum locks,
`EvaluateTaskSeries`, the lexical retrieval service and PostgreSQL fixture
seeder/cleanup. PostgreSQL remains the only system of record. This is an
internal CLI/evaluation contract, with no public API or runtime policy change.

## Goals / Non-Goals

The owner is Stele's benchmark maintainer. Goal: a reproducible offline v3
experiment whose source visibility, independent observations and release gates
can be audited. No model execution, task inference, automatic correction or
live resident selection occurs. Input condition snapshots are supplied by an
authorized harness; their names do not prove the production EV1–EV4 paths ran.

## Decisions

### Extend existing task series

Keep v1/v2 default constants and optional JSON fields absent for old fixtures.
V3 uses `task-series-v3`/`task-series-metrics-v3`, exactly five roles in order:
`no_memory`, `source_records`, `organized`, `verified_corrected`, `resident`.
Roles are fixed enums; condition IDs remain private. Only the first is baseline.
Every condition has identical stage IDs, query text, held-out flags and
superseded flags. Stage 1 is preparation; at least one later held-out task is
required. Queries reference one common session to retain earlier eligible
evidence across tasks. Conditions may provide distinct external verdicts and
source snapshots. Missing v3 verdict maps to inconclusive.

A `TaskSeriesExperiment` locks SHA-256 model/runtime/evaluation-criteria
identities, 1–32 distinct nonnegative seeds, and common limits (1–64 items,
1–65536 UTF-8 bytes, 1–16384 whitespace tokens, 1–64 citations, 1–60000 ms).
Hashes are local experiment identity, not attestation. External criteria must
define task and cost acceptance before data collection; service success means
contract/gate conformance only, never demonstrated task benefit.

### Source windows prevent future leakage

Each condition declares at most 64 source windows, one per corpus event:
reference, positive immutable source version, SHA-256 literal content,
disclosable source metadata, first available stage (1-based), optional first
withdrawn stage (exclusive). Every event is exact-scope, nonempty, at most
64 KiB; conversations are rejected in v3 so transcripts cannot bypass windows.
Window validity and metadata are checked before SQL. Stage expected evidence
and all observed aliases must be stage-visible. Future qrels are allowed in
the locked full corpus, but the replay snapshot removes future events, qrels,
evidence groups and query answer annotations before any database write.
The no-memory corpus has zero events/windows/qrels and no expected evidence.

Stage snapshots are seeded in fresh unique owned scopes using the established
fixture seeder. The seeder's generic labels are replaced only inside these
owned fixture rows with the exact declared raw/canonical/immutable disclosure
metadata. Search sets disclosure required; scope/lifecycle/citation lineage
are checked against actual returned rows. No-memory queries an empty owned
scope through the same service. Restricted windows are refused before seeding.
Returned whole items are packed under all four equal independent limits;
bytes/tokens count literal content, not model usage or Provider wire bytes.

### Separate observations, gates and baseline replay

Optional external `model_tokens`/`tool_calls` are nonnegative integers up to
1,000,000 each. Aggregate coverage includes all non-superseded stages, including
inconclusive utility. Costs have independent observed counts, totals and
nullable averages. V2 uses this corrected denominator for repeated error and
maintenance, preserving source locks and observed values.

Stage observations additionally carry nullable measured latency in microseconds,
disclosure/leakage status and rollback equivalence. Pure aggregation without
these observations is inconclusive. PostgreSQL replay supplies them from
actual searches, validates before aggregation, then reruns every baseline stage
after candidate conditions and compares redacted stable evidence identities.
No live policy is toggled. Failed/absent gates remain independent of utility.
Reports expose experiment/snapshot/observation digests and bounded counters,
not IDs, query/content/actor/scope or raw criteria. Wall clock measurements may
vary; replay identity and task metrics remain deterministic for fixed inputs.

### Bounds and compatibility

Retain the 64 MiB loader and v1/v2 condition/stage limits. V3 is additionally
limited to 64 stages and 64 sources per condition (at most 320 stage searches,
plus 64 baseline replays). Canonical JSON keys, duplicate-key and alias rejection
apply to v3 loaded bundles, avoiding lock ambiguity. Errors are bounded internal
manifest/prerequisite failures; no partial quality pass is emitted on runner
errors. Missing v3 fields are refused on legacy schema. No new dependency is
needed; standard Go, pgx, existing retrieval and fixture infrastructure suffice.

## Risks / Trade-offs

- Externally prepared snapshots may be semantically wrong → provenance and
  attribution do not establish truth; report observationally and require real
  held-out task observations for benefit claims.
- Per-stage seed/cleanup costs more than one whole-corpus seed → cap stages and
  sources, measure retrieval latency separately from maintenance overhead.
- Synthetic labels can make a favorable story → fixture outcomes remain
  inconclusive by default; tests separately inject explicit false/zero and
  failures to prove coverage and gates, never infer successes.
- Stage windows depend on honest experiment setup → source checksum binds them;
  changes require a new lock, and held-out isolation is enforced in SQL inputs.

## Migration Plan

No database migration or public contract migration. Existing v1/v2 CLI bundles
continue unchanged. V2 reports with inconclusive stages may show corrected
coverage; regenerate report artifacts rather than compare the former invalid
coverage above one. V3 is explicitly selected by schema/metric identity.
Rollback is stopping the offline run or using v1/v2; production remains unchanged.
Archive with `scripts/openspec-archive-seq.ps1` after focused/full/race/vet/docs,
strict OpenSpec and owned-stack checks, then commit, merge, push and audit.
