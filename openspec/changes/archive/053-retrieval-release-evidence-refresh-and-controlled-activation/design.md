## Context

See `proposal.md` for motivation. The current release boundary in
`internal/retrieval/release_evidence.go` validates an explicitly supplied DSN,
evaluates redacted baseline/candidate reports, and produces an eligibility
verdict. The owned PostgreSQL fixture runner and
`scripts/retrieval-evaluation.ps1` already enforce a separate evaluation DSN
and produce bounded artifacts. Existing exact-scope ranking rollout governance
already provides draft, diagnostics-only, dry-run/shadow, active-for-scope,
disablement, and rollback state transitions.

The remaining gap is the binding between one fresh owned evidence result and an
activation request. A passing local report must not be substituted for another
scope, strategy, policy, fixture, source watermark, or later activation.

## Goals / Non-Goals

**Goals:**

- Reuse the existing owned evaluation, evidence report, and ranking rollout
  boundaries to create a single authoritative release-evidence identity.
- Bind activation eligibility to one exact scope and compatible release-policy,
  strategy, fixture, provider, source-watermark, and dependency identities.
- Fail closed before a result-affecting rollout is resolved when evidence is
  absent, expired, rejected, incompatible, unsafe, or not rollback-tested.
- Keep the real-stack command independently reproducible and useful for CI and
  operator review without exposing sensitive values.

**Non-Goals:**

- Do not create a new rollout table, generic policy engine, evidence database,
  or activation endpoint where existing ranking rollout governance suffices.
- Do not change retrieval strategy semantics, make parent-first non-shadow,
  enable a provider, or promote diagnostic data into canonical memory.
- Do not make a passed evidence run automatically activate a policy.

## Decisions

### Derive a stable, redacted evidence identity from compatibility inputs

Add a bounded release-evidence identity derived from the exact scope hash,
release-policy version, fixture/representation/fusion/ranking/provider/analysis
identities, source watermark, strategy mode, and fixed evaluation time or
window. The report contains only this opaque identity and existing aggregates;
it never serializes the DSN, raw scope, records, queries, or provider values.

The owned runner supplies all required compatibility values from its fixture and
evaluation output, validates them before persistence or report emission, and
uses a fixed clock for replay assertions. Equivalent input produces the same
identity; a scope, watermark, policy, or dependency mismatch produces a stable
non-pass category.

Alternative considered: use a random run ID alone. Rejected because it proves
that a run existed but cannot prove compatibility with a later activation.

### Extend the existing release report and activation gate rather than adding a parallel service

`ReleaseEvidenceReport` becomes the sole eligibility input for this proposal.
It gains bounded freshness and identity metadata plus any missing stable failure
categories. The existing ranking rollout activation gate consumes a compact
release-evidence attestation containing the redacted identity, verdict,
scope-hash, policy/dependency identities, generated time, and rollback/replay
outcomes. Domain validation confirms it matches the target policy and exact
scope before transition to `active_for_scope`.

The repository persists only the attestation reference or digest needed for the
existing rollout audit trail. It must never persist the report's raw fixture
content or secret-bearing inputs. If the existing rollout audit already has a
suitable versioned metadata field, reuse it; otherwise add narrow nullable
forward-only columns and update repository scans/writes atomically.

Alternative considered: expose a new public release-activation API. Rejected
because the admin ranking rollout lifecycle already owns authorization,
auditability, disablement, and rollback.

### Centralize fail-closed compatibility and freshness validation

Introduce a pure validator shared by evaluation and activation. It verifies:

- exact normalized scope and scope hash;
- allowed `passed` real-stack verdict only, bounded evidence age, and tested
  deterministic replay/rollback;
- matching release-policy and selected strategy/dependency identities;
- green protected recall, temporal/multi-hop coverage, lifecycle/isolation,
  semantic-hit, trajectory/integrity, resource, and rollback gates;
- no diagnostics-only, shadow-only, skipped, degraded, rejected, or stale
  result can influence activation.

The resolver defensively repeats the activation eligibility check at request
time. This makes expiry, policy change, direct repository manipulation, and a
post-activation dependency mismatch return the approved baseline rather than
changing caller results.

Alternative considered: validate only at the administrative activation request.
Rejected because a once-valid policy can later expire or become incompatible.

### Keep the real-stack workflow opt-in, bounded, and evidence-only

Extend `scripts/retrieval-evaluation.ps1` and its PostgreSQL fixture test with
an explicit timeout, an isolated report directory, exact fixture cleanup, and
stable exit semantics. A missing DSN remains exit code 2 with
`SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED`; an unavailable prerequisite, failed
gate, or report redaction failure is a nonzero non-pass. The wrapper never
prints the DSN and never falls back to `STELE_POSTGRES_DSN`.

The runner verifies the same redacted report used by the activation gate. It
must cover baseline equivalence, progressive and parent-first shadow results,
adaptive planning, temporal cases, semantic retrieval proof, trajectory and
integrity reports, deterministic replay, and rollback. The real-stack run
creates evidence only; the separately authorized admin transition remains the
sole activation step.

Alternative considered: make the evaluation command activate the candidate at
the end of a passing run. Rejected because testing authority must be separate
from production rollout authority.

### Calibrate roadmap status as part of the change

Update the roadmap after proposal creation so P8.7/change 052 is archived and
P8.8 is the active proposal, including the corresponding self-hosting status
test. This change does not claim active rollout before the owned evidence and
activation gates pass.

## Risks / Trade-offs

- **[Risk] A passing report is reused for a different scope or policy** -> Bind
  activation to an opaque identity built from exact-scope and compatibility
  inputs and revalidate at resolution time.
- **[Risk] Evidence freshness becomes a hidden operational dependency** -> Use
  an explicit bounded age, a stable stale category, and visible report/checklist
  fields; stale evidence falls back to baseline.
- **[Risk] Real PostgreSQL execution is flaky or mutates a developer database**
  -> Require an explicit owned DSN/marker, bounded timeout, unique fixture
  identifiers, exact cleanup, and no runtime-DSN fallback.
- **[Risk] Extending audit metadata leaks sensitive content** -> Permit only
  hashes, logical identities, aggregate counts, and stable categories; add
  redaction tests at report, persistence, and command boundaries.
- **[Trade-off] Activation needs another operator step after a passing run** ->
  Preserve separation of evaluation and deployment authority, which is required
  for scoped, reviewable rollout.

## Migration Plan

1. Add pure release-evidence identity, compatibility/freshness validation, and
   failing unit tests without changing policy resolution.
2. Extend the existing rollout audit persistence with narrow forward-only
   metadata only if existing fields cannot retain the attestation reference.
3. Bind admin activation and runtime resolution to the validator; retain
   diagnostics/shadow and the approved baseline as fallbacks.
4. Extend the opt-in owned PostgreSQL + pgvector harness and wrapper, then run
   it against an explicit disposable database to capture redacted evidence.
5. Update release checklist, self-hosting guide, roadmap, and status tests.
6. Roll back by disabling or rolling back the exact-scope policy. The resolver
   immediately returns the existing baseline; reports and audit history remain
   append-only and no canonical-memory migration is reversed.
