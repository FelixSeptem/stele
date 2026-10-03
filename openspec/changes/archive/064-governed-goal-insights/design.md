## Context

See `proposal.md` for the motivation and externally visible scope. The current
repository already has a provider-neutral reasoning envelope, reserved insight
activation policy, append-only derived insight records, contradiction-specific
temporal handling, governed operation precedence, and redacted reasoning
telemetry. `goal` is reserved vocabulary but has no type-specific contract.

The implementation must keep PostgreSQL as the only system of record, preserve
tenant/project/namespace isolation, avoid in-place canonical mutation, and keep
offline/shadow reasoning non-authoritative. Existing default retrieval and
context behavior must remain stable.

## Goals / Non-Goals

**Goals:**

- Add a provider-neutral, bounded goal candidate envelope that can be validated,
  replayed, reviewed, and audited using existing reasoning and activation paths.
- Preserve goal-specific state, validity, evidence, freshness, uncertainty, and
  review metadata without creating a new canonical memory class or storage
  system.
- Make policy, review, replay, rollback, and telemetry behavior deterministic
  and exact-scope.
- Keep ordinary retrieval and context assembly fail-closed for goals until a
  later, separately governed visibility policy is enabled.

**Non-Goals:**

- No autonomous task planner, task executor, scheduler, reminder system, or
  final-answer generation.
- No provider-specific goal semantics or direct model invocation contract.
- No default active goal admission, default retrieval visibility, or context
  ranking behavior.
- No rewrite of canonical memory, evidence, or prior derived insight versions.

## Decisions

### 1. Reuse the reasoning envelope and add a narrow goal metadata layer

Represent goal-specific data as normalized metadata attached to the existing
reasoning candidate and derived insight envelopes. The normalized fields are a
bounded title, summary, state, optional validity interval, evidence digest,
source watermark, uncertainty, provider/schema identity, policy version,
review state, and replay identity.

This keeps provider adapters type-neutral and makes replay identity stable. A
separate goal table or provider-specific payload would duplicate provenance and
create another lifecycle boundary. Metadata remains rebuildable from existing
PostgreSQL records and can be indexed later only if an observed query requires
it.

### 2. Treat goal state as type metadata, not a new canonical lifecycle

The allowed goal states are validated values carried by the derived goal
envelope. Derived insight lifecycle and audit transitions remain authoritative
for persistence. The default goal policy allows `proposed` and reviewable
transitions only; `active` is a declared state for policy compatibility but is
not admitted by the default implementation path.

This avoids creating a second state machine while still allowing a later policy
to define a controlled active transition. Provider output cannot set either the
derived lifecycle or goal state without the admission/review decision.

### 3. Reuse precedence and add goal checks at the type-specific stage

Every goal admission runs the existing precedence evaluator first: exact scope,
lifecycle visibility, principal grant, policy/approval, replay identity,
handoff, then derived mutation. Goal-specific checks run inside the policy and
handoff stages for evidence freshness, validity interval, allowed state,
uncertainty, and review. Earlier failures stop before later lookups and do not
disclose hidden records.

This preserves consistent denial ordering across intents, contradictions,
providers, and goals. A separate goal authorization path would make precedence
and redaction diverge.

### 4. Store goal data in existing derived insight provenance fields

Persist normalized goal metadata through the existing derived insight metadata,
provenance, evidence, and lifecycle history surfaces. Add a migration only if
the current PostgreSQL schema cannot preserve the bounded fields or if a
conformance query requires a durable index; do not add a new source-of-record
table. The migration must be append-only and reversible.

### 5. Make default visibility an explicit fail-closed filter

Goal candidates, quarantined records, and derived goals remain excluded from
ordinary retrieval and context assembly by the same lifecycle-safe defaults
used for reserved insights. Authorized diagnostics may return aggregate goal
state and disposition categories with redacted references. An explicit future
visibility policy must be required before any goal can enter ordinary context.

### 6. Use deterministic fixtures and an owned real-stack conformance path

Unit tests cover normalization, evidence and validity checks, replay identity,
policy/review decisions, rollback, and redaction. PostgreSQL tests verify
append-only persistence and isolation. A bounded product-verification fixture
uses an explicitly supplied PostgreSQL + pgvector DSN and exercises offline,
shadow, review-required, disabled-policy, replay, rollback, and default-
visibility behavior. Missing prerequisites produce a skipped/degraded result,
never a readiness claim.

## Risks / Trade-offs

- **[Goal metadata grows beyond bounded fields]** → enforce maximum title,
  summary, evidence, and interval sizes at the envelope boundary and emit only
  bucketed telemetry.
- **[A provider smuggles task instructions into a goal summary]** → treat all
  provider text as untrusted evidence-backed content, reject direct execution
  directives, and keep the execution/task systems out of this change.
- **[Goal records leak through existing insight retrieval]** → add explicit
  reserved-goal filtering tests at retrieval and context boundaries and keep
  the default policy disabled.
- **[Policy and review state diverge during retries]** → include policy,
  review, evidence watermark, and goal state in the replay/idempotency identity
  and preserve all transitions append-only.
- **[Existing metadata storage cannot support a required query]** → fail the
  conformance check closed and add a small reversible migration only after the
  storage requirement is demonstrated.
- **[Real-stack evidence is unavailable]** → record prerequisite skip/degraded
  status and keep goal processing at offline/shadow/review-only maximum.

## Migration Plan

1. Add the goal capability and policy defaults with all active admission and
   ordinary visibility disabled.
2. Deploy the validator, replay, review, persistence, filtering, telemetry,
   and conformance changes together so no partial path can expose goals.
3. Run unit, repository, OpenSpec, and owned PostgreSQL + pgvector conformance
   checks. Treat missing or stale prerequisites as non-pass evidence.
4. Roll back by disabling the goal policy and reverting any optional migration;
   preserve already recorded candidates, review outcomes, and audit history.

## Open Questions

None. The remaining choices, such as whether a later policy may admit `active`
goals or expose them to a dedicated context section, belong to a separate
change with its own evidence and rollout decision.
