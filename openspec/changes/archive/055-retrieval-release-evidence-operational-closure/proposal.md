## Why

P8.8 established owned PostgreSQL + pgvector release evidence and controlled
activation, but the operator workflow still has gaps around failure diagnosis,
run lifecycle cleanup, and evidence handoff. Missing or reused evaluation
resources, timed-out runs, stale source watermarks, and rollback/disablement
events need stable, reviewable evidence so an operator can distinguish an
environment problem from a quality regression without weakening fail-closed
activation. This change closes that operational loop without introducing a new
retrieval strategy, rollout system, or default behavior change.

## What Changes

- Define stable, redacted categories for missing evaluation DSN, missing or
  invalid ownership marker, DSN reuse, PostgreSQL/pgvector prerequisite
  failures, fixture incompatibility, timeout, and incomplete-run cleanup.
- Make evaluation runs explicitly bounded and isolated: enforce timeout,
  report-directory isolation, deterministic cleanup of failed/incomplete runs,
  and retention rules for completed redacted evidence.
- Bind release evidence to a stable run identity, exact-scope logical identity,
  source watermark/freshness verdict, compatible policy/fixture versions, and
  an attestation or handoff record that activation can verify.
- Record activation disablement and rollback outcomes as reviewable redacted
  evidence, preserving the approved baseline when evidence is missing, stale,
  incompatible, or unsafe.
- Add operator-facing redacted summaries plus low-cardinality metrics and
  bounded logs for evaluation lifecycle, prerequisite status, evidence
  freshness, cleanup, attestation, activation disablement, and rollback.
- Calibrate the roadmap/OpenSpec status so archived change 054 is no longer
  shown as active and this proposal is the single active bounded post-v1 item.

## Non-goals

- No new retrieval ranking, progressive-context, parent-first, calibration, or
  provider strategy.
- No automatic authorization of `active_for_scope`, autonomous rollout, or a
  second activation/rollback policy system.
- No fallback from the evaluation DSN to runtime `STELE_POSTGRES_DSN` and no
  reuse of the service database as an evaluation target.
- No change to default retrieval, default context assembly, OpenAPI response
  behavior, canonical memory, lifecycle semantics, or scope boundaries.
- No SDK, UI, hosted product, or end-user product logic.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `retrieval-release-evidence-run`: require operationally classified,
  bounded/isolated runs, deterministic cleanup, evidence retention, and
  attestation-linked freshness and rollback records.
- `retrieval-release-gate-and-progressive-context-evaluation`: require release
  evidence handoff and activation/disablement verification to remain exact-scope,
  fail-closed, and tied to the same compatible run and source watermark.
- `service-observability`: add bounded operator summaries, metrics, and logs
  for evaluation prerequisites, run lifecycle, cleanup, freshness, attestation,
  activation disablement, and rollback without high-cardinality data.

## Impact

- Evaluation wrapper/scripts and the release-evidence execution path will gain
  stable categories, timeout/cleanup handling, and redacted artifact retention.
- Release-gate/admin inspection paths will validate run, watermark, policy, and
  attestation identity before exposing activation eligibility or rollback proof.
- Metrics and structured logs will expose only fixed categories and buckets;
  DSNs, credentials, raw queries, prompts, source content, provider payloads,
  scope values, and record identifiers remain excluded.
- Documentation and roadmap reconciliation will be updated alongside the
  capability deltas. No PostgreSQL system-of-record or public API contract is
  replaced.

## References

- [Retrieval release evidence run](../../specs/retrieval-release-evidence-run/spec.md)
- [Retrieval release gate and progressive context evaluation](../../specs/retrieval-release-gate-and-progressive-context-evaluation/spec.md)
- [Service observability](../../specs/service-observability/spec.md)
- [Retrieval release checklist](../../../docs/retrieval-release-checklist.md)
- [Retrieval release gate](../../../docs/retrieval-release-gate.md)
- `openspec status --change retrieval-release-evidence-operational-closure --json`
- `openspec validate --all`
