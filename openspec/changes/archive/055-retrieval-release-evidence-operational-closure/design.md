## Context

See `proposal.md` for the motivation and scope. The repository already has an
owned PostgreSQL + pgvector evaluation wrapper, redacted release reports,
freshness/watermark checks, and a separately authorized activation boundary.
The remaining gap is that the operator cannot consistently distinguish
preflight failure, incomplete execution, stale evidence, and failed handoff;
the resulting artifacts and telemetry also need one bounded lifecycle.

The design must preserve PostgreSQL as the only system of record, keep
evaluation isolated from the runtime service DSN, and leave default retrieval,
context assembly, OpenAPI behavior, and canonical memory unchanged.

## Goals / Non-Goals

**Goals:**

- Give every evaluation attempt a stable, low-cardinality operational outcome
  from preflight through cleanup and evidence handoff.
- Make timeout, cancellation, directory isolation, incomplete-run cleanup, and
  completed-artifact retention deterministic and reviewable.
- Bind activation eligibility to one exact-scope run, source watermark,
  compatible policy identities, integrity summary, and rollback verdict.
- Make disablement and rollback observable without creating a second rollout or
  authorization system.
- Keep summaries useful to self-hosted operators while preventing sensitive
  values from reaching logs, metrics, or retained reports.

**Non-Goals:**

- Changing ranking, progressive-context, parent-first, calibration, or provider
  behavior.
- Granting `active_for_scope` automatically or changing the authorization
  boundary for activation.
- Introducing a new persistence store, public API family, SDK, UI, or hosted
  control plane.

## Decisions

### 1. Use one explicit run-state and category vocabulary

Preflight, execution, cleanup, attestation, disablement, and rollback emit a
small set of stable operation/result/category values. The state machine is
append-only at the evidence level, while a run may transition to terminal
`skipped`, `degraded`, `failed`, `completed`, or `rolled_back` outcomes.

This is preferred over exposing raw process errors because operators need
stable automation and metrics labels. Free-form error text remains an internal
diagnostic and never becomes part of the evidence contract.

### 2. Keep evaluation target proof explicit and fail closed

The wrapper continues to require the dedicated evaluation DSN and ownership
marker. Before any fixture work, it verifies the marker and target identity
constraints available to the evaluator; an inability to prove ownership or
non-reuse is a non-pass preflight result. The runtime `STELE_POSTGRES_DSN` is
never a fallback.

This is preferred over heuristic DSN comparison or implicit environment
selection, both of which can accidentally run destructive fixtures against the
service database.

### 3. Treat report directories as disposable run sandboxes

Each attempt receives a unique isolated `run-*` directory. A terminal
non-complete outcome removes temporary fixtures and report fragments, while a
completed outcome retains only redacted JSON/TXT evidence plus bounded
retention metadata. Cleanup is idempotent so retries and process restarts do
not create a second evidence bundle.

This keeps the existing filesystem-based wrapper practical without turning the
filesystem into a second source of record: the durable handoff is the
redacted, logically identified result already consumed by the release-gate
contract.

### 4. Make attestation a logical compatibility check, not a new authority

The handoff contains stable logical identities for run, exact scope, source
watermark/freshness, fixture/policy versions, integrity summary, and rollback
verdict. The release gate compares those identities before exposing eligibility;
it still requires the existing separately authorized activation operation.

An alternative was to add a new signed-token or rollout registry. That would
duplicate authorization state and broaden the security boundary, so it is not
adopted. The evidence link is a verification record, not a grant.

### 5. Share bounded observability vocabulary across metrics, logs, and summary

Metrics and logs use fixed categories and duration/age buckets. The operator
summary uses the same vocabulary and can point to a stable redacted run
identity, but never includes raw scope, record IDs, DSNs, source content,
queries, prompts, or provider payloads. Existing admin/evaluation
authorization remains the access boundary for reading summaries.

This favors operational consistency over dumping detailed command output; raw
diagnostics remain local to a controlled debugging session.

### 6. Calibrate roadmap state as part of the same change

The roadmap reconciliation section will treat archived change 054 as archived,
show no stale active proposal, and name this change as the active bounded
post-v1 proposal after its directory exists. The update is documentation-only
and does not alter OpenSpec archive numbering or branch policy.

## Risks / Trade-offs

- **[Risk] Ownership proof cannot detect every form of DSN aliasing.** → Keep
  explicit ownership markers mandatory, reject ambiguous target proof, and
  document the operator responsibility to provision a dedicated database.
- **[Risk] Aggressive cleanup could remove artifacts needed for diagnosis.** →
  Retain a bounded redacted terminal summary and category; preserve full
  reports only for completed runs under configured retention.
- **[Risk] Category vocabulary can drift between wrapper, gate, and telemetry.**
  → Define one shared set of constants/validation fixtures and add contract
  tests that compare report, summary, metrics, and logs.
- **[Risk] Evidence may become stale between review and activation.** → Recheck
  watermark/freshness and attestation identity at the existing activation
  boundary; stale or mismatched evidence remains fail closed.
- **[Risk] More lifecycle checks increase operator-facing complexity.** → Keep
  the public surface bounded to summaries and categories, with detailed
  implementation diagnostics outside the release contract.

## Migration Plan

1. Add the operational categories and redacted summary fields behind the
   existing evaluation wrapper and release-evidence report format.
2. Enforce isolated run cleanup and terminal retention for new runs; previously
   retained redacted reports remain readable, while missing new fields make
   them ineligible for activation until regenerated.
3. Add handoff/attestation validation and disablement/rollback evidence at the
   existing release-gate boundary. No candidate is auto-activated during the
   migration.
4. Add focused wrapper, release-gate, observability, and isolation tests,
   followed by the repository's normal validation and owned PostgreSQL +
   pgvector smoke path where available.
5. If the change must be reverted, disable the new handoff consumer and use
   the previously approved strategy; remove only newly generated incomplete
   artifacts and retain completed redacted history for audit.

## Open Questions

None that change the contract or task breakdown. Exact category constant names
and retention durations can be selected during implementation as long as they
remain stable, bounded, redacted, and compatible with the delta specs.
