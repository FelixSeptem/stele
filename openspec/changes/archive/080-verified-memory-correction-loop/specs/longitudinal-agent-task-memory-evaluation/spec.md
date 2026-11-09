## ADDED Requirements

### Requirement: Longitudinal reports separately measure repeated-error and maintenance observations
The task-series evaluator SHALL accept bounded, externally labeled repeated-error
and maintenance-cost observations for each declared condition/stage and report
their aggregate rates separately from task utility, stale/noisy harm, provenance
coverage, and safety. Missing labels MUST remain absent or inconclusive. The
report MUST describe cross-condition differences as observational and MUST NOT
infer an error, correction effect, token cost, tool cost, or causal outcome.

#### Scenario: Corrected condition has later external outcomes
- **WHEN** a valid task-series fixture supplies externally judged later-stage
  repeated-error labels and maintenance observations for baseline and corrected
  conditions
- **THEN** the deterministic aggregate report includes separate condition-level
  repeated-error and maintenance metrics without converting them into an
  activation or causal claim

#### Scenario: Repeated-error input is incomplete
- **WHEN** a task-series fixture omits a required repeated-error or maintenance
  label for an evaluated stage
- **THEN** the evaluator records incomplete coverage for that metric and does
  not synthesize a zero value or infer a task outcome
