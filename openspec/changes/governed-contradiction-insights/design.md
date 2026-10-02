## Context

The proposal builds on the existing canonical version model, bi-temporal
validity rules, governed experience insights, reserved-insight activation,
durable replay, and reasoning candidate envelope. See `proposal.md` for the
motivation and externally visible behavior. The implementation must keep
PostgreSQL as the system of record, preserve append-only history, and enforce
exact tenant/project/namespace isolation.

## Goals / Non-Goals

**Goals:**

- Make contradiction semantics deterministic enough to evaluate without
  trusting a provider's unsupported claim.
- Separate true temporal conflict from facts that are both correct in
  different validity intervals.
- Reuse the current candidate, activation, replay, context, and telemetry
  boundaries instead of creating a second reasoning pipeline.
- Keep the first release offline/shadow by default and make any activation
  independently reviewable and reversible.

**Non-Goals:**

- No general natural-language consistency engine or unconstrained pairwise
  comparison.
- No automatic correction, merge, deletion, or rewrite of either source fact.
- No activation of other reserved insight types.

## Decisions

### 1. Use a deterministic contradiction key before optional provider reasoning

Candidate selection starts from a normalized fact identity containing the
subject/entity, attribute or predicate, scope, and mutually-exclusive policy
class. Only fact versions sharing that key enter the bounded pair or group
comparison. This prevents an embedding similarity score from turning unrelated
facts into contradictions. A provider may help classify a bounded pair when
the policy permits it, but the provider cannot create the pair, choose its
scope, or override evidence and temporal validation.

Alternatives considered:

- Compare every fact pair with a provider: rejected because pair volume,
  leakage risk, and unsupported similarity would be unbounded.
- Use only lexical equality: rejected because normalized entities and values
  can have different surface forms, while still requiring a stable identity
  before semantic classification.

### 2. Treat valid-time overlap as a required conflict signal

The temporal evaluator compares half-open validity intervals from the source
versions. Disjoint intervals produce `temporal_coexistence`; invalid or missing
intervals produce `unresolved_temporal`; overlapping intervals are eligible for
contradiction classification. An explicit policy may permit an operator to
review an unknown interval, but unknown time never becomes an automatic active
insight.

This reuses the bi-temporal fact contract and avoids labeling a historical
change as an error. Recorded time remains provenance metadata and is not used
as a substitute for fact-valid time.

### 3. Persist a pair/group candidate, not a replacement fact

The candidate stores the contradiction key, sorted source-version references,
validity relationship, evidence digest, uncertainty, source watermark,
provider/schema identity, policy version, review state, and replay identity.
Activation creates a derived insight version that points to those sources. A
source correction or lifecycle transition marks the derived candidate stale or
queues bounded rebuild work; it never mutates either source fact.

### 4. Make review a policy field and keep default behavior shadow-only

Each contradiction policy declares whether operator review is required,
minimum evidence and uncertainty bounds, freshness, and rollback state. The
default policy state is disabled, and offline/shadow results remain
non-authoritative. Apply requests enter durable work and call the existing
reserved activation path, which owns idempotency, audit, and lifecycle
transitions.

This keeps the candidate useful for measurement while preventing a diagnostic
run from becoming an implicit release mechanism.

### 5. Expose only aggregate contradiction diagnostics

Telemetry and admin diagnostics use fixed categories for pair selection,
temporal disposition, evidence eligibility, review, activation, rollback, and
freshness. They never include source text, claim values, scope values, raw
scores, provider payloads, or identifiers. An authorized context section may
return cited active insights, but ordinary retrieval remains unchanged.

## Risks / Trade-offs

- **False positives from weak normalization** -> Require a stable contradiction
  key and explicit mutually-exclusive policy class before any provider call;
  keep uncertain pairs review-only.
- **Historical corrections create stale derived records** -> Persist source
  watermarks and temporal identities, mark stale candidates explicitly, and
  rebuild from PostgreSQL rather than refreshing evidence silently.
- **Pair explosion in dense subjects** -> Enforce per-key, per-run, evidence,
  provider, and output budgets with deterministic truncation categories.
- **Review state leaks into ordinary context** -> Keep candidates and
  non-authoritative records outside default context and require an explicit
  section policy for reviewed active insights.
- **Feedback suppresses useful conflict evidence** -> Preserve both feedback
  and prior evidence history; make feedback-driven lifecycle changes append-only
  and policy governed.

## Migration Plan

1. Add contradiction key, temporal comparison, candidate, replay, and review
   contracts without enabling any activation policy.
2. Run offline and shadow fixtures covering overlap, disjoint intervals,
   missing time, hidden/foreign evidence, pair budgets, deterministic replay,
   and provider refusal.
3. Run owned PostgreSQL + pgvector integration checks for append-only
   persistence, exact scope, stale source handling, durable apply recovery,
   default context exclusion, and redacted diagnostics.
4. Enable an exact-scope contradiction policy only after the evidence and
   replay release gates pass. Rollback disables new admissions and preserves
   candidate, source, review, and audit history.

