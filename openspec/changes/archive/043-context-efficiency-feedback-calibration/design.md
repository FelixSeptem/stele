## Context

See `proposal.md` for motivation and the delta specifications for behavior.
Stele already has deterministic context packing, context projections, diversity
selection, feedback and task-quality records, ranking rollout policies, and
redacted evaluation reports. RQ1 supplies a bounded request plan, RQ2 supplies
fact-valid eligibility, and RQ3 supplies bounded relation-path candidates. The
missing layer is a compatible, reproducible way to determine whether a context
strategy uses its fixed budget more efficiently and whether weak feedback can
calibrate ordering without becoming a new visibility or canonical-memory rule.

The PostgreSQL database remains the sole system of record. Feedback, task
evaluation, verification, and quality records remain durable source evidence;
efficiency summaries and calibration summaries are derived, versioned, scoped,
and rebuildable. Public search and context response models must not expand.

## Goals / Non-Goals

**Goals:**

- Add a common, redacted evidence contract that compares baseline and calibrated
  context behavior under identical request and source conditions.
- Make feedback influence explicitly weak: minimum evidence, confidence
  threshold, expiration/decay, per-signal cap, deterministic ordering, and
  exact-scope policy identity.
- Ensure the request hot path reads one bounded derived calibration summary; it
  does not scan raw feedback, task evaluations, or audit history.
- Preserve existing packing, scope, lifecycle, temporal, graph, citation, and
  caller-budget contracts before calibration is evaluated.
- Make shadow, active, disablement, and rollback evidence reproducible through
  the existing rollout/release gate path.

**Non-Goals:**

- No global popularity score, adaptive online learning loop, or unbounded
  reinforcement from clicks or requests.
- No change to canonical memory content, state, provenance, temporal history,
  or relation projections.
- No new external ranking service, provider dependency, queue, graph store, or
  public API response field.
- No inference from raw feedback text in the retrieval request path.
- No automatic activation: lack of a compatible summary or release evidence
  always retains the approved baseline.

## Decisions

### 1. Treat efficiency as evaluation evidence, not an online score

Introduce immutable `ContextEfficiencyEvidence` for fixtures, shadow runs, and
release reports. It contains version identities and bounded aggregate metrics:

- relevant-token ratio: eligible expected evidence token estimate divided by
  selected token estimate;
- evidence density: distinct expected evidence groups covered per selected token
  bucket;
- duplicate-token rate: token estimate attributable to a repeated validated
  identity/lineage/diversity cluster divided by selected token estimate;
- stale-token rate: selected derived/projection token estimate whose approved
  freshness category is stale divided by selected token estimate;
- quality-per-budget: protected quality/coverage normalized by the requested
  context budget bucket;
- per-pass candidate count, selected-context cost, omission categories, and
  latency buckets.

All ratios are validated in `[0,1]`; counts, budgets, and elapsed values are
bounded by existing hard envelopes. A report names fixed fixture, renderer,
retrieval-plan, temporal, graph, feedback-summary, calibration-policy, and
release-policy identities. It does not carry raw query, content, IDs, scope,
DSN, credentials, feedback text, raw score, or exact token count.

**Why:** metrics become comparable only if source/request identities match. An
evaluation record, rather than request telemetry, can enforce that compatibility
and retain a redacted audit trail.

**Alternatives considered:**

- Record only Prometheus counters. This cannot compare exact fixtures or prove
  rollback/replay compatibility.
- Attach raw per-item cost and feedback to reports. This breaches the existing
  diagnostic redaction boundary.
- Use an LLM judge for relevance. It may be optional advisory evidence later,
  but cannot replace deterministic qrels/evidence groups or hard gates.

### 2. Build a scoped, versioned calibration summary asynchronously

Add an additive PostgreSQL derived-summary record keyed by exact scope,
calibration policy version, source evidence watermark, and summary identity.
The worker rebuilds it from active non-superseded usefulness feedback,
task-evaluation/verification summaries, and approved quality summaries. It
stores only bounded feature buckets and counts, plus effective decay/cap/minimum
threshold versions; detailed source identities remain in existing scoped durable
records for admin audit.

The summary builder applies deterministic ordering, ignores source evidence
outside scope or lifecycle eligibility, ignores expired/superseded source
records, enforces a minimum evidence count, and clamps each signal contribution
to a policy cap. Its source watermark makes summary freshness explicit. A
missing, stale, malformed, or incompatible summary is unavailable, not a
partial score.

**Why:** raw feedback joins in request-time ranking make latency, privacy, and
replay behavior depend on mutable history. One bounded derived read keeps the
hot path predictable and makes repair/rebuild possible.

**Alternatives considered:**

- Compute feedback weights synchronously from raw tables for every request.
  Rejected because it adds unbounded work and snapshot instability.
- Persist mutable scores on canonical memories. Rejected because it violates
  append-only canonical history and entangles quality experiments with truth.
- Use one global tenant score. Rejected because calibration must remain exact
  `tenant/project/namespace` scoped.

### 3. Apply calibration after eligibility and before final packing

The context assembler keeps its current sequence:

```text
request scope/authorization
  -> lifecycle + RQ2 valid-time eligibility
  -> RQ1 channel/retrieval plan and bounded candidates
  -> RQ3 qualified graph candidates, when enabled
  -> identity/lineage deduplication and diversity selection
  -> optional compatible calibration summary adjustment
  -> existing deterministic section packing and caller budget enforcement
  -> citations and ordinary response rendering
```

Calibration can adjust only the relative priority of already eligible,
deduplicated candidates. It cannot add a candidate, change its class/section,
extend a path, revive a hidden record, alter citations, or allocate tokens beyond
the baseline envelope. The baseline tie-breaker is retained for equal calibrated
priority. A protected evidence group cannot be displaced if doing so would make
the evaluated request violate the policy's protected-recall constraint.

**Why:** visibility and evidence truth remain primary; calibration is a bounded
packing hint, not another retrieval channel.

**Alternatives considered:**

- Apply calibration before retrieval. Rejected because it could affect channel
  access and candidate expansion rather than context efficiency alone.
- Apply calibration before deduplication/diversity. Rejected because duplicate
  feedback could amplify repeated evidence.
- Add calibrated items as a new public context section. Rejected because the
  ordinary API contract must remain stable.

### 4. Reuse governed rollout with calibration-specific gates

Extend the existing exact-scope ranking rollout payload with an optional
calibration block. It declares policy/version identity, summary version,
minimum evidence threshold, decay window, cap bucket, protected efficiency and
recall thresholds, and status. Deployment configuration sets absolute hard
limits for every numeric value; policy values may only narrow them.

Diagnostics-only and shadow execute comparison/evidence collection while
returning the approved baseline. Active-for-exact-scope requires compatible
fixed-fixture, replay, rollback, safety, protected-recall, budget, and
efficiency evidence. Disablement/rollback removes the calibration adjustment on
the next matching request and never mutates source evidence or canonical data.

**Why:** the service already has a durable rollout/audit model. A separate
calibration switch would duplicate authorization and make rollback ambiguous.

**Alternatives considered:**

- Enable calibration with a process-wide config flag. Rejected because it lacks
  exact scope, evidence, and rollback attribution.
- Make per-request feedback-aware flags immediately active without policy.
  Retained only for existing bounded diagnostic behavior; they cannot bypass
  calibration gates for default rollout.

### 5. Make efficiency regressions release-hard failures

Extend release comparison to require baseline/candidate identity compatibility
and fail a candidate if it has any of: protected recall/evidence decline beyond
policy, cross-scope/lifecycle/temporal/citation failure, context budget overflow,
unbounded candidate/pass work, stale/duplicate rate above policy, summary
freshness failure, nondeterministic replay, ordinary API leakage, or failed
rollback. Positive quality-per-budget cannot override any such result.

Performance/quality thresholds are policy data rather than constants; the
implementation validates that they are bounded and that a policy version changes
when any threshold changes.

## Risks / Trade-offs

- **Token estimates vary by renderer/model** → bind evidence to a renderer and
  token-estimator identity; compare only compatible reports and use budget
  buckets in telemetry.
- **Feedback can be sparse or biased** → enforce a minimum evidence threshold,
  decay, contribution cap, exact scope, and baseline fallback; report
  insufficient evidence rather than manufacturing a score.
- **Derived summaries can lag source feedback** → record source watermark and
  freshness; shadow/active policies treat stale/missing summaries as unavailable.
- **More report fields can leak sensitive data** → use allowlisted buckets and
  serialization tests that reject query/content/IDs/scope/raw-score fields.
- **Efficiency could trade away rare critical facts** → retain protected evidence
  groups and make protected recall a hard gate, independent of aggregate gains.
- **Migration/rebuild work can contend with retrieval** → perform bounded
  asynchronous batching with existing leases and rate limits; request paths only
  read a compatible summary.

## Migration Plan

1. Add additive migration(s) for derived calibration summaries, source
   watermark/version fields, unique exact-scope identity, bounded audit fields,
   and lookup indexes. Do not rewrite canonical memories or feedback history.
2. Ship configuration parsing and disabled-by-default policy validation. Existing
   requests retain baseline behavior because no calibration policy is active.
3. Ship summary rebuild/repair and deterministic fixtures in diagnostics-only.
4. Produce baseline-versus-candidate release evidence through shadow runs.
5. Permit active-for-exact-scope only after the existing owned PostgreSQL +
   pgvector evidence contract and new calibration hard gates pass.
6. Roll back operationally by disabling the exact-scope policy; retain derived
   summaries for audit/repair and remove only expired derived records through
   governed retention. Down migrations remain reversible and never delete
   canonical source history.
