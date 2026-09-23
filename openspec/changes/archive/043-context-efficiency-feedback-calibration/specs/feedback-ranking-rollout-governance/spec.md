## MODIFIED Requirements

### Requirement: Ranking rollout requires dry-run impact evidence

The service MUST support dry-run reports that compare baseline ranking with
feedback and task-success-aware ranking before activation. A context-calibration
dry-run MUST additionally report bounded token efficiency, duplicate/stale
rates, protected evidence coverage, budget outcomes, deterministic replay
identity, and rollback readiness for the same exact scope and surfaces.

#### Scenario: Administrator runs ranking dry-run

- **WHEN** an authorized administrator runs a dry-run for a rollout policy over
  bounded queries or fixture requests
- **THEN** the service records baseline ranking, adjusted ranking, changed
  subjects, bounded reason codes, evidence counts, and lifecycle-safe impact
  summaries

#### Scenario: Context calibration dry-run lacks efficiency evidence

- **WHEN** a calibration dry-run lacks compatible token, freshness,
  duplication, protected-recall, or rollback evidence
- **THEN** the service reports insufficient evidence and prevents activation
  unless the policy explicitly remains diagnostics-only

#### Scenario: Dry-run lacks evidence threshold

- **WHEN** a policy dry-run has insufficient active feedback, task evaluation,
  verification, quality, or context-efficiency evidence for the configured
  threshold
- **THEN** the service reports insufficient evidence and prevents activation
  unless the policy explicitly remains diagnostics-only

#### Scenario: Dry-run references hidden evidence

- **WHEN** hidden, suppressed, forgotten, deleted, or out-of-scope evidence
  contributes to a dry-run decision
- **THEN** the dry-run report exposes only aggregate lifecycle-safe diagnostics
  and does not surface hidden content

### Requirement: Active rollout changes ranking without changing visibility

The service SHALL apply an active scoped rollout policy as a bounded ranking hint
layer for configured search or context surfaces while preserving lifecycle and
scope isolation as primary visibility controls. Context calibration MUST preserve
the approved context budget, section contract, protected evidence groups,
citations, and deterministic tie-breaking.

#### Scenario: Active policy applies to context assembly

- **WHEN** a caller assembles context in a scope with an active matching rollout
  policy and compatible calibration evidence
- **THEN** the service can adjust candidate priority within budget and section
  rules while preserving citations, lifecycle visibility, and context safety

#### Scenario: Active policy applies to search

- **WHEN** a caller searches memory in a scope with an active matching rollout
  policy
- **THEN** the service can adjust ranking using approved bounded signals while
  excluding hidden or out-of-scope memory from results

#### Scenario: Active policy references provider credentials

- **WHEN** a rollout policy is created or activated
- **THEN** the durable policy stores only logical provider, model, strategy, and
  calibration identities, never endpoint URLs, API keys, DSNs, or raw provider
  payloads

#### Scenario: Active policy has stale or superseded signals

- **WHEN** the active policy's signal summary contains expired, superseded, or
  below-threshold feedback
- **THEN** those signals are excluded and the request uses the remaining bounded
  signals or baseline ordering

#### Scenario: No matching active policy exists

- **WHEN** a caller searches or assembles context without an active matching
  policy
- **THEN** the service preserves baseline ranking and packing behavior

#### Scenario: Request asks for diagnostics while policy is active

- **WHEN** an authorized request asks for ranking diagnostics or dry-run
  comparison while an active policy applies to the resolved scope
- **THEN** the service may include bounded policy impact diagnostics while
  ordinary result ordering follows the active policy unless the request is
  explicitly diagnostics-only

### Requirement: Ranking signals are rebuildable and bounded

The service SHALL derive ranking signals from durable active feedback,
task-evaluations, session verification, quality findings, and versioned
quality-feature summaries using bounded categories and rebuildable summaries.
Context-efficiency calibration MUST record its decay window, cap, minimum
evidence threshold, source versions, and deterministic summary identity.

#### Scenario: Signal summary is rebuilt

- **WHEN** ranking signal aggregation is rerun or repaired
- **THEN** the service recomputes summaries from durable source evidence and
  records the feature, decay, cap, and policy versions used

#### Scenario: Single negative event exists

- **WHEN** a single low-confidence negative event exists below the policy
  threshold
- **THEN** the service records diagnostics but does not apply that signal as a
  default ranking adjustment

#### Scenario: Quality provider fails during aggregation

- **WHEN** an optional provider is unavailable while signals are being evaluated
- **THEN** the service marks the provider signal unavailable and retains the
  deterministic baseline

#### Scenario: Superseded feedback exists

- **WHEN** feedback or task evidence has been superseded or corrected
- **THEN** active rollout signals exclude superseded evidence by default while
  preserving history for authorized inspection
