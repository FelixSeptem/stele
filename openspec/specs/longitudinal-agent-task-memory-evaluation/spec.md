# longitudinal-agent-task-memory-evaluation Specification

## Purpose
Evaluate externally judged, repeated task sequences against explicit memory conditions so Stele can measure longitudinal usefulness and harm without executing agents or claiming authority over task outcomes.

## Requirements

### Requirement: Task-series inputs are reproducible and externally judged
The evaluation capability SHALL accept a bounded task-series manifest with an ordered set of task stages, declared memory conditions, exact evaluation scope, source and qrel checksums, and an externally supplied verdict for each evaluated stage and condition. Missing verdicts SHALL remain missing or inconclusive and MUST NOT be inferred by Stele.

#### Scenario: Complete task series is loaded
- **WHEN** a manifest has unique ordered stages, complete declared conditions, valid exact scope, compatible identities, and supported external verdicts
- **THEN** the evaluator accepts it and preserves stage order for deterministic evaluation

#### Scenario: Manifest identity is incomplete or conflicting
- **WHEN** a manifest omits a required checksum, repeats a stage/condition identity, changes scope across evidence, or supplies an unsupported verdict
- **THEN** the evaluator rejects it with a bounded validation result and emits no quality pass

#### Scenario: A task outcome is not externally judged
- **WHEN** a stage/condition has no externally supplied supported verdict
- **THEN** the stage is recorded as missing or inconclusive and is excluded from success-rate denominators without Stele inferring an outcome

### Requirement: Longitudinal metrics distinguish utility, harm, and evidence integrity
The evaluator SHALL report per-condition task-success utility and sequence coverage separately from stale/noisy-memory harm, source-provenance coverage, and scope/lifecycle safety. Task utility SHALL use a versioned, deterministic mapping of externally supplied verdict categories; stale/noisy harm SHALL count only evidence explicitly labeled stale/noisy by the input contract; provenance coverage SHALL use only verifiable evidence lineage. No metric may imply that memory caused an outcome.

#### Scenario: Repeated task sequence is evaluated
- **WHEN** valid stages are evaluated for each declared memory condition
- **THEN** the report includes deterministic condition-level utility, evaluated-stage coverage, stale/noisy harm, verifiable provenance coverage, and safety results over the ordered sequence

#### Scenario: Memory improves task utility while increasing harm
- **WHEN** a condition has higher externally reported utility but also has stale/noisy evidence or integrity failures
- **THEN** the report preserves the separate metrics and does not collapse them into an unqualified quality pass

#### Scenario: Outcome comparison is described observationally
- **WHEN** results from two or more memory conditions are compared
- **THEN** the report labels differences as observed condition differences and MUST NOT assert causal effect or attribute success/failure to memory

### Requirement: Longitudinal safety is a hard gate
Every task stage, retrieval observation, and evidence mapping SHALL remain within one exact tenant/project/namespace evaluation scope and satisfy lifecycle visibility. Any foreign, hidden, suppressed, forgotten, deleted, or otherwise ineligible evidence SHALL count as a safety failure and MUST fail the overall evaluation regardless of task utility.

#### Scenario: Evidence crosses scope or lifecycle boundaries
- **WHEN** a task-stage mapping or retrieved result is foreign or not lifecycle-visible
- **THEN** the report fails the safety gate without disclosing the record identity, content, or existence details

#### Scenario: Quality is high but safety fails
- **WHEN** task utility or retrieval coverage passes its threshold while any scope/lifecycle violation exists
- **THEN** the overall evaluation is non-passing and no rollout or activation eligibility is emitted

### Requirement: Reports are bounded, redacted, and replayable
Reports SHALL include the manifest/schema/metric versions, source and qrel checksums, normalized corpus identity, retrieval strategy identity, PostgreSQL/pgvector runtime identity for real-stack runs, evaluated-stage and condition counts, bounded aggregate metrics, and separate quality and safety verdicts. Serialized reports MUST NOT include task objectives, answer text, raw queries, scope values, memory/event identifiers, hidden candidates, scores, prompts, provider payloads, or credentials. Identical compatible inputs SHALL yield stable metric ordering and values.

#### Scenario: Identical run is replayed
- **WHEN** identical manifests, source records, strategy configuration, and metric versions are evaluated
- **THEN** aggregate metric values and serialized ordering are stable across replay

#### Scenario: Report is exported
- **WHEN** a report is serialized for local evaluation output
- **THEN** it includes enough bounded provenance to reproduce and audit the run while excluding sensitive or unbounded per-task and per-memory details

### Requirement: Evaluation does not execute agents or alter production behavior
The evaluation capability MUST NOT execute an external agent, invoke a model, construct prompts, generate answers, infer task success, mutate canonical memory, change default retrieval behavior, or authorize rollout. It MAY consume externally supplied task verdicts and run bounded read-only retrieval observations against an isolated owned scope.

#### Scenario: Evaluation consumes supplied outcomes
- **WHEN** a local or PostgreSQL-backed evaluation run is executed
- **THEN** it uses only the declared fixture, supplied external verdicts, and read-only retrieval observations and leaves production memory and retrieval policy unchanged

#### Scenario: Evaluation data is incomplete
- **WHEN** a required external outcome, source watermark, or compatibility identity is unavailable
- **THEN** the run is incomplete or non-passing and does not synthesize missing evidence

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
