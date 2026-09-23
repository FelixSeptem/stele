## MODIFIED Requirements

### Requirement: Budget-aware context shaping

The service MUST support bounded context packing so the assembled response stays
within a caller-provided or service-default budget. Projection-backed and live
retrieval items MUST use the same deterministic budget accounting and MUST fail
closed when an item cannot fit. After normal eligibility, summary preference,
identity/lineage deduplication, and applicable diversity policy, an explicitly
enabled feedback-calibration policy MAY adjust ordering only within the same
budget and section envelope. Calibration MUST NOT increase the budget, change
section names, broaden scope, omit required citations, or bypass lifecycle
visibility.

#### Scenario: Context budget is constrained

- **WHEN** a client requests context with a limited budget and an active
  calibration policy is eligible for the exact scope
- **THEN** the assembler applies deterministic baseline packing plus bounded
  calibration ordering and returns only evidence that fits the original budget

#### Scenario: Projection item exceeds remaining budget

- **WHEN** a lifecycle-visible projection item cannot fit within the remaining
  character/token budget
- **THEN** the item is omitted with a bounded budget reason and the assembler
  does not increase the requested budget or fetch a broader scope

#### Scenario: Feedback signal would exceed the envelope

- **WHEN** a calibration signal would require additional candidates, tokens, or
  sections beyond the request envelope
- **THEN** the signal is ignored or the request falls back to baseline packing
  without changing visibility or budget

#### Scenario: Diversity selection sees incomplete coverage attributes

- **WHEN** a visible eligible candidate lacks a source session, entity, or time
  slice attribute used by the active diversity policy
- **THEN** the assembler uses a bounded unknown category without loading broader
  source data, changing lifecycle visibility, widening the resolved scope, or
  treating absent feedback as a positive calibration signal

### Requirement: Feedback-aware ranking is explicit

The service MUST NOT silently change default context ranking based on usefulness
feedback or task-success evidence unless feedback-aware ranking is explicitly
requested for one context request or an authorized scoped rollout policy is
active. Any enabled feedback calibration MUST use bounded, decayed,
non-superseded aggregate signals and MUST preserve the deterministic baseline
tie-breaker and protected evidence groups.

#### Scenario: Default context assembly runs without active policy

- **WHEN** a caller assembles context without an explicit feedback-aware ranking
  request and no active matching policy exists
- **THEN** feedback and task-success signals may appear in authorized
  diagnostics but do not alter default context ranking

#### Scenario: Feedback-aware ranking is requested

- **WHEN** a caller explicitly enables feedback-aware ranking for one context
  request
- **THEN** the service may use bounded calibrated summaries as ranking hints
  while preserving lifecycle, scope, budget, section, and citation safety

#### Scenario: Calibration evidence is insufficient

- **WHEN** active signal counts or confidence do not meet the configured
  threshold
- **THEN** the request retains baseline ordering and records an insufficient-
  evidence category only on authorized diagnostics

#### Scenario: Governed scope policy is active

- **WHEN** an authorized ranking rollout policy is active for the context
  surface and resolved scope
- **THEN** context assembly may use compatible bounded calibration summaries
  while preserving lifecycle, budget, section, citation, and protected-evidence
  safety rules

#### Scenario: Scope-wide ranking policy is requested without governance

- **WHEN** an operator attempts to enable feedback-aware context ranking as a
  default scope-wide setting outside the ranking rollout governance contract
- **THEN** the service rejects or ignores that setting and preserves baseline
  context ranking

#### Scenario: Ranking policy references hidden evidence

- **WHEN** active ranking signals reference suppressed, forgotten, expired,
  deleted, or out-of-scope memory
- **THEN** context assembly excludes hidden memory from ordinary sections and
  exposes only lifecycle-safe aggregate diagnostics where authorized

### Requirement: Context assembly exposes feedback-aware diagnostics

The service SHALL expose bounded usefulness and efficiency diagnostics only for
an authorized scope. Diagnostics MAY include reason categories, aggregate
candidate/token buckets, freshness/duplication buckets, policy identity, and
quality-per-budget buckets, but MUST NOT expose feedback history, raw scores,
query text, content, scope values, or hidden identifiers.

#### Scenario: Context item has feedback history

- **WHEN** an assembled context item has useful, noisy, stale, irrelevant,
  unsafe, or needs-review feedback history
- **THEN** authorized diagnostics can include bounded reason categories and
  aggregate efficiency impact without exposing the feedback record

#### Scenario: Ordinary caller requests diagnostics

- **WHEN** an ordinary public search or context caller requests calibration or
  efficiency internals
- **THEN** the service preserves the public response contract and returns no
  policy, candidate-pool, metric, or trajectory internals

#### Scenario: Feedback references hidden memory

- **WHEN** feedback history references suppressed, forgotten, expired, deleted,
  or out-of-scope memory
- **THEN** context assembly does not expose the hidden content and only surfaces
  lifecycle-safe aggregate diagnostics when authorized
