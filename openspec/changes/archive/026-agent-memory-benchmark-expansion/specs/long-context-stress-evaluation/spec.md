## ADDED Requirements

### Requirement: Stress datasets use explicit resource budgets
The system SHALL support controlled subsets of Needle-in-a-Haystack, OpenAI MRCR, LongBench-v2, and VTCBench with explicit context-length, sample-count, timeout, disk, and memory budgets.

#### Scenario: Skip an over-budget stress run
- **WHEN** the selected subset exceeds the configured local budget
- **THEN** the runner returns a stable capacity prerequisite status before importing all data or exhausting the host

### Requirement: Text and multimodal capability are declared
The system SHALL identify whether a VTCBench run uses text or visual input and SHALL refuse visual mode when the required local image capability is unavailable.

#### Scenario: Run VTCBench text mode
- **WHEN** a user selects the text mode and the text artifact checksum is valid
- **THEN** the runner executes the text subset and records the mode in the report

#### Scenario: Refuse unavailable visual mode
- **WHEN** a user selects visual mode without the required local image artifacts or capability
- **THEN** the runner returns prerequisite status and does not silently fall back to text mode

### Requirement: Stress results describe degradation, not memory quality
The system SHALL report context-length buckets, needle count or depth, latency, capacity failures, and retrieval/answer metrics under a `stress` family identity and SHALL NOT use them as the primary memory product gate.

#### Scenario: Produce a stress report
- **WHEN** a stress subset completes across multiple context buckets
- **THEN** the report includes per-bucket outcomes, run budget, input checksums, capability mode, and an explicit non-gating classification
