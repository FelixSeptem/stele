## Context

See proposal.md. The existing materializer restricts always-visible content to
profiles with confidence and size gates. PostgreSQL stores immutable source
versions and projection history. EV3 provides shared bounded comparison metrics
and source/current-valid-time checks, while task outcomes remain externally
judged. This change evaluates a candidate; it does not activate placement.

## Goals / Non-Goals

Produce reviewable promotion/retention/demotion plans and observational
baseline/candidate comparisons. Preserve class policy, independent budgets,
lineage and production default behavior. Do not expand verification APIs to
profiles or treat a reported task success as claim verification.

## Decisions

### Offline selection over existing projections

An internal evaluator accepts exact-scope canonical evidence and immutable
version references, the current resident IDs, a fixed clock, a policy identity,
and externally supplied support/utility observations. It runs the existing
profile materializer for both the confidence/size baseline and the filtered
candidate. On-demand classes retain ordinary source-search access. Selection
IDs stay private to the evaluation result; only aggregate reports serialize.
Direct live strategy changes and a new background promotion worker were
rejected because EV4 first requires utility and rollback evidence.

### Independent support and held-out utility

Support carries a target ID/version and SHA-256 claim fingerprint, evidence
kind, attributable raw-event record, actor, observed-at and expiry. Only tool
observations, explicit user corrections or external evaluation support can
qualify; transcript/model assertions never do. Explicit stable/applicable
observations accompany support. Utility is separately supplied, tied to the
same source version and held-out task stage after support, and must be active,
fresh and positive for promotion. Negative/stale observations cause proposed
demotion; missing ones yield on-demand placement. No success is inferred.
All support, utility and comparison records require disclosable `attested_data`
source metadata from the existing exact-scope attestation contract. The actor
field alone cannot establish attribution, and attribution never proves truth.
Exact JSON field whitelists reject case aliases, duplicate keys and nulls.
Only compatible validated support/utility for evaluated eligible sources
advances the stage that subsequent condition observations must exceed.

Reading frequency has no influence on eligibility or ranking. Candidate order
uses the materializer's deterministic order and exact references, with stable
tie-breaking rather than popularity. Evidence bounds are validated before
selection and fingerprints/source versions are checked again against the
snapshot used to materialize the candidate.

### Separate independent budgets and aggregate boundary

Both conditions use equal positive limits (default 8 items, 4096 UTF-8 content
bytes, 1024 whitespace tokens and 16 citations), intersected with existing
projection per-item constraints. These are evaluator limits, not new Provider
request semantics. Omissions stay bounded and on-demand; source facts are not
deleted or summarized away. Reports contain fixed enums, aggregate counts,
source/policy/output digests and externally supplied condition utility/error/
maintenance metrics. Raw source/evidence/task text, IDs and scopes are absent.
All report fields bind to a deterministic run hash; zero/missing utility remain
distinct. Disablement/rollback returns the baseline plan with no store writes.

### Verification with existing storage

Use unit fixtures for popularity, stale/corrected support, source changes,
future/expired validity, hidden/foreign/disclosure evidence, equal budgets,
Unicode bytes, determinism, redaction and incomplete external observations.
An owned temporary PostgreSQL/pgvector harness persists canonical versions and
baseline/candidate projections, verifies selected source references and
history across restart, and proves rollback matches baseline. The evaluator
does not need its own persistent report table: the report is a redacted
verification artifact, while sources and projection history remain PostgreSQL
records. This avoids another store and an unnecessary schema migration.

## Risks / Trade-offs

- External support is an observation, not service-discovered truth → require
  attribution, matching claims/versions and explicit expiration; no activation.
- Strict evidence reduces resident coverage → preserve on-demand search and
  measure utility separately instead of assuming every promotion is beneficial.
- Synthetic corpus overfitting → label conformance explicitly; require external
  held-out task observations before claims about task benefit.

## Migration Plan

No runtime configuration or public migration. Land the internal evaluator and
docs, verify baseline equivalence and owned persistence, archive and publish.
Rollback means not invoking the evaluator; a future live policy requires a
separate proposal and public contract review.
