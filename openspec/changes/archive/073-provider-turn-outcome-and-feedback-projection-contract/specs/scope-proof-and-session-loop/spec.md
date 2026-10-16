## ADDED Requirements

### Requirement: Session outcome projection preserves revision and correction evidence

The service SHALL allow scoped session/turn outcome evidence to be projected
through the dedicated Provider contract with bounded effect identity and source
revision while preserving existing session history. A correction MUST retain the
prior session/outcome evidence and SHALL NOT change external execution semantics.

#### Scenario: Corrected outcome follows an earlier session turn
- **WHEN** a later source revision corrects an outcome associated with a scoped turn
- **THEN** the session evidence exposes an auditable correction relationship and
  retains the earlier outcome without marking an external task or lease changed

