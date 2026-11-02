## ADDED Requirements

### Requirement: Checksum-locked BFCL V4 memory inputs have bounded Provider conformance
The system SHALL accept a locally supplied, checksum-locked BFCL V4 memory
input bundle for `memory_kv`, `memory_rec_sum`, and `memory_vector`, normalize
the source prerequisite/snapshot state and supported calls into provider-contract
cases, and execute those cases without network access, model invocation, remote
tool execution, or an agent harness. The resulting outcome SHALL identify the
source subset, compatibility state, supported-operation coverage, and explicit
unsupported or incomplete operations; it SHALL NOT be represented as an
official BFCL score or leaderboard result.

#### Scenario: Run a compatible local memory subset
- **WHEN** a local BFCL V4 memory bundle has a valid source manifest, matching
  checksum, supported subset, compatible prerequisite/snapshot state, and
  supported operations
- **THEN** the runner produces deterministic Provider-contract outcomes with
  separate source compatibility and operation coverage evidence

#### Scenario: Source requires an unsupported operation
- **WHEN** a checksum-valid BFCL V4 memory record requires a source operation
  that Stele does not support for Provider-contract replay
- **THEN** the runner records that record as `unsupported` without silently
  substituting a different operation or counting it as a successful call

#### Scenario: Local official input is unavailable or incompatible
- **WHEN** the declared source artifact is absent, checksum-mismatched, or
  incompatible with the supported source format or prerequisite state
- **THEN** the runner returns a stable `prerequisite_missing`,
  `checksum_mismatch`, `invalid_manifest`, or `incomplete` outcome and performs
  no network fallback

### Requirement: BFCL source normalization preserves Provider safety boundaries
The BFCL V4 source adapter SHALL preserve source operation order, declared
prerequisite/snapshot identity, project/tenant/namespace/session expectations,
and lifecycle/refusal expectations when it produces local Provider-contract
cases. A source field, tool declaration, or snapshot entry SHALL NOT widen a
scope, override a lifecycle restriction, alter a disclosure policy, or grant
access to a foreign memory path.

#### Scenario: Source snapshot includes a foreign-scope record
- **WHEN** a BFCL source snapshot or operation attempts to read or mutate a
  record outside the declared case scope
- **THEN** the conformance result records the denied scope-safety outcome and
  does not expose the foreign record or treat the operation as supported

#### Scenario: Source case targets a hidden lifecycle state
- **WHEN** a normalized BFCL source case targets suppressed, forgotten, or
  deleted memory without an authorized debug/admin contract
- **THEN** the runner records the required refusal and preserves the existing
  default lifecycle exclusion behavior
