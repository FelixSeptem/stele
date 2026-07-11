## ADDED Requirements

### Requirement: Derived insight replay obeys insight governance
The service MUST apply the same derived insight governance rules during replay as during scheduled derivation, including evidence thresholds, lifecycle transitions, confidence updates, feedback policy, and audit history.

#### Scenario: Replay finds repeated failure evidence
- **WHEN** replay evaluates historical evidence that satisfies the governed `failure_pattern` threshold within one authorized scope
- **THEN** the service can create or update the corresponding derived insight through the same evidence-backed governance model as scheduled derivation

#### Scenario: Replay finds insufficient evidence
- **WHEN** replay evaluates evidence that does not satisfy the governed threshold for an active insight
- **THEN** the service records a skipped, preserved, or suppressed decision according to policy rather than creating an unsupported active insight

#### Scenario: Replay consumes feedback state
- **WHEN** replay evaluates an insight with active quality feedback
- **THEN** the replay decision accounts for effective quality state without deleting feedback records or rewriting prior evidence history

### Requirement: Replay does not activate reserved insight vocabulary
The service SHALL NOT use replay to autonomously activate reserved `hypothesis`, `goal`, `contradiction`, or `causal_link` insight types.

#### Scenario: Replay request includes unsupported type
- **WHEN** a replay request includes an insight type that is reserved but not supported for active derivation
- **THEN** the service rejects or skips that type and records the unsupported-type reason in the replay response or report
